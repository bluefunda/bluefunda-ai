package adt

import (
	"net/http"
	"testing"
)

const unitTestRunResultXML = `<?xml version="1.0" encoding="utf-8"?>
<runResult xmlns="http://www.sap.com/adt/aunit">
<program name="ZCL_FOO" uri="/sap/bc/adt/oo/classes/zcl_foo">
<testClasses><testClass name="LTCL_TEST" uri="x">
<testMethods>
<testMethod name="test_passes" uri="x"/>
<testMethod name="test_fails" uri="x"><alerts><alert kind="failedAssertion" severity="critical"><title>Assertion failed</title><details><detail text="expected 1, got 2"/></details></alert></alerts></testMethod>
</testMethods>
</testClass></testClasses>
</program>
</runResult>`

func TestRunUnitTests_ParsesPassAndFail(t *testing.T) {
	var gotContentType, gotAccept string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(unitTestRunResultXML))
	})

	result, err := c.RunUnitTests(t.Context(), "class", "ZCL_FOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotContentType != "application/vnd.sap.adt.abapunit.testruns.config.v4+xml" {
		t.Errorf("unexpected Content-Type: %q", gotContentType)
	}
	if gotAccept != "application/vnd.sap.adt.abapunit.testruns.result.v2+xml" {
		t.Errorf("unexpected Accept: %q", gotAccept)
	}
	if result.TotalTests != 2 || result.Passed != 1 || result.Failed != 1 {
		t.Fatalf("unexpected counts: %+v", result)
	}
	if result.AllPassed {
		t.Error("expected AllPassed=false when a test failed")
	}
	if len(result.TestClasses) != 1 || len(result.TestClasses[0].Methods) != 2 {
		t.Fatalf("unexpected test class structure: %+v", result.TestClasses)
	}
	failed := result.TestClasses[0].Methods[1]
	if failed.Status != "failed" || failed.Message != "Assertion failed: expected 1, got 2" {
		t.Errorf("unexpected failed method result: %+v", failed)
	}
}

func TestRunUnitTests_AllPassedWhenNoFailures(t *testing.T) {
	const allPassXML = `<?xml version="1.0" encoding="utf-8"?>
<runResult xmlns="http://www.sap.com/adt/aunit">
<program name="ZCL_FOO" uri="x"><testClasses><testClass name="LTCL_TEST" uri="x">
<testMethods><testMethod name="test_ok" uri="x"/></testMethods>
</testClass></testClasses></program>
</runResult>`
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(allPassXML))
	})

	result, err := c.RunUnitTests(t.Context(), "class", "ZCL_FOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AllPassed || result.Failed != 0 || result.Passed != 1 {
		t.Errorf("unexpected result: %+v", result)
	}
}

func TestRunUnitTests_HTTPFailure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})
	if _, err := c.RunUnitTests(t.Context(), "class", "ZCL_FOO"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestRunUnitTests_EmptyObjectName(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no HTTP request expected for an empty object name")
	})
	if _, err := c.RunUnitTests(t.Context(), "class", "   "); err == nil {
		t.Fatal("expected an error")
	}
}
