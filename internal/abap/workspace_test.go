package abap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/tools"
)

func newTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	ws, err := NewWorkspace(t.TempDir(), 3)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestWorkspace_FileNames(t *testing.T) {
	cases := map[string]string{
		"class": "zcl_x.clas.abap", "interface": "zcl_x.intf.abap", "program": "zcl_x.prog.abap",
		"ddls": "zcl_x.ddls.asddls", "srvd": "zcl_x.srvd.srvdsrv", "table": "zcl_x.tabl.astabl",
	}
	for typ, want := range cases {
		if got, err := FileName("ZCL_X", typ); err != nil || got != want {
			t.Errorf("%s: got %q, %v", typ, got, err)
		}
	}
	if _, err := FileName("ZX", "srvb"); err == nil {
		t.Error("srvb has no file")
	}
}

func TestWorkspace_CheckoutApplyAndConflicts(t *testing.T) {
	c, g := newTestClient(t)
	ws := newTestWorkspace(t)
	tr := NewTracker()

	// Checkout writes the file and records the base.
	path, lines, err := ws.Checkout(c, "zhello", "program", false)
	if err != nil || lines != 1 {
		t.Fatalf("Checkout: %v lines=%d", err, lines)
	}
	if b, _ := os.ReadFile(path); string(b) != "REPORT zhello." {
		t.Fatalf("file = %q", b)
	}

	// A second checkout over unchanged local file is fine; over an edit it is refused.
	if _, _, err := ws.Checkout(c, "ZHELLO", "program", false); err != nil {
		t.Fatalf("re-checkout of unchanged file: %v", err)
	}
	if err := os.WriteFile(path, []byte("REPORT zhello.\nWRITE 'x'."), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.Checkout(c, "ZHELLO", "program", false); err == nil || !strings.Contains(err.Error(), "local changes") {
		t.Errorf("checkout over local edit: %v", err)
	}

	// Apply with SAP unchanged deploys the edit and moves the base.
	if _, err := applyFile(c, ws, tr, "ZHELLO", "program", false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if g.sources["ZHELLO"] != "REPORT zhello.\nWRITE 'x'." {
		t.Errorf("SAP source = %q", g.sources["ZHELLO"])
	}
	objs := tr.Objects()
	if len(objs) != 1 || objs[0].Path != path || objs[0].Source != "" {
		t.Errorf("tracker should keep the path, not the source: %+v", objs)
	}

	// Someone changes it on SAP: apply now refuses.
	g.sources["ZHELLO"] = "REPORT zhello. \" changed in SE38"
	if _, err := applyFile(c, ws, tr, "ZHELLO", "program", false); !IsConflict(err) {
		t.Errorf("want conflict, got %v", err)
	}
	// Force overrides it.
	if _, err := applyFile(c, ws, tr, "ZHELLO", "program", true); err != nil {
		t.Errorf("forced apply: %v", err)
	}
}

func TestWorkspace_NewObjectApply(t *testing.T) {
	c, g := newTestClient(t)
	ws := newTestWorkspace(t)

	// A new object: written locally, absent on SAP, applies cleanly.
	p, _ := ws.Path("ZNEW", "program")
	if err := os.WriteFile(p, []byte("REPORT znew."), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := applyFile(c, ws, nil, "ZNEW", "program", false); err != nil {
		t.Fatalf("apply new: %v", err)
	}
	if b, ok, _ := ws.Base("ZNEW", "program"); !ok || b.Hash != HashSource("REPORT znew.") {
		t.Errorf("base after apply = %+v %v", b, ok)
	}

	// A "new" file whose object already exists on SAP is a conflict.
	g.sources["ZOTHER"] = "REPORT zother."
	p, _ = ws.Path("ZOTHER", "program")
	if err := os.WriteFile(p, []byte("REPORT zother. \" mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := applyFile(c, ws, nil, "ZOTHER", "program", false); !IsConflict(err) {
		t.Errorf("want conflict for unchecked-out existing object, got %v", err)
	}
}

func TestWorkspace_HashIgnoresCRLF(t *testing.T) {
	if HashSource("a\r\nb") != HashSource("a\nb") {
		t.Error("CRLF should not change the hash")
	}
}

func TestVerify_ReadsWorkspaceFile(t *testing.T) {
	c, g := newTestClient(t)
	ws := newTestWorkspace(t)
	p, _ := ws.Path("ZBAD", "program")
	if err := os.WriteFile(p, []byte("REPORT zbad."), 0o644); err != nil {
		t.Fatal(err)
	}
	tr := NewTracker()
	tr.RecordFile("ZBAD", "program", p, nil)
	g.syntaxErr["ZBAD"] = "boom"
	res := Verify(c, tr.Objects())
	if len(res) != 1 || res[0].Syntax != "failed" {
		t.Errorf("verify should syntax-check the file: %+v", res)
	}
}

func TestPack_WorkspaceToolsOnlyWhenEnabled(t *testing.T) {
	c, _ := newTestClient(t)
	names := func(p tools.Pack) map[string]bool {
		m := map[string]bool{}
		for _, tl := range p.Tools {
			m[tl.Name] = true
		}
		return m
	}
	off := Pack(Env{NewClient: func() (*Client, error) { return c, nil }})
	if names(off)["abap_checkout"] || strings.Contains(off.Instructions, "abap_apply") {
		t.Error("workspace tools must not exist when the workspace is off")
	}
	on := Pack(Env{NewClient: func() (*Client, error) { return c, nil }, Workspace: newTestWorkspace(t)})
	if !names(on)["abap_checkout"] || !names(on)["abap_apply"] {
		t.Error("workspace tools missing when on")
	}
}

func TestPack_LargeSourcePreview(t *testing.T) {
	c, g := newTestClient(t)
	g.sources["ZBIG"] = "REPORT zbig.\nWRITE 1.\nWRITE 2.\nWRITE 3.\nWRITE 4."
	ws := newTestWorkspace(t) // inline max 3 lines
	unregister, err := tools.RegisterPack(Pack(Env{NewClient: func() (*Client, error) { return c, nil }, Workspace: ws}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unregister)
	out, err := tools.Execute("abap_get_source", `{"name":"zbig","type":"program"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "(3 of 5 lines shown)") || strings.Contains(out, "WRITE 4.") || !strings.Contains(out, "abap_checkout") {
		t.Errorf("preview = %q", out)
	}
	// Checkout through the tool writes the full file and returns no source.
	out, err = tools.Execute("abap_checkout", `{"name":"zbig","type":"program"}`)
	if err != nil || strings.Contains(out, "WRITE") || !strings.Contains(out, "5 lines") {
		t.Errorf("checkout out=%q err=%v", out, err)
	}
	if b, _ := os.ReadFile(filepath.Join(ws.Root, "zbig.prog.abap")); !strings.Contains(string(b), "WRITE 4.") {
		t.Error("full source not written")
	}
}
