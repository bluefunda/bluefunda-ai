package adt

import (
	"net/http"
	"strings"
	"testing"
)

const syntaxCheckErrorXML = `<?xml version="1.0" encoding="utf-8"?>
<checkRunReports xmlns="http://www.sap.com/adt/checkrun"><checkReport>
<checkMessageList><checkMessage uri="/sap/bc/adt/programs/programs/zfoo/source/main#start=4,9" type="E" shortText="Field &quot;X&quot; is unknown"/>
<checkMessage uri="/sap/bc/adt/programs/programs/zfoo/source/main#start=1,1" type="W" shortText="Unused variable"/>
</checkMessageList></checkReport></checkRunReports>`

func TestSyntaxCheck_ParsesMessagesAndLocation(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case strings.Contains(r.URL.Path, "/checkruns"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(syntaxCheckErrorXML))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	result, err := c.SyntaxCheck(t.Context(), "program", "ZFOO", "REPORT zfoo.\nWRITE x.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d: %+v", len(result.Messages), result.Messages)
	}
	if result.Messages[0].Severity != "error" || result.Messages[0].Line != 4 || result.Messages[0].Column != 9 {
		t.Errorf("unexpected first message: %+v", result.Messages[0])
	}
	if result.Messages[1].Severity != "warning" {
		t.Errorf("expected the second message to be a warning, got %+v", result.Messages[1])
	}
	// Writes the candidate source to the working copy (lock/PUT/unlock) before
	// checking, so SAP checks the provided source rather than the stored one.
	if len(log.calls) != 4 {
		t.Fatalf("expected lock, PUT, unlock, then checkruns POST — got %d: %v", len(log.calls), log.methods())
	}
}

func TestSyntaxCheck_NoMessagesMeansClean(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("_action") == "LOCK" {
			w.WriteHeader(http.StatusNotFound) // object doesn't exist locally; SyntaxCheck tolerates lock failure
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<checkRunReports xmlns="http://www.sap.com/adt/checkrun"/>`))
	})

	result, err := c.SyntaxCheck(t.Context(), "program", "ZFOO", "REPORT zfoo.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Messages) != 0 {
		t.Errorf("expected no messages, got %+v", result.Messages)
	}
}

const completionResponseXML = `<?xml version="1.0" encoding="utf-8"?>
<completionProposals xmlns="http://www.sap.com/adt/abapsource">
<proposal identifier="LT_TABLE" description="Local variable" kind="variable" insertText="lt_table"/>
<proposal identifier="WRITE" description="ABAP keyword" kind="keyword" insertText="WRITE"/>
</completionProposals>`

func TestGetCompletionProposals(t *testing.T) {
	var gotAccept, gotContentType string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(completionResponseXML))
	})

	proposals, err := c.GetCompletionProposals(t.Context(), "program", "ZFOO", "REPORT zfoo.\nDATA lt_table.\nWR", 3, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAccept != "application/vnd.sap.as+xml" {
		t.Errorf("unexpected Accept: %q", gotAccept)
	}
	if gotContentType != "text/plain; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", gotContentType)
	}
	if len(proposals) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(proposals))
	}
	if proposals[0].Kind != "variable" {
		t.Errorf("expected 'variable' kind mapped from 'variable', got %q", proposals[0].Kind)
	}
	if proposals[1].Kind != "keyword" {
		t.Errorf("expected default 'keyword' kind, got %q", proposals[1].Kind)
	}
}

func TestGetCompletionProposals_HTTPFailure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if _, err := c.GetCompletionProposals(t.Context(), "program", "ZFOO", "REPORT zfoo.", 1, 1); err == nil {
		t.Fatal("expected an error")
	}
}

const navigationResponseXML = `<?xml version="1.0" encoding="utf-8"?>
<objectReference xmlns="http://www.sap.com/adt/core" uri="/sap/bc/adt/programs/programs/zbar/source/main" name="ZBAR" type="PROG/P" line="10" column="3"/>`

func TestGetNavigationTarget(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(navigationResponseXML))
	})

	target, err := c.GetNavigationTarget(t.Context(), "program", "ZFOO", "REPORT zfoo.\nPERFORM foo IN PROGRAM zbar.", 2, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target == nil {
		t.Fatal("expected a non-nil navigation target")
	}
	if target.ObjectName != "ZBAR" || target.Line != 10 || target.Column != 3 {
		t.Errorf("unexpected target: %+v", target)
	}
}

func TestGetNavigationTarget_EmptyBodyMeansNoTarget(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	target, err := c.GetNavigationTarget(t.Context(), "program", "ZFOO", "REPORT zfoo.", 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != nil {
		t.Errorf("expected a nil target for an empty response body, got %+v", target)
	}
}

func TestFormatSource(t *testing.T) {
	var gotPath string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("REPORT zfoo.\n\nWRITE 'hello'."))
	})

	formatted, err := c.FormatSource(t.Context(), "report zfoo.\nwrite 'hello'.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/sap/bc/adt/abapsource/prettyprinter" {
		t.Errorf("unexpected path: %q", gotPath)
	}
	if formatted != "REPORT zfoo.\n\nWRITE 'hello'." {
		t.Errorf("unexpected formatted output: %q", formatted)
	}
}

func TestFormatSource_NotAuthenticated(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request should be made when not authenticated")
	})
	if _, err := c.FormatSource(t.Context(), "report zfoo."); err == nil {
		t.Fatal("expected an error")
	}
}
