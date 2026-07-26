package agent

import "testing"

func TestLocalTimezoneName_TZEnvOverride(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	if got := localTimezoneName(); got != "America/New_York" {
		t.Errorf("localTimezoneName() = %q, want %q", got, "America/New_York")
	}
}

func TestLocalTimezoneName_EmptyTZFallsBackToLocaltime(t *testing.T) {
	t.Setenv("TZ", "")
	// Can't control /etc/localtime in a test, so just assert it never returns
	// the misleading literal "Local" that time.Local.String() would produce.
	if got := localTimezoneName(); got == "Local" {
		t.Errorf("localTimezoneName() returned literal %q, want an IANA zone or empty string", got)
	}
}
