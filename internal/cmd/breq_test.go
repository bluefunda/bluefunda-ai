package cmd

import (
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/app"
)

func TestBreqRoot_CommandSurface(t *testing.T) {
	root := newBreqRoot()
	// Everything the standalone breq v1.9 CLI had.
	for _, path := range [][]string{
		{"login"}, {"version"}, {"update"}, {"health"}, {"completion"},
		{"user", "info"}, {"events", "subscribe"}, {"events", "publish"}, {"rpc", "request"},
		{"cr", "list"}, {"cr", "get"}, {"cr", "create"}, {"cr", "update"}, {"cr", "delete"},
		{"cr", "stage"}, {"cr", "watch"},
		{"cr", "comment", "list"}, {"cr", "comment", "add"}, {"cr", "comment", "update"}, {"cr", "comment", "delete"},
		{"a4h", "status"}, {"a4h", "start"}, {"a4h", "extend"}, {"a4h", "terminate"}, {"a4h", "watch"},
	} {
		if c, _, err := root.Find(path); err != nil || c == root {
			t.Errorf("breq %v not found", path)
		}
	}
	for _, baiOnly := range []string{"chat", "code", "abap"} {
		if c, _, _ := root.Find([]string{baiOnly}); c != root {
			t.Errorf("bai-only command %q must not leak into breq", baiOnly)
		}
	}
}

func TestRequestsCommands_FreshTrees(t *testing.T) {
	a, b := NewRequestsCommands(), NewRequestsCommands()
	if a[0] == b[0] {
		t.Error("NewRequestsCommands must build new commands each call (they mount under two parents)")
	}
}

func TestBreqIdentity(t *testing.T) {
	if Breq.Dir != ".breq" || Breq.EnvPrefix != "BREQ_" || Breq.ClientID != "bai" ||
		Breq.BFFURL != "requests.bluefunda.com:443" || Breq.HomebrewCask != "breq" {
		t.Errorf("breq identity changed: %+v — existing installs depend on these", Breq)
	}
	if app.Current().Name != "bai" {
		t.Error("building the breq root must not switch the process identity; only ExecuteBREQ does")
	}
}
