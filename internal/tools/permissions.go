package tools

import (
	"encoding/json"
	"strings"
)

// PermissionAction is the outcome of evaluating allow/deny lists for a tool call.
type PermissionAction int

const (
	// PermitDefault means no rule matched; fall through to NeedsApproval logic.
	PermitDefault PermissionAction = iota
	// PermitAuto means an allow rule matched; auto-approve without a prompt.
	PermitAuto
	// PermitDeny means a deny rule matched; block execution entirely.
	PermitDeny
)

// CheckPermissions evaluates deny then allow lists against a tool call.
//
// Pattern syntax:
//
//	"web_search"          – matches the tool by name, any arguments
//	"bash:rm *"           – matches bash when the command starts with "rm "
//	"write_file:/etc/*"   – matches write_file when the path matches the glob
//	"web_fetch:https://evil.com/*" – matches web_fetch when the URL matches
//
// Evaluation order:
//  1. Deny-first: if any deny pattern matches → PermitDeny (caller blocks).
//  2. Allow-list mode: if allow is non-empty and no allow pattern matches →
//     PermitDefault (caller falls through to NeedsApproval, requiring a prompt).
//  3. Allow match found → PermitAuto (caller skips the approval prompt).
//  4. Empty allow+deny lists → PermitDefault (existing behaviour unchanged).
func CheckPermissions(allow, deny []string, toolName, argumentsJSON string) PermissionAction {
	for _, p := range deny {
		if matchesPermission(p, toolName, argumentsJSON) || bashSegmentMatches(p, toolName, argumentsJSON) {
			return PermitDeny
		}
	}
	if len(allow) == 0 {
		return PermitDefault
	}
	if toolName == "bash" {
		if bashAllowedByRules(allow, argumentsJSON) {
			return PermitAuto
		}
		return PermitDefault
	}
	for _, p := range allow {
		if matchesPermission(p, toolName, argumentsJSON) {
			return PermitAuto
		}
	}
	return PermitDefault
}

// bashSegmentMatches reports whether a bash deny pattern matches any single
// segment of a compound command, so "bash:rm *" also blocks "ls; rm -rf x".
func bashSegmentMatches(pattern, toolName, argumentsJSON string) bool {
	tool, glob, hasGlob := strings.Cut(pattern, ":")
	if toolName != "bash" || tool != "bash" || !hasGlob {
		return false
	}
	segs, _ := ShellSegments(ExtractPrimaryArg(toolName, argumentsJSON))
	for _, s := range segs {
		if globMatch(glob, s) {
			return true
		}
	}
	return false
}

// bashAllowedByRules reports whether allow rules cover a bash command: a bare
// "bash" rule allows everything; otherwise every segment must be read-only or
// match a "bash:<glob>" rule on its own. Matching the whole string would let
// "git push *" approve "git push; rm -rf ~".
func bashAllowedByRules(allow []string, argumentsJSON string) bool {
	var globs []string
	for _, p := range allow {
		tool, glob, hasGlob := strings.Cut(p, ":")
		if tool != "bash" {
			continue
		}
		if !hasGlob {
			return true
		}
		globs = append(globs, glob)
	}
	if len(globs) == 0 {
		return false
	}
	segs, ok := ShellSegments(ExtractPrimaryArg("bash", argumentsJSON))
	if !ok {
		return false
	}
	for _, s := range segs {
		if segmentReadOnly(s) {
			continue
		}
		matched := false
		for _, g := range globs {
			if globMatch(g, s) {
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

// matchesPermission tests whether pattern matches the given tool call.
// Pattern format: "<tool>" or "<tool>:<arg-glob>".
func matchesPermission(pattern, toolName, argumentsJSON string) bool {
	tool, argGlob, hasGlob := strings.Cut(pattern, ":")
	if tool != toolName {
		return false
	}
	if !hasGlob {
		return true
	}
	arg := ExtractPrimaryArg(toolName, argumentsJSON)
	return globMatch(argGlob, arg)
}

// globMatch returns true when s matches pattern. * matches any sequence of
// characters (including path separators); ? matches any single character.
// Unlike filepath.Match, * crosses / boundaries, which is intentional for
// matching bash commands and URLs that contain slashes.
func globMatch(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == "" {
		return s == ""
	}
	if pattern[0] == '*' {
		for i := 0; i <= len(s); i++ {
			if globMatch(pattern[1:], s[i:]) {
				return true
			}
		}
		return false
	}
	if s == "" {
		return false
	}
	if pattern[0] == '?' || pattern[0] == s[0] {
		return globMatch(pattern[1:], s[1:])
	}
	return false
}

// ExtractPrimaryArg returns the principal string argument for a tool call,
// used for glob matching in permission patterns and for building new
// "<tool>:<arg>" patterns (e.g. to persist an "always allow" approval).
func ExtractPrimaryArg(toolName, argumentsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return ""
	}
	if t, ok := lookupPackTool(toolName); ok {
		return strArg(args, t.PrimaryArg)
	}
	switch toolName {
	case "bash":
		return strArg(args, "command")
	case "write_file", "edit_file", "read_file", "list_dir", "search_files", "read_notebook", "edit_notebook":
		return strArg(args, "path")
	case "web_fetch":
		return strArg(args, "url")
	case "web_search":
		return strArg(args, "query")
	case "memory_read", "memory_write", "memory_delete":
		return strArg(args, "key")
	case "search_content":
		return strArg(args, "pattern")
	default:
		for _, k := range []string{"path", "command", "url", "query"} {
			if v := strArg(args, k); v != "" {
				return v
			}
		}
		return ""
	}
}

func strArg(args map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
