package tools

import (
	"testing"
)

func TestCheckPermissions(t *testing.T) {
	bashRm := `{"command":"rm -rf /tmp/foo"}`
	bashGit := `{"command":"git status"}`
	writeEtc := `{"path":"/etc/passwd"}`
	writeHome := `{"path":"/home/user/main.go"}`
	webURL := `{"url":"https://evil.com/payload"}`
	webSafe := `{"url":"https://pkg.go.dev/fmt"}`

	tests := []struct {
		name      string
		allow     []string
		deny      []string
		toolName  string
		argsJSON  string
		want      PermissionAction
	}{
		// Empty lists → default (no change to existing behaviour)
		{"empty lists bash", nil, nil, "bash", bashGit, PermitDefault},
		{"empty lists web_search", nil, nil, "web_search", `{"query":"go generics"}`, PermitDefault},

		// Deny matches → PermitDeny regardless of allow
		{"deny rm matches", nil, []string{"bash:rm *"}, "bash", bashRm, PermitDeny},
		{"deny write_file /etc/* matches", nil, []string{"write_file:/etc/*"}, "write_file", writeEtc, PermitDeny},
		{"deny write_file /etc/* no match on home", nil, []string{"write_file:/etc/*"}, "write_file", writeHome, PermitDefault},
		{"deny exact tool name", nil, []string{"web_search"}, "web_search", `{"query":"x"}`, PermitDeny},
		{"deny does not match different tool", nil, []string{"bash:rm *"}, "write_file", writeEtc, PermitDefault},
		{"deny wins over allow", []string{"bash:rm *"}, []string{"bash:rm *"}, "bash", bashRm, PermitDeny},

		// Allow matches → PermitAuto
		{"allow git *", []string{"bash:git *"}, nil, "bash", bashGit, PermitAuto},
		{"allow exact tool", []string{"web_search"}, nil, "web_search", `{"query":"anything"}`, PermitAuto},
		{"allow web_fetch safe URL", []string{"web_fetch:https://pkg.go.dev/*"}, nil, "web_fetch", webSafe, PermitAuto},
		{"allow web_fetch does not match evil URL", []string{"web_fetch:https://pkg.go.dev/*"}, nil, "web_fetch", webURL, PermitDefault},

		// Allow list non-empty but no match → PermitDefault (requires prompt)
		{"allow non-empty no match", []string{"bash:git *"}, nil, "bash", bashRm, PermitDefault},
		{"allow non-empty different tool", []string{"web_search"}, nil, "bash", bashGit, PermitDefault},

		// Multi-pattern deny
		{"multi deny first matches", nil, []string{"bash:rm *", "write_file:/etc/*"}, "bash", bashRm, PermitDeny},
		{"multi deny second matches", nil, []string{"bash:rm *", "write_file:/etc/*"}, "write_file", writeEtc, PermitDeny},
		{"multi deny none match", nil, []string{"bash:rm *", "write_file:/etc/*"}, "bash", bashGit, PermitDefault},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckPermissions(tt.allow, tt.deny, tt.toolName, tt.argsJSON)
			if got != tt.want {
				t.Errorf("CheckPermissions(%v, %v, %q, %q) = %v, want %v",
					tt.allow, tt.deny, tt.toolName, tt.argsJSON, got, tt.want)
			}
		})
	}
}

func TestMatchesPermission(t *testing.T) {
	tests := []struct {
		pattern  string
		toolName string
		argsJSON string
		want     bool
	}{
		// Exact tool name, no colon
		{"bash", "bash", `{"command":"echo hi"}`, true},
		{"bash", "write_file", `{"path":"/tmp/x"}`, false},

		// bash: command glob
		{"bash:git *", "bash", `{"command":"git log --oneline"}`, true},
		{"bash:git *", "bash", `{"command":"rm -rf /"}`, false},
		{"bash:rm *", "bash", `{"command":"rm file.txt"}`, true},

		// write_file: path glob
		{"write_file:/tmp/*", "write_file", `{"path":"/tmp/output.txt"}`, true},
		{"write_file:/tmp/*", "write_file", `{"path":"/etc/passwd"}`, false},

		// web_fetch: URL glob
		{"web_fetch:https://docs.go.dev/*", "web_fetch", `{"url":"https://docs.go.dev/cmd/go"}`, true},
		{"web_fetch:https://docs.go.dev/*", "web_fetch", `{"url":"https://evil.com/"}`, false},

		// Invalid JSON args → no match (safe default)
		{"bash:git *", "bash", `not-json`, false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"@"+tt.toolName, func(t *testing.T) {
			got := matchesPermission(tt.pattern, tt.toolName, tt.argsJSON)
			if got != tt.want {
				t.Errorf("matchesPermission(%q, %q, %q) = %v, want %v",
					tt.pattern, tt.toolName, tt.argsJSON, got, tt.want)
			}
		})
	}
}
