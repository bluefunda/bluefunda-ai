package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold .bai/ configuration for the current project",
	Long: `Create .bai/context.md (project instructions for the agent),
.bai/settings.yaml (project-level config), and .bai/hooks/ directories.

Safe to re-run: existing files are never overwritten.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	baiDir := filepath.Join(cwd, ".bai")
	if err := os.MkdirAll(baiDir, 0755); err != nil {
		return fmt.Errorf("create .bai/: %w", err)
	}

	lang := detectLanguage(cwd)
	created := 0

	// context.md
	contextPath := filepath.Join(baiDir, "context.md")
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		if err := os.WriteFile(contextPath, []byte(contextTemplate(repoName(cwd), lang)), 0644); err != nil {
			return fmt.Errorf("write context.md: %w", err)
		}
		fmt.Printf("  created  .bai/context.md\n")
		created++
	} else {
		fmt.Printf("  exists   .bai/context.md  (skipped)\n")
	}

	// settings.yaml
	settingsPath := filepath.Join(baiDir, "settings.yaml")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte(settingsTemplate()), 0644); err != nil {
			return fmt.Errorf("write settings.yaml: %w", err)
		}
		fmt.Printf("  created  .bai/settings.yaml\n")
		created++
	} else {
		fmt.Printf("  exists   .bai/settings.yaml  (skipped)\n")
	}

	// hook directories
	for _, dir := range []string{"hooks/pre-tool", "hooks/post-tool"} {
		p := filepath.Join(baiDir, dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	fmt.Printf("  created  .bai/hooks/pre-tool/   .bai/hooks/post-tool/\n")

	// .gitignore hint
	fmt.Println()
	if created > 0 {
		fmt.Println("Next steps:")
		fmt.Printf("  1. Edit .bai/context.md with your project's conventions\n")
		fmt.Printf("  2. Commit .bai/context.md and .bai/settings.yaml (they're team-shared)\n")
		fmt.Printf("  3. Add .bai/hooks/ to .gitignore or commit hook scripts as needed\n")
	} else {
		fmt.Println(".bai/ is already initialised.")
	}
	return nil
}

// detectLanguage returns the primary language of the project.
func detectLanguage(dir string) string {
	checks := []struct {
		file string
		lang string
	}{
		{"go.mod", "Go"},
		{"package.json", "TypeScript/JavaScript"},
		{"Cargo.toml", "Rust"},
		{"pyproject.toml", "Python"},
		{"pom.xml", "Java"},
		{"build.gradle", "Java/Kotlin"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.lang
		}
	}
	return ""
}

// repoName returns the git repo name or the directory basename.
func repoName(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return filepath.Base(strings.TrimSpace(string(out)))
	}
	return filepath.Base(dir)
}

func contextTemplate(repo, lang string) string {
	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(repo)
	sb.WriteString("\n\n")

	if lang != "" {
		fmt.Fprintf(&sb, "**Language:** %s\n\n", lang)
	}

	sb.WriteString(`## Project overview

<!-- Describe the project in 2-3 sentences. The agent reads this at the start of every session. -->

## Key conventions

<!-- List coding conventions, naming rules, architectural patterns, etc. Examples:
- All exported functions must have godoc comments
- Use structured errors with fmt.Errorf("context: %w", err)
- Feature branches follow feat/<description> naming
-->

## Directory structure

<!-- Briefly describe the important directories. Example:
- internal/cmd/   — Cobra CLI commands
- internal/tools/ — local tool implementations for bai code
-->

## Commands

<!-- Common commands the agent should use to build, test, and lint. Example:
make build    # build the binary
make test     # run tests
make lint     # run golangci-lint
-->

## Out of scope

<!-- Things the agent should NOT do or touch. Example:
- Do not modify .github/workflows/ without explicit instruction
- Do not add dependencies without asking first
-->
`)
	return sb.String()
}

func settingsTemplate() string {
	return `# .bai/settings.yaml — project-level configuration
# Committed to the repo; shared by all team members.
# Values here override ~/.bai/config.yaml but are overridden by CLI flags.

# model: claude          # override the default LLM model for this project
# max_turns: 30          # override --max-turns default
# endpoint: ""           # override BFF gRPC endpoint (leave empty for default)

# Local MCP servers available in bai code sessions.
# Each entry spawns a subprocess using stdio transport.
#
# mcp_servers:
#   github:
#     command: npx
#     args: ["-y", "@modelcontextprotocol/server-github"]
#     env:
#       GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
#   sqlite:
#     command: uvx
#     args: ["mcp-server-sqlite", "--db-path", "./dev.sqlite"]
`
}
