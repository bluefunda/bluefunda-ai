package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/audit"
	"github.com/bluefunda/bluefunda-ai/internal/checkpoint"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/hooks"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/session"
	"github.com/bluefunda/bluefunda-ai/internal/tools"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
	"github.com/bluefunda/bluefunda-ai/internal/worktree"
)

var (
	codeModel            string
	codeDir              string
	codeAutoApply        bool
	codeAuto             bool
	codeMaxTurns         int
	codeMaxContextTokens int     // 0 = use defaultCompactionThreshold
	codeMaxBudgetUSD     float64 // 0 = no limit
	codePrint            bool
	codeOutputFormat     string
	codeResume           string
	codeContinue         bool
	codeNoTools          bool
	codeWorktree         bool
	codePlanMode         bool
	// codeAllowedTools / codeDisallowedTools are per-run permission rules from
	// --allowed-tools / --disallowed-tools (#406), layered on top of the
	// settings-file rules using the same pattern syntax.
	codeAllowedTools    []string
	codeDisallowedTools []string
	// codeInputFormat is --input-format: "text" (a prompt) or "stream-json"
	// (one JSON user message per line on stdin). codeJSONSchema is
	// --json-schema: the final answer must be JSON matching it.
	codeInputFormat string
	codeJSONSchema  string
	// codeTools limits the session to these tools (--tools, #408). Empty
	// means every tool. task subagents of a restricted type run with it.
	codeTools     []string
	codeRetryMode bool // set by `bai chat retry <id>`: derive the prompt
	// from the last turn in the resumed session instead of requiring one on
	// the command line.
)

// sessionSlashCommands and sessionDeny are extra TUI slash commands and
// permission deny rules that a product command group (e.g. `bai abap`) adds
// to the interactive session before calling runAgenticSession.
var (
	sessionSlashCommands []tui.SlashCommand
	sessionDeny          []string

	// codeMaxTurnsSet records whether --max-turns was passed, so an
	// interactive session without it runs with no step limit.
	codeMaxTurnsSet bool
)

// defaultMaxTurnsFlag is the --max-turns default, shared by both flag
// registrations (root.go's primary `bai` command and this file's deprecated
// `bai code` alias) and used as the sentinel for "the user didn't explicitly
// pass --max-turns" when deciding whether a project config's max_turns
// should apply instead.
const defaultMaxTurnsFlag = 50

// resolveMaxTurns returns the effective --max-turns value: the project
// config's max_turns wins when flagValue is still at its default — a
// sentinel-based "was it explicitly set" check, so a user who explicitly
// passes --max-turns 50 (matching the default) is indistinguishable from one
// who didn't pass the flag at all and will also get the project override.
// A non-positive project value is ignored either way.
func resolveMaxTurns(flagValue, projectMaxTurns int) int {
	if projectMaxTurns > 0 && flagValue == defaultMaxTurnsFlag {
		return projectMaxTurns
	}
	return flagValue
}

// interactiveMaxTurns returns the step limit for an interactive session:
// none (0) unless the user asked for one.
func interactiveMaxTurns(resolved int, requested bool) int {
	if !requested {
		return 0
	}
	return resolved
}

// codeCmd is a deprecated alias for the root 'bai' command.
// It is hidden from help but preserved for backward compatibility.
var codeCmd = &cobra.Command{
	Use:    "code [prompt]",
	Short:  "Deprecated: use 'bai' directly",
	Hidden: true,
	Args:   cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Note: 'bai code' is now just 'bai'. This alias will be removed in a future release.")
		codeMaxTurnsSet = cmd.Flags().Changed("max-turns")
		return runAgenticSession(args)
	},
}

func init() {
	// Keep flags on codeCmd so existing scripts using 'bai code --auto' etc. still work.
	codeCmd.Flags().StringVar(&codeModel, "model", "", "LLM model to use")
	codeCmd.Flags().StringVar(&codeDir, "dir", ".", "Working directory for file operations")
	codeCmd.Flags().BoolVar(&codeAutoApply, "auto-apply", false, "Execute write/bash tools without prompting")
	codeCmd.Flags().BoolVar(&codeAuto, "auto", false, "Same as --auto-apply")
	codeCmd.Flags().IntVar(&codeMaxTurns, "max-turns", defaultMaxTurnsFlag, "Maximum agentic loop iterations before stopping")
	codeCmd.Flags().IntVar(&codeMaxContextTokens, "max-context-tokens", 0, "Max context tokens before auto-compaction (default 100000; env BAI_MAX_CONTEXT_TOKENS)")
	codeCmd.Flags().Float64Var(&codeMaxBudgetUSD, "max-budget-usd", 0, "Stop session when estimated cost exceeds this USD amount (0 = no limit; env BAI_MAX_BUDGET_USD)")
	codeCmd.Flags().BoolVarP(&codePrint, "print", "p", false, "Non-interactive mode: print output to stdout")
	codeCmd.Flags().StringVar(&codeOutputFormat, "output-format", "text", "Output format for --print: text, json, stream-json")
	codeCmd.Flags().StringVar(&codeResume, "resume", "", "Resume a previous session by ID")
	codeCmd.Flags().BoolVarP(&codeContinue, "continue", "c", false, "Resume the most recent code session")
	registerPermissionFlags(codeCmd.Flags(), &codeAllowedTools, &codeDisallowedTools)
	registerToolsFlag(codeCmd.Flags(), &codeTools)
	registerHeadlessFlags(codeCmd.Flags(), &codeInputFormat, &codeJSONSchema)
}

// registerToolsFlag adds --tools, which limits the session to the named
// tools (Claude Code's --tools). The task tool passes it to subagents of a
// restricted type such as explore (#408).
func registerToolsFlag(fs *pflag.FlagSet, dst *[]string) {
	fs.StringSliceVar(dst, "tools", nil,
		`Only offer these tools to the model, e.g. "read_file,search_content" (default: all)`)
}

// restrictTools keeps only the tools named in only, and returns the names it
// removed so they can also be denied: a model that calls a hidden tool anyway
// is refused rather than obeyed.
func restrictTools(schemas string, only []string) (string, []string, error) {
	if len(only) == 0 || schemas == "" {
		return schemas, nil, nil
	}
	keep := make(map[string]bool, len(only))
	for _, n := range only {
		if n = strings.TrimSpace(n); n != "" {
			keep[n] = true
		}
	}
	return tools.FilterSchemas(schemas, func(name string) bool { return keep[name] })
}

// loadAgentTypes registers the task tool's agent types: the built-ins, then
// ~/.bai/agents/*.md, then <project>/.bai/agents/*.md (project wins). Files
// that fail to parse are reported and skipped.
func loadAgentTypes(errW io.Writer) {
	var dirs []string
	if d, err := app.HomePath("agents"); err == nil {
		dirs = append(dirs, d)
	}
	root, err := worktree.GitRoot()
	if err != nil {
		root, _ = os.Getwd()
	}
	if root != "" {
		dirs = append(dirs, filepath.Join(root, app.Current().Dir, "agents"))
	}
	types, errs := tools.LoadAgentTypes(dirs...)
	for _, e := range errs {
		fmt.Fprintf(errW, "[%s] skipping agent definition: %v\n", app.Current().Name, e)
	}
	tools.SetAgentTypes(types)
}

// loadSkills registers the session's skills: ~/.bai/skills/*/SKILL.md,
// then <project>/.bai/skills/*/SKILL.md (project wins on a name clash).
func loadSkills(errW io.Writer) {
	var dirs []string
	if d, err := app.HomePath("skills"); err == nil {
		dirs = append(dirs, d)
	}
	root, err := worktree.GitRoot()
	if err != nil {
		root, _ = os.Getwd()
	}
	if root != "" {
		dirs = append(dirs, filepath.Join(root, app.Current().Dir, "skills"))
	}
	skills, errs := tools.LoadSkills(dirs...)
	for _, e := range errs {
		fmt.Fprintf(errW, "[%s] skipping skill: %v\n", app.Current().Name, e)
	}
	tools.SetSkills(skills)
}

// registerPermissionFlags adds --allowed-tools / --disallowed-tools to fs,
// plus Claude Code's camelCase spellings (--allowedTools, --disallowedTools)
// as hidden aliases via flag-name normalization (#406).
func registerPermissionFlags(fs *pflag.FlagSet, allowed, disallowed *[]string) {
	fs.StringSliceVar(allowed, "allowed-tools", nil,
		`Tool rules to auto-approve for this run, e.g. "bash:go test *" (comma-separated or repeated)`)
	fs.StringSliceVar(disallowed, "disallowed-tools", nil,
		`Tool rules to block for this run, e.g. "write_file" or "bash:rm *" (comma-separated or repeated)`)
	prev := fs.GetNormalizeFunc()
	fs.SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		switch name {
		case "allowedTools":
			name = "allowed-tools"
		case "disallowedTools":
			name = "disallowed-tools"
		}
		return prev(f, name)
	})
}

// mergePermissionRules returns new slices holding the settings rules followed
// by the per-run flag rules, with blank flag entries dropped. Deny is still
// evaluated before allow by tools.CheckPermissions, so a deny rule from any
// source wins.
func mergePermissionRules(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	for _, r := range extra {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// codeMessage mirrors the ConversationMsg format expected by cai-llm-router.
type codeMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []codeToolCall `json:"tool_calls,omitempty"`
}

// lastUserIndex returns the index of the last Role=="user" message in
// history, or -1 if none exists. Used by both the interactive /retry slash
// command and the non-interactive `bai chat retry` to find and drop the
// prior turn before resubmitting it (#289).
func lastUserIndex(history []codeMessage) int {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return i
		}
	}
	return -1
}

// codeToolCall matches messages.ToolCall nested format in cai-llm-router.
type codeToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function codeFuncCall `json:"function"`
}

type codeFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// cliPayloadVersion is incremented on any breaking change to cliCodePayload.
// cai-llm-router validates this and returns a clear error on mismatch so
// users know to upgrade their bai client.
const cliPayloadVersion = 1

// cliCodePayload is encoded in Prompt to work around proto fields 8+ being stripped
// by the load balancer between cli.bluefunda.com:443 and the BFF gRPC endpoint.
type cliCodePayload struct {
	V        int           `json:"v"` // payload format version — must match cliPayloadVersion in cai-llm-router
	History  []codeMessage `json:"history"`
	Tools    string        `json:"tools"`
	Timezone string        `json:"timezone,omitempty"` // IANA zone, e.g. "America/New_York"; empty if undetectable
}

// localTimezoneName returns the machine's IANA timezone identifier, or "" if
// it can't be determined. time.Local.String() is not useable for this: Go
// reports it as the literal string "Local", not an IANA name.
func localTimezoneName() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	link, err := os.Readlink("/etc/localtime")
	if err != nil {
		return ""
	}
	const zoneinfo = "zoneinfo/"
	if idx := strings.Index(link, zoneinfo); idx != -1 {
		return link[idx+len(zoneinfo):]
	}
	return ""
}

func runAgenticSession(args []string) error {
	if codeAuto {
		codeAutoApply = true
	}

	conn, cfg, err := bffConn()
	if err != nil {
		return err
	}
	defer conn.Close()

	model := codeModel
	if model == "" {
		model = cfg.EffectiveModel()
	}
	model = resolveModelAlias(model)

	initialPrompt := strings.Join(args, " ")

	if err := os.Chdir(codeDir); err != nil {
		return fmt.Errorf("chdir to %s: %w", codeDir, err)
	}

	// --- Worktree isolation (#126) ---
	// When --worktree is set, create a detached git worktree and switch the
	// process cwd into it so all tool file/bash operations go there instead of
	// the user's main working tree.
	var (
		worktreePath string
		gitRootPath  string
	)
	if codeWorktree {
		if codePrint {
			return fmt.Errorf("--worktree is not supported in --print / headless mode")
		}
		var err error
		gitRootPath, err = worktree.GitRoot()
		if err != nil {
			return fmt.Errorf("--worktree requires a git repository: %w", err)
		}
		// sessionID is determined later; use a placeholder UUID for the worktree path.
		// We reassign sessionID below before using it for session persistence.
	}

	// Apply project-level max_turns override if the flag wasn't set explicitly.
	maxTurnsRequested := codeMaxTurnsSet
	if projCfgEarly := config.FindProjectConfig("."); projCfgEarly != nil {
		codeMaxTurns = resolveMaxTurns(codeMaxTurns, projCfgEarly.MaxTurns)
		maxTurnsRequested = maxTurnsRequested || projCfgEarly.MaxTurns > 0
	}

	// Resolve effective context token limit: CLI flag > BAI_MAX_CONTEXT_TOKENS > default.
	maxContextTokens := codeMaxContextTokens
	if maxContextTokens == 0 {
		if v := app.Getenv("MAX_CONTEXT_TOKENS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				maxContextTokens = n
			}
		}
	}
	if maxContextTokens == 0 {
		maxContextTokens = defaultCompactionThreshold
	}

	// Resolve effective budget limit: CLI flag > BAI_MAX_BUDGET_USD > 0 (no limit).
	maxBudgetUSD := codeMaxBudgetUSD
	if maxBudgetUSD == 0 {
		if v := app.Getenv("MAX_BUDGET_USD"); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				maxBudgetUSD = f
			}
		}
	}

	var toolSchemas string
	if !codeNoTools {
		loadAgentTypes(os.Stderr)
		loadSkills(os.Stderr)
		var err error
		toolSchemas, err = tools.LocalToolSchemas()
		if err != nil {
			return fmt.Errorf("build tool schemas: %w", err)
		}
	}

	workDir, _ := os.Getwd()
	p := printer(cfg)

	// --- Pre-flight token check (#186) ---
	// Refresh the token before any UI launches so that expired credentials are
	// handled on a clean terminal rather than mid-session inside the TUI.
	// NearExpiry(5m) catches already-expired tokens and tokens about to expire,
	// avoiding a mid-session interruption for short sessions too.
	if conn.TS.NearExpiry(5 * time.Minute) {
		if err := conn.TS.EnsureValidToken(); err != nil {
			if authErr := reAuthenticate(cfg, p); authErr != nil {
				return authErr
			}
		}
	}

	// Agents included for this user (on in chat mode), as in the web app.
	sessionAgents = loadIncludedAgents(conn)
	defer func() { sessionAgents = nil }()

	// --- Local MCP servers (#85) ---
	// Start servers defined in .bai/settings.yaml mcp_servers and merge their
	// tools into the schema sent to the LLM on every agentic turn.
	projCfg := config.FindProjectConfig(".")
	mcpMgr := mcp.NewManager(context.Background(), projCfg)

	// --- Permission policy (#123) ---
	// Load allow/deny lists from the project config (empty = no restrictions).
	var permAllow, permDeny []string
	// permSources records where each rule came from, for /permissions.
	var permSources []ruleSource
	if projCfg != nil {
		permAllow = projCfg.Permissions.Allow
		permDeny = projCfg.Permissions.Deny
		permSources = append(permSources, ruleSource{app.Current().Dir + "/settings.yaml", permAllow, permDeny})
	}
	// User-wide rules from ~/.bai/settings.yaml apply in every directory.
	if us, _, err := config.LoadUserSettings(); err != nil {
		return fmt.Errorf("user settings: %w", err)
	} else {
		permAllow = mergePermissionRules(permAllow, us.Permissions.Allow)
		permDeny = mergePermissionRules(permDeny, us.Permissions.Deny)
		permSources = append(permSources, ruleSource{"~/" + app.Current().Dir + "/settings.yaml", us.Permissions.Allow, us.Permissions.Deny})
	}
	permDeny = mergePermissionRules(permDeny, sessionDeny)
	permAllow = mergePermissionRules(permAllow, codeAllowedTools)
	permDeny = mergePermissionRules(permDeny, codeDisallowedTools)
	permSources = append(permSources,
		ruleSource{"built in to " + app.Current().Name, nil, sessionDeny},
		ruleSource{"--allowed-tools / --disallowed-tools", codeAllowedTools, codeDisallowedTools})

	// --- Personal, local "always allow" rules + workspace trust gate (#331) ---
	// .bai/settings.local.yaml is gitignored and holds per-developer rules
	// persisted via the 'a' key in the approval dialog — opt-in only (see
	// permissions.persist_always_allow below) and never applied until the
	// user explicitly trusts this workspace.
	localCfg, localCfgPath := config.FindLocalConfig(".")
	if localCfg != nil && (len(localCfg.Permissions.Allow) > 0 || len(localCfg.Permissions.Deny) > 0) {
		if !localCfg.Trusted && !codePrint && isTerminal() {
			n := len(localCfg.Permissions.Allow) + len(localCfg.Permissions.Deny)
			fmt.Printf("\nThis project has %d saved permission rule(s) in .bai/settings.local.yaml.\n", n)
			fmt.Print("Trust this workspace and apply them? [y/N] > ")
			var choice string
			_, _ = fmt.Scanln(&choice)
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(choice)), "y") {
				localCfg.Trusted = true
				_ = config.SaveLocalConfig(localCfgPath, localCfg)
			}
		}
		if localCfg.Trusted {
			permAllow = append(permAllow, localCfg.Permissions.Allow...)
			permDeny = append(permDeny, localCfg.Permissions.Deny...)
			permSources = append(permSources, ruleSource{app.Current().Dir + "/settings.local.yaml", localCfg.Permissions.Allow, localCfg.Permissions.Deny})
		}
	}
	// task subagents run with --auto, so the deny list is their only
	// guard: forward every deny rule in force here so a child can never do
	// more than its parent (#406).
	tools.SetTaskDisallowedTools(permDeny)
	defer mcpMgr.Close()
	if extra := mcpMgr.ToolSchemas(); len(extra) > 0 {
		merged, mergeErr := tools.MergeSchemas(toolSchemas, extra)
		if mergeErr == nil {
			toolSchemas = merged
		}
	}

	// --- Plugin tools (#128) ---
	// Load .bai/plugins/*/plugin.yaml from user-level and project directories.
	pluginMgr := plugins.NewManager(".")
	if extra := pluginMgr.ToolSchemas(); len(extra) > 0 {
		merged, mergeErr := tools.MergeSchemas(toolSchemas, extra)
		if mergeErr == nil {
			toolSchemas = merged
		}
		for _, p := range pluginMgr.All() {
			fmt.Printf("[bai] plugin %s: loaded (%s)\n", p.Manifest.Name, p.SourcePath)
		}
	}

	// --- Tool restriction (--tools, #408) ---
	// Applied after MCP and plugin tools are merged so it covers them too.
	if filtered, dropped, err := restrictTools(toolSchemas, codeTools); err != nil {
		return fmt.Errorf("--tools: %w", err)
	} else if len(dropped) > 0 {
		toolSchemas = filtered
		permDeny = append(permDeny, dropped...)
		permSources = append(permSources, ruleSource{"--tools (hidden tools)", nil, dropped})
		tools.SetTaskDisallowedTools(permDeny)
	}

	// --- Session persistence (#82) ---
	// --continue/-c resumes the most recent session for this working directory.
	sessionID := codeResume
	if sessionID == "" && codeContinue {
		if infos, err := session.List(workDir); err == nil && len(infos) > 0 {
			sessionID = infos[0].ID
		}
	}
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// Create the worktree now that we have a stable sessionID.
	if codeWorktree && gitRootPath != "" {
		var err error
		worktreePath, err = worktree.Create(gitRootPath, sessionID)
		if err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
		if err := os.Chdir(worktreePath); err != nil {
			_ = worktree.Remove(gitRootPath, worktreePath, true)
			return fmt.Errorf("chdir to worktree: %w", err)
		}
		// Recompute workDir — everything from here on uses the worktree path.
		workDir, _ = os.Getwd()
		p.Info(fmt.Sprintf("Isolated worktree: %s", worktreePath))
	}

	sessPath, _ := session.Path(workDir, sessionID)

	// --- Audit logging (#81) ---
	auditLog, _ := audit.NewLogger(sessionID)
	defer auditLog.Close()
	auditLog.LogSessionStart(model, workDir, Version)

	// --- Hook runner (#80) ---
	hooksDir := hooks.FindHooksDir(".")
	hookRunner := hooks.New(hooksDir, sessionID, workDir)

	// Tools approved via "always allow for this session" (the 'a' key in the
	// approval dialog) — exact tool+args match, session-scoped only by
	// default. If the project opted into permissions.persist_always_allow,
	// also persist approvals to .bai/settings.local.yaml (#331) so they
	// survive future sessions once the workspace is trusted.
	sessionAllow := newSessionAllowSet()
	if projCfg != nil && projCfg.Permissions.PersistAlwaysAllow {
		sessionAllow.persistPath = localCfgPath
		projectRoot := filepath.Dir(filepath.Dir(localCfgPath))
		if ensureGitignoreEntry(filepath.Join(projectRoot, ".gitignore"), ".bai/settings.local.yaml") {
			fmt.Println("[bai] added .bai/settings.local.yaml to .gitignore")
		}
	}

	// --- History: context + memory + optional resume (#82, #144, #273) ---
	history := buildContextHistory(".")
	if ctx := hookRunner.SessionStart(model, Version); ctx != "" {
		history = append(history, codeMessage{Role: "system", Content: ctx})
	}
	if codeResume != "" || codeContinue {
		if msgs, err := session.Load(sessPath); err == nil {
			for _, m := range msgs {
				history = append(history, codeMessage{
					Role:       m.Role,
					Content:    m.Content,
					ToolCallID: m.ToolCallID,
				})
			}
		}
	}

	// --- `bai chat retry <id>` (#289) ---
	// Derive the prompt from the last turn in the resumed session instead of
	// requiring one on the command line, and drop that turn from history so
	// the replayed response replaces it rather than piling up as a 2nd attempt.
	if codeRetryMode {
		idx := lastUserIndex(history)
		if idx < 0 {
			return fmt.Errorf("no previous turn found to retry in session %s", sessionID)
		}
		initialPrompt = stripPromptContext(history[idx].Content)
		history = history[:idx]
	}

	// --- Headless print mode (#77) ---
	if codePrint || !isTerminal() {
		if err := checkPrintFormats(codeInputFormat, codeOutputFormat, initialPrompt); err != nil {
			return err
		}
		if initialPrompt == "" && codeInputFormat != "stream-json" {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			initialPrompt = strings.TrimSpace(string(b))
		}
		if initialPrompt == "" && codeInputFormat != "stream-json" {
			return fmt.Errorf("prompt required in --print mode (pass as argument or pipe via stdin)")
		}
		return runCodePrint(conn, cfg, sessionID, model, toolSchemas, initialPrompt, history,
			codeMaxTurns, maxContextTokens, maxBudgetUSD, permAllow, permDeny, codeAutoApply, codePlanMode, sessionAllow, codeOutputFormat, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p)
	}

	// --- Interactive TUI mode ---
	// Use sessionID as the chatID so all turns of the same session share one
	// identifier on the server. Previously a new UUID was generated per turn,
	// making server-side grouping impossible (#183).
	chatID := sessionID

	// autoApplyState and toolSchemasState are shared between the TUI (/auto,
	// /chat, /code toggles) and the submit closure, so changes take effect on
	// the very next turn.
	autoApplyState := codeAutoApply
	planModeState := codePlanMode
	toolSchemasState := toolSchemas
	// Interactive sessions have no step limit unless one was asked for
	// (--max-turns or the project's max_turns), as in Claude Code: the user
	// is watching and can interrupt. --print keeps the default as a guard.
	maxTurnsState := interactiveMaxTurns(codeMaxTurns, maxTurnsRequested)
	turns := 0
	isFirstTurn := true
	var turnMu sync.Mutex
	// File checkpoints for /rewind (Esc Esc).
	checkpoints := checkpoint.New()
	sessionCheckpoints = checkpoints
	defer func() { sessionCheckpoints = nil }()
	// pendingShell holds "!" commands the user ran since the last message,
	// with their output; they go to the model ahead of the next message.
	var (
		pendingMu    sync.Mutex
		pendingShell []string
	)

	submitFn := func(cid, mdl, input string, isNew bool) <-chan tui.StreamEvent {
		if conn.TS.NearExpiry(2 * time.Minute) {
			if err := conn.TS.EnsureValidToken(); err != nil {
				if authErr := reAuthenticate(cfg, p); authErr != nil {
					ch := make(chan tui.StreamEvent, 1)
					ch <- tui.StreamEvent{Kind: "error", ErrMsg: "auth: " + authErr.Error()}
					close(ch)
					return ch
				}
			}
		}

		ch := make(chan tui.StreamEvent, 64)
		currentAutoApply := autoApplyState // snapshot at turn start
		currentPlanMode := planModeState   // snapshot at turn start
		currentSchemas := toolSchemasState // snapshot (empty = chat mode)

		go func() {
			defer close(ch)
			// Held for the whole turn so /compact can't rewrite history
			// under a running turn.
			turnMu.Lock()
			defer turnMu.Unlock()
			// user-prompt-submit hooks (#407) run here, off the TUI's update
			// loop, since a script may take up to the hook timeout.
			userMsg, blocked := applyPromptHooks(hookRunner, input)
			if blocked != "" {
				ch <- tui.StreamEvent{Kind: "error", ErrMsg: blocked}
				ch <- tui.StreamEvent{Kind: "done"}
				return
			}
			userMsg = expandMentions(userMsg)
			pendingMu.Lock()
			if len(pendingShell) > 0 {
				userMsg = strings.Join(pendingShell, "\n\n") + "\n\n" + userMsg
				pendingShell = nil
			}
			pendingMu.Unlock()
			checkpoints.Begin(input, userMsg, len(history))
			history = append(history, codeMessage{Role: "user", Content: userMsg})
			newHistory, loopErr := agenticLoopTUI(
				conn, cfg, cid, mdl, currentSchemas,
				history, isFirstTurn && isNew, permAllow, permDeny, currentAutoApply, currentPlanMode, sessionAllow,
				maxTurnsState, maxContextTokens, maxBudgetUSD, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch,
			)
			history = newHistory
			turns++
			isFirstTurn = false
			// Persist session after every turn (#82).
			session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
			if loopErr != nil {
				if !caigrpc.IsAuthError(loopErr) {
					ch <- tui.StreamEvent{Kind: "error", ErrMsg: ui.RewriteError(loopErr)}
				}
			}
			ch <- tui.StreamEvent{Kind: "done"}
		}()

		return ch
	}

	// retryFn resends the last user turn (#289): drop it (and everything the
	// agentic loop produced after it) from history, then call submitFn again
	// with that same prompt text. submitFn independently re-appends the user
	// message, so the net effect is "replace the last turn," not "append a
	// duplicate." modelOverride falls back to the session's current model.
	retryFn := func(modelOverride string) (<-chan tui.StreamEvent, string, error) {
		idx := lastUserIndex(history)
		if idx < 0 {
			return nil, "", fmt.Errorf("no previous turn to retry")
		}
		prompt := stripPromptContext(history[idx].Content)
		history = history[:idx]
		useModel := model
		if modelOverride != "" {
			useModel = modelOverride
		}
		return submitFn(chatID, useModel, prompt, false), prompt, nil
	}

	compactCmd := compactCommand(&turnMu, func(focus string) (string, error) {
		before := estimateTokens(history)
		if lastUserIndex(history) < 0 {
			return "Nothing to compact yet.", nil
		}
		compacted, err := compactHistory(conn, chatID, model, history, focus, 0)
		if err != nil {
			return "", err
		}
		history = compacted
		session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
		return fmt.Sprintf("Compacted the conversation into a summary: ~%s → ~%s tokens.",
			humanTokens(before), humanTokens(estimateTokens(history))), nil
	})

	tuiCfg := tui.SessionConfig{
		ChatID:        chatID,
		Model:         model,
		IsCode:        true,
		WorkDir:       workDir,
		Version:       formatVersion(Version),
		AutoApply:     codeAutoApply,
		PlanMode:      codePlanMode,
		InitialPrompt: initialPrompt,
		RepoName:      gitRepoName(),
		// First wins on a name clash: the user's .bai/commands override
		// the built-in commands.
		CustomCommands: mergeSlashCommands(loadCustomSlashCommands("."), sessionSlashCommands, promptCommands(),
			[]tui.SlashCommand{permissionsCommand(permSources, sessionAllow), agentsCommand(), todosCommand(), skillsCommand(), compactCmd},
			skillCommands(tools.Skills())),
		RetryFn: retryFn,
		MCPListFn: func() ([]tui.MCPInfo, error) {
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()
			resp, err := conn.Client.GetMcpInfo(ctx, &pb.GetMcpInfoRequest{})
			if err != nil {
				return nil, err
			}
			var items []tui.MCPInfo
			for _, s := range resp.GetMcpServers() {
				items = append(items, tui.MCPInfo{
					Name: s.GetName(), Type: s.GetType(), Available: s.GetIsAvailable(),
					Description: s.GetShortDescription(),
					Included:    sessionAgents.isIncluded(s.GetName()), Off: sessionAgents.isOff(s.GetName()),
				})
			}
			return items, nil
		},
		MCPIncludeFn: sessionAgents.setOn,
		FilesFn:      mentionCandidates,
		RewindPointsFn: func() []tui.RewindPoint {
			locked := turnMu.TryLock()
			if locked {
				defer turnMu.Unlock()
			}
			var out []tui.RewindPoint
			for _, p := range checkpoints.Points() {
				out = append(out, tui.RewindPoint{Turn: p.Turn, Prompt: p.Prompt, At: p.At, Files: p.Files,
					CanChat: locked && conversationAt(history, p)})
			}
			return out
		},
		RewindFn: func(turn int, code, chat bool) (string, error) {
			if !turnMu.TryLock() {
				return "", errTurnRunning
			}
			defer turnMu.Unlock()
			return rewind(checkpoints, &history, turn, code, chat, func() {
				session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
			})
		},
		ShellFn: func(command string) (string, error) {
			out, err := tools.Bash(command)
			if err != nil {
				return "", err
			}
			out = tools.CapOutput(out, tools.MaxOutputChars)
			pendingMu.Lock()
			pendingShell = append(pendingShell, shellContext(command, out))
			pendingMu.Unlock()
			return out, nil
		},
		StatusFn: func() []string {
			ctxLine := "Context:    a turn is running"
			if turnMu.TryLock() {
				ctxLine = fmt.Sprintf("Context:    ~%s tokens (compacts at ~%s; /compact to do it now)",
					humanTokens(estimateTokens(history)), humanTokens(maxContextTokens))
				turnMu.Unlock()
			}
			lines := statusLines(ctxLine, permAllow, permDeny, mcpMgr.Status())
			if on := sessionAgents.active(); len(on) > 0 {
				lines = append(lines, "Included:   "+strings.Join(on, ", ")+" (in chat mode: /chat or --no-tools)")
			}
			return lines
		},
		DisableUpdateCheck: !cfg.UpdateCheckEnabled(),
		AccountFn: func() (*tui.AccountInfo, error) {
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()
			resp, err := conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
			if err != nil {
				return nil, err
			}
			return &tui.AccountInfo{
				Name:     resp.GetName(),
				Email:    resp.GetEmail(),
				Username: resp.GetPreferredUsername(),
			}, nil
		},
		UsageFn: func() (*tui.UsageInfo, error) {
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()
			resp, err := conn.Client.QueryRateLimit(ctx, &pb.QueryRateLimitRequest{})
			if err != nil {
				return nil, err
			}
			info := &tui.UsageInfo{}
			if s := resp.GetUserStats(); s != nil {
				info.PlanType = s.GetPlanType()
				info.RPMUsed = s.GetRpmUsed()
				info.RPMLimit = s.GetRpmLimit()
				info.RPMPercent = s.GetRpmPercentage()
				info.DailyPercent = s.GetDailyPercentage()
				info.MonthlyPercent = s.GetMonthlyPercentage()
			}
			if u := resp.GetTokenUsage(); u != nil {
				info.InputTokens = int64(u.GetInputTokens())
				info.OutputTokens = int64(u.GetOutputTokens())
				info.TotalTokens = int64(u.GetTotalTokens())
			}
			if resetAt := resp.GetResetAt(); resetAt > 0 {
				if secs := int(time.Until(time.Unix(resetAt, 0)).Seconds()); secs > 0 {
					info.RetryAfter = secs
				}
			}
			return info, nil
		},
		ListModelsFn: func() ([]tui.ModelInfo, error) {
			ctx, cancel := caigrpc.ContextWithTimeout()
			defer cancel()
			resp, err := conn.Client.GetLLMModels(ctx, &pb.GetLLMModelsRequest{})
			if err != nil {
				return nil, err
			}
			items := make([]tui.ModelInfo, 0, len(resp.GetModels()))
			for _, mdl := range resp.GetModels() {
				items = append(items, tui.ModelInfo{
					Name:    mdl.GetName(),
					OwnedBy: mdl.GetOwnedBy(),
				})
			}
			return items, nil
		},
		SetAutoApplyFn: func(enabled bool) {
			autoApplyState = enabled
		},
		MaxTurns: maxTurnsState,
		SetMaxTurnsFn: func(n int) {
			maxTurnsState = n
		},
		SetPlanModeFn: func(enabled bool) {
			planModeState = enabled
		},
		SetAcceptEditsFn: sessionAllow.setAcceptEdits,
		SetCodeModeFn: func(enabled bool) {
			if enabled {
				toolSchemasState = toolSchemas // restore full tool schemas
			} else {
				toolSchemasState = "" // clear tools → chat mode
			}
		},
	}
	m := tui.New(tuiCfg, submitFn)
	err = tui.Run(m)
	auditLog.LogSessionEnd(turns, "end_turn")
	hookRunner.SessionEnd(turns, "end_turn")

	// --- Worktree teardown (#126) ---
	if worktreePath != "" {
		if wtErr := handleWorktreeEnd(worktreePath, gitRootPath, p); wtErr != nil {
			fmt.Fprintf(os.Stderr, "worktree cleanup: %v\n", wtErr)
		}
	}

	return err
}

// handleWorktreeEnd shows changed files and prompts the user to apply, discard,
// or keep the isolated worktree created by --worktree.
func handleWorktreeEnd(worktreePath, gitRootPath string, p *ui.Printer) error {
	changed, _ := worktree.ChangedFiles(worktreePath)

	if len(changed) == 0 {
		p.Info("No changes in worktree — cleaning up.")
		return worktree.Remove(gitRootPath, worktreePath, true)
	}

	fmt.Printf("\n%d file(s) changed in worktree:\n", len(changed))
	for _, f := range changed {
		fmt.Printf("  %s\n", f)
	}
	fmt.Printf("\n[A]pply to main tree  [D]iscard worktree  [K]eep worktree\n> ")

	var choice string
	_, _ = fmt.Scanln(&choice)
	choice = strings.ToLower(strings.TrimSpace(choice))

	switch {
	case strings.HasPrefix(choice, "a"):
		if err := worktree.Apply(worktreePath, gitRootPath); err != nil {
			return fmt.Errorf("apply changes: %w", err)
		}
		_ = worktree.Remove(gitRootPath, worktreePath, true)
		p.Success("Changes applied to main tree.")
	case strings.HasPrefix(choice, "k"):
		p.Info(fmt.Sprintf("Worktree kept at: %s", worktreePath))
	default:
		_ = worktree.Remove(gitRootPath, worktreePath, true)
		p.Info("Worktree discarded. Main tree unchanged.")
	}
	return nil
}

// runCodePrint runs the agentic loop in headless mode, writing output to stdout.
func runCodePrint(
	conn *caigrpc.Conn,
	cfg *config.Config,
	chatID, model, toolSchemas, prompt string,
	history []codeMessage,
	maxTurns int,
	maxContextTokens int,
	maxBudgetUSD float64,
	allow, deny []string,
	autoApply bool,
	planMode bool,
	sessionAllow *sessionAllowSet,
	outputFormat string,
	sessPath string,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
) error {
	schema, err := loadJSONSchema(codeJSONSchema)
	if err != nil {
		return err
	}
	// loop runs the agentic loop for history, the last message of which is
	// the user's.
	first := true
	loop := func(h []codeMessage, ch chan<- tui.StreamEvent) ([]codeMessage, error) {
		isNew := first
		first = false
		return agenticLoopTUI(
			conn, cfg, chatID, model, toolSchemas,
			h, isNew, allow, deny, autoApply, planMode, sessionAllow,
			maxTurns, maxContextTokens, maxBudgetUSD, sessPath, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch,
		)
	}

	turns, failed := 0, false
	// turn runs one user message and prints its output, ending with a
	// result line (or object).
	turn := func(prompt string) {
		var structured any
		ch := make(chan tui.StreamEvent, 128)
		go func() {
			defer close(ch)
			userMsg, blocked := applyPromptHooks(hookRunner, prompt)
			if blocked != "" {
				ch <- tui.StreamEvent{Kind: "error", ErrMsg: blocked}
				ch <- tui.StreamEvent{Kind: "done"}
				return
			}
			turns++
			var loopErr error
			history, structured, loopErr = runPrintTurn(history, userMsg, schema, ch, loop)
			session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
			if loopErr != nil {
				ch <- tui.StreamEvent{Kind: "error", ErrMsg: ui.RewriteError(loopErr)}
			}
			ch <- tui.StreamEvent{Kind: "done"}
		}()
		var result func() (any, bool)
		if schema != nil {
			result = func() (any, bool) { return structured, structured != nil }
		}
		if drainPrintStreamWith(ch, outputFormat, os.Stdout, os.Stderr, result) != 0 {
			failed = true
		}
	}

	if codeInputFormat == "stream-json" {
		readStreamJSONInput(os.Stdin, os.Stdout, turn)
	} else {
		turn(prompt)
	}

	stopReason := "end_turn"
	if failed {
		stopReason = "error"
	}
	auditLog.LogSessionEnd(turns, stopReason)
	hookRunner.SessionEnd(turns, stopReason)
	if failed {
		os.Exit(1)
	}
	return nil
}

// Hook context is appended to the user turn inside these tags, the same
// framing Claude Code uses for UserPromptSubmit output, so the model can tell
// it apart from what the user typed and /retry can strip it back off.
const (
	promptHookOpen  = "\n\n<user-prompt-submit-hook>\n"
	promptHookClose = "\n</user-prompt-submit-hook>"
)

// applyPromptHooks runs user-prompt-submit hooks (#407) for prompt. It returns
// the user message to send (prompt plus any hook context) or, when a hook
// blocked the prompt, a non-empty reason to show instead of sending it.
func applyPromptHooks(r *hooks.Runner, prompt string) (userMsg, blocked string) {
	res := r.UserPromptSubmit(prompt)
	if res.Block {
		return "", "Prompt blocked by user-prompt-submit hook: " + res.SystemMessage
	}
	if res.AdditionalContext == "" {
		return prompt, ""
	}
	return prompt + promptHookOpen + res.AdditionalContext + promptHookClose, ""
}

// stripPromptContext removes hook context added by applyPromptHooks, giving
// back the text the user typed — so a retry re-runs the hooks on the original
// prompt instead of stacking a second copy of the context.
func stripPromptContext(msg string) string {
	if !strings.HasSuffix(msg, promptHookClose) {
		return msg
	}
	if i := strings.LastIndex(msg, promptHookOpen); i >= 0 {
		return msg[:i]
	}
	return msg
}

// drainPrintStream reads StreamEvents from ch and writes formatted output.
// Returns 0 on success, 1 if any error event was received.
// Extracted from runCodePrint so the formatting logic can be unit-tested
// independently of the agentic loop and gRPC stack.
func drainPrintStream(ch <-chan tui.StreamEvent, outputFormat string, w, errW io.Writer) int {
	return drainPrintStreamWith(ch, outputFormat, w, errW, nil)
}

// drainPrintStreamWith is drainPrintStream for a --json-schema run: result
// returns the validated answer when the turn is done, and it is reported as
// structured_output (json, stream-json) or printed alone (text, where the
// model's own text is not).
func drainPrintStreamWith(ch <-chan tui.StreamEvent, outputFormat string, w, errW io.Writer, resultFn func() (any, bool)) int {
	var textBuf strings.Builder
	exitCode := 0

	for ev := range ch {
		switch ev.Kind {
		case "chunk":
			if resultFn != nil && outputFormat == "text" {
				continue // only the validated JSON is printed
			}
			switch outputFormat {
			case "stream-json":
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "text", "text": ev.Chunk}) //nolint:errcheck
			case "json":
				textBuf.WriteString(ev.Chunk)
			default:
				fmt.Fprint(w, ev.Chunk)
			}
		case "tool_call":
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "tool_use", "name": ev.ToolName, "input": ev.ToolArgs}) //nolint:errcheck
			}
		case "tool_exec":
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "tool_result", "name": ev.ToolName, "status": ev.Status, "duration_ms": ev.DurationMs}) //nolint:errcheck
			}
		case "approval":
			// Headless mode has nobody to ask: decline, like Claude Code's -p,
			// instead of blocking the agentic loop forever on the reply (#406).
			// Tools that should run unattended need --auto or --allowed-tools.
			if ev.ReplyCh != nil {
				ev.ReplyCh <- tui.ApprovalDecision{Approved: false}
			}
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "permission_denied", "name": ev.ToolName, "input": ev.ToolArgs}) //nolint:errcheck
			} else {
				fmt.Fprintf(errW, "denied: %s needs approval (allow it with --allowed-tools or --auto)\n", ev.ToolName)
			}
		case "error":
			exitCode = 1
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "error", "error": ev.ErrMsg}) //nolint:errcheck
			} else {
				fmt.Fprintln(errW, "error:", ev.ErrMsg)
			}
		case "rate_limited":
			exitCode = 1
			if outputFormat == "stream-json" {
				enc := json.NewEncoder(w)
				enc.Encode(map[string]any{"type": "error", "error": ev.ErrMsg}) //nolint:errcheck
			} else {
				fmt.Fprintln(errW, "error:", ev.ErrMsg)
			}
		case "done":
			result, structured := map[string]any{"type": "result", "stop_reason": "end_turn"}, any(nil)
			if resultFn != nil {
				if v, ok := resultFn(); ok {
					result["structured_output"], structured = v, v
				} else {
					result["stop_reason"] = "error"
				}
			}
			if resultFn != nil && outputFormat == "text" {
				if structured != nil {
					out, _ := json.MarshalIndent(structured, "", "  ")
					fmt.Fprintln(w, string(out))
				}
				continue
			}
			switch outputFormat {
			case "stream-json":
				enc := json.NewEncoder(w)
				enc.Encode(result) //nolint:errcheck
			case "json":
				result["text"] = textBuf.String()
				enc := json.NewEncoder(w)
				enc.Encode([]map[string]any{result}) //nolint:errcheck
			}
		}
	}

	return exitCode
}

// isTerminal returns true when stdout is connected to an interactive terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// toSessionMsgs converts internal codeMessages to the exported session.Message type.
func toSessionMsgs(msgs []codeMessage) []session.Message {
	out := make([]session.Message, len(msgs))
	for i, m := range msgs {
		out[i] = session.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
	}
	return out
}

const (
	rateLimitInitialDelay = 10 * time.Second
	rateLimitMaxDelay     = 5 * time.Minute
	rateLimitMaxRetries   = 3
	maxContextRetries     = 2 // max drop-and-retry attempts for context-window errors
)

// isContextWindowError reports whether err is a context-length / token-limit
// rejection arriving as a stream error event (not a gRPC status code).
// Different providers use different phrasing, so we match on loose keywords.
// Routing failures are explicitly excluded: a wrong model name won't improve
// with a smaller history, so we must not retry those.
func isContextWindowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "routing failed") {
		return false
	}
	if strings.Contains(msg, "413") {
		return true
	}
	// Explicit context/token keywords used by OpenAI, Anthropic, etc.
	if (strings.Contains(msg, "context") || strings.Contains(msg, "token")) &&
		(strings.Contains(msg, "length") || strings.Contains(msg, "window") ||
			strings.Contains(msg, "limit") || strings.Contains(msg, "exceed")) {
		return true
	}
	// Bare "LLM error" is the generic phrase Groq (and some other providers)
	// emit when the upstream model rejects a request due to context overflow.
	return msg == "llm error"
}

// agenticLoopTUI runs the agentic loop for one user turn, sending tool events
// and content chunks to ch for TUI rendering. It handles approval prompts by
// sending StreamEvent{Kind:"approval"} and blocking on the reply channel.
func agenticLoopTUI(
	conn *caigrpc.Conn,
	cfg *config.Config,
	chatID, model, toolSchemas string,
	history []codeMessage,
	isFirstTurn bool,
	allow, deny []string,
	autoApply bool,
	planMode bool,
	sessionAllow *sessionAllowSet,
	maxTurns int,
	maxContextTokens int,
	maxBudgetUSD float64,
	sessPath string,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) ([]codeMessage, error) {
	if maxContextTokens <= 0 {
		maxContextTokens = defaultCompactionThreshold
	}

	// Generate a chat title from the first user prompt on new sessions (#182).
	// Fire-and-forget: title generation failure is silently ignored.
	if isFirstTurn {
		for _, m := range history {
			if m.Role == "user" && m.Content != "" {
				prompt := m.Content
				if len(prompt) > 200 {
					prompt = prompt[:200]
				}
				go func(p string) {
					tCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_, _ = conn.Client.GenerateTitle(tCtx, &pb.GenerateTitleRequest{ChatId: chatID, Prompt: p})
				}(prompt)
				break
			}
		}
	}

	rateLimitRetries := 0
	contextRetries := 0          // drop-and-retry count for context-window errors
	var lastPromptTokens int32   // prompt token count from the most recent iteration
	var estimatedCostUSD float64 // cumulative estimated cost for this session turn

	// maxTurns <= 0 means no limit.
	for iteration := 0; maxTurns <= 0 || iteration < maxTurns; iteration++ {
		// Compact context when the client-side estimate or server-reported token count
		// exceeds the configured limit. Client-side estimate fires even on iteration 0
		// (e.g. resumed history already large). Server count is a secondary signal.
		estTokens := estimateTokens(history)
		if estTokens >= maxContextTokens || (iteration > 0 && lastPromptTokens > int32(maxContextTokens)) {
			if preCompact := hookRunner.PreCompact(estTokens); preCompact.Block {
				msg := preCompact.SystemMessage
				if msg == "" {
					msg = "compaction blocked by hook"
				}
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⏸ %s — proceeding without compaction this round.\n", msg)}
			} else {
				fmt.Fprintf(os.Stderr, "[bai] compacting context (est. ~%dk tokens → summary)…\n", estTokens/1000)
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf(
					"\n⚡ Context at ~%dk tokens — compacting history...\n",
					estTokens/1000,
				)}
				if compacted, compactErr := compactHistory(conn, chatID, model, history, "", 4); compactErr == nil {
					history = compacted
					lastPromptTokens = 0
					session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
					ch <- tui.StreamEvent{Kind: "chunk", Chunk: "✅ History compacted. Continuing...\n\n"}
				} else {
					fmt.Fprintf(os.Stderr, "[bai] context compaction failed, dropping oldest turns\n")
					ch <- tui.StreamEvent{Kind: "chunk", Chunk: "\n⚠ Compaction failed — dropping oldest turns...\n"}
					history = dropOldestTurns(history, maxContextTokens*4/5)
					lastPromptTokens = 0
					session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
					// If dropping still leaves us over the limit (e.g. large tool results
					// from GitHub issues exceed the model's context window), bail now with
					// a clear message rather than looping on every subsequent iteration.
					if estimateTokens(history) >= maxContextTokens {
						mdl := model
						if mdl == "" {
							mdl = "the current model"
						}
						return history, fmt.Errorf(
							"context too large for %s (est. %dk tokens) — "+
								"try a model with a larger context window or use --max-context-tokens",
							mdl, estimateTokens(history)/1000,
						)
					}
				}
			}
		}

		req := buildCodeRequest(chatID, model, toolSchemas, history, isFirstTurn && iteration == 0)
		if toolSchemas == "" {
			// Chat mode (/chat, --no-tools): the backend runs the user's
			// server-side agents, so the included ones go with every turn.
			// With local tools it uses those alone.
			req.McpServers = mcpServerRefs("", sessionAgents.active())
		}

		ctx, cancel := context.WithCancel(context.Background())
		stream, err := conn.Client.Chat(ctx, req)
		if err != nil {
			cancel()
			// Rate limit backoff (#83)
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				if rateLimitRetries >= rateLimitMaxRetries {
					return history, err
				}
				delay := rateLimitDelay(rateLimitRetries)
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (attempt %d/%d)...\n", delay.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)}
				time.Sleep(delay)
				rateLimitRetries++
				iteration-- // don't count this iteration
				continue
			}
			return history, err
		}
		rateLimitRetries = 0

		// Pump this iteration's stream into ch, collecting tool calls.
		toolCalls, text, usage, err := pumpCodeStream(stream, cancel, ch)
		if usage.PromptTokens > 0 {
			lastPromptTokens = usage.PromptTokens
		}
		if err != nil {
			// Rate limit backoff on stream errors too (#83)
			if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				if rateLimitRetries < rateLimitMaxRetries {
					delay := rateLimitDelay(rateLimitRetries)
					ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n⏳ Rate limited. Retrying in %s (attempt %d/%d)...\n", delay.Round(time.Second), rateLimitRetries+1, rateLimitMaxRetries)}
					time.Sleep(delay)
					rateLimitRetries++
					iteration--
					continue
				}
			}
			// Context-window / token-limit errors (HTTP 413, stream error events
			// containing "context length", "token limit", etc.). Different models
			// and providers phrase these differently, so we match on loose keywords.
			// Cap retries so a model with a very small window (e.g. gpt-oss-120b)
			// cannot loop indefinitely when dropping never helps.
			if isContextWindowError(err) {
				if contextRetries >= maxContextRetries {
					mdl := model
					if mdl == "" {
						mdl = "the current model"
					}
					return history, fmt.Errorf(
						"context window limit hit %d times for %s (est. %dk tokens) — "+
							"try a model with a larger context window or use --max-context-tokens (last error: %v)",
						maxContextRetries, mdl, estimateTokens(history)/1000, err,
					)
				}
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: "\n⚠ Context too large — dropping older messages and retrying...\n"}
				// Drop to half of the CURRENT history size, not the compaction
				// threshold, so each retry actually reduces content even when
				// the history is small (e.g. a single large web_fetch result).
				dropTarget := estimateTokens(history) / 2
				if dropTarget < 500 {
					dropTarget = 500
				}
				history = dropOldestTurns(history, dropTarget)
				lastPromptTokens = 0
				session.Save(sessPath, toSessionMsgs(history)) //nolint:errcheck
				contextRetries++
				iteration--
				continue
			}
			return history, err
		}
		rateLimitRetries = 0
		contextRetries = 0

		// Budget check: accumulate estimated cost and stop before the next iteration.
		if maxBudgetUSD > 0 {
			estimatedCostUSD += estimateIterationCost(model, usage, estimateTokens(history))
			if estimatedCostUSD >= maxBudgetUSD {
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf(
					"\n💰 Session budget limit $%.2f reached (est. $%.4f used). Stopping.\n",
					maxBudgetUSD, estimatedCostUSD,
				)}
				return history, fmt.Errorf("budget limit $%.2f reached (est. $%.4f used)", maxBudgetUSD, estimatedCostUSD)
			}
		}

		// Keep the assistant's text in history: without it, the next request
		// shows earlier questions as unanswered and the model goes back to them (#390).
		if len(toolCalls) == 0 {
			if text != "" {
				history = append(history, codeMessage{Role: "assistant", Content: text})
			}
			if stopRes := hookRunner.Stop(); stopRes.Block {
				msg := stopRes.SystemMessage
				if msg == "" {
					msg = "Continue."
				}
				ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n🔁 stop hook: %s\n", msg)}
				history = append(history, codeMessage{Role: "user", Content: msg})
				continue
			}
			return history, nil
		}

		// Build assistant tool-call turn in history
		assistantMsg := codeMessage{Role: "assistant", Content: text}
		for _, tc := range toolCalls {
			assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, codeToolCall{
				ID:       tc.ID,
				Type:     "function",
				Function: codeFuncCall{Name: tc.Name, Arguments: tc.Arguments},
			})
		}
		history = append(history, assistantMsg)

		// Execute tools — run concurrently when all can be auto-approved,
		// fall back to sequential when any requires a TUI approval prompt.
		toolResults := executeTools(toolCalls, allow, deny, autoApply, planMode, sessionAllow, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
		for i, tc := range toolCalls {
			r := toolResults[i]
			history = append(history, codeMessage{
				Role:       "tool",
				Content:    r.result,
				ToolCallID: tc.ID,
			})
			// Failures were already reported by executeWithApprovalTUI: a
			// second report would mark the next call of the tool as failed.
		}
	}

	// Not an error: the work so far is in history, so the user can resume.
	// The notice is rendered as Markdown, which would drop a "<n>"
	// placeholder as an HTML tag: suggest a concrete value in a code span.
	hint := fmt.Sprintf("Type `continue` to keep going, or `/max-turns %d` to raise the limit for this session.", maxTurns*2)
	if codePrint {
		hint = "Rerun with --max-turns N (or --continue) to keep going."
	}
	ch <- tui.StreamEvent{Kind: "chunk", Chunk: fmt.Sprintf("\n\n⚠  Paused after %d steps (max-turns limit). %s\n", maxTurns, hint)}
	return history, nil
}

// rateLimitDelay returns exponential backoff delay for the given retry index.
func rateLimitDelay(retry int) time.Duration {
	d := rateLimitInitialDelay * (1 << retry)
	if d > rateLimitMaxDelay {
		d = rateLimitMaxDelay
	}
	return d
}

// iterationUsage holds token counts reported by the backend for one agentic iteration.
type iterationUsage struct {
	PromptTokens     int32
	CompletionTokens int32
}

// pumpCodeStream reads a gRPC stream for one agentic iteration, forwarding
// content/tool events to ch. It returns the tool calls and the assistant text
// it forwarded, so the caller can keep both in history.
func pumpCodeStream(
	stream interface {
		Recv() (*pb.ChatEvent, error)
	},
	cancelFn context.CancelFunc,
	ch chan<- tui.StreamEvent,
) ([]ui.ToolCallEvent, string, iterationUsage, error) {
	defer cancelFn()
	tf := &tui.ExportedThinkFilter{}
	var toolCalls []ui.ToolCallEvent
	var text strings.Builder
	chunk := func(s string) {
		text.WriteString(s)
		ch <- tui.StreamEvent{Kind: "chunk", Chunk: s}
	}

	for {
		ev, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				if tail := tf.Flush(); tail != "" {
					chunk(tail)
				}
				return toolCalls, text.String(), iterationUsage{}, nil
			}
			return toolCalls, text.String(), iterationUsage{}, fmt.Errorf("stream recv: %w", err)
		}

		switch ev.GetType() {
		case "content", "stream_chunk":
			filtered := tf.Filter(ev.GetContent())
			// Surface <think> text as its own event so the TUI can show a live,
			// collapsible "Thinking…" line instead of a silent spinner. Not
			// added to text: it never becomes part of the conversation history.
			if th := tf.TakeThinking(); th != "" {
				ch <- tui.StreamEvent{Kind: "thinking", Chunk: th}
			}
			if filtered != "" {
				chunk(filtered)
			}

		case "done", "stream_end":
			if tail := tf.Flush(); tail != "" {
				chunk(tail)
			}
			return toolCalls, text.String(), iterationUsage{
				PromptTokens:     ev.GetUsagePromptTokens(),
				CompletionTokens: ev.GetUsageCompletionTokens(),
			}, nil

		case "usage_warning":
			// Show as a non-blocking system notice, not as inline assistant text.
			msg := ev.GetContent()
			if msg == "" {
				msg = ev.GetError()
			}
			if msg != "" {
				ch <- tui.StreamEvent{Kind: "usage_warning", ErrMsg: msg}
			}

		case "rate_limited":
			// Rate limit hit — forward as a styled system notice so the TUI can
			// show "⚠ You've reached your limit" instead of a blank response.
			msg := ev.GetContent()
			if msg == "" {
				msg = "You've reached your usage limit."
			}
			ch <- tui.StreamEvent{Kind: "rate_limited", ErrMsg: msg}

		case "live_usage_pct":
			// Live usage tick — forward so the header bar updates during streaming.
			content := ev.GetContent()
			if content != "" {
				parts := strings.SplitN(content, "|", 2)
				if pct, err := strconv.ParseFloat(parts[0], 64); err == nil {
					ch <- tui.StreamEvent{Kind: "live_usage_pct", LivePct: pct}
				}
			}

		case "error", "stream_error":
			return toolCalls, text.String(), iterationUsage{}, fmt.Errorf("%s", ev.GetError())

		case "tool_call":
			var tc ui.ToolCallEvent
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			if jsonErr := json.Unmarshal([]byte(data), &tc); jsonErr == nil {
				toolCalls = append(toolCalls, tc)
				ch <- tui.StreamEvent{
					Kind:     "tool_call",
					ToolID:   tc.ID,
					ToolName: tc.Name,
					ToolArgs: tc.Arguments,
				}
			}

		case "stream_progress":
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			var prog struct {
				Tools     []string `json:"tools"`
				Iteration int      `json:"iteration"`
			}
			if jsonErr := json.Unmarshal([]byte(data), &prog); jsonErr == nil {
				ch <- tui.StreamEvent{
					Kind:      "progress",
					Iteration: prog.Iteration,
					Tools:     prog.Tools,
				}
			}

		case "stream_tool_execution":
			data := ev.GetData()
			if data == "" {
				data = ev.GetContent()
			}
			var te struct {
				ToolName      string `json:"tool_name"`
				Status        string `json:"status"`
				DurationMs    int64  `json:"duration_ms"`
				ResultSummary string `json:"result_summary"`
			}
			if jsonErr := json.Unmarshal([]byte(data), &te); jsonErr == nil {
				ch <- tui.StreamEvent{
					Kind:       "tool_exec",
					ToolName:   te.ToolName,
					Status:     te.Status,
					DurationMs: te.DurationMs,
					Summary:    te.ResultSummary,
				}
			}

		case "stream_heartbeat":
			ch <- tui.StreamEvent{Kind: "heartbeat"}
		}
	}
}

// defaultCompactionThreshold is the default max context tokens before compaction.
// 100k is safely below the 128k floor across all supported models (claude-sonnet
// 200k, gpt-4.1 128k), leaving headroom for the compaction summary itself.
const defaultCompactionThreshold = 100_000

// estimateTokens counts characters in all message content and tool-call
// arguments and divides by 4 (standard chars-per-token approximation).
func estimateTokens(history []codeMessage) int {
	chars := 0
	for _, m := range history {
		chars += len(m.Content)
		for _, tc := range m.ToolCalls {
			chars += len(tc.Function.Arguments)
		}
	}
	return chars / 4
}

// modelPrice holds approximate per-million-token pricing for one model.
type modelPrice struct {
	inputPerMToken  float64 // USD per 1M prompt tokens
	outputPerMToken float64 // USD per 1M completion tokens
}

// modelPricing returns approximate pricing for a model name or alias.
// Prices are estimates as of mid-2026 and may drift — they are used only
// for session-level budget enforcement, not billing.
func modelPricing(model string) modelPrice {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return modelPrice{15.00, 75.00}
	case strings.Contains(m, "haiku"):
		return modelPrice{0.25, 1.25}
	case strings.Contains(m, "gpt-4o"):
		return modelPrice{2.50, 10.00}
	case strings.Contains(m, "gpt-4.1") || strings.Contains(m, "gpt-4"):
		return modelPrice{2.00, 8.00}
	case m == "fast": // Groq llama-class
		return modelPrice{0.06, 0.18}
	default: // claude-sonnet, auto, think, unknown — conservative estimate
		return modelPrice{3.00, 15.00}
	}
}

// estimateIterationCost computes the approximate USD cost of one agentic iteration.
// When the backend provides usage counts they are used directly; otherwise prompt
// tokens are estimated from the current history size and completion tokens are
// estimated at 15% of prompt (conservative for code tasks).
func estimateIterationCost(model string, usage iterationUsage, historyTokens int) float64 {
	pt := int(usage.PromptTokens)
	ct := int(usage.CompletionTokens)
	if pt == 0 {
		pt = historyTokens
	}
	if ct == 0 {
		ct = pt * 15 / 100
	}
	p := modelPricing(model)
	return float64(pt)/1_000_000*p.inputPerMToken + float64(ct)/1_000_000*p.outputPerMToken
}

// compactionSafeTokens is the maximum estimated tokens sent to the LLM for
// the summarisation request. Capping the compaction input prevents the
// compaction request itself from triggering a 413 on the server when the
// history has grown very large.
const compactionSafeTokens = 50_000

// compactHistory sends the full history to the LLM with a summarisation prompt
// (no tools, single turn) and returns a trimmed history containing only system
// messages, the summary, and the last four messages (two exchanges).
//
// focus adds the user's instructions for what the summary should keep
// (/compact <focus>). tail is how many recent assistant rounds to keep
// verbatim after the summary: auto-compaction mid-turn keeps a few so the
// turn can go on; a manual /compact keeps none, as in Claude Code.
func compactHistory(
	conn *caigrpc.Conn,
	chatID, model string,
	history []codeMessage,
	focus string,
	tail int,
) ([]codeMessage, error) {
	summaryPrompt := "Summarise the conversation so far in a single assistant message. " +
		"Include: the overall goal, key decisions, every file created or modified (with paths), " +
		"tool results that matter, and any open questions. Be concise but complete enough that " +
		"the session can continue without the original messages."
	if focus = strings.TrimSpace(focus); focus != "" {
		summaryPrompt += "\n\nThe user asked the summary to focus on: " + focus
	}

	// Pre-truncate so the compaction request itself stays below server size limits.
	// We keep the most recent messages (most context-relevant) and system messages.
	safeHistory := history
	if estimateTokens(history) > compactionSafeTokens {
		safeHistory = dropOldestTurns(history, compactionSafeTokens)
	}

	summaryHistory := append(append([]codeMessage(nil), safeHistory...),
		codeMessage{Role: "user", Content: summaryPrompt},
	)

	// No tool schemas — one-shot text response only.
	req := buildCodeRequest(chatID, model, "", summaryHistory, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := conn.Client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	tf := &tui.ExportedThinkFilter{}
	for {
		ev, recvErr := stream.Recv()
		if recvErr != nil {
			if recvErr != io.EOF {
				return nil, fmt.Errorf("compaction recv: %w", recvErr)
			}
			break
		}
		switch ev.GetType() {
		case "content", "stream_chunk":
			buf.WriteString(tf.Filter(ev.GetContent()))
		case "done", "stream_end":
			goto summarised
		case "error", "stream_error":
			return nil, fmt.Errorf("compaction: %s", ev.GetError())
		}
	}
summarised:
	summary := strings.TrimSpace(buf.String())
	if summary == "" {
		return history, nil // nothing to compact
	}

	// Collect system messages from the original history.
	compact := make([]codeMessage, 0, 8)
	for _, m := range history {
		if m.Role == "system" {
			compact = append(compact, m)
		}
	}

	// Add the summary as a system message (not assistant) so there is no
	// orphaned assistant turn before the first user message in the tail.
	// An assistant message with no preceding user turn violates the
	// alternating structure many LLM APIs enforce, causing "LLM error for a prompt".
	compact = append(compact, codeMessage{
		Role:    "system",
		Content: "[Conversation summary]\n" + summary,
	})
	if tail > 0 {
		compact = append(compact, recentTail(history, tail)...)
	}
	return compact, nil
}

// toolResult holds the outcome of one tool call.
type toolResult struct {
	result string
	err    error
}

// sessionAllowSet holds the session's "don't ask again" approvals (option 2
// in the permission dialog). Each approval is remembered at the broadest
// scope that is still safe to state plainly in the dialog:
//
//   - bash: the command prefix of each segment that needed approval ("gh api",
//     "python3"), matched segment by segment, so approving "gh api x" covers
//     later "gh api y" calls but never "gh api y; rm -rf ~";
//   - file-editing tools: all of them, like Claude Code's accept-edits mode
//     (paths are still confined to the workspace by the tools themselves);
//   - any other tool (MCP, plugin, task, memory): that tool.
//
// A bash command that can't be generalised safely (command substitution,
// file redirection, …) falls back to its exact text. A nil *sessionAllowSet
// is an always-empty set with a no-op add, so tests can pass nil.
type sessionAllowSet struct {
	mu           sync.Mutex
	exact        map[string]bool // toolName + "\x00" + argsJSON
	tools        map[string]bool // whole tools
	bashPrefixes []string
	// acceptEdits approves every file edit: Claude Code's accept-edits
	// mode, switched with Shift+Tab.
	acceptEdits bool

	// persistPath, when non-empty, opts this set into also writing approvals
	// to the local config at this path (#331) — set only when the project
	// enabled permissions.persist_always_allow and the workspace is trusted.
	persistPath string
}

// editTools are approved together by "allow all edits".
var editTools = []string{"write_file", "edit_file", "patch_file", "edit_notebook"}

func newSessionAllowSet() *sessionAllowSet {
	return &sessionAllowSet{exact: make(map[string]bool), tools: make(map[string]bool)}
}

// alwaysScope works out what a "don't ask again" approval of this call
// covers: the bash prefixes, the tools, and the label shown in the dialog.
// Empty prefixes and tools mean "this exact call".
func alwaysScope(toolName, argsJSON string) (prefixes, toolNames []string, label string) {
	switch {
	case toolName == "bash":
		prefixes = tools.SuggestBashPrefixes(tools.ExtractPrimaryArg("bash", argsJSON))
		if len(prefixes) == 0 {
			return nil, nil, ""
		}
		quoted := make([]string, len(prefixes))
		for i, p := range prefixes {
			quoted[i] = "`" + p + "`"
		}
		return prefixes, nil, strings.Join(quoted, ", ") + " commands"
	case slices.Contains(editTools, toolName):
		return nil, editTools, "file edits"
	default:
		return nil, []string{toolName}, toolName
	}
}

// describe lists the session's approvals as rules, for /permissions.
func (s *sessionAllowSet) describe() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, p := range s.bashPrefixes {
		out = append(out, "bash:"+p+" *")
	}
	for t := range s.tools {
		out = append(out, t)
	}
	for k := range s.exact {
		name, args, _ := strings.Cut(k, "\x00")
		if a := tools.ExtractPrimaryArg(name, args); a != "" {
			args = a
		}
		out = append(out, name+" "+args+" (this exact call)")
	}
	slices.Sort(out[len(s.bashPrefixes):])
	return out
}

func (s *sessionAllowSet) add(toolName, argsJSON string) {
	if s == nil {
		return
	}
	prefixes, toolNames, _ := alwaysScope(toolName, argsJSON)
	s.mu.Lock()
	switch {
	case len(prefixes) > 0:
		for _, p := range prefixes {
			if !slices.Contains(s.bashPrefixes, p) {
				s.bashPrefixes = append(s.bashPrefixes, p)
			}
		}
	case len(toolNames) > 0:
		for _, t := range toolNames {
			s.tools[t] = true
		}
	default:
		s.exact[toolName+"\x00"+argsJSON] = true
	}
	persistPath := s.persistPath
	s.mu.Unlock()
	if persistPath != "" {
		persistAlwaysAllow(persistPath, toolName, argsJSON, prefixes, toolNames)
	}
}

// persistAlwaysAllow writes the approval as permissions.allow rules in the
// local config at path: "bash:<prefix> *" per bash prefix (matched per
// segment by tools.CheckPermissions), a bare tool name per tool, or else a
// literal "<tool>:<arg>" rule. A literal argument containing '*' or '?' is
// skipped — the glob matcher has no escape syntax — and stays session-only.
func persistAlwaysAllow(path, toolName, argsJSON string, prefixes, toolNames []string) {
	switch {
	case len(prefixes) > 0:
		for _, p := range prefixes {
			_ = config.AddLocalAllowRule(path, "bash:"+p+" *")
		}
		return
	case len(toolNames) > 0:
		for _, t := range toolNames {
			_ = config.AddLocalAllowRule(path, t)
		}
		return
	}
	arg := tools.ExtractPrimaryArg(toolName, argsJSON)
	if strings.ContainsAny(arg, "*?") {
		return
	}
	pattern := toolName
	if arg != "" {
		pattern = toolName + ":" + arg
	}
	_ = config.AddLocalAllowRule(path, pattern)
}

// setAcceptEdits turns accept-edits mode on or off.
func (s *sessionAllowSet) setAcceptEdits(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acceptEdits = on
}

func (s *sessionAllowSet) contains(toolName, argsJSON string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exact[toolName+"\x00"+argsJSON] || s.tools[toolName] {
		return true
	}
	if s.acceptEdits && (slices.Contains(editTools, toolName) ||
		toolName == "bash" && tools.BashCreatesOnly(tools.ExtractPrimaryArg("bash", argsJSON))) {
		return true
	}
	return toolName == "bash" && len(s.bashPrefixes) > 0 &&
		tools.BashAllowedByPrefixes(tools.ExtractPrimaryArg("bash", argsJSON), s.bashPrefixes)
}

// executeTools runs toolCalls sequentially or concurrently depending on whether
// any call requires interactive TUI approval. When all tools can be auto-approved
// (read-only ops, safe bash prefixes, --auto-apply) they execute in parallel,
// preserving result order. Sequential fallback is used when any tool needs a
// user prompt, to avoid multiple concurrent approval dialogs.
func executeTools(
	toolCalls []ui.ToolCallEvent,
	allow, deny []string,
	autoApply bool,
	planMode bool,
	sessionAllow *sessionAllowSet,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) []toolResult {
	// Fast path: single tool — no parallelism overhead.
	if len(toolCalls) <= 1 {
		results := make([]toolResult, len(toolCalls))
		for i, tc := range toolCalls {
			r, e := executeWithApprovalTUI(tc, allow, deny, autoApply, planMode, sessionAllow, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
			results[i] = toolResult{result: r, err: e}
		}
		return results
	}

	// Check whether all tools can run without interactive approval.
	// Denied tools return immediately (no dialog); allow-matched tools skip the prompt.
	allAutoApproved := autoApply
	if !allAutoApproved {
		allAutoApproved = true
		for _, tc := range toolCalls {
			action := tools.CheckPermissions(allow, deny, tc.Name, tc.Arguments)
			if action == tools.PermitDeny || action == tools.PermitAuto || sessionAllow.contains(tc.Name, tc.Arguments) {
				continue
			}
			if plugins.IsPluginTool(tc.Name) && pluginMgr.ApprovalMode(tc.Name) == "never" {
				continue
			}
			needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name) || plugins.IsPluginTool(tc.Name)
			if needsApproval && (tc.Name != "bash" || !tools.IsSafeBashCommand(tc.Arguments)) {
				allAutoApproved = false
				break
			}
		}
	}

	if !allAutoApproved {
		// Sequential: approval dialogs must not overlap.
		results := make([]toolResult, len(toolCalls))
		for i, tc := range toolCalls {
			r, e := executeWithApprovalTUI(tc, allow, deny, autoApply, planMode, sessionAllow, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
			results[i] = toolResult{result: r, err: e}
		}
		return results
	}

	// Parallel: all tools are auto-approved — run concurrently, collect in order.
	results := make([]toolResult, len(toolCalls))
	var wg sync.WaitGroup
	for i, tc := range toolCalls {
		wg.Add(1)
		go func(idx int, t ui.ToolCallEvent) {
			defer wg.Done()
			r, e := executeWithApprovalTUI(t, allow, deny, true, planMode, sessionAllow, auditLog, hookRunner, mcpMgr, pluginMgr, p, ch)
			results[idx] = toolResult{result: r, err: e}
		}(i, tc)
	}
	wg.Wait()
	return results
}

// isPlanModeBlocked reports whether toolName should be blocked while plan mode
// is active. Plan mode is a hard policy, checked before hooks/permissions/
// --auto — the only way past it is to exit plan mode via /plan.
func isPlanModeBlocked(toolName, argsJSON string) bool {
	if tools.IsReadOnlyTool(toolName) {
		return false
	}
	if toolName == "bash" {
		return !tools.IsSafeBashCommand(argsJSON)
	}
	if mcp.IsMCPTool(toolName) {
		return !mcp.IsReadOnlyMCPTool(toolName)
	}
	return true // write_file, edit_file, patch_file, edit_notebook, task,
	// memory_write, memory_delete, plugin tools, unknown tools
}

// executeWithApprovalTUI runs a tool, running hooks, audit logging, and the
// TUI approval flow as needed. MCP tools (mcp__<server>__<tool>) are routed
// through mcpMgr; all other tools go through the local tools.Execute.
func executeWithApprovalTUI(
	tc ui.ToolCallEvent,
	allow, deny []string,
	autoApply bool,
	planMode bool,
	sessionAllow *sessionAllowSet,
	auditLog *audit.Logger,
	hookRunner *hooks.Runner,
	mcpMgr *mcp.Manager,
	pluginMgr *plugins.Manager,
	p *ui.Printer,
	ch chan<- tui.StreamEvent,
) (string, error) {
	if strings.TrimSpace(tc.Arguments) == "" {
		tc.Arguments = "{}" // no-argument call sent as "" by some models
	}

	// --- Plan mode (hard policy — checked before hooks/permissions/--auto) ---
	if planMode && isPlanModeBlocked(tc.Name, tc.Arguments) {
		msg := fmt.Sprintf("📝 Blocked by plan mode (read-only): %s — type /plan to allow changes.", tc.Name)
		auditLog.LogToolCall(tc.Name, tc.Arguments, false, false)
		auditLog.LogToolResult(tc.Name, 0, false)
		reportNotRun(ch, tc, "blocked by plan mode")
		return msg, nil
	}

	// --- Pre-tool hooks (#80) ---
	hookResult := hookRunner.PreToolUse(tc.Name, tc.Arguments)
	if hookResult.Block {
		msg := hookResult.SystemMessage
		if msg == "" {
			msg = "Tool blocked by pre-tool hook."
		}
		auditLog.LogToolCall(tc.Name, tc.Arguments, false, false)
		auditLog.LogToolResult(tc.Name, 0, false)
		reportNotRun(ch, tc, "blocked by pre-tool hook")
		return msg, nil
	}
	// Use modified input if hook rewrote it.
	argsJSON := tc.Arguments
	if hookResult.ModifiedInput != nil {
		if b, err := json.Marshal(hookResult.ModifiedInput); err == nil {
			argsJSON = string(b)
		}
	}

	// --- Permission policy (#123) ---
	// Evaluate project allow/deny lists before the TUI approval gate.
	// Deny-first: a matching deny rule blocks execution immediately.
	// Allow match: skip the NeedsApproval prompt for this call.
	effectiveAutoApply := autoApply
	switch tools.CheckPermissions(allow, deny, tc.Name, argsJSON) {
	case tools.PermitDeny:
		auditLog.LogToolCall(tc.Name, argsJSON, false, false)
		auditLog.LogToolResult(tc.Name, 0, false)
		reportNotRun(ch, tc, "blocked by permissions policy")
		return fmt.Sprintf("Tool call blocked by permissions policy: %s", tc.Name), nil
	case tools.PermitAuto:
		effectiveAutoApply = true
	}

	// --- Approval gate ---
	// MCP and plugin tools always require approval by default (they can do anything).
	needsApproval := tools.NeedsApproval(tc.Name) || mcp.IsMCPTool(tc.Name) || plugins.IsPluginTool(tc.Name)
	autoApproved := false
	// Safe bash commands are auto-approved to reduce friction.
	if needsApproval && tc.Name == "bash" && tools.IsSafeBashCommand(argsJSON) {
		needsApproval = false
		autoApproved = true
	}
	// Plugin-specific approval mode overrides the default and the --auto flag.
	if plugins.IsPluginTool(tc.Name) {
		switch pluginMgr.ApprovalMode(tc.Name) {
		case "always":
			needsApproval = true
			effectiveAutoApply = false // force dialog even with --auto or allow rule
		case "never":
			needsApproval = false
			autoApproved = true
		}
	}
	sessionApproved := sessionAllow.contains(tc.Name, argsJSON)
	approved := !needsApproval || effectiveAutoApply || sessionApproved
	if needsApproval && !effectiveAutoApply && !sessionApproved {
		replyCh := make(chan tui.ApprovalDecision, 1)
		_, _, alwaysLabel := alwaysScope(tc.Name, argsJSON)
		ch <- tui.StreamEvent{
			Kind:        "approval",
			ToolID:      tc.ID,
			ToolName:    tc.Name,
			ToolArgs:    argsJSON,
			ReplyCh:     replyCh,
			AlwaysLabel: alwaysLabel,
		}
		decision := <-replyCh
		approved = decision.Approved
		if approved && decision.AlwaysAllow {
			sessionAllow.add(tc.Name, argsJSON)
		}
		if !approved {
			auditLog.LogToolCall(tc.Name, argsJSON, false, false)
			auditLog.LogToolResult(tc.Name, 0, false)
			if fb := strings.TrimSpace(decision.Feedback); fb != "" {
				reportNotRun(ch, tc, "declined")
				return "User declined to execute this tool and said what to do instead: " + fb, nil
			}
			reportNotRun(ch, tc, "declined")
			return "User declined to execute this tool.", nil
		}
	}

	auditLog.LogToolCall(tc.Name, argsJSON, approved, autoApproved)

	// --- Execute ---
	saveBeforeEdit(tc.Name, argsJSON)
	// The row turns from queued to running (several calls from one round
	// run one at a time).
	ch <- tui.StreamEvent{Kind: "tool_start", ToolID: tc.ID, ToolName: tc.Name}
	start := time.Now()
	var result string
	var execErr error
	switch {
	case mcp.IsMCPTool(tc.Name):
		result, execErr = mcpMgr.Execute(context.Background(), tc.Name, argsJSON)
	case plugins.IsPluginTool(tc.Name):
		result, execErr = pluginMgr.Execute(context.Background(), tc.Name, argsJSON)
	default:
		// A task call streams its subagent's activity under its row.
		ctx := tools.WithProgress(context.Background(), func(line string) {
			ch <- tui.StreamEvent{Kind: "tool_progress", ToolID: tc.ID, ToolName: tc.Name, Summary: line}
		})
		result, execErr = tools.ExecuteContext(ctx, tc.Name, argsJSON)
	}
	elapsed := time.Since(start)
	auditLog.LogToolResult(tc.Name, elapsed.Milliseconds(), execErr == nil)

	// --- Post-tool hooks (#80) ---
	hookRunner.PostToolUse(tc.Name, argsJSON, result)

	status := "ok"
	if execErr != nil {
		status = "error"
	}
	summary := result
	if execErr != nil {
		summary = execErr.Error()
	}
	if len(summary) > 80 {
		summary = summary[:77] + "..."
	}
	ch <- tui.StreamEvent{
		Kind:       "tool_exec",
		ToolID:     tc.ID,
		ToolName:   tc.Name,
		Status:     status,
		DurationMs: elapsed.Milliseconds(),
		Summary:    summary,
	}

	if execErr != nil {
		return "Error: " + tools.CapOutput(execErr.Error(), tools.MaxOutputChars), execErr
	}
	return tools.CapOutput(result, tools.MaxOutputChars), nil
}

// reportNotRun tells the UI a tool call ended without running, so its row
// doesn't stay "running". Every call gets exactly one tool_exec event.
func reportNotRun(ch chan<- tui.StreamEvent, tc ui.ToolCallEvent, why string) {
	ch <- tui.StreamEvent{Kind: "tool_exec", ToolID: tc.ID, ToolName: tc.Name, Status: "error", Summary: why}
}

// buildCodeRequest constructs the gRPC ChatRequest for one agentic iteration.
// The history and tool schemas are encoded in Prompt as a JSON cliCodePayload and the
// model is prefixed with "cli/" so cai-llm-router can decode them. This works around
// the load balancer stripping proto fields 8+ (local_tools, code_messages) in transit.
func buildCodeRequest(chatID, model, toolSchemas string, history []codeMessage, isNewChat bool) *pb.ChatRequest {
	payload := cliCodePayload{V: cliPayloadVersion, History: history, Tools: toolSchemas, Timezone: localTimezoneName()}
	payloadJSON, _ := json.Marshal(payload)
	return &pb.ChatRequest{
		ChatId:    chatID,
		Model:     "cli/" + model,
		IsNewChat: isNewChat,
		Prompt:    string(payloadJSON),
	}
}
