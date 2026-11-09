package adt

import (
	"net/http"
	"net/url"
	"testing"
)

// requestLog records every request a scripted test server sees, so a test
// can assert on call order and count without hand-rolling a state machine
// per case.
type requestLog struct {
	t     *testing.T
	calls []loggedRequest
}

type loggedRequest struct {
	Method string
	Path   string
	Query  url.Values
}

func (l *requestLog) record(r *http.Request) {
	l.calls = append(l.calls, loggedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query()})
}

func (l *requestLog) methods() []string {
	out := make([]string, len(l.calls))
	for i, c := range l.calls {
		out[i] = c.Method + " " + c.Path
	}
	return out
}

// TestCreateProgram_FullSequence drives the real CreateProgram flow end to
// end: POST create-shell -> POST lock -> PUT source -> POST unlock -> POST
// activate, and confirms the object ends up "created" only when every step
// (including activation) actually succeeds.
func TestCreateProgram_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/programs/programs":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistSuccessBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateProgram(t.Context(), "ZFOO", "Test program", "$TMP", "REPORT zfoo.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSequence := []string{
		"POST /sap/bc/adt/programs/programs",
		"POST /sap/bc/adt/programs/programs/zfoo",
		"PUT /sap/bc/adt/programs/programs/zfoo/source/main",
		"POST /sap/bc/adt/programs/programs/zfoo",
		"POST /sap/bc/adt/activation",
	}
	got := log.methods()
	if len(got) != len(wantSequence) {
		t.Fatalf("expected %d requests, got %d: %v", len(wantSequence), len(got), got)
	}
	for i, want := range wantSequence {
		if got[i] != want {
			t.Errorf("step %d: expected %q, got %q", i, want, got[i])
		}
	}
}

// TestCreateProgram_AlreadyExists confirms the create-shell step's failure
// (SAP's real "already exists" response) propagates as an error rather than
// silently continuing to lock/write/activate a program that was never
// actually created by this call.
func TestCreateProgram_AlreadyExists(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/programs/programs" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`<exc:exception xmlns:exc="http://www.sap.com/abapxml/types/communicationframework">` +
				`<message lang="EN">A program or include already exists with the name ZFOO</message></exc:exception>`))
			return
		}
		t.Fatalf("no further requests expected after create-shell fails, got %s %s", r.Method, r.URL.Path)
	})

	err := c.CreateProgram(t.Context(), "ZFOO", "Test program", "$TMP", "REPORT zfoo.")
	if err == nil {
		t.Fatal("expected an error")
	}
}

// TestCreateProgram_ActivationFailure is the exact live scenario found this
// session: source uploads fine, but activation reports a real compiler
// error (e.g. an unknown field reference) — CreateProgram must surface that,
// not report success.
func TestCreateProgram_ActivationFailure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/programs/programs":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK) // SAP answers 200 even when activation itself failed.
			_, _ = w.Write([]byte(activationChecklistErrorBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateProgram(t.Context(), "ZFOO", "Test program", "$TMP", "REPORT zfoo.")
	if err == nil {
		t.Fatal("expected an error for an activation checklist containing a type=E message")
	}
}

// TestCreateClass_FullSequence mirrors TestCreateProgram_FullSequence for
// CreateClass, which uses its own createClassMetadata/setClassSource/
// activateClass trio rather than the generic createAndPopulate helper.
func TestCreateClass_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/oo/classes":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistSuccessBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateClass(t.Context(), "ZCL_FOO", "Test class", "$TMP", "CLASS zcl_foo DEFINITION PUBLIC.\nENDCLASS.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 5 {
		t.Fatalf("expected 5 requests (shell, lock, source, unlock, activate), got %d: %v", len(log.calls), log.methods())
	}
}

// TestCreateInterface_FullSequence, TestCreateFunctionGroup_FullSequence,
// TestCreateInclude_FullSequence, TestCreateStructure_FullSequence and
// TestCreateTable_FullSequence all funnel through the shared
// createObjectMetadata + createAndPopulate helpers (unlike Program/Class,
// which have their own bespoke activate step), so they share one table test
// asserting each hits its documented create endpoint and completes.
func TestCreateAndPopulateBackedTypes_FullSequence(t *testing.T) {
	cases := []struct {
		name       string
		createPath string
		objectPath string
		call       func(c *ADTClientImpl) error
	}{
		{
			name:       "interface",
			createPath: "/sap/bc/adt/oo/interfaces",
			objectPath: "/sap/bc/adt/oo/interfaces/zfoo",
			call: func(c *ADTClientImpl) error {
				return c.CreateInterface(t.Context(), "ZFOO", "Test interface", "INTERFACE zfoo PUBLIC.\nENDINTERFACE.")
			},
		},
		{
			name:       "function group",
			createPath: "/sap/bc/adt/functions/groups",
			objectPath: "/sap/bc/adt/functions/groups/zfoo",
			call: func(c *ADTClientImpl) error {
				return c.CreateFunctionGroup(t.Context(), "ZFOO", "Test FG", "FUNCTION-POOL zfoo.")
			},
		},
		{
			name:       "include",
			createPath: "/sap/bc/adt/programs/includes",
			objectPath: "/sap/bc/adt/programs/includes/zfoo",
			call: func(c *ADTClientImpl) error {
				return c.CreateInclude(t.Context(), "ZFOO", "Test include", "* include")
			},
		},
		{
			name:       "structure",
			createPath: "/sap/bc/adt/ddic/structures",
			objectPath: "/sap/bc/adt/ddic/structures/zfoo",
			call: func(c *ADTClientImpl) error {
				return c.CreateStructure(t.Context(), "ZFOO", "Test structure", "define structure zfoo { field1 : abap.char(10); }")
			},
		},
		{
			name:       "table",
			createPath: "/sap/bc/adt/ddic/tables",
			objectPath: "/sap/bc/adt/ddic/tables/zfoo",
			call: func(c *ADTClientImpl) error {
				return c.CreateTable(t.Context(), "ZFOO", "Test table", "define table zfoo { key client : abap.clnt not null; }")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			log := &requestLog{t: t}
			c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
				log.record(r)
				switch {
				case r.Method == http.MethodPost && r.URL.Path == tc.createPath:
					w.WriteHeader(http.StatusCreated)
				case r.Method == http.MethodPost && r.URL.Path == tc.objectPath && r.URL.Query().Get("_action") == "LOCK":
					_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
				case r.Method == http.MethodPut:
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodPost && r.URL.Path == tc.objectPath && r.URL.Query().Get("_action") == "UNLOCK":
					w.WriteHeader(http.StatusOK)
				case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(activationChecklistSuccessBody))
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
			})

			if err := tc.call(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(log.calls) != 5 {
				t.Fatalf("expected 5 requests (shell, lock, source, unlock, activate), got %d: %v", len(log.calls), log.methods())
			}
		})
	}
}

// TestCreateFunction_RequiresFunctionGroup and the endpoint-shape assertion
// below cover CreateFunction, which nests under an existing function group
// and so has a distinct endpoint shape from the other create flows.
func TestCreateFunction_RequiresFunctionGroup(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected when function group is missing")
	})
	err := c.CreateFunction(t.Context(), "ZFM_FOO", "", "Test FM", "FUNCTION zfm_foo.\nENDFUNCTION.")
	if err == nil {
		t.Fatal("expected an error when functionGroup is empty")
	}
}

func TestCreateFunction_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/functions/groups/zfg/fmodules":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistSuccessBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateFunction(t.Context(), "ZFM_FOO", "ZFG", "Test FM", "FUNCTION zfm_foo.\nENDFUNCTION.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 5 {
		t.Fatalf("expected 5 requests, got %d: %v", len(log.calls), log.methods())
	}
}
