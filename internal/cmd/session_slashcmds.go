package cmd

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/bluefunda/bluefunda-ai/internal/app"
	"github.com/bluefunda/bluefunda-ai/internal/mcp"
	"github.com/bluefunda/bluefunda-ai/internal/tools"

	"github.com/bluefunda/bluefunda-ai/internal/ui/tui"
)

// errTurnRunning is returned by commands that rewrite the session's history
// while a turn is using it.
var errTurnRunning = errors.New("a turn is still running; try again when it finishes")

// compactCommand is /compact [focus]: replace the conversation with a
// summary to free context, as Claude Code's /compact does. turnMu is held by
// every running turn; run does the compaction.
func compactCommand(turnMu *sync.Mutex, run func(focus string) (string, error)) tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/compact",
		Description: "Summarise the conversation to free context  /compact [what to keep]",
		Run: func(focus string) (string, error) {
			if !turnMu.TryLock() {
				return "", errTurnRunning
			}
			defer turnMu.Unlock()
			return run(focus)
		},
	}
}

// humanTokens formats a token count as "850" or "12.3k".
func humanTokens(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

// mergeSlashCommands concatenates command lists, keeping the first command
// of each name.
func mergeSlashCommands(lists ...[]tui.SlashCommand) []tui.SlashCommand {
	seen := map[string]bool{}
	var out []tui.SlashCommand
	for _, l := range lists {
		for _, c := range l {
			if !seen[c.Name] {
				seen[c.Name] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// promptCommands are built-in commands that send a prompt, like Claude
// Code's /init. A file of the same name in .bai/commands replaces one.
func promptCommands() []tui.SlashCommand {
	return []tui.SlashCommand{
		{
			Name:        "/init",
			Description: "Write or improve AGENTS.md with what an agent needs to work in this repo",
			Prompt:      initPrompt,
		},
		{
			Name:        "/review",
			Description: "Review the current changes, or a PR  /review 123",
			Prompt:      reviewPrompt,
		},
		{
			Name:        "/security-review",
			Description: "Security review of the current changes, or a PR  /security-review 123",
			Prompt:      securityReviewPrompt,
		},
	}
}

// securityReviewPrompt is /review narrowed to vulnerabilities that are
// actually reachable.
const securityReviewPrompt = `Do a security review of code changes. Do not modify any files.

What to review: $ARGUMENTS
- A number means that pull request (` + "`gh pr diff <n>`" + `); a branch or path means the changes there; nothing means this branch's commits since the default branch plus uncommitted changes.

For each change, trace where its inputs come from and where its outputs go. Look for: injection (shell, SQL, path traversal, template), missing or bypassable authentication and authorisation, secrets in code or logs, unsafe deserialisation, SSRF, weak or misused cryptography, unsafe file permissions, and dependency changes that pull in risk.

Report only issues an attacker could actually reach, most severe first. For each: file:line, the vulnerability, how it is exploited (the attacker's input and the path it takes), and the fix. Say plainly if you find nothing exploitable; do not list theoretical issues with no reachable path.`

// reviewPrompt reviews read-only. What to review comes after the command: a
// PR number, a branch, a path, or nothing for the work in progress.
const reviewPrompt = `Review code changes for bugs. Do not modify any files.

What to review: $ARGUMENTS
- A number means that pull request: read it with ` + "`gh pr view <n>`" + ` and ` + "`gh pr diff <n>`" + `.
- A branch or path means the changes there.
- Nothing means the work in progress: this branch's commits since it left the default branch (` + "`git merge-base`" + ` with origin's default branch) plus uncommitted changes.

Read enough of the surrounding code to understand each change; a diff alone hides most bugs. Look for: wrong logic and edge cases, error handling that loses or ignores errors, concurrency problems, resource leaks, broken callers of a changed function, security issues, and missing tests for new behaviour. Check the change against the project's own instructions (AGENTS.md or .bai/context.md) where it has them.

Report only real problems, most severe first. For each: file:line, what is wrong, and a concrete input or sequence that triggers it. Leave out style preferences unless they hide a bug. If you find nothing worth fixing, say so plainly rather than padding the list.`

// initPrompt asks for the project instructions file bai loads at session
// start (findContextFile: .bai/context.md, else AGENTS.md).
const initPrompt = `Set up this repository's agent instructions file.

If .bai/context.md exists, that is the file to update; otherwise AGENTS.md at the repository root. If the file exists, read it first and improve it: keep what is still correct, fix what is wrong, add what is missing.

Investigate before writing. Read the build files (Makefile, package.json, go.mod, pyproject.toml, Cargo.toml and the like), the CI configuration, the README and the top-level layout, then write down what a coding agent needs to work here:

- the exact commands to build, test (including a single test), lint and format, taken from the repo, not guessed
- how the code is organised: the main directories and what lives in each
- conventions the code follows that aren't obvious from one file: error handling, naming, test style, commit message format
- traps: generated files not to edit, required environment, slow or flaky steps, anything that surprised you

Keep it short and specific to this repo. Skip generic advice that applies to every project, and never invent a command or rule you did not find evidence for. When you are done, say which file you wrote and summarise what is in it.

$ARGUMENTS`

// ruleSource is one place permission rules came from.
type ruleSource struct {
	name        string
	allow, deny []string
}

// permissionsCommand is /permissions: the rules in force, by source, and
// what was allowed for the rest of this session.
func permissionsCommand(sources []ruleSource, session *sessionAllowSet) tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/permissions",
		Description: "List permission rules and where they come from",
		Instant:     true,
		Run: func(string) (string, error) {
			return permissionsReport(sources, session.describe()), nil
		},
	}
}

func permissionsReport(sources []ruleSource, session []string) string {
	var b strings.Builder
	b.WriteString("Permission rules (deny wins over allow; read-only tools and commands never ask):")
	found := false
	for _, s := range sources {
		if len(s.allow) == 0 && len(s.deny) == 0 {
			continue
		}
		found = true
		fmt.Fprintf(&b, "\n\n%s", s.name)
		for _, r := range s.allow {
			fmt.Fprintf(&b, "\n  allow  %s", r)
		}
		for _, r := range s.deny {
			fmt.Fprintf(&b, "\n  deny   %s", r)
		}
	}
	if !found {
		b.WriteString("\n\n(none)")
	}
	if len(session) > 0 {
		b.WriteString("\n\nAllowed for this session (\"don't ask again\")")
		for _, r := range session {
			fmt.Fprintf(&b, "\n  allow  %s", r)
		}
	}
	fmt.Fprintf(&b, "\n\nAdd rules to ~/%[1]s/settings.yaml (every project) or %[1]s/settings.yaml (this one), e.g. \"bash:make test\".", app.Current().Dir)
	return b.String()
}

// agentsCommand is /agents: the subagent types the task tool can start.
func agentsCommand() tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/agents",
		Description: "List subagent types for the task tool",
		Instant:     true,
		Run: func(string) (string, error) {
			return agentsReport(tools.AgentTypes()), nil
		},
	}
}

func agentsReport(types []tools.AgentType) string {
	var b strings.Builder
	b.WriteString("Subagent types (the model picks one with the task tool):")
	for _, t := range types {
		toolList := "all tools"
		if len(t.Tools) > 0 {
			toolList = strings.Join(t.Tools, ", ")
		}
		fmt.Fprintf(&b, "\n\n%s  (%s)\n  %s\n  tools: %s", t.Name, t.Source, t.Description, toolList)
		if t.Model != "" {
			fmt.Fprintf(&b, "\n  model: %s", t.Model)
		}
	}
	fmt.Fprintf(&b, "\n\nAdd your own as ~/%[1]s/agents/<name>.md or %[1]s/agents/<name>.md: frontmatter with name, description, tools, model; the body is its instructions.", app.Current().Dir)
	return b.String()
}

// statusLines are the session's lines for /status.
func statusLines(ctxLine string, allow, deny []string, servers []mcp.ServerStatus) []string {
	lines := []string{ctxLine,
		fmt.Sprintf("Rules:      %d allow, %d deny (/permissions)", len(allow), len(deny))}
	if len(servers) > 0 {
		names := make([]string, 0, len(servers))
		for _, s := range servers {
			n := s.Name
			if !s.Started {
				n += " (failed)"
			}
			names = append(names, n)
		}
		lines = append(lines, "MCP:        "+strings.Join(names, ", "))
	}
	agents := tools.AgentTypes()
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return append(lines, "Agents:     "+strings.Join(names, ", ")+" (/agents)")
}

// shellContext tells the model about a command the user ran with "!".
func shellContext(command, output string) string {
	return fenced("I ran this command in my shell:\n$ "+command, output)
}

// todosCommand is /todos: the task list the model keeps with todo_write.
func todosCommand() tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/todos",
		Description: "Show the current task list",
		Instant:     true,
		Run: func(string) (string, error) {
			return todosReport(tools.Todos()), nil
		},
	}
}

func todosReport(todos []tools.TodoItem) string {
	if len(todos) == 0 {
		return "No todos. The model keeps a task list for multi-step work."
	}
	mark := map[string]string{"completed": "☒", "in_progress": "▶", "pending": "☐"}
	lines := []string{"Todos:"}
	for _, t := range todos {
		lines = append(lines, "  "+mark[t.Status]+" "+t.Content)
	}
	return strings.Join(lines, "\n")
}

// skillCommands lets the user run a skill directly as /<name>, as in Claude
// Code. A command file or built-in of the same name takes precedence.
func skillCommands(skills []tools.Skill) []tui.SlashCommand {
	out := make([]tui.SlashCommand, 0, len(skills))
	for _, s := range skills {
		out = append(out, tui.SlashCommand{
			Name:        "/" + s.Name,
			Description: "(skill) " + s.Description,
			Prompt: fmt.Sprintf("Use the %q skill. Its instructions (folder: %s; load supporting files with the skill tool):\n\n%s\n\n$ARGUMENTS",
				s.Name, s.Dir, s.Body),
		})
	}
	return out
}

// skillsCommand is /skills: the skills this session has.
func skillsCommand() tui.SlashCommand {
	return tui.SlashCommand{
		Name:        "/skills",
		Description: "List skills (instructions the model loads when relevant)",
		Instant:     true,
		Run: func(string) (string, error) {
			return skillsReport(tools.Skills()), nil
		},
	}
}

func skillsReport(skills []tools.Skill) string {
	var b strings.Builder
	if len(skills) == 0 {
		b.WriteString("No skills.")
	} else {
		b.WriteString("Skills (the model loads one when a task matches it; run one yourself with /<name>):")
		for _, s := range skills {
			fmt.Fprintf(&b, "\n\n/%s  (%s)\n  %s", s.Name, s.Dir, s.Description)
		}
	}
	fmt.Fprintf(&b, "\n\nAdd one as ~/%[1]s/skills/<name>/SKILL.md or %[1]s/skills/<name>/SKILL.md: frontmatter with name and description, then the instructions. Other files in the folder can be loaded too.", app.Current().Dir)
	return b.String()
}
