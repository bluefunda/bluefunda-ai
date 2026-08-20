package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// loadCustomSlashCommands discovers .bai/commands/*.md files at project level
// (walking upward from cwd to the git root) and ~/.bai/commands/*.md at user
// level, merging the two with project commands taking precedence on a name
// collision. Each file becomes a slash command: the filename (without .md)
// is the command name and the file body is the prompt sent to the LLM when
// the command is invoked.
//
// Optional YAML front-matter (lines between --- delimiters) is stripped from
// the prompt; the "description:" field is used as the autocomplete hint.
func loadCustomSlashCommands(cwd string) []tui.SlashCommand {
	byName := make(map[string]tui.SlashCommand)

	if home, err := os.UserHomeDir(); err == nil {
		for _, c := range loadCommandsDir(filepath.Join(home, ".bai", "commands")) {
			byName[c.Name] = c
		}
	}
	for _, c := range loadCommandsDir(findCommandsDir(cwd)) {
		byName[c.Name] = c // project overrides user on collision
	}
	if len(byName) == 0 {
		return nil
	}

	cmds := make([]tui.SlashCommand, 0, len(byName))
	for _, c := range byName {
		cmds = append(cmds, c)
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

// findCommandsDir walks upward from cwd to the git root looking for a
// .bai/commands/ directory, returning "" if none exists.
func findCommandsDir(cwd string) string {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(abs, ".bai", "commands")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
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
	return ""
}

// loadCommandsDir loads every *.md file in dir as a slash command. Returns
// nil if dir is "" or doesn't exist.
func loadCommandsDir(dir string) []tui.SlashCommand {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var cmds []tui.SlashCommand
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		name := "/" + strings.TrimSuffix(e.Name(), ".md")
		desc, prompt := parseFrontmatter(string(data))
		if desc == "" {
			desc = "custom command"
		}
		prompt = strings.TrimSpace(prompt)
		if prompt == "" {
			continue
		}
		cmds = append(cmds, tui.SlashCommand{
			Name:        name,
			Description: desc,
			Prompt:      prompt,
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
