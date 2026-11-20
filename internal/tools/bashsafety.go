package tools

import (
	"regexp"
	"slices"
	"strings"
)

// Shell command analysis for approval decisions.
//
// A bash command is judged segment by segment: it is split at ;, &&, ||, |,
// & and newlines, and every segment must pass on its own. Judging only the
// start of the string would let "cat x; rm -rf ~" through on the strength of
// "cat". Commands using constructs this parser can't see into — command or
// process substitution, backticks, heredocs, subshells — and output
// redirection to a file are never auto-approved.

// ShellSegments splits command into simple commands. ok is false when the
// command uses a construct that makes per-segment judgement unsound (see
// above), or has unbalanced quotes.
func ShellSegments(command string) (segments []string, ok bool) {
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			segments = append(segments, s)
		}
		cur.Reset()
	}
	var quote byte // 0, '\'' or '"'
	for i := 0; i < len(command); i++ {
		c := command[i]
		next := byte(0)
		if i+1 < len(command) {
			next = command[i+1]
		}
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
			cur.WriteByte(c)
			continue
		case quote == '"':
			// Substitution still happens inside double quotes.
			if c == '`' || (c == '$' && next == '(') {
				return nil, false
			}
			if c == '\\' && next != 0 {
				cur.WriteByte(c)
				cur.WriteByte(next)
				i++
				continue
			}
			if c == '"' {
				quote = 0
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
			cur.WriteByte(c)
		case '\\':
			cur.WriteByte(c)
			if next != 0 {
				cur.WriteByte(next)
				i++
			}
		case '`', '(', ')', '{', '}':
			return nil, false
		case '$':
			if next == '(' {
				return nil, false
			}
			cur.WriteByte(c)
		case '<':
			if next == '<' || next == '(' { // heredoc / process substitution
				return nil, false
			}
			cur.WriteByte(c)
		case ';', '\n':
			flush()
		case '|':
			if next == '|' {
				i++
			}
			flush()
		case '&':
			switch {
			case next == '&':
				i++
				flush()
			case next == '>': // &> redirection, judged per segment
				cur.WriteByte(c)
			case i > 0 && (command[i-1] == '>' || command[i-1] == '<'): // >&2, <&0
				cur.WriteByte(c)
			default: // background
				flush()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if quote != 0 {
		return nil, false
	}
	flush()
	return segments, len(segments) > 0
}

// shellWords splits a segment into words, removing quotes. Good enough for
// classification: it doesn't expand variables or globs.
func shellWords(seg string) []string {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote byte
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '\\' && quote == '"' && i+1 < len(seg) {
				i++
				cur.WriteByte(seg[i])
			} else {
				cur.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote, inWord = c, true
		case c == '\\' && i+1 < len(seg):
			i++
			cur.WriteByte(seg[i])
			inWord = true
		case c == ' ' || c == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteByte(c)
			inWord = true
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words
}

// harmlessRedirect matches redirections that don't write files.
var harmlessRedirect = regexp.MustCompile(`^(\d?>&\d|&>/dev/null|\d?>>?/dev/null)$`)

// stripRedirects removes harmless redirections ("2>&1", ">/dev/null") from
// words and reports whether any other output redirection remains. A bare
// "<" input redirection is a read and is kept as a word pair.
func stripRedirects(words []string) (kept []string, writes bool) {
	for i := 0; i < len(words); i++ {
		w := words[i]
		if harmlessRedirect.MatchString(w) {
			continue
		}
		if (w == ">" || w == ">>" || w == "2>" || w == "&>" || w == "1>") && i+1 < len(words) && words[i+1] == "/dev/null" {
			i++
			continue
		}
		if strings.Contains(w, ">") {
			return nil, true
		}
		kept = append(kept, w)
	}
	return kept, false
}

// readOnlyPrograms never change state whatever their arguments (beyond the
// flag exclusions in segmentReadOnly).
var readOnlyPrograms = []string{
	"ls", "cat", "head", "tail", "wc", "stat", "pwd", "which", "type", "echo", "printf",
	"grep", "egrep", "fgrep", "rg", "cd", "file", "tree", "du", "df", "sort", "uniq",
	"cut", "tr", "diff", "cmp", "jq", "true", "false", "date", "basename", "dirname",
	"realpath", "readlink", "whoami", "uname", "id", "nproc", "column", "nl", "test", "[",
	"getent", "uptime", "free", "lsb_release",
}

var sedPrintScript = regexp.MustCompile(`^(\d+|\$)(,(\d+|\$))?p$`)

// segmentReadOnly reports whether one simple command only reads state.
func segmentReadOnly(seg string) bool {
	words, writes := stripRedirects(shellWords(seg))
	if writes {
		return false
	}
	return wordsReadOnly(words)
}

// xargsArgFlags are the xargs options that consume the following word.
var xargsArgFlags = []string{"-n", "-L", "-P", "-I", "-d", "-s", "-E", "-a", "--max-args", "--max-lines", "--max-procs", "--delimiter", "--arg-file"}

// wordsReadOnly reports whether a command (already split into words, with
// redirections handled) only reads state.
func wordsReadOnly(words []string) bool {
	if len(words) == 0 {
		return false
	}
	prog, args := words[0], words[1:]
	if strings.Contains(prog, "=") || strings.Contains(prog, "/") {
		return false // env assignment or explicit path: could run anything
	}
	has := func(flags ...string) bool {
		for _, a := range args {
			for _, f := range flags {
				if a == f || strings.HasPrefix(a, f+"=") {
					return true
				}
			}
		}
		return false
	}
	switch prog {
	case "find":
		return !has("-delete", "-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls")
	case "sort":
		return !has("-o", "--output")
	case "sed":
		// Only the common read form: sed -n 'N,Mp' file.
		if !has("-n") {
			return false
		}
		for _, a := range args {
			if strings.HasPrefix(a, "-i") || strings.HasPrefix(a, "--in-place") {
				return false
			}
		}
		for _, a := range args {
			if !strings.HasPrefix(a, "-") {
				return sedPrintScript.MatchString(a) // first non-flag is the script
			}
		}
		return false
	case "env", "printenv":
		return len(args) == 0 || prog == "printenv" // `env X=1 cmd` runs cmd
	case "git":
		return gitReadOnly(args)
	case "gh":
		return ghReadOnly(args)
	case "go":
		return goReadOnly(args)
	case "make":
		return len(args) == 1 && slices.Contains([]string{"build", "test", "lint", "vet", "check"}, args[0])
	case "npm":
		return (len(args) == 1 && args[0] == "test") ||
			(len(args) == 2 && args[0] == "run" && slices.Contains([]string{"test", "lint", "build"}, args[1]))
	case "hostname":
		return len(args) == 0 || (len(args) == 1 && strings.HasPrefix(args[0], "-") && args[0] != "-F" && args[0] != "--file")
	case "dpkg":
		return len(args) > 0 && slices.Contains([]string{"-l", "--list", "-s", "--status", "-L", "--listfiles", "-S", "--search", "--version"}, args[0])
	case "xargs":
		// xargs runs the command after its own options: judge that command.
		i := 0
		for i < len(args) && strings.HasPrefix(args[i], "-") {
			if slices.Contains(xargsArgFlags, args[i]) {
				i++
			}
			i++
		}
		return i < len(args) && args[i] != "xargs" && wordsReadOnly(args[i:])
	case "python", "python3":
		// Only the stdlib JSON pretty-printer: python3 -m json.tool [file].
		return len(args) >= 2 && args[0] == "-m" && args[1] == "json.tool" && len(args) <= 3
	case "golangci-lint":
		return !has("--fix")
	case "rg":
		return !has("--pre") // runs a preprocessor command per file
	case "tree":
		return !has("-o")
	}
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
		return true // any program's own version query
	}
	return slices.Contains(readOnlyPrograms, prog)
}

func gitReadOnly(args []string) bool {
	// Skip global options that take no effect on state.
	for len(args) > 0 {
		switch {
		case args[0] == "-C" && len(args) > 1:
			args = args[2:]
		case args[0] == "--no-pager" || args[0] == "-P":
			args = args[1:]
		default:
			goto sub
		}
	}
sub:
	if len(args) == 0 {
		return false
	}
	sub, rest := args[0], args[1:]
	for _, a := range rest {
		// Flags that write a file or run an external program.
		if strings.HasPrefix(a, "--output") || strings.HasPrefix(a, "--open-files-in-pager") ||
			(strings.HasPrefix(a, "-O") && sub == "grep") || a == "--ext-diff" || strings.HasPrefix(a, "--exec") {
			return false
		}
	}
	flagsOnly := func(banned ...string) bool {
		for _, a := range rest {
			if !strings.HasPrefix(a, "-") || slices.Contains(banned, a) {
				return false
			}
		}
		return true
	}
	switch sub {
	case "status", "log", "diff", "show", "blame", "ls-files", "ls-tree", "rev-parse",
		"describe", "shortlog", "grep", "cat-file", "rev-list", "merge-base", "version":
		return true
	case "branch":
		return flagsOnly("-d", "-D", "--delete", "-m", "-M", "--move", "-c", "-C", "--copy",
			"-f", "--force", "-u", "--set-upstream-to", "--unset-upstream", "--edit-description")
	case "tag":
		return len(rest) == 0 || (rest[0] == "-l" || rest[0] == "--list")
	case "remote":
		return len(rest) == 0 || rest[0] == "-v" || rest[0] == "show" || rest[0] == "get-url"
	case "stash":
		return len(rest) > 0 && (rest[0] == "list" || rest[0] == "show")
	case "config":
		return len(rest) > 0 && (rest[0] == "--get" || rest[0] == "--list" || rest[0] == "-l" || rest[0] == "--get-all")
	case "reflog":
		return len(rest) == 0 || rest[0] == "show"
	}
	return false
}

func ghReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "--version", "version":
		return true
	case "auth":
		return len(args) > 1 && args[1] == "status"
	case "search":
		return true
	case "api":
		for i, a := range args[1:] {
			switch {
			case a == "-X" || a == "--method":
				if i+2 >= len(args) || !strings.EqualFold(args[i+2], "GET") {
					return false
				}
			case strings.HasPrefix(a, "-X") && len(a) > 2 && !strings.EqualFold(a[2:], "GET"),
				strings.HasPrefix(a, "--method=") && !strings.EqualFold(strings.TrimPrefix(a, "--method="), "GET"):
				return false
			case a == "-f" || a == "-F" || a == "--field" || a == "--raw-field" || a == "--input",
				strings.HasPrefix(a, "--field=") || strings.HasPrefix(a, "--raw-field=") || strings.HasPrefix(a, "--input="):
				return false // gh api switches to POST when fields are given
			}
		}
		return true
	case "issue", "pr", "repo", "release", "run", "workflow", "label", "gist", "cache", "ruleset":
		return len(args) > 1 && slices.Contains([]string{"list", "view", "status", "diff", "checks"}, args[1])
	}
	return false
}

func goReadOnly(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "build", "test", "vet", "list", "version", "doc":
		return true
	case "env":
		for _, a := range args[1:] {
			if a == "-w" || a == "-u" {
				return false
			}
		}
		return true
	case "mod":
		return len(args) > 1 && slices.Contains([]string{"verify", "graph", "why"}, args[1])
	}
	return false
}

// BashReadOnly reports whether every segment of command only reads state,
// so it can run without asking.
func BashReadOnly(command string) bool {
	segs, ok := ShellSegments(command)
	if !ok {
		return false
	}
	for _, s := range segs {
		if !segmentReadOnly(s) {
			return false
		}
	}
	return true
}

// multiCommandPrograms take a subcommand as their second word, which a
// "don't ask again" prefix should include (gh api, not just gh).
var multiCommandPrograms = []string{
	"git", "gh", "go", "npm", "pnpm", "yarn", "make", "docker", "kubectl", "cargo",
	"pip", "pip3", "uv", "poetry", "helm", "terraform", "bai",
}

// segmentPrefix returns the word prefix a "don't ask again" rule would use
// for seg: the program, plus its subcommand for multi-command tools.
func segmentPrefix(seg string) []string {
	words, writes := stripRedirects(shellWords(seg))
	if writes || len(words) == 0 || strings.Contains(words[0], "=") {
		return nil
	}
	if slices.Contains(multiCommandPrograms, words[0]) && len(words) > 1 && !strings.HasPrefix(words[1], "-") {
		return words[:2]
	}
	return words[:1]
}

// SuggestBashPrefixes returns the command prefixes (e.g. "gh api",
// "python3") that a "don't ask again" approval of command should allow: one
// per segment that isn't read-only, deduplicated. nil means the command
// can't be generalised safely (substitution, file redirection, assignments),
// so only the exact command should be remembered.
func SuggestBashPrefixes(command string) []string {
	segs, ok := ShellSegments(command)
	if !ok {
		return nil
	}
	var out []string
	for _, s := range segs {
		if segmentReadOnly(s) {
			continue
		}
		p := segmentPrefix(s)
		if p == nil {
			return nil
		}
		if j := strings.Join(p, " "); !slices.Contains(out, j) {
			out = append(out, j)
		}
	}
	return out
}

// BashAllowedByPrefixes reports whether every segment of command is either
// read-only or starts with one of prefixes (word-wise: "gh api" allows
// "gh api repos/x" but not "gh apiary").
func BashAllowedByPrefixes(command string, prefixes []string) bool {
	segs, ok := ShellSegments(command)
	if !ok || len(prefixes) == 0 {
		return false
	}
	for _, s := range segs {
		if segmentReadOnly(s) {
			continue
		}
		words, writes := stripRedirects(shellWords(s))
		if writes {
			return false
		}
		matched := false
		for _, p := range prefixes {
			pw := strings.Fields(p)
			if len(pw) > 0 && len(words) >= len(pw) && slices.Equal(words[:len(pw)], pw) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
