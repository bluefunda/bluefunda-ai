package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/plugins"
	"github.com/bluefunda/bluefunda-ai/internal/session"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose your environment",
	RunE:  runDoctor,
}

type checkResult struct {
	label  string
	status string // "ok" | "warn" | "error" | "info"
	detail string
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var checks []checkResult
	warnings := 0
	errors := 0

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)

	// 1. Config file
	cfg, err := config.Load()
	if err != nil {
		checks = append(checks, checkResult{"Config file", "error", err.Error()})
		errors++
	} else {
		checks = append(checks, checkResult{"Config file", "ok", "~/.bai/config.yaml"})
	}

	// 2. Authentication
	if cfg.Auth.AccessToken == "" {
		checks = append(checks, checkResult{"Authentication", "error", "not signed in — run `bai login`"})
		errors++
		printChecks(out, checks, warnings, errors)
		return nil
	}
	if !cfg.TokenValid() {
		checks = append(checks, checkResult{"Authentication", "warn", "token expired — run `bai login`"})
		warnings++
	} else {
		dur := time.Until(cfg.Auth.TokenExpiry).Round(time.Minute)
		checks = append(checks, checkResult{"Authentication", "ok", fmt.Sprintf("token valid (expires in %v)", dur)})
	}

	// 3. Backend reachable
	start := time.Now()
	if err := caigrpc.Ping(cfg.BFFURL); err != nil {
		checks = append(checks, checkResult{"Backend reachable", "error", cfg.BFFURL + " — " + err.Error()})
		errors++
	} else {
		latency := time.Since(start).Round(time.Millisecond)
		checks = append(checks, checkResult{"Backend reachable", "ok", fmt.Sprintf("%s  (%v)", cfg.BFFURL, latency)})
	}

	// 4. Account info — validates auth end-to-end
	conn, _, connErr := bffConn()
	if connErr == nil {
		defer conn.Close()
		ctx, cancel := caigrpc.ContextWithTimeout()
		defer cancel()

		resp, err := conn.Client.GetUserInfo(ctx, &pb.GetUserInfoRequest{})
		if err != nil {
			checks = append(checks, checkResult{"Account", "error", "could not fetch account info"})
			errors++
		} else {
			checks = append(checks, checkResult{"Account", "ok", resp.GetEmail()})
		}
	}

	// 5. Default model
	if cfg.Defaults.Model == "" || cfg.Defaults.Model == "openai" {
		checks = append(checks, checkResult{"Model default", "warn", `not configured — run: bai config set model=claude-sonnet`})
		warnings++
	} else {
		checks = append(checks, checkResult{"Model default", "ok", cfg.Defaults.Model})
	}

	// 6. ripgrep — used by search_content for fast code search
	if _, err := exec.LookPath("rg"); err != nil {
		checks = append(checks, checkResult{"ripgrep (rg)", "warn",
			"not found — search_content falls back to pure-Go (slower); install via package manager"})
		warnings++
	} else {
		checks = append(checks, checkResult{"ripgrep (rg)", "ok", "available"})
	}

	// 7. MCP servers (informational)
	checks = append(checks, checkResult{"MCP servers", "info", "run `bai mcp list` to view available integrations"})

	// 8. git available
	if _, err := exec.LookPath("git"); err != nil {
		checks = append(checks, checkResult{"git", "warn", "not found — worktree isolation and git tools unavailable"})
		warnings++
	} else {
		checks = append(checks, checkResult{"git", "ok", "available"})
	}

	// 9. Project context files
	cwd, _ := os.Getwd()
	hasContext := false
	for _, name := range []string{".bai/context.md", "AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(cwd, name)); err == nil {
			checks = append(checks, checkResult{"Project context", "ok", name})
			hasContext = true
			break
		}
	}
	if !hasContext {
		checks = append(checks, checkResult{"Project context", "info", "no .bai/context.md or AGENTS.md found — create one to give the agent project context"})
	}

	// 10. Hooks
	hooksDir := filepath.Join(cwd, ".bai", "hooks")
	hookEntries, _ := os.ReadDir(hooksDir)
	var hookCount int
	for _, e := range hookEntries {
		if !e.IsDir() {
			hookCount++
		}
	}
	if hookCount == 0 {
		checks = append(checks, checkResult{"Hooks", "info", "none configured — see docs for PreToolUse/PostToolUse hooks"})
	} else {
		checks = append(checks, checkResult{"Hooks", "ok", fmt.Sprintf("%d hook script(s) in .bai/hooks/", hookCount)})
	}

	// 11. Plugins loaded
	pm := plugins.NewManager(cwd)
	pluginList := pm.All()
	if len(pluginList) == 0 {
		checks = append(checks, checkResult{"Plugins", "info", "none loaded — see docs for plugin.yaml"})
	} else {
		names := make([]string, 0, len(pluginList))
		for _, p := range pluginList {
			names = append(names, p.Manifest.Name)
		}
		checks = append(checks, checkResult{"Plugins", "ok", fmt.Sprintf("%d plugin(s): %s", len(names), joinStrings(names, ", "))})
	}

	// 12. Local sessions
	codeSessions, _ := session.List(cwd)
	if len(codeSessions) == 0 {
		checks = append(checks, checkResult{"Local sessions", "info", "none for this directory"})
	} else {
		checks = append(checks, checkResult{"Local sessions", "ok", fmt.Sprintf("%d session(s) — run `bai sessions` to list", len(codeSessions))})
	}

	// 13. bai version / update check
	if latest, err := fetchLatestTag("bluefunda", "bluefunda-ai"); err != nil {
		// Offline or rate-limited — just show installed version.
		checks = append(checks, checkResult{"bai version", "ok", Version})
	} else if isNewerVersion(latest, Version) {
		checks = append(checks, checkResult{"bai version", "warn",
			fmt.Sprintf("%s installed, %s available — run `bai update`", Version, latest)})
		warnings++
	} else {
		checks = append(checks, checkResult{"bai version", "ok", fmt.Sprintf("%s (up to date)", Version)})
	}

	printChecks(out, checks, warnings, errors)
	return nil
}

func joinStrings(ss []string, sep string) string {
	return strings.Join(ss, sep)
}

// fetchLatestTag returns the latest GitHub release tag for the given repo.
func fetchLatestTag(owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bai-doctor/1.0")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var r struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	return r.TagName, nil
}

// isNewerVersion returns true if candidate is strictly newer than installed.
func isNewerVersion(candidate, installed string) bool {
	normalize := func(v string) string {
		v = strings.TrimSpace(v)
		if !strings.HasPrefix(v, "v") {
			return "v" + v
		}
		return v
	}
	c, b := normalize(candidate), normalize(installed)
	if c == b || b == "dev" || b == "v" {
		return false
	}
	parse := func(v string) [3]int {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		var out [3]int
		for i, p := range parts {
			if i >= 3 {
				break
			}
			p, _, _ = strings.Cut(p, "-")
			n := 0
			for _, ch := range p {
				if ch < '0' || ch > '9' {
					break
				}
				n = n*10 + int(ch-'0')
			}
			out[i] = n
		}
		return out
	}
	cv, bv := parse(c), parse(b)
	for i := range cv {
		if cv[i] != bv[i] {
			return cv[i] > bv[i]
		}
	}
	return false
}

func printChecks(out interface{ Write([]byte) (int, error) }, checks []checkResult, warnings, errors int) {
	okStyle := color.New(color.FgGreen)
	warnStyle := color.New(color.FgYellow)
	errStyle := color.New(color.FgRed)
	infoStyle := color.New(color.Faint)
	dimStyle := color.New(color.Faint)

	for _, c := range checks {
		var icon string
		switch c.status {
		case "ok":
			icon = okStyle.Sprint("✓")
		case "warn":
			icon = warnStyle.Sprint("!")
		case "error":
			icon = errStyle.Sprint("✗")
		default:
			icon = infoStyle.Sprint("ℹ")
		}
		label := dimStyle.Sprintf("%-20s", c.label)
		fmt.Fprintf(out, "  %s  %s %s\n", icon, label, c.detail)
	}

	fmt.Fprintln(out)
	switch {
	case errors > 0:
		_, _ = errStyle.Fprintf(out, "  %d error(s)", errors)
		if warnings > 0 {
			fmt.Fprintf(out, ", %d warning(s)", warnings) //nolint:errcheck
		}
		fmt.Fprintln(out)
	case warnings > 0:
		_, _ = warnStyle.Fprintf(out, "  %d warning(s)\n", warnings)
	default:
		_, _ = okStyle.Fprintln(out, "  All checks passed")
	}
	fmt.Fprintln(out)
}
