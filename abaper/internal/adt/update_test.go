package adt

import (
	"net/http"
	"strings"
	"testing"
)

// TestUpdateSourceObject_AllTypes covers every Update* wrapper (both the ones
// funneling through the shared updateSourceObject helper, and the
// structurally-identical bespoke ones like UpdateProgram/UpdateClass) and
// confirms each does lock -> PUT source -> unlock, with no activation step
// (updates never auto-activate; that's an explicit separate call).
func TestUpdateSourceObject_AllTypes(t *testing.T) {
	cases := []struct {
		name       string
		objectPath string
		call       func(c *ADTClientImpl) error
	}{
		{"program", "/sap/bc/adt/programs/programs/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateProgram(t.Context(), "zfoo", "REPORT zfoo.")
		}},
		{"class", "/sap/bc/adt/oo/classes/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateClass(t.Context(), "zfoo", "CLASS zfoo DEFINITION PUBLIC.\nENDCLASS.")
		}},
		{"include", "/sap/bc/adt/programs/includes/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateInclude(t.Context(), "zfoo", "* include")
		}},
		{"interface", "/sap/bc/adt/oo/interfaces/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateInterface(t.Context(), "zfoo", "INTERFACE zfoo PUBLIC.\nENDINTERFACE.")
		}},
		{"function group", "/sap/bc/adt/functions/groups/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateFunctionGroup(t.Context(), "zfoo", "FUNCTION-POOL zfoo.")
		}},
		{"table", "/sap/bc/adt/ddic/tables/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateTable(t.Context(), "zfoo", "define table zfoo { key client : abap.clnt not null; }")
		}},
		{"structure", "/sap/bc/adt/ddic/structures/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateStructure(t.Context(), "zfoo", "define structure zfoo { field1 : abap.char(10); }")
		}},
		{"ddls", "/sap/bc/adt/ddic/ddl/sources/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateDDLS(t.Context(), "zfoo", "@AbapCatalog.sqlViewName: 'ZFOO'\ndefine view zfoo as select from t000 {\n  mandt\n}")
		}},
		{"srvd", "/sap/bc/adt/ddic/srvd/sources/zfoo", func(c *ADTClientImpl) error {
			return c.UpdateSRVD(t.Context(), "zfoo", "@EndUserText.label: 'x'\ndefine service zfoo { }")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &requestLog{t: t}
			c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
				log.record(r)
				switch {
				case r.Method == http.MethodPost && r.URL.Path == tc.objectPath && r.URL.Query().Get("_action") == "LOCK":
					_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
				case r.Method == http.MethodPut:
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodPost && r.URL.Path == tc.objectPath && r.URL.Query().Get("_action") == "UNLOCK":
					w.WriteHeader(http.StatusOK)
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			})

			if err := tc.call(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(log.calls) != 3 {
				t.Fatalf("expected exactly 3 requests (lock, source, unlock — no activate), got %d: %v", len(log.calls), log.methods())
			}
			if log.calls[0].Query.Get("_action") != "LOCK" || log.calls[2].Query.Get("_action") != "UNLOCK" {
				t.Errorf("expected lock-then-unlock bracketing the source PUT, got %v", log.methods())
			}
		})
	}
}

// TestUpdateProgram_DoesNotExist is the exact bug found live this session:
// deploying a brand-new object via the update/save path (rather than
// create) fails at the lock step because the object was never created, and
// the client must surface that clearly rather than a generic error.
func TestUpdateProgram_DoesNotExist(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("_action") == "LOCK" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`<exc:exception xmlns:exc="http://www.sap.com/abapxml/types/communicationframework">` +
				`<message lang="EN">ZFOO does not exist</message></exc:exception>`))
			return
		}
		t.Fatalf("no further requests expected after lock fails, got %s %s", r.Method, r.URL.Path)
	})

	err := c.UpdateProgram(t.Context(), "ZFOO", "REPORT zfoo.")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected the real ADT error text to survive, got: %v", err)
	}
}

func TestUpdateFunction_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/functions/groups/zfg/fmodules/zfm_foo" && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.UpdateFunction(t.Context(), "ZFM_FOO", "ZFG", "FUNCTION zfm_foo.\nENDFUNCTION.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 3 {
		t.Fatalf("expected 3 requests, got %d: %v", len(log.calls), log.methods())
	}
}

func TestUpdateFunction_ValidatesInputs(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected for invalid input")
	})
	if err := c.UpdateFunction(t.Context(), "ZFM_FOO", "", "FUNCTION zfm_foo.\nENDFUNCTION."); err == nil {
		t.Fatal("expected an error when functionGroup is empty")
	}
	if err := c.UpdateFunction(t.Context(), "ZFM_FOO", "ZFG", "   "); err == nil {
		t.Fatal("expected an error when source is blank")
	}
}
