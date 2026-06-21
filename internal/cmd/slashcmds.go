package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// loadCustomSlashCommands discovers .bai/commands/*.md files by walking upward
// from cwd to the git root. Each file becomes a slash command: the filename
// (without .md) is the command name and the file body is the prompt sent to
// the LLM when the command is invoked.
//
// Optional YAML front-matter (lines between --- delimiters) is stripped from
// the prompt; the "description:" field is used as the autocomplete hint.
func loadCustomSlashCommands(cwd string) []tui.SlashCommand {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil
	}

	// Walk upward to git root looking for .bai/commands/.
	var commandsDir string
	for {
		candidate := filepath.Join(abs, ".bai", "commands")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			commandsDir = candidate
			break
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	if commandsDir == "" {
		return nil
	}

	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		return nil
	}

	var cmds []tui.SlashCommand
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(commandsDir, e.Name()))
		if err != nil {
			continue
		}
		name := "/" + strings.TrimSuffix(e.Name(), ".md")
		desc, prompt := parseFrontmatter(string(data))
		if desc == "" {
			desc = "custom command"
		}
		if prompt == "" {
			continue
		}
		cmds = append(cmds, tui.SlashCommand{
			Name:        name,
			Description: desc,
			Prompt:      strings.TrimSpace(prompt),
		})
	}
	return cmds
}

// parseFrontmatter extracts description and body from optional YAML front-matter.
// Front-matter is delimited by lines containing only "---".
func parseFrontmatter(content string) (description, body string) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", content
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
		if after, ok := strings.CutPrefix(lines[i], "description:"); ok {
			description = strings.TrimSpace(after)
		}
	}
	if end < 0 {
		return "", content
	}
	body = strings.Join(lines[end+1:], "\n")
	return description, body
}
