package adt

import (
	"net/http"
	"testing"
)

// TestAuthenticate_HappyPath drives the full four-step handshake
// (testConnectivity HEAD, performLogin GET /discovery, getCSRFToken GET
// /discovery with X-CSRF-Token: Fetch, validateSession GET /discovery) and
// confirms the client ends up authenticated with the CSRF token captured.
func TestAuthenticate_HappyPath(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-CSRF-Token") == "Fetch" {
			w.Header().Set("X-CSRF-Token", "fresh-csrf-token")
		}
		w.WriteHeader(http.StatusOK)
	})

	if c.IsAuthenticated() {
		t.Fatal("expected client to start unauthenticated")
	}
	if err := c.Authenticate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !c.IsAuthenticated() {
		t.Fatal("expected client to be authenticated after Authenticate()")
	}
	if c.csrfToken != "fresh-csrf-token" {
		t.Errorf("expected csrfToken to be captured from the CSRF-fetch response, got %q", c.csrfToken)
	}
}

// TestAuthenticate_LoginRejected covers the invalid-credentials path: login
// (performLogin) returns 401 before any CSRF token is ever requested.
func TestAuthenticate_LoginRejected(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	err := c.Authenticate()
	if err == nil {
		t.Fatal("expected an error for rejected login")
	}
	if c.IsAuthenticated() {
		t.Error("client must not be marked authenticated after a failed login")
	}
}

// TestAuthenticate_CSRFTokenMissing covers a 200 response that omits the
// X-CSRF-Token header — getCSRFToken must fail rather than silently
// authenticate with an empty token (IsAuthenticated requires a non-empty one).
func TestAuthenticate_CSRFTokenMissing(t *testing.T) {
	loginCalls := 0
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		loginCalls++
		// Never sets X-CSRF-Token, even on the "Fetch" request.
		w.WriteHeader(http.StatusOK)
	})

	err := c.Authenticate()
	if err == nil {
		t.Fatal("expected an error when no CSRF token is returned")
	}
	if c.IsAuthenticated() {
		t.Error("client must not be marked authenticated without a CSRF token")
	}
	if loginCalls < 2 {
		t.Errorf("expected at least login+CSRF-fetch requests to have been made, got %d", loginCalls)
	}
}

// TestDoRequest_ReauthenticatesOn401 confirms doRequest transparently retries
// a request exactly once after a full re-Authenticate() when the server
// answers 401 mid-session (session expiry), per its documented contract.
func TestDoRequest_ReauthenticatesOn401(t *testing.T) {
	authAttempts := 0
	targetCalls := 0
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/sap/bc/adt/discovery" && r.Header.Get("X-CSRF-Token") == "Fetch":
			w.Header().Set("X-CSRF-Token", "reissued-csrf-token")
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/sap/bc/adt/discovery":
			authAttempts++
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/sap/bc/adt/programs/programs/ZFOO/source/main":
			targetCalls++
			if targetCalls == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("REPORT zfoo."))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})

	src, err := c.GetProgram(t.Context(), "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Source != "REPORT zfoo." {
		t.Errorf("expected source to be returned after re-auth retry, got %q", src.Source)
	}
	if targetCalls != 2 {
		t.Errorf("expected exactly 2 attempts at the target URL (1 401 + 1 retry), got %d", targetCalls)
	}
}
