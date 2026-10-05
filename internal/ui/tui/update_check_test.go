package tui

import "testing"

func TestCheckForUpdateCmd_DisabledReturnsNilRegardlessOfVersion(t *testing.T) {
	for _, v := range []string{"", "dev", "v1.0.0"} {
		if cmd := checkForUpdateCmd(v, true); cmd != nil {
			t.Errorf("checkForUpdateCmd(%q, disabled=true) returned a non-nil Cmd, want nil", v)
		}
	}
}

func TestCheckForUpdateCmd_DevOrEmptyVersionReturnsNilEvenWhenEnabled(t *testing.T) {
	for _, v := range []string{"", "dev"} {
		if cmd := checkForUpdateCmd(v, false); cmd != nil {
			t.Errorf("checkForUpdateCmd(%q, disabled=false) returned a non-nil Cmd, want nil", v)
		}
	}
}
