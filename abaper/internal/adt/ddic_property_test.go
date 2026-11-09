package adt

import (
	"net/http"
	"testing"

	"github.com/bluefunda/abaper/types"
)

// TestCreateDomain_FullSequence drives the DDIC property-object flow:
// create-shell -> lock -> PUT properties -> unlock -> activate (in that
// order — activation must happen AFTER unlock, unlike source-text objects
// which activate while still holding no lock at all).
func TestCreateDomain_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/ddic/domains":
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

	err := c.CreateDomain(t.Context(), "ZFOO", types.DomainProperties{
		Description: "Test domain",
		DataType:    "CHAR",
		Length:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantSequence := []string{
		"POST /sap/bc/adt/ddic/domains",
		"POST /sap/bc/adt/ddic/domains/zfoo",
		"PUT /sap/bc/adt/ddic/domains/zfoo",
		"POST /sap/bc/adt/ddic/domains/zfoo",
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
	// Activation must be the adtcore:-prefixed form, not a source-text
	// object's own inline activate — verified indirectly by confirming the
	// generic activation endpoint was reached last, after unlock.
	if got[3] != "POST /sap/bc/adt/ddic/domains/zfoo" || log.calls[3].Query.Get("_action") != "UNLOCK" {
		t.Errorf("expected unlock to happen before activation, got %v", got)
	}
}

// TestUpdateDomain_PropertyWriteFailureStillUnlocks confirms that when the
// PUT-properties step fails, the object is still unlocked (not left locked
// forever) and the original error is surfaced.
func TestUpdateDomain_PropertyWriteFailureStillUnlocks(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid domain properties"))
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.UpdateDomain(t.Context(), "ZFOO", types.DomainProperties{Description: "x", DataType: "CHAR", Length: 10})
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(log.calls) != 3 {
		t.Fatalf("expected lock, failed PUT, and unlock (cleanup) — got %d requests: %v", len(log.calls), log.methods())
	}
	if log.calls[2].Query.Get("_action") != "UNLOCK" {
		t.Errorf("expected the object to still be unlocked after a property-write failure, got %v", log.methods())
	}
}

func TestCreateDataElement_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/ddic/dataelements":
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

	err := c.CreateDataElement(t.Context(), "ZFOO", types.DataElementProperties{
		Description: "Test data element",
		DomainName:  "ZDOMAIN",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 5 {
		t.Fatalf("expected 5 requests (shell, lock, properties, unlock, activate), got %d: %v", len(log.calls), log.methods())
	}
}
