package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluerequests/api/proto/bff"
	"github.com/bluefunda/bluerequests/internal/a4h"
	"github.com/bluefunda/bluerequests/internal/config"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
	"github.com/bluefunda/bluerequests/internal/tui"
	"github.com/bluefunda/bluerequests/internal/ui"
)

// ─── a4h ─────────────────────────────────────────────────────────────────────

var a4hCmd = &cobra.Command{
	Use:   "a4h",
	Short: "On-demand SAP ABAP trial (A4H) session commands",
	Long: `Start, inspect, extend and end your own temporary SAP ABAP trial system.

A session takes about four minutes to become ready, ends after three hours
(extend it one hour at a time) or after an hour of inactivity, and the system
is deleted when it ends.`,
}

// a4hCall is one session RPC, run with the given context.
type a4hCall func(ctx context.Context) (*pb.A4HSessionResponse, error)

// callA4H runs a session RPC, signing in again once if the token was rejected,
// and returns the session.
func callA4H(cfg *config.Config, p *ui.Printer, call a4hCall) (*pb.A4HSession, error) {
	ctx, cancel := trmgrpc.ContextWithTimeout()
	resp, err := call(ctx)
	cancel()
	if err != nil && trmgrpc.IsAuthError(err) {
		if reErr := reAuthenticate(cfg, p); reErr != nil {
			return nil, reErr
		}
		ctx, cancel = trmgrpc.ContextWithTimeout()
		resp, err = call(ctx)
		cancel()
	}
	if err != nil {
		return nil, a4hError(err)
	}
	return resp.GetSession(), nil
}

// a4hError turns the status codes trm-bff uses for sessions into plain text.
func a4hError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return errors.New("no running session")
	case codes.Unavailable:
		return fmt.Errorf("the A4H session service is not responding; try again in a moment (%s)", status.Convert(err).Message())
	default:
		return err
	}
}

// sessionRows presents a session as FIELD/VALUE rows.
func sessionRows(s *pb.A4HSession, now time.Time) [][]string {
	rows := [][]string{{"Status", a4h.Label(s.GetState())}}
	if s.GetInstanceName() != "" {
		rows = append(rows, []string{"System", s.GetInstanceName()})
	}
	if s.GetPublicIp() != "" && a4h.IsActive(s.GetState()) {
		c := a4h.ConnectionFor(s.GetPublicIp())
		rows = append(rows,
			[]string{"Host", c.Host},
			[]string{"SAP GUI", c.SAPGUI},
			[]string{"ADT (Eclipse)", c.ADTHTTP + "  " + c.ADTTLS},
		)
		if s.GetState() != a4h.StateSAPRunning {
			rows = append(rows, []string{"Note", "SAP is still starting; connect once the status is Ready"})
		}
	}
	if s.GetState() == a4h.StateSAPRunning && s.GetDeadline() != "" {
		rows = append(rows,
			[]string{"Time remaining", a4h.Remaining(s.GetDeadline(), now)},
			[]string{"Ends at", s.GetDeadline()},
		)
	}
	return rows
}

// printSession writes the session to stdout in the selected output format.
func printSession(p *ui.Printer, s *pb.A4HSession) {
	switch p.Format {
	case ui.FormatJSON:
		p.ProtoJSON(s)
	case ui.FormatQuiet:
		_, _ = fmt.Fprintln(p.Out, s.GetState())
	default:
		p.Table([]string{"FIELD", "VALUE"}, sessionRows(s, time.Now()))
	}
}

// ─── a4h status ──────────────────────────────────────────────────────────────

var a4hStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show your session (state \"none\" if you have not started one)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		p := printer(cfg)

		s, err := callA4H(cfg, p, func(ctx context.Context) (*pb.A4HSessionResponse, error) {
			return conn.Client.GetA4HSession(ctx, &pb.GetA4HSessionRequest{})
		})
		if err != nil {
			return err
		}
		printSession(p, s)
		return nil
	},
}

// ─── a4h start ───────────────────────────────────────────────────────────────

var (
	a4hStartWait    bool
	a4hStartTimeout time.Duration
)

var a4hStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a session, or show the one already running",
	Long: `Start your SAP ABAP trial system. If you already have a session it is returned
unchanged. Startup takes about four minutes; use --wait to follow it until SAP
is ready, or 'breq a4h watch' for a live screen.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		p := printer(cfg)

		s, err := callA4H(cfg, p, func(ctx context.Context) (*pb.A4HSessionResponse, error) {
			return conn.Client.StartA4HSession(ctx, &pb.StartA4HSessionRequest{})
		})
		if err != nil {
			return err
		}

		if a4hStartWait && a4h.IsStarting(s.GetState()) {
			w := &a4hWaiter{
				get: func() (*pb.A4HSession, error) {
					return callA4H(cfg, p, func(ctx context.Context) (*pb.A4HSessionResponse, error) {
						return conn.Client.GetA4HSession(ctx, &pb.GetA4HSessionRequest{})
					})
				},
				sleep: time.Sleep,
				now:   time.Now,
				p:     p,
			}
			if s, err = w.untilReady(s, a4hStartTimeout); err != nil {
				return err
			}
		}

		printSession(p, s)
		if !a4hStartWait && a4h.IsStarting(s.GetState()) {
			p.Info("Starting. Follow it with 'breq a4h watch' or check 'breq a4h status'.")
		}
		return nil
	},
}

// a4hWaiter follows a starting session until SAP answers.
type a4hWaiter struct {
	get   func() (*pb.A4HSession, error)
	sleep func(time.Duration)
	now   func() time.Time
	p     *ui.Printer
}

// untilReady polls until the session is ready, reporting each state change on
// stderr. It fails if the session ends instead, or after timeout.
func (w *a4hWaiter) untilReady(s *pb.A4HSession, timeout time.Duration) (*pb.A4HSession, error) {
	deadline := w.now().Add(timeout)
	last := ""
	for {
		if state := s.GetState(); state != last {
			w.p.Info(a4h.Label(state))
			last = state
		}
		switch {
		case s.GetState() == a4h.StateSAPRunning:
			return s, nil
		case !a4h.IsStarting(s.GetState()):
			return s, fmt.Errorf("the session ended before SAP was ready (state: %s)", s.GetState())
		}
		if !w.now().Before(deadline) {
			return s, fmt.Errorf("SAP was not ready after %s; check 'breq a4h status'", timeout)
		}
		w.sleep(a4h.PollInterval(s.GetState()))

		next, err := w.get()
		if err != nil {
			return s, err
		}
		s = next
	}
}

// ─── a4h extend ──────────────────────────────────────────────────────────────

var a4hExtendCmd = &cobra.Command{
	Use:   "extend",
	Short: "Extend the running session by one hour",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		p := printer(cfg)

		s, err := callA4H(cfg, p, func(ctx context.Context) (*pb.A4HSessionResponse, error) {
			return conn.Client.ExtendA4HSession(ctx, &pb.ExtendA4HSessionRequest{})
		})
		if err != nil {
			return err
		}
		p.Success("Session extended by one hour.")
		printSession(p, s)
		return nil
	},
}

// ─── a4h terminate ───────────────────────────────────────────────────────────

var a4hTerminateYes bool

var a4hTerminateCmd = &cobra.Command{
	Use:     "terminate",
	Aliases: []string{"end", "stop"},
	Short:   "End your session and delete the system",
	Long: `End your session. The system and anything saved on it is deleted; this cannot
be undone. Asks for confirmation unless --yes is given (required when stdin is
not a terminal).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !a4hTerminateYes {
			if !stdinIsTerminal() {
				return errors.New("refusing to end the session without confirmation; pass --yes")
			}
			ok, err := confirm(os.Stdin, os.Stderr, "End your SAP trial system? It will be deleted. [y/N]: ")
			if errors.Is(err, errNoAnswer) {
				return errors.New("no confirmation given; pass --yes to end the session without asking")
			}
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("cancelled")
			}
		}

		conn, cfg, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		p := printer(cfg)

		s, err := callA4H(cfg, p, func(ctx context.Context) (*pb.A4HSessionResponse, error) {
			return conn.Client.TerminateA4HSession(ctx, &pb.TerminateA4HSessionRequest{})
		})
		if err != nil {
			return err
		}
		p.Success("Session ending; the system is being deleted.")
		printSession(p, s)
		return nil
	},
}

func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// errNoAnswer means the input ended before any answer was given (closed
// stdin, /dev/null, an empty pipe).
var errNoAnswer = errors.New("no answer given")

// confirm asks a yes/no question and returns true only for "y" or "yes".
func confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	_, _ = fmt.Fprint(out, prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		if errors.Is(err, io.EOF) {
			return false, errNoAnswer
		}
		return false, fmt.Errorf("read answer: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// ─── a4h watch ───────────────────────────────────────────────────────────────

var a4hWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Live screen: progress, connection details, start / extend / end",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, _, err := bffConn()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		return tui.RunA4H(conn)
	},
}

func init() {
	a4hStartCmd.Flags().BoolVar(&a4hStartWait, "wait", false, "wait until SAP is ready, reporting progress on stderr")
	a4hStartCmd.Flags().DurationVar(&a4hStartTimeout, "timeout", 15*time.Minute, "how long --wait waits before giving up")
	a4hTerminateCmd.Flags().BoolVarP(&a4hTerminateYes, "yes", "y", false, "do not ask for confirmation")

	a4hCmd.AddCommand(a4hStatusCmd, a4hStartCmd, a4hExtendCmd, a4hTerminateCmd, a4hWatchCmd)
}
