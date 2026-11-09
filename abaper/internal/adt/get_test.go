package adt

import (
	"errors"
	"net/http"
	"testing"

	"github.com/bluefunda/abaper/types"
)

// TestGetSource_AllObjectTypes exercises every single-name Get* wrapper
// (they all funnel through the shared getSource helper) and confirms each
// hits its documented endpoint and returns the source body.
func TestGetSource_AllObjectTypes(t *testing.T) {
	cases := []struct {
		objectType string
		wantPath   string
		call       func(c *ADTClientImpl) (string, error)
	}{
		{"program", "/sap/bc/adt/programs/programs/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetProgram(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
		{"class", "/sap/bc/adt/oo/classes/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetClass(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
		{"include", "/sap/bc/adt/programs/includes/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetInclude(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
		{"interface", "/sap/bc/adt/oo/interfaces/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetInterface(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
		{"structure", "/sap/bc/adt/ddic/structures/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetStructure(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
		{"table", "/sap/bc/adt/ddic/tables/ZFOO/source/main", func(c *ADTClientImpl) (string, error) {
			r, err := c.GetTable(t.Context(), "zfoo")
			return srcOrEmpty(r), err
		}},
	}

	for _, tc := range cases {
		t.Run(tc.objectType, func(t *testing.T) {
			var gotPath string
			c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("ETag", "v1")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("REPORT zfoo."))
			})

			src, err := tc.call(c)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotPath != tc.wantPath {
				t.Errorf("expected path %q, got %q", tc.wantPath, gotPath)
			}
			if src != "REPORT zfoo." {
				t.Errorf("expected source body, got %q", src)
			}
		})
	}
}

func srcOrEmpty(src *types.ADTSourceCode) string {
	if src == nil {
		return ""
	}
	return src.Source
}

func TestGetSource_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetProgram(t.Context(), "ZMISSING")
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected errors.Is(err, ErrNotFound), got: %v", err)
	}
}

// TestGetSource_Unauthorized covers a 401 that survives doRequest's
// re-authenticate-and-retry: re-Authenticate() itself succeeds (the
// /discovery handshake is healthy), but the retried request still comes
// back 401 (e.g. missing authorization for this specific object) — getSource
// must surface its own "session may have expired" error rather than the
// zero-value success it would get by ignoring the status.
func TestGetSource_Unauthorized(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sap/bc/adt/discovery" {
			if r.Header.Get("X-CSRF-Token") == "Fetch" {
				w.Header().Set("X-CSRF-Token", "reissued-csrf-token")
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.GetProgram(t.Context(), "ZFOO")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestGetSource_NotAuthenticated(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request should be made when the client isn't authenticated")
	})
	_, err := c.GetProgram(t.Context(), "ZFOO")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestCheckObjectExists(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("REPORT zfoo."))
		})
		exists, err := c.CheckObjectExists(t.Context(), "PROGRAM", "ZFOO")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected exists=true")
		}
	})

	t.Run("does not exist", func(t *testing.T) {
		c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		exists, err := c.CheckObjectExists(t.Context(), "PROGRAM", "ZMISSING")
		if err != nil {
			t.Fatalf("expected no error for a not-found object, got: %v", err)
		}
		if exists {
			t.Error("expected exists=false")
		}
	})

	t.Run("unsupported type propagates as error, not false", func(t *testing.T) {
		c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("no HTTP request expected for an unsupported object type")
		})
		exists, err := c.CheckObjectExists(t.Context(), "DOMAIN", "ZFOO")
		if err == nil {
			t.Fatal("expected an error for an unsupported object type")
		}
		if exists {
			t.Error("expected exists=false alongside the error")
		}
	})
}
