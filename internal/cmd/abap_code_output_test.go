package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/abap"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files in testdata/")

// deployBFF asks for one abap_deploy call, then finishes with a line of text.
type deployBFF struct {
	pb.UnimplementedBFFServiceServer
	mu    sync.Mutex
	calls int
}

func (f *deployBFF) Chat(_ *pb.ChatRequest, stream grpc.ServerStreamingServer[pb.ChatEvent]) error {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call == 1 {
		data, _ := json.Marshal(map[string]string{
			"tool_call_id": "t1", "tool_name": "abap_deploy",
			"arguments": `{"name":"ZCL_FOO","type":"class","source":"CLASS zcl_foo DEFINITION PUBLIC. ENDCLASS. CLASS zcl_foo IMPLEMENTATION. ENDCLASS."}`,
		})
		return stream.Send(&pb.ChatEvent{Type: "tool_call", Data: string(data)})
	}
	if err := stream.Send(&pb.ChatEvent{Type: "content", Content: "Deployed ZCL_FOO."}); err != nil {
		return err
	}
	return stream.Send(&pb.ChatEvent{Type: "done"})
}

// codeGateway is abaper-gw for one class: save, activate, syntax check and
// unit tests. failTests makes the unit tests fail.
func codeGateway(failTests bool) http.Handler {
	ok := func(w http.ResponseWriter, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/abaper/api/v1/objects/create":
			ok(w, map[string]any{})
		case "/abaper/api/v1/activate":
			ok(w, abap.ActivateResult{ObjectName: "ZCL_FOO", Activated: true})
		case "/abaper/api/v1/syntax-check":
			ok(w, abap.SyntaxCheckResult{ObjectName: "ZCL_FOO"})
		case "/abaper/api/v1/unit-tests":
			res := abap.UnitTestResult{ObjectName: "ZCL_FOO", TotalTests: 1, Passed: 1, AllPassed: true,
				TestClasses: []abap.TestClassResult{{Name: "LTC", Methods: []abap.TestMethodResult{{Name: "T1", Status: "passed"}}}}}
			if failTests {
				res.Passed, res.Failed, res.AllPassed = 0, 1, false
				res.TestClasses[0].Methods[0] = abap.TestMethodResult{Name: "T1", Status: "failed", Message: "expected 2, got 1"}
			}
			ok(w, res)
		default:
			http.NotFound(w, r)
		}
	})
}

func streamTestJWT() string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." +
		enc([]byte(`{"sub":"u1","iss":"https://auth.example.com/realms/individual"}`)) + ".sig"
}

// runCodeStream runs `abaper code -o stream-json` end to end: the real agent
// loop and ABAP pack against an in-process BFF and a fake gateway.
func runCodeStream(t *testing.T, failTests bool) (string, error) {
	t.Helper()
	t.Cleanup(func() { cfgOutput = "" })
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterBFFServiceServer(srv, &deployBFF{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	gw := httptest.NewServer(codeGateway(failTests))
	t.Cleanup(gw.Close)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("BAI_BFF", lis.Addr().String())
	t.Setenv("BAI_ACCESS_TOKEN", streamTestJWT())
	t.Setenv("BAI_ABAP_GATEWAY", gw.URL)
	reg, err := abap.LoadSystems()
	if err != nil {
		t.Fatal(err)
	}
	reg.Add(abap.SAPSystem{Name: "DEV", Host: "https://sap.example.com", Client: "100", Username: "DEV", Password: "pw"})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	wd, _ := os.Getwd()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	root := newABAPerRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"code", "-o", "stream-json", "--repair-turns", "0", "create ZCL_FOO"})
	err = root.Execute()
	return strings.ReplaceAll(out.String(), work, "<cwd>"), err
}

func TestABAPCode_StreamJSONGolden(t *testing.T) {
	golden, _ := filepath.Abs(filepath.Join("testdata", "abap_code_stream.golden"))
	got, err := runCodeStream(t, false)
	if err != nil {
		t.Fatalf("code: %v\n%s", err, got)
	}
	if *updateGolden {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("stream-json output changed — field changes are breaking for editor clients.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestABAPCode_StreamJSONVerifyFailure(t *testing.T) {
	got, err := runCodeStream(t, true)
	if err == nil {
		t.Fatal("a failed verify must make the command fail")
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	var verify, done map[string]any
	for _, l := range lines {
		var ev map[string]any
		if jerr := json.Unmarshal([]byte(l), &ev); jerr != nil {
			t.Fatalf("stdout line is not JSON: %q", l)
		}
		switch ev["type"] {
		case "verify":
			verify = ev
		case "done":
			done = ev
		}
	}
	if verify == nil || verify["passed"] != false {
		t.Errorf("verify event = %v, want passed=false", verify)
	}
	var last map[string]any
	_ = json.Unmarshal([]byte(lines[len(lines)-1]), &last)
	if last["type"] != "done" || done["exit"] != float64(1) {
		t.Errorf("last event = %v, want done with exit 1", last)
	}
}
