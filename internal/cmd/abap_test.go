package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// abapTestEnv isolates HOME, signs in via BAI_ACCESS_TOKEN, and points the
// ABAP gateway at srvURL (when non-empty).
func abapTestEnv(t *testing.T, srvURL string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BAI_NO_KEYCHAIN", "1")
	t.Setenv("BAI_ACCESS_TOKEN", "test-token")
	t.Setenv("ABAPER_BASE_URL", "")
	t.Setenv("BAI_ABAP_GATEWAY", srvURL)
	t.Cleanup(func() { cfgOutput = "" })
}

// runCobra executes cmd with args and returns what it wrote to stdout.
func runCobra(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	cmd.SetArgs(args)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	runErr := cmd.Execute()
	os.Stdout = stdout
	_ = w.Close()
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

func TestABAPSystem_AddUseList(t *testing.T) {
	abapTestEnv(t, "")

	if _, err := runCobra(t, abapSystemCmd(), "add", "--name", "dev", "--host", "https://dev.example.com", "--user", "DEV", "--password", "pw"); err != nil {
		t.Fatalf("add dev: %v", err)
	}
	if _, err := runCobra(t, abapSystemCmd(), "add", "--name", "qa", "--host", "https://qa.example.com", "--user", "QA", "--password", "pw"); err != nil {
		t.Fatalf("add qa: %v", err)
	}
	if _, err := runCobra(t, abapSystemCmd(), "add", "--name", "qa", "--host", "x", "--user", "y", "--password", "z"); err == nil {
		t.Error("duplicate system name should be rejected")
	}
	if _, err := runCobra(t, abapSystemCmd(), "use", "qa"); err != nil {
		t.Fatalf("use qa: %v", err)
	}

	cfgOutput = "json"
	out, err := runCobra(t, abapSystemCmd())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var rows []map[string]string
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("list output is not JSON: %v\n%s", err, out)
	}
	if len(rows) != 2 || rows[1]["NAME"] != "qa" || rows[1]["ACTIVE"] != "*" || rows[0]["ACTIVE"] != "" {
		t.Errorf("rows = %v", rows)
	}
	if strings.Contains(out, "pw") {
		t.Error("system listing must not print passwords")
	}
}

func TestABAPSearch_QuietPrintsNames(t *testing.T) {
	var gotAuth, gotHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotHost = r.Header.Get("Authorization"), r.Header.Get("X-SAP-Host")
		_, _ = w.Write([]byte(`{"success":true,"data":{"Objects":[{"type":"PROG/P","name":"ZHELLO"},{"type":"CLAS/OC","name":"ZCL_HELLO"}]}}`))
	}))
	defer srv.Close()
	abapTestEnv(t, srv.URL)

	if _, err := runCobra(t, abapSystemCmd(), "add", "--name", "dev", "--host", "https://dev.example.com", "--user", "DEV", "--password", "pw"); err != nil {
		t.Fatal(err)
	}

	cfgOutput = "quiet"
	out, err := runCobra(t, abapSearchCmd(), "zh*")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if out != "ZHELLO\nZCL_HELLO\n" {
		t.Errorf("quiet output = %q, want one name per line", out)
	}
	if gotAuth != "Bearer test-token" || gotHost != "https://dev.example.com" {
		t.Errorf("auth=%q host=%q", gotAuth, gotHost)
	}
}

func TestABAPSearch_NoSystemHint(t *testing.T) {
	abapTestEnv(t, "http://127.0.0.1:1")
	_, err := runCobra(t, abapSearchCmd(), "Z*")
	if err == nil || !strings.Contains(err.Error(), "bai abap system add") {
		t.Errorf("err = %v, want the system-add hint", err)
	}
}
