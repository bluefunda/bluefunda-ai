package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/bluefunda/bluefunda-ai/internal/abap"
	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/auth"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/sdk/agent"
)

// abapTracker records the objects the ABAP agent deploys in this process.
var abapTracker = abap.NewTracker()

// abapEnv wires the ABAP pack to the platform login and the active SAP system.
var abapEnv = abap.Env{NewClient: newABAPClient, Tracker: abapTracker}

// newABAPClient builds a gateway client from the signed-in platform user and
// the active SAP system.
func newABAPClient() (*abap.Client, error) {
	cfg := loadConfig()
	if cfg.Auth.AccessToken == "" {
		return nil, fmt.Errorf("not signed in — run `%s login`", app.Current().Name)
	}
	ts := caigrpc.NewTokenSource(cfg, func() (string, error) {
		tok, err := auth.Refresh(cfg.EffectiveDomain(), cfg.Realm, cfg.Auth.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("token refresh failed — run `%s login`: %w", app.Current().Name, err)
		}
		if err := saveAuthTokens(cfg, tok); err != nil {
			return "", fmt.Errorf("save tokens: %w", err)
		}
		return tok.AccessToken, nil
	})
	reg, err := abap.LoadSystems()
	if err != nil {
		return nil, err
	}
	return abap.NewClient(abap.GatewayURL(), cfg.Realm, ts.Token, reg.ActiveSystem()), nil
}

// registerABAPPack adds the ABAP tools and instructions to this process.
func registerABAPPack() error {
	_, err := tools.RegisterPack(abap.Pack(abapEnv))
	return err
}

// NewABAPCommands returns the ABAP command set. bai mounts it under
// `bai abap`; the abaper binary mounts the same commands at its root.
func NewABAPCommands() []*cobra.Command {
	return []*cobra.Command{
		abapStatusCmd(),
		abapSystemCmd(),
		abapSearchCmd(),
		abapGetCmd(),
		abapListCmd(),
		abapCheckCmd(),
		abapDeployCmd(),
		abapTestCmd(),
		abapCodeCmd(),
		abapGenerateCmd(),
	}
}

// runABAPSession opens the interactive agent with the ABAP pack and slash commands.
func runABAPSession(args []string) error {
	if err := registerABAPPack(); err != nil {
		return err
	}
	sessionSlashCommands = append(sessionSlashCommands, abap.SlashCommands(abapEnv)...)
	return runAgenticSession(args)
}

var abapCmd = &cobra.Command{
	Use:   "abap",
	Short: "SAP ABAP development: systems, objects, and the ABAP agent",
	Long: `Work with SAP ABAP systems from bai.

With no subcommand, opens the interactive agent with ABAP tools
(search, read, syntax-check, deploy, activate, unit-test) and ABAP
slash commands (/system, /search, /source, /activate, /test, /verify).

Start by registering a system:
  bai abap system add --name dev --host https://sap.example.com:44300 --client 100 -u DEVELOPER`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runABAPSession(nil)
	},
}

func init() {
	abapCmd.AddCommand(NewABAPCommands()...)
}

// ── status ────────────────────────────────────────────────────────────────────

// systemCheckTimeout bounds each SAP connectivity check so one unreachable
// system can't stall the report.
const systemCheckTimeout = 5 * time.Second

// abapSystemStatus is one SAP system's line in `status`.
type abapSystemStatus struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Active    bool   `json:"active"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// abapStatusReport is what `status` prints, and its JSON output.
type abapStatusReport struct {
	SignedIn         bool               `json:"signedIn"`
	SessionExpired   bool               `json:"sessionExpired"`
	Gateway          string             `json:"gateway"`
	GatewayReachable bool               `json:"gatewayReachable"`
	GatewayStatus    string             `json:"gatewayStatus,omitempty"`
	Systems          []abapSystemStatus `json:"systems"`
}

// collectABAPStatus checks the session, the gateway and every SAP system.
// c is nil when there is no stored session. A session counts as expired when
// its token can't be obtained or refreshed. The gateway is checked either way;
// SAP systems need a session, so without one they are listed but not checked.
func collectABAPStatus(c *abap.Client, gateway, realm string, reg *abap.Systems) abapStatusReport {
	status := abapStatusReport{Gateway: gateway, Systems: []abapSystemStatus{}}
	if c == nil {
		c = abap.NewClient(gateway, realm, nil, nil)
	} else if _, err := c.TokenFunc(); err != nil {
		status.SessionExpired = true
		anon := *c
		anon.TokenFunc = nil // don't retry the failed refresh for the health check
		c = &anon
	} else {
		status.SignedIn = true
	}
	if st, err := c.Health(); err == nil {
		status.GatewayReachable, status.GatewayStatus = true, st
	}

	active := reg.ActiveSystem()
	status.Systems = make([]abapSystemStatus, len(reg.Systems))
	var wg sync.WaitGroup
	for i := range reg.Systems {
		sys := &reg.Systems[i]
		status.Systems[i] = abapSystemStatus{Name: sys.Name, Host: sys.Host, Active: active != nil && sys.ID == active.ID}
		if !status.SignedIn {
			continue
		}
		wg.Add(1)
		go func(st *abapSystemStatus) {
			defer wg.Done()
			if err := c.WithSystem(sys, systemCheckTimeout).Connect(); err != nil {
				st.Error = err.Error()
				return
			}
			st.Reachable = true
		}(&status.Systems[i])
	}
	wg.Wait()
	return status
}

func abapStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sign-in, gateway, and SAP system connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			p := printer(cfg)
			reg, err := abap.LoadSystems()
			if err != nil {
				return err
			}
			c, err := newABAPClient()
			if err != nil {
				c = nil // no stored session
			}
			status := collectABAPStatus(c, abap.GatewayURL(), cfg.Realm, reg)

			if p.Format == ui.FormatJSON {
				p.JSON(status)
				return nil
			}
			login := app.Current().Name + " login"
			signedIn := "true"
			switch {
			case status.SessionExpired:
				signedIn = fmt.Sprintf("false (session expired — run `%s`)", login)
			case !status.SignedIn:
				signedIn = fmt.Sprintf("false (run `%s`)", login)
			}
			fmt.Printf("Signed in:        %s\n", signedIn)
			fmt.Printf("Gateway:          %s (reachable: %v)\n", status.Gateway, status.GatewayReachable)
			if len(reg.Systems) == 0 {
				fmt.Printf("SAP systems:      none — add one with `%s`\n", abap.SystemAddHint())
				return nil
			}
			rows := make([][]string, 0, len(status.Systems))
			for _, s := range status.Systems {
				mark, state := "", "reachable"
				if s.Active {
					mark = "*"
				}
				switch {
				case !status.SignedIn:
					state = "not checked (sign in first)"
				case !s.Reachable:
					state = "unreachable " + s.Error
				}
				rows = append(rows, []string{s.Name, s.Host, mark, state})
			}
			fmt.Println()
			p.Table([]string{"NAME", "HOST", "ACTIVE", "STATUS"}, rows)
			return nil
		},
	}
}

// ── system ────────────────────────────────────────────────────────────────────

func abapSystemCmd() *cobra.Command {
	system := &cobra.Command{
		Use:   "system",
		Short: "Manage SAP system connections",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return printSystems() },
	}

	var sys abap.SAPSystem
	add := &cobra.Command{
		Use:   "add",
		Short: "Register an SAP system (password from --password, $SAP_PASSWORD, or a prompt)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if sys.Host == "" || sys.Username == "" {
				return fmt.Errorf("--host and --username are required")
			}
			if sys.Password == "" {
				sys.Password = os.Getenv("SAP_PASSWORD")
			}
			if sys.Password == "" && term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprint(os.Stderr, "SAP password: ")
				pw, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				sys.Password = string(pw)
			}
			reg, err := abap.LoadSystems()
			if err != nil {
				return err
			}
			if sys.Name != "" && reg.Find(sys.Name) != nil {
				return fmt.Errorf("a system named %q already exists", sys.Name)
			}
			reg.Add(sys)
			if err := reg.Save(); err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("Added SAP system %s", firstNonEmpty(sys.Name, sys.Host)))
			return nil
		},
	}
	add.Flags().StringVar(&sys.Name, "name", "", "Display name (defaults to host)")
	add.Flags().StringVar(&sys.Host, "host", "", "SAP host URL, e.g. https://sap.example.com:44300")
	add.Flags().StringVar(&sys.Client, "client", "100", "SAP client")
	add.Flags().StringVarP(&sys.Username, "username", "u", "", "SAP user")
	add.Flags().StringVar(&sys.Username, "user", "", "Alias for --username")
	_ = add.Flags().MarkHidden("user")
	add.Flags().StringVarP(&sys.Password, "password", "p", "", "SAP password (prefer $SAP_PASSWORD or the prompt)")

	use := &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active SAP system",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := abap.LoadSystems()
			if err != nil {
				return err
			}
			s := reg.Find(args[0])
			if s == nil {
				return fmt.Errorf("no system named %q", args[0])
			}
			reg.Active = s.ID
			if err := reg.Save(); err != nil {
				return err
			}
			ui.Success("Active SAP system: " + s.Name)
			return nil
		},
	}

	remove := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an SAP system",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := abap.LoadSystems()
			if err != nil {
				return err
			}
			s := reg.Find(args[0])
			if s == nil {
				return fmt.Errorf("no system named %q", args[0])
			}
			name := s.Name
			reg.Remove(s.ID)
			if err := reg.Save(); err != nil {
				return err
			}
			ui.Success("Removed SAP system " + name)
			return nil
		},
	}

	test := &cobra.Command{
		Use:   "test [name]",
		Short: "Check that the gateway can log on to an SAP system (default: the active one)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				reg, err := abap.LoadSystems()
				if err != nil {
					return err
				}
				if c.System = reg.Find(args[0]); c.System == nil {
					return fmt.Errorf("no system named %q", args[0])
				}
			}
			if err := c.Connect(); err != nil {
				return err
			}
			ui.Success("Connected to " + c.System.Name)
			return nil
		},
	}

	system.AddCommand(add, use, remove, test)
	return system
}

func printSystems() error {
	reg, err := abap.LoadSystems()
	if err != nil {
		return err
	}
	if len(reg.Systems) == 0 {
		ui.Info(fmt.Sprintf("No SAP systems configured — add one with `%s`", abap.SystemAddHint()))
		return nil
	}
	active := reg.ActiveSystem()
	rows := make([][]string, 0, len(reg.Systems))
	for _, s := range reg.Systems {
		mark := ""
		if active != nil && s.ID == active.ID {
			mark = "*"
		}
		rows = append(rows, []string{s.Name, s.Host, s.Client, s.Username, mark})
	}
	printer(loadConfig()).Table([]string{"NAME", "HOST", "CLIENT", "USER", "ACTIVE"}, rows)
	return nil
}

// ── object commands ───────────────────────────────────────────────────────────

type objectFlags struct {
	name, objType, sourceFile string
}

func (f *objectFlags) register(cmd *cobra.Command, withSource bool) {
	cmd.Flags().StringVar(&f.name, "name", "", "Object name (required)")
	cmd.Flags().StringVar(&f.objType, "type", "program", "Object type: "+strings.Join(abap.ObjectTypes, ", "))
	_ = cmd.MarkFlagRequired("name")
	if withSource {
		cmd.Flags().StringVar(&f.sourceFile, "source-file", "", "Path to the source file (required)")
		_ = cmd.MarkFlagRequired("source-file")
	}
}

func (f *objectFlags) source() (string, error) {
	b, err := os.ReadFile(f.sourceFile)
	if err != nil {
		return "", fmt.Errorf("read source file: %w", err)
	}
	return string(b), nil
}

func abapSearchCmd() *cobra.Command {
	var objType string
	cmd := &cobra.Command{
		Use:   "search <pattern>",
		Short: "Search SAP objects by name pattern (SAP wildcards, e.g. ZCL_*)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			objs, err := c.Search(strings.ToUpper(args[0]), objType)
			if err != nil {
				return err
			}
			printObjectRows(objs)
			return nil
		},
	}
	cmd.Flags().StringVar(&objType, "type", "", "ADT type filter, e.g. CLAS/OC, PROG/P")
	return cmd
}

func abapListCmd() *cobra.Command {
	var pkg, objType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the objects in a package",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			objs, err := c.ListPackage(strings.ToUpper(pkg), objType)
			if err != nil {
				return err
			}
			printObjectRows(objs)
			return nil
		},
	}
	cmd.Flags().StringVar(&pkg, "package", "", "Package name, e.g. ZORDERS or $TMP (required)")
	cmd.Flags().StringVar(&objType, "type", "", "Object type filter")
	_ = cmd.MarkFlagRequired("package")
	return cmd
}

// printObjectRows prints gateway object maps as a table whose columns are the
// union of their keys, so the output follows whatever the gateway returns.
func printObjectRows(objs []map[string]any) {
	p := printer(loadConfig())
	if p.Format == ui.FormatJSON {
		p.JSON(objs)
		return
	}
	keySet := map[string]bool{}
	for _, o := range objs {
		for k := range o {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	// Name-like columns first so quiet mode prints one object name per line.
	sort.SliceStable(keys, func(i, j int) bool {
		ri, rj := columnRank(keys[i]), columnRank(keys[j])
		if ri != rj {
			return ri < rj
		}
		return keys[i] < keys[j]
	})
	rows := make([][]string, 0, len(objs))
	for _, o := range objs {
		row := make([]string, len(keys))
		for i, k := range keys {
			if v, ok := o[k]; ok && v != nil {
				row[i] = fmt.Sprint(v)
			}
		}
		rows = append(rows, row)
	}
	headers := make([]string, len(keys))
	for i, k := range keys {
		headers[i] = strings.ToUpper(k)
	}
	p.Table(headers, rows)
}

func columnRank(key string) int {
	switch strings.ToLower(key) {
	case "name", "object_name", "objectname":
		return 0
	case "type", "object_type", "objecttype":
		return 1
	}
	return 2
}

func abapGetCmd() *cobra.Command {
	var f objectFlags
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Print an object's source from SAP",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			src, err := c.Source(strings.ToUpper(f.name), f.objType)
			if err != nil {
				return err
			}
			p := printer(loadConfig())
			if p.Format == ui.FormatJSON {
				p.JSON(map[string]string{"name": strings.ToUpper(f.name), "type": f.objType, "source": src})
				return nil
			}
			fmt.Print(src)
			if !strings.HasSuffix(src, "\n") {
				fmt.Println()
			}
			return nil
		},
	}
	f.register(cmd, false)
	return cmd
}

func abapCheckCmd() *cobra.Command {
	var f objectFlags
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Syntax-check a local source file against SAP (saves nothing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := f.source()
			if err != nil {
				return err
			}
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			res, err := c.SyntaxCheck(strings.ToUpper(f.name), f.objType, src)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(res.Messages))
			errors := 0
			for _, m := range res.Messages {
				if m.Severity == "error" {
					errors++
				}
				rows = append(rows, []string{fmt.Sprint(m.Line), fmt.Sprint(m.Column), m.Severity, m.Text})
			}
			p := printer(loadConfig())
			if p.Format == ui.FormatJSON {
				p.JSON(res)
			} else if len(rows) > 0 {
				p.Table([]string{"LINE", "COL", "SEVERITY", "MESSAGE"}, rows)
			}
			if errors > 0 {
				return fmt.Errorf("syntax check found %d error(s)", errors)
			}
			ui.Success("Syntax OK")
			return nil
		},
	}
	f.register(cmd, true)
	return cmd
}

func abapDeployCmd() *cobra.Command {
	var f objectFlags
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Upload a local source file to SAP and activate it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			src, err := f.source()
			if err != nil {
				return err
			}
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			name := strings.ToUpper(f.name)
			ui.Info(fmt.Sprintf("Deploying %s %s…", f.objType, name))
			if err := abap.Deploy(c, name, f.objType, src); err != nil {
				return err
			}
			ui.Success(fmt.Sprintf("Deployed and activated %s %s", f.objType, name))
			return nil
		},
	}
	f.register(cmd, true)
	return cmd
}

func abapTestCmd() *cobra.Command {
	var f objectFlags
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run an object's ABAP Unit tests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			res, err := c.UnitTests(strings.ToUpper(f.name), f.objType)
			if err != nil {
				return err
			}
			p := printer(loadConfig())
			if p.Format == ui.FormatJSON {
				p.JSON(res)
			} else {
				var rows [][]string
				for _, tc := range res.TestClasses {
					for _, m := range tc.Methods {
						rows = append(rows, []string{tc.Name, m.Name, m.Status, m.Message})
					}
				}
				p.Table([]string{"CLASS", "METHOD", "STATUS", "MESSAGE"}, rows)
			}
			if res.Failed > 0 {
				return fmt.Errorf("%d of %d unit test(s) failed", res.Failed, res.TotalTests)
			}
			ui.Success(fmt.Sprintf("%d unit test(s) passed", res.Passed))
			return nil
		},
	}
	f.register(cmd, false)
	return cmd
}

// abapGenerateCmd keeps the standalone abaper `generate` command: save an
// object (from a file, or a minimal skeleton) without activating it.
func abapGenerateCmd() *cobra.Command {
	var f objectFlags
	cmd := &cobra.Command{
		Use:    "generate",
		Short:  "Create an object on SAP without activating it (default: a minimal skeleton)",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToUpper(f.name)
			src := skeletonSource(f.objType, name)
			if f.sourceFile != "" {
				var err error
				if src, err = f.source(); err != nil {
					return err
				}
			}
			c, err := newABAPClient()
			if err != nil {
				return err
			}
			if err := c.Save(name, f.objType, src); err != nil {
				return fmt.Errorf("generate failed: %w", err)
			}
			ui.Success(fmt.Sprintf("Created %s %s", f.objType, name))
			return nil
		},
	}
	f.register(cmd, false)
	cmd.Flags().StringVar(&f.sourceFile, "source-file", "", "Path to the source file (default: a minimal skeleton)")
	return cmd
}

func skeletonSource(objectType, name string) string {
	switch objectType {
	case "class":
		return fmt.Sprintf("CLASS %s DEFINITION PUBLIC.\nENDCLASS.\n\nCLASS %s IMPLEMENTATION.\nENDCLASS.", name, name)
	case "interface":
		return fmt.Sprintf("INTERFACE %s PUBLIC.\nENDINTERFACE.", name)
	default:
		return fmt.Sprintf("REPORT %s.", name)
	}
}

// ── code: headless ABAP agent with a verification gate ───────────────────────

func abapCodeCmd() *cobra.Command {
	var (
		model       string
		maxTurns    int
		noVerify    bool
		repairTurns int
		verbose     bool
	)
	cmd := &cobra.Command{
		Use:   "code <prompt>",
		Short: "Run the ABAP agent headlessly, then verify every object it deployed",
		Long: `Run the ABAP agent non-interactively. It can read and edit local
files and deploy to SAP through native ABAP tools.

When the loop ends, every object the agent deployed is syntax-checked,
activated and unit-tested. Failures go back to the agent for up to
--repair-turns rounds; the command exits non-zero if any remain.

With -o stream-json, stdout carries one JSON event per line (session,
text, tool_use, tool_result, objects, verify, repair, done) for editor
integrations; status messages stay on stderr. The last event is always
done, and its exit field matches the exit code.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var rep codeReporter = textReporter{out: cmd.OutOrStdout(), verbose: verbose}
			if cfgOutput == outputStreamJSON {
				rep = newStreamReporter(cmd.OutOrStdout())
			}
			err := runABAPCode(strings.Join(args, " "), model, maxTurns, noVerify, repairTurns, rep)
			rep.Done(err)
			return err
		},
	}
	cmd.Flags().StringVar(&model, "model", "auto", "LLM model alias: auto, fast, think")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 20, "Maximum agentic loop iterations")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Skip the post-run syntax check / activation / unit test gate")
	cmd.Flags().IntVar(&repairTurns, "repair-turns", 2, "Rounds the agent gets to fix objects that fail verification")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show every tool result")
	return cmd
}

// runABAPCode runs the headless ABAP agent on prompt, then the verify gate.
func runABAPCode(prompt, model string, maxTurns int, noVerify bool, repairTurns int, rep codeReporter) error {
	unregister, err := tools.RegisterPack(abap.Pack(abapEnv))
	if err != nil {
		return err
	}
	defer unregister()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	system := ""
	if reg, err := abap.LoadSystems(); err == nil {
		if sys := reg.ActiveSystem(); sys != nil {
			system = sys.Name
		}
	}
	cwd, _ := os.Getwd()
	rep.Start(model, system, cwd)

	runner := agent.New(agent.Options{
		Model:    model,
		MaxTurns: maxTurns,
		// The headless agent cannot ask for approval; deny the
		// sub-agent tool so it doesn't spawn a child without the pack.
		Deny:    []string{"task"},
		OnEvent: rep.Event,
	})
	defer func() { _ = runner.Close() }()

	abapTracker.Reset()
	run := func(prompt string) error { return runner.Run(ctx, prompt) }
	if err := run(prompt); err != nil {
		return err
	}
	objs := abapTracker.Objects()
	rep.Objects(objs)
	if noVerify || len(objs) == 0 {
		return failedDeploys(objs)
	}
	c, err := newABAPClient()
	if err != nil {
		return fmt.Errorf("verification: %w", err)
	}
	return verifyAndRepair(ctx, c, run, repairTurns, rep)
}

func verifyAndRepair(ctx context.Context, c abap.Checker, run func(string) error, repairTurns int, rep codeReporter) error {
	for round := 0; ; round++ {
		rep.Verifying()
		results := abap.Verify(c, abapTracker.Objects())
		rep.Verified(round, results)
		if abap.AllPassed(results) {
			return nil
		}
		if round >= repairTurns {
			return fmt.Errorf("verification failed after %d repair round(s)", repairTurns)
		}
		rep.Repairing(round+1, repairTurns)
		if err := run(abap.RepairPrompt(results)); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
	}
}

func failedDeploys(objs []abap.Object) error {
	for _, o := range objs {
		if !o.OK {
			return fmt.Errorf("%s %s was not deployed: %s", o.Type, o.Name, o.Detail)
		}
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
