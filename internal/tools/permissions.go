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
		if matchesPermission(p, toolName, argumentsJSON) {
			return PermitDeny
		}
	}
	if len(allow) == 0 {
		return PermitDefault
	}
	for _, p := range allow {
		if matchesPermission(p, toolName, argumentsJSON) {
			return PermitAuto
		}
	}
	return PermitDefault
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
	arg := extractPrimaryArg(toolName, argumentsJSON)
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

// extractPrimaryArg returns the principal string argument for a tool call,
// used for glob matching in permission patterns.
func extractPrimaryArg(toolName, argumentsJSON string) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
		return ""
	}
	switch toolName {
	case "bash":
		return strArg(args, "command")
	case "write_file", "edit_file", "read_file", "list_dir", "search_files":
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
