package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bluefunda/bluefunda-ai/internal/daemon"
	"github.com/bluefunda/bluefunda-ai/internal/schedule"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// tickInterval is how often the daemon checks for due schedules. Cron
// granularity is one minute, so anything well under that is sufficient.
const tickInterval = 30 * time.Second

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run or control the schedule daemon (Phase 1 of #177 — foreground process, no OS service integration)",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the schedule daemon in the foreground",
	Long: "Polls ~/.bai/schedules.yaml and fires any due scheduled runs.\n" +
		"Runs in the foreground until interrupted (Ctrl+C) or stopped from another\n" +
		"terminal with `bai daemon stop`. To keep it running after the terminal\n" +
		"closes, use your OS's own tools: nohup, tmux/screen, a systemd unit, or a\n" +
		"launchd agent.",
	RunE: runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running daemon",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether the daemon is running",
	RunE:  runDaemonStatus,
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
}

func daemonPaths() (pidFile, logDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".bai", "daemon.pid"), filepath.Join(home, ".bai", "daemon", "logs"), nil
}

// readDaemonPID returns the PID in pidFile if it's a live process, or 0 if
// the file is missing, unreadable, or the process is gone.
func readDaemonPID(pidFile string) int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return 0
	}
	// On unix, FindProcess always succeeds; signal 0 checks liveness
	// without actually sending a signal.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return 0
	}
	return pid
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	pidFile, logDir, err := daemonPaths()
	if err != nil {
		return err
	}
	if pid := readDaemonPID(pidFile); pid != 0 {
		return fmt.Errorf("daemon already running (pid %d) — stop it first with `bai daemon stop`", pid)
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(pidFile), err)
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	defer os.Remove(pidFile)

	path, err := schedule.DefaultPath()
	if err != nil {
		return err
	}
	runner := &daemon.Runner{
		Store:   schedule.New(path),
		Execute: runScheduledEntry,
		LogDir:  logDir,
	}

	p := printer(loadConfig())
	p.Info(fmt.Sprintf("Daemon started (pid %d), checking schedules every %s. Ctrl+C or `bai daemon stop` to stop.", os.Getpid(), tickInterval))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.Info("Daemon stopped.")
			return nil
		case <-ticker.C:
			results, err := runner.Tick(ctx)
			if err != nil {
				p.Error("tick: " + err.Error())
				continue
			}
			for _, r := range results {
				if r.Err != nil {
					p.Error(fmt.Sprintf("schedule %s (%s) failed: %v", r.Entry.ID, r.Entry.Name, r.Err))
				} else {
					p.Success(fmt.Sprintf("schedule %s (%s) ran", r.Entry.ID, r.Entry.Name))
				}
			}
		}
	}
}

// runScheduledEntry runs one scheduled entry to completion via the embedded
// SDK runner (no interactive approval — every tool call auto-executes, same
// as any other headless/scripted use of sdk/agent) and returns a transcript
// for the log file.
func runScheduledEntry(ctx context.Context, e schedule.Entry) (string, error) {
	var sb strings.Builder
	r := agent.New(agent.Options{
		Model:   e.Model,
		WorkDir: e.Dir,
		OnEvent: func(ev agent.Event) {
			switch ev.Type {
			case "text":
				sb.WriteString(ev.Text)
			case "tool_use":
				fmt.Fprintf(&sb, "\n[tool] %s(%s)\n", ev.ToolName, ev.ToolInput)
			case "tool_result":
				fmt.Fprintf(&sb, "[result] %s\n", ev.ToolOutput)
			case "error":
				fmt.Fprintf(&sb, "\n[error] %v\n", ev.Err)
			}
		},
	})
	defer r.Close()
	err := r.Run(ctx, e.Prompt)
	return sb.String(), err
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	pidFile, _, err := daemonPaths()
	if err != nil {
		return err
	}
	pid := readDaemonPID(pidFile)
	if pid == 0 {
		return fmt.Errorf("daemon is not running")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop daemon (pid %d): %w", pid, err)
	}

	p := printer(loadConfig())
	p.Success(fmt.Sprintf("Sent stop signal to daemon (pid %d)", pid))
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	pidFile, _, err := daemonPaths()
	if err != nil {
		return err
	}
	p := printer(loadConfig())
	if pid := readDaemonPID(pidFile); pid != 0 {
		p.Success(fmt.Sprintf("Daemon running (pid %d)", pid))
	} else {
		p.Info("Daemon not running")
	}
	return nil
}
