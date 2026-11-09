package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &Client{
		BaseURL:    ts.URL,
		Token:      "test-token",
		Realm:      "test-realm",
		HTTPClient: ts.Client(),
	}, ts
}

func writeAPIResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"data":    data,
	})
}

func TestDo_SetsAuthHeaders(t *testing.T) {
	var gotAuth, gotRealm, gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotRealm = r.Header.Get("X-Realm")
		gotPath = r.URL.Path
		writeAPIResponse(t, w, map[string]string{})
	})

	_, err := Post[map[string]string](c, "/api/v1/health", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("expected bearer token header, got %q", gotAuth)
	}
	if gotRealm != "test-realm" {
		t.Errorf("expected realm header, got %q", gotRealm)
	}
	if gotPath != "/abaper/api/v1/health" {
		t.Errorf("expected path prefixed with /abaper, got %q", gotPath)
	}
}

func TestDo_SetsSAPHeadersWhenConfigured(t *testing.T) {
	var gotHost string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Header.Get("X-SAP-Host")
		writeAPIResponse(t, w, map[string]string{})
	})
	c.SAPHost = "https://a4h.example.com"
	c.SAPClient = "001"
	c.SAPUser = "developer"
	c.SAPPassword = "secret"

	if _, err := Post[map[string]string](c, "/api/v1/health", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHost != "https://a4h.example.com" {
		t.Errorf("expected X-SAP-Host header, got %q", gotHost)
	}
}

func TestDo_RetriesOnTransientStatus(t *testing.T) {
	attempts := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeAPIResponse(t, w, map[string]string{"ok": "true"})
	})
	// Speed up retry backoff isn't configurable, so keep this test's attempt
	// count small (default retry loop is 3 attempts total).
	if _, err := Post[map[string]string](c, "/api/v1/health", nil); err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_DoesNotRetryPlain500(t *testing.T) {
	// A 500 is almost always a deterministic application error (already
	// exists, syntax error, activation failure) — retrying it just wastes
	// time and can turn a clear error into a confusing "already exists" on
	// a later attempt after a partial create. It should fail immediately
	// with the real body intact, not a generic "HTTP 500".
	attempts := 0
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "no component exists with the name CARRID",
		})
	})
	_, err := Post[map[string]string](c, "/api/v1/health", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt (no retry on plain 500), got %d", attempts)
	}
	if !strings.Contains(err.Error(), "CARRID") {
		t.Errorf("expected real error body to survive, got: %v", err)
	}
}

func TestPost_PropagatesAPIError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "object_type is required",
		})
	})
	_, err := Post[map[string]string](c, "/api/v1/objects/list", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListObjects_DecodesBareArrayResponse(t *testing.T) {
	var gotBody map[string]string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/abaper/api/v1/objects/list" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeAPIResponse(t, w, []map[string]any{
			{"type": "PROG/P", "name": "ZHELLO_WORLD"},
		})
	})

	objects, err := c.ListObjects("$TMP", "PROG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["package"] != "$TMP" || gotBody["object_type"] != "PROG" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if len(objects) != 1 || objects[0]["name"] != "ZHELLO_WORLD" {
		t.Errorf("unexpected objects: %+v", objects)
	}
}

func TestPackageContents_UsesObjectsGetRoute(t *testing.T) {
	var gotPath string
	var gotBody map[string]string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeAPIResponse(t, w, map[string]any{
			"name":    "$TMP",
			"objects": []map[string]any{{"type": "PROG/P", "name": "ZHELLO_WORLD"}},
		})
	})

	objects, err := c.PackageContents("$TMP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/abaper/api/v1/objects/get" {
		t.Errorf("expected PackageContents to hit /api/v1/objects/get, got %q", gotPath)
	}
	if gotBody["object_type"] != "package" || gotBody["object_name"] != "$TMP" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if len(objects) != 1 || objects[0]["name"] != "ZHELLO_WORLD" {
		t.Errorf("unexpected objects: %+v", objects)
	}
}

func TestSearchObjects(t *testing.T) {
	var gotBody map[string]string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		writeAPIResponse(t, w, map[string]any{
			"Objects": []map[string]any{{"type": "PROG/P", "name": "ZHELLO_WORLD"}},
		})
	})

	objects, err := c.SearchObjects("ZHELL*", "PROG/P")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["object_name"] != "ZHELL*" || gotBody["object_type"] != "PROG/P" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if len(objects) != 1 || objects[0]["name"] != "ZHELLO_WORLD" {
		t.Errorf("unexpected objects: %+v", objects)
	}
}

func TestGetObject(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIResponse(t, w, map[string]any{"object_name": "ZFOO", "source": "REPORT zfoo."})
	})
	obj, err := c.GetObject("program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if (*obj)["object_name"] != "ZFOO" {
		t.Errorf("unexpected object: %+v", *obj)
	}
}

func TestCreateObject(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeAPIResponse(t, w, map[string]any{"created": true})
	})
	if err := c.CreateObject("ZFOO", "program", "REPORT zfoo."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/abaper/api/v1/objects/create" {
		t.Errorf("unexpected path: %s", gotPath)
	}
}

func TestActivate_UsesActivateRoute(t *testing.T) {
	var gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeAPIResponse(t, w, map[string]any{"activated": true})
	})
	result, err := c.Activate("ZFOO", "program")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/abaper/api/v1/activate" {
		t.Errorf("expected Activate to hit /api/v1/activate (the /objects/activate route 404s through the gateway), got %q", gotPath)
	}
	if !result.Activated {
		t.Errorf("expected Activated to be true")
	}
}

func TestActivate_ReportsFailedActivation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIResponse(t, w, map[string]any{
			"activated": false,
			"messages": []map[string]any{
				{"severity": "E", "text": `No component exists with the name "CARRID".`, "line": 4},
			},
		})
	})
	result, err := c.Activate("ZFOO", "program")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Activated {
		t.Errorf("expected Activated to be false")
	}
	if len(result.Messages) != 1 || result.Messages[0].Severity != "E" {
		t.Errorf("expected one error message, got %+v", result.Messages)
	}
}

func TestSyntaxCheck(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIResponse(t, w, map[string]any{"messages": []any{}})
	})
	if _, err := c.SyntaxCheck("ZFOO", "program", "REPORT zfoo."); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormatCode(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIResponse(t, w, map[string]string{"source": "REPORT zfoo."})
	})
	source, err := c.FormatCode("report zfoo.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if source != "REPORT zfoo." {
		t.Errorf("unexpected formatted source: %q", source)
	}
}

func TestTransportInfo(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/abaper/api/v1/transports/info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeAPIResponse(t, w, map[string]any{"transports": []any{}})
	})
	if _, err := c.TransportInfo(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateTransport(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeAPIResponse(t, w, map[string]string{"transport": "TR0001"})
	})
	number, err := c.CreateTransport("test transport", "$TMP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if number != "TR0001" {
		t.Errorf("unexpected transport number: %q", number)
	}
}

func TestRunUnitTests(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/abaper/api/v1/unit-tests" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		writeAPIResponse(t, w, map[string]any{"all_passed": true})
	})
	if _, err := c.RunUnitTests("ZCL_DEMO", "class"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompletionAndNavigation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/abaper/api/v1/completion", "/abaper/api/v1/navigation":
			writeAPIResponse(t, w, map[string]any{})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	if _, err := c.Completion("ZFOO", "program", "REPORT zfoo.", 1, 1); err != nil {
		t.Fatalf("unexpected completion error: %v", err)
	}
	if _, err := c.Navigation("ZFOO", "program", "REPORT zfoo.", 1, 1); err != nil {
		t.Fatalf("unexpected navigation error: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/abaper/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})
	result, err := c.HealthCheck()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "healthy" {
		t.Errorf("unexpected health result: %+v", result)
	}
}

func TestSystemConnect(t *testing.T) {
	var gotHost, gotUser string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Header.Get("X-SAP-Host")
		gotUser = r.Header.Get("X-SAP-User")
		w.WriteHeader(http.StatusOK)
	})
	err := c.SystemConnect(context.Background(), "https://a4h.example.com", "001", "developer", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHost != "https://a4h.example.com" || gotUser != "developer" {
		t.Errorf("unexpected SAP headers: host=%q user=%q", gotHost, gotUser)
	}
}

func TestSystemConnect_PropagatesFailure(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":"invalid credentials"}`))
	})
	err := c.SystemConnect(context.Background(), "https://a4h.example.com", "001", "developer", "wrong")
	if err == nil {
		t.Fatal("expected error for failed connection, got nil")
	}
}

func TestGatewayVersion(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/abaper/version" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"features": "CLI and REST Services (No AI)"})
	})
	version, err := c.GatewayVersion()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version["features"] == "" {
		t.Errorf("unexpected version response: %+v", version)
	}
}
