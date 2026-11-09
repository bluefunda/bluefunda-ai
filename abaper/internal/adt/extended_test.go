package adt

import (
	"net/http"
	"strings"
	"testing"
)

// TestGetTypeInfo_TriesDomainThenDataElement covers the fallback: GetTypeInfo
// tries the domain endpoint first, and only falls back to data element if
// that request fails outright (not found or any other error).
func TestGetTypeInfo_DomainSucceeds(t *testing.T) {
	var gotPath string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("domain source"))
	})

	info, err := c.GetTypeInfo(t.Context(), "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/sap/bc/adt/ddic/domains/ZFOO" {
		t.Errorf("unexpected path: %q", gotPath)
	}
	if info.TypeKind != "DOMAIN" || info.Source != "domain source" {
		t.Errorf("unexpected result: %+v", info)
	}
}

func TestGetTypeInfo_FallsBackToDataElement(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/ddic/domains/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "/ddic/dataelements/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data element source"))
			return
		}
		t.Fatalf("unexpected request: %s", r.URL.Path)
	})

	info, err := c.GetTypeInfo(t.Context(), "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.TypeKind != "DATA_ELEMENT" || info.Source != "data element source" {
		t.Errorf("unexpected result: %+v", info)
	}
}

func TestGetTypeInfo_NeitherFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetTypeInfo(t.Context(), "ZMISSING"); err == nil {
		t.Fatal("expected an error when neither domain nor data element is found")
	}
}

func TestGetTransaction(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<objectProperties/>`))
	})
	info, err := c.GetTransaction(t.Context(), "se80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.TransactionCode != "SE80" {
		t.Errorf("expected the transaction code to be uppercased, got %q", info.TransactionCode)
	}
}

func TestGetTransaction_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetTransaction(t.Context(), "ZBOGUS"); err == nil {
		t.Fatal("expected an error for a missing transaction")
	}
}

func TestGetTableContents(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"table_name":"ZFOO","row_count":2,"columns":[{"name":"ID"}],"rows":[{"ID":"1"},{"ID":"2"}]}`))
	})

	data, err := c.GetTableContents(t.Context(), "zfoo", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.TableName != "ZFOO" || data.RowCount != 2 || len(data.Rows) != 2 {
		t.Errorf("unexpected result: %+v", data)
	}
}

// TestGetTableContents_RequiresCustomService documents that this endpoint
// needs a non-standard SAP service (per the comment in client.go) and
// surfaces a distinct, actionable error rather than a generic 404.
func TestGetTableContents_RequiresCustomService(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetTableContents(t.Context(), "ZFOO", 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "custom SAP service") {
		t.Errorf("expected an actionable error about the missing custom service, got: %v", err)
	}
}

// TestGetTransports is a stub today (returns an empty slice, no HTTP call —
// see the comment in client.go). This test locks in that documented
// behavior so a future real implementation is a deliberate change, not a
// silent one.
func TestGetTransports(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("GetTransports is currently a stub and must not make any HTTP request")
	})
	transports, err := c.GetTransports(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(transports) != 0 {
		t.Errorf("expected an empty slice from the current stub implementation, got %+v", transports)
	}
}

func TestTestConnection_Success(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sap/bc/adt/discovery" && r.Header.Get("X-CSRF-Token") == "Fetch" {
			w.Header().Set("X-CSRF-Token", "fresh-token")
		}
		w.WriteHeader(http.StatusOK)
	})
	if err := c.TestConnection(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.IsAuthenticated() {
		t.Error("expected TestConnection to leave the client authenticated on success")
	}
}

func TestTestConnection_AuthenticationFails(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	if err := c.TestConnection(); err == nil {
		t.Fatal("expected an error when authentication fails")
	}
}

// TestDoRequest_RefreshesCSRFOn403 confirms doRequest's other retry branch
// (alongside the 401 re-auth path already covered in auth_test.go): a 403
// response to a request that carried a CSRF token triggers a token refresh
// and a single retry, rather than surfacing the 403 directly.
func TestDoRequest_RefreshesCSRFOn403(t *testing.T) {
	targetAttempts := 0
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sap/bc/adt/discovery" && r.Header.Get("X-CSRF-Token") == "Fetch":
			w.Header().Set("X-CSRF-Token", "reissued-csrf-token")
			w.WriteHeader(http.StatusOK)
		case r.URL.Query().Get("_action") == "LOCK":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.URL.Path == "/sap/bc/adt/programs/programs/zfoo/source/main":
			targetAttempts++
			if targetAttempts == 1 {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		case r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	err := c.UpdateProgram(t.Context(), "ZFOO", "REPORT zfoo.")
	// UpdateProgram locks first (a separate path); the 403 in this test is
	// scripted for the source PUT specifically via the path match above, so
	// a lock-not-found style failure here would indicate the retry didn't
	// happen as expected.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if targetAttempts != 2 {
		t.Errorf("expected exactly 2 attempts at the source PUT (1 forbidden + 1 retry after CSRF refresh), got %d", targetAttempts)
	}
	if c.csrfToken != "reissued-csrf-token" {
		t.Errorf("expected the client's csrfToken to be updated after refresh, got %q", c.csrfToken)
	}
}
