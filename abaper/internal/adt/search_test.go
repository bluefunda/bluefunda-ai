package adt

import (
	"net/http"
	"strings"
	"testing"
)

// searchResultXML mirrors the real ADT repository search response: a
// namespace-qualified <objectReferences> root (encoding/xml's Unmarshal
// enforces the XMLName namespace declared on types.ADTSearchResult, so this
// must be exact) containing <objectReference> entries.
const searchResultXML = `<?xml version="1.0" encoding="utf-8"?>
<objectReferences xmlns="http://www.sap.com/adt/core" total="2">
<objectReference name="ZFOO" type="PROG/P" description="Foo program" packageName="$TMP"/>
<objectReference name="ZBAR" type="CLAS/OC" description="Bar class" packageName="$TMP"/>
</objectReferences>`

func TestSearchObjects(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchResultXML))
	})

	result, err := c.SearchObjects(t.Context(), "ZFOO*", []string{"PROG/P"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "/repository/informationsystem/search") {
		t.Errorf("expected the search endpoint to be hit, got %q", gotURL)
	}
	if !strings.Contains(gotURL, "objectType=PROG%2FP") {
		t.Errorf("expected objectType filter in query, got %q", gotURL)
	}
	if len(result.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(result.Objects))
	}
	if result.Objects[0].Name != "ZFOO" || result.Objects[0].Type != "PROG/P" {
		t.Errorf("unexpected first object: %+v", result.Objects[0])
	}
	if result.Total != 2 {
		t.Errorf("expected total=2, got %d", result.Total)
	}
}

func TestSearchObjects_NotAuthenticated(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request should be made when not authenticated")
	})
	if _, err := c.SearchObjects(t.Context(), "Z*", nil); err == nil {
		t.Fatal("expected an error")
	}
}

func TestListPackages(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchResultXML))
	})

	packages, err := c.ListPackages(t.Context(), "Z*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "objectType=DEVC/K") {
		t.Errorf("expected package-object-type filter DEVC/K, got %q", gotURL)
	}
	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}
}

func TestListPackages_DefaultsPatternToWildcard(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchResultXML))
	})
	if _, err := c.ListPackages(t.Context(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "query=%2A") {
		t.Errorf("expected an empty pattern to default to '*', got %q", gotURL)
	}
}

func TestGetPackageContents(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(searchResultXML))
	})

	pkg, err := c.GetPackageContents(t.Context(), "$TMP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "packageName=%24TMP") {
		t.Errorf("expected packageName filter in query, got %q", gotURL)
	}
	if len(pkg.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(pkg.Objects))
	}
}

func TestGetPackageContents_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetPackageContents(t.Context(), "ZMISSING"); err == nil {
		t.Fatal("expected an error for a missing package")
	}
}

func TestGetNodeContents(t *testing.T) {
	var gotMethod, gotURL, gotAccept string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotURL = r.URL.String()
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleNodeStructureXML))
	})

	result, err := c.GetNodeContents(t.Context(), "$TMP")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if !strings.Contains(gotURL, "/repository/nodestructure") || !strings.Contains(gotURL, "parent_name=%24TMP") {
		t.Errorf("unexpected URL: %q", gotURL)
	}
	if gotAccept != "application/vnd.sap.as+xml" {
		t.Errorf("expected the as+xml Accept header (application/xml is rejected with 406), got %q", gotAccept)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (virtual folder filtered out), got %d", len(result.Nodes))
	}
}

func TestGetNodeContents_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetNodeContents(t.Context(), "ZMISSING"); err == nil {
		t.Fatal("expected an error for a missing package")
	}
}
