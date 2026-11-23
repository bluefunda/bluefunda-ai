package serve

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/abap"
)

// serve must never pick up a shared SAP identity unless --allow-static-sap is
// passed: every request then has to carry its own X-SAP-* headers.

func storedSystems() (*abap.Systems, error) {
	return &abap.Systems{
		Systems: []abap.SAPSystem{{ID: "a1", Name: "dev", Host: "https://dev.example.com", Client: "100", Username: "STORED", Password: "pw"}},
		Active:  "a1",
	}, nil
}

func emptySystems() (*abap.Systems, error) { return &abap.Systems{}, nil }

func sapEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

var fullSAPEnv = sapEnv(map[string]string{
	"SAP_HOST": "https://env.example.com", "SAP_USERNAME": "ENVUSER", "SAP_PASSWORD": "pw", "SAP_CLIENT": "200",
})

func TestResolveStaticSystem_DefaultIgnoresStoredSystemAndEnv(t *testing.T) {
	loadCalled := false
	load := func() (*abap.Systems, error) { loadCalled = true; return storedSystems() }

	sys, err := resolveStaticSystem(false, "", load, fullSAPEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sys != nil {
		t.Fatalf("got static system %q; without --allow-static-sap there must be none", sys.Username)
	}
	if loadCalled {
		t.Error("stored systems must not be read without --allow-static-sap")
	}
}

func TestResolveStaticSystem_SystemFlagNeedsAllowStatic(t *testing.T) {
	_, err := resolveStaticSystem(false, "dev", storedSystems, fullSAPEnv)
	if err == nil || !strings.Contains(err.Error(), "--allow-static-sap") {
		t.Fatalf("error = %v, want one naming --allow-static-sap", err)
	}
}

func TestResolveStaticSystem_AllowStaticUsesActiveSystem(t *testing.T) {
	sys, err := resolveStaticSystem(true, "", storedSystems, fullSAPEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sys == nil || sys.Username != "STORED" {
		t.Fatalf("got %+v, want the stored active system", sys)
	}
}

func TestResolveStaticSystem_AllowStaticSelectsNamedSystem(t *testing.T) {
	sys, err := resolveStaticSystem(true, "dev", storedSystems, sapEnv(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sys == nil || sys.ID != "a1" {
		t.Fatalf("got %+v, want system a1", sys)
	}
	if _, err := resolveStaticSystem(true, "nope", storedSystems, sapEnv(nil)); err == nil {
		t.Fatal("expected an error for an unknown --system")
	}
}

func TestResolveStaticSystem_AllowStaticFallsBackToEnv(t *testing.T) {
	sys, err := resolveStaticSystem(true, "", emptySystems, fullSAPEnv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sys == nil || sys.Username != "ENVUSER" || sys.Client != "200" {
		t.Fatalf("got %+v, want the SAP_* environment system", sys)
	}
}

func TestResolveStaticSystem_AllowStaticWithNothingConfiguredFails(t *testing.T) {
	if _, err := resolveStaticSystem(true, "", emptySystems, sapEnv(nil)); err == nil {
		t.Fatal("expected an error when --allow-static-sap finds no SAP system")
	}
}

func TestResolveStaticSystem_LoadError(t *testing.T) {
	load := func() (*abap.Systems, error) { return nil, errors.New("boom") }
	if _, err := resolveStaticSystem(true, "", load, sapEnv(nil)); err == nil {
		t.Fatal("expected the load error")
	}
}
