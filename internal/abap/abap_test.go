package abap

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

// fakeGateway is an in-memory abaper-gw: a repository of saved sources plus
// canned activation / syntax / unit-test outcomes per object.
type fakeGateway struct {
	mu        sync.Mutex
	sources   map[string]string // NAME → source
	inactive  map[string]string // NAME → activation error text
	syntaxErr map[string]string // NAME → syntax error text
	failTest  map[string]string // NAME → failing test message
	headers   http.Header
	calls     []string
}

func newFakeGateway() *fakeGateway {
	return &fakeGateway{
		sources:   map[string]string{"ZHELLO": "REPORT zhello."},
		inactive:  map[string]string{},
		syntaxErr: map[string]string{},
		failTest:  map[string]string{},
	}
}

func (g *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.headers = r.Header.Clone()
	g.calls = append(g.calls, r.URL.Path)
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	name := body["object_name"]

	ok := func(data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}
	switch r.URL.Path {
	case "/abaper/health":
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case "/abaper/api/v1/system/connect":
		w.WriteHeader(http.StatusOK)
	case "/abaper/api/v1/objects/search":
		ok(map[string]any{"Objects": []map[string]any{{"name": "ZHELLO", "type": "PROG/P"}}})
	case "/abaper/api/v1/objects/get":
		ok(map[string]any{"source": g.sources[name]})
	case "/abaper/api/v1/objects/list":
		ok([]map[string]any{{"name": "ZHELLO", "type": "PROG/P", "package": body["package"]}})
	case "/abaper/api/v1/objects/create":
		if strings.Contains(body["source"], "TAKEN") {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "object already exists"})
			return
		}
		g.sources[name] = body["source"]
		ok(map[string]any{})
	case "/abaper/api/v1/activate":
		if msg := g.inactive[name]; msg != "" {
			ok(ActivateResult{ObjectName: name, Messages: []ActivateMessage{{Severity: "E", Text: msg, Line: 3}}})
			return
		}
		ok(ActivateResult{ObjectName: name, Activated: true})
	case "/abaper/api/v1/syntax-check":
		res := SyntaxCheckResult{ObjectName: name}
		if msg := g.syntaxErr[name]; msg != "" {
			res.Messages = []SyntaxMessage{{Severity: "error", Text: msg, Line: 2}}
		}
		ok(res)
	case "/abaper/api/v1/unit-tests":
		res := UnitTestResult{ObjectName: name, TotalTests: 1, Passed: 1, AllPassed: true,
			TestClasses: []TestClassResult{{Name: "LTC", Methods: []TestMethodResult{{Name: "T1", Status: "passed"}}}}}
		if msg := g.failTest[name]; msg != "" {
			res.Passed, res.Failed, res.AllPassed = 0, 1, false
			res.TestClasses[0].Methods[0] = TestMethodResult{Name: "T1", Status: "failed", Message: msg}
		}
		ok(res)
	default:
		http.NotFound(w, r)
	}
}

var testSystem = &SAPSystem{ID: "s1", Name: "dev", Host: "https://sap.example.com", Client: "100", Username: "DEV", Password: "pw"}

func newTestClient(t *testing.T) (*Client, *fakeGateway) {
	t.Helper()
	g := newFakeGateway()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "individual", func() (string, error) { return "tok", nil }, testSystem)
	c.retryDelay = 0
	return c, g
}

func TestClient_SendsAuthAndSAPHeaders(t *testing.T) {
	c, g := newTestClient(t)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	want := map[string]string{
		"Authorization": "Bearer tok", "X-Realm": "individual",
		"X-Sap-Host": "https://sap.example.com", "X-Sap-Client": "100",
		"X-Sap-User": "DEV", "X-Sap-Password": "pw",
	}
	for k, v := range want {
		if got := g.headers.Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
}

func TestHealth_TokenOptional(t *testing.T) {
	c, g := newTestClient(t)
	if st, err := c.Health(); err != nil || st != "ok" || g.headers.Get("Authorization") != "Bearer tok" {
		t.Errorf("signed in: status %q, err %v, auth %q", st, err, g.headers.Get("Authorization"))
	}
	c.TokenFunc = func() (string, error) { return "", errors.New("token refresh failed") }
	if st, err := c.Health(); err != nil || st != "ok" {
		t.Errorf("expired session: status %q, err %v — the gateway is reachable without a token", st, err)
	}
	if a := g.headers.Get("Authorization"); a != "" {
		t.Errorf("expired session sent Authorization %q", a)
	}
	c.TokenFunc = nil
	if _, err := c.Health(); err != nil {
		t.Errorf("no token source: %v", err)
	}
}

func TestClient_NoSystem(t *testing.T) {
	c, _ := newTestClient(t)
	c.System = nil
	if _, err := c.Source("ZHELLO", "program"); err == nil || !strings.Contains(err.Error(), "system add") {
		t.Errorf("err = %v, want a hint to add a system", err)
	}
}

func TestClient_EnvelopeErrorSurfaces(t *testing.T) {
	c, _ := newTestClient(t)
	err := c.Save("ZX", "program", "TAKEN")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
}

func TestDeploy_InactiveIsAnError(t *testing.T) {
	c, g := newTestClient(t)
	g.inactive["ZBROKEN"] = "Statement is not accessible"
	err := Deploy(c, "zbroken", "program", "REPORT zbroken.")
	if err == nil || !strings.Contains(err.Error(), "line 3: Statement is not accessible") {
		t.Errorf("err = %v", err)
	}
	if err := Deploy(c, "zok", "program", "REPORT zok."); err != nil {
		t.Errorf("Deploy ok: %v", err)
	}
	if g.sources["ZOK"] != "REPORT zok." {
		t.Error("source not saved under the uppercased name")
	}
}

// ── Tool pack ─────────────────────────────────────────────────────────────────

func registerTestPack(t *testing.T, c *Client, tr *Tracker) {
	t.Helper()
	unregister, err := tools.RegisterPack(Pack(Env{NewClient: func() (*Client, error) { return c, nil }, Tracker: tr}))
	if err != nil {
		t.Fatalf("RegisterPack: %v", err)
	}
	t.Cleanup(unregister)
}

func TestPack_DeployFromFileRecordsTracker(t *testing.T) {
	c, g := newTestClient(t)
	tr := NewTracker()
	registerTestPack(t, c, tr)

	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "zcl_order.abap"), []byte("CLASS zcl_order DEFINITION."), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := tools.Execute("abap_deploy", `{"name":"zcl_order","type":"class","source_file":"zcl_order.abap"}`)
	if err != nil || !strings.Contains(out, "Deployed and activated class ZCL_ORDER") {
		t.Fatalf("deploy = %q, %v", out, err)
	}
	if g.sources["ZCL_ORDER"] != "CLASS zcl_order DEFINITION." {
		t.Errorf("saved source = %q", g.sources["ZCL_ORDER"])
	}

	_, err = tools.Execute("abap_deploy", `{"name":"ZTAKEN","type":"program","source":"TAKEN"}`)
	if err == nil {
		t.Fatal("expected deploy failure")
	}

	objs := tr.Objects()
	if len(objs) != 2 || !objs[0].OK || objs[1].OK || !strings.Contains(objs[1].Detail, "already exists") {
		t.Errorf("tracker = %+v", objs)
	}
}

func TestPack_SourceFileConfinedToWorkspace(t *testing.T) {
	c, _ := newTestClient(t)
	registerTestPack(t, c, nil)
	t.Chdir(t.TempDir())
	_, err := tools.Execute("abap_deploy", `{"name":"ZX","type":"program","source_file":"/etc/passwd"}`)
	if err == nil {
		t.Error("source_file outside the workspace must be rejected")
	}
}

func TestPack_ReadToolsAndValidation(t *testing.T) {
	c, _ := newTestClient(t)
	registerTestPack(t, c, nil)

	if out, err := tools.Execute("abap_get_source", `{"name":"zhello","type":"program"}`); err != nil || out != "REPORT zhello." {
		t.Errorf("get_source = %q, %v", out, err)
	}
	if out, _ := tools.Execute("abap_search", `{"pattern":"zh*"}`); !strings.Contains(out, "name=ZHELLO") {
		t.Errorf("search = %q", out)
	}
	if out, _ := tools.Execute("abap_run_unit_tests", `{"name":"ZHELLO","type":"program"}`); !strings.HasPrefix(out, "1 tests: 1 passed") {
		t.Errorf("unit tests = %q", out)
	}
	if _, err := tools.Execute("abap_get_source", `{"name":"ZHELLO","type":"widget"}`); err == nil {
		t.Error("unknown object type should be rejected")
	}
	if tools.NeedsApproval("abap_get_source") || !tools.NeedsApproval("abap_deploy") {
		t.Error("read tools must not need approval; deploy must")
	}
}

// ── Verification gate ────────────────────────────────────────────────────────

func TestVerify_ReportsEachFailingStep(t *testing.T) {
	c, g := newTestClient(t)
	g.syntaxErr["ZBAD_SYNTAX"] = "unknown statement"
	g.inactive["ZBAD_ACT"] = "inactive dependency"
	g.failTest["ZBAD_TEST"] = "expected 2, got 3"

	tr := NewTracker()
	tr.Record("ZGOOD", "class", "src", nil)
	tr.Record("ZBAD_SYNTAX", "program", "src", nil)
	tr.Record("ZBAD_ACT", "interface", "src", nil)
	tr.Record("ZBAD_TEST", "class", "src", nil)
	tr.Record("ZNEVER", "program", "src", errString("already exists"))

	vs := Verify(c, tr.Objects())
	if AllPassed(vs) || !vs[0].Passed() {
		t.Fatalf("only ZGOOD should pass: %s", Report(vs))
	}
	if vs[0].Tests != "ok" || vs[2].Tests != "skipped" {
		t.Errorf("unit tests should run only for classes/programs: %s", Report(vs))
	}
	report := Report(vs)
	for _, want := range []string{"syntax error line 2: unknown statement", "not active: [E] line 3: inactive dependency", "LTC->T1 failed: expected 2, got 3", "last deploy failed: already exists"} {
		if !strings.Contains(report, want) {
			t.Errorf("report missing %q:\n%s", want, report)
		}
	}
	repair := RepairPrompt(vs)
	if strings.Contains(repair, "ZGOOD") || !strings.Contains(repair, "ZBAD_TEST") {
		t.Errorf("repair prompt should list only failures:\n%s", repair)
	}
}

func TestTracker_CountsAttemptsAndReset(t *testing.T) {
	tr := NewTracker()
	tr.Record("ZA", "program", "v1", errString("boom"))
	tr.Record("ZA", "program", "v2", nil)
	objs := tr.Objects()
	if len(objs) != 1 || objs[0].Attempts != 2 || !objs[0].OK || objs[0].Source != "v2" {
		t.Errorf("objects = %+v", objs)
	}
	if !strings.Contains(Summary(objs), "(2 attempts)") {
		t.Errorf("summary = %q", Summary(objs))
	}
	tr.Reset()
	if len(tr.Objects()) != 0 {
		t.Error("Reset should clear objects")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

// ── Systems registry and slash commands ──────────────────────────────────────

func TestSystems_RoundTripAndSlashSwitch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	reg, err := LoadSystems()
	if err != nil || len(reg.Systems) != 0 {
		t.Fatalf("empty registry: %+v, %v", reg, err)
	}
	reg.Add(SAPSystem{Name: "dev", Host: "h1", Username: "u"})
	reg.Add(SAPSystem{Name: "qa", Host: "h2", Username: "u"})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	path, _ := systemsPath()
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("systems.json perms = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	cmds := SlashCommands(Env{})
	var system func(string) (string, error)
	for _, c := range cmds {
		if c.Name == "/system" {
			system = c.Run
		}
	}
	if out, err := system("qa"); err != nil || !strings.Contains(out, "Active SAP system: qa") {
		t.Fatalf("/system qa = %q, %v", out, err)
	}
	reg, _ = LoadSystems()
	if reg.ActiveSystem().Name != "qa" || reg.Systems[0].Client != "100" {
		t.Errorf("active = %+v", reg.ActiveSystem())
	}
	if out, _ := system(""); !strings.Contains(out, "* qa") {
		t.Errorf("/system list should mark qa active:\n%s", out)
	}
	if _, err := system("nope"); err == nil {
		t.Error("unknown system should error")
	}
}
