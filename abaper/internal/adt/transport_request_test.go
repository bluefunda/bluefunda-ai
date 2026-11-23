package adt

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/bluefunda/bluefunda-ai/abaper/types"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestNormalizeTransportNumber(t *testing.T) {
	valid := map[string]string{
		"DEVK900123":   "DEVK900123",
		" devk900123 ": "DEVK900123",
		"S4HK000001":   "S4HK000001",
		"a4hk912345":   "A4HK912345",
	}
	for in, want := range valid {
		got, err := NormalizeTransportNumber(in)
		if err != nil || got != want {
			t.Errorf("NormalizeTransportNumber(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "DEVK90012", "DEVK9001234", "DEVX900123", "1EVK900123",
		"DEVK90012A", "DEV/K90012", "../K900123", "DEVK900123/../x", "DEVK 900123"} {
		if _, err := NormalizeTransportNumber(in); !errors.Is(err, ErrInvalidTransportNumber) {
			t.Errorf("NormalizeTransportNumber(%q): want ErrInvalidTransportNumber, got %v", in, err)
		}
	}
}

func TestGetTransportRequest(t *testing.T) {
	fixture := loadFixture(t, "transportrequest_two_tasks.xml")
	var gotPath, gotAccept, gotMethod, gotClient string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotClient = r.URL.Query().Get("sap-client")
		w.Header().Set("Content-Type", transportOrganizerMediaType)
		_, _ = w.Write(fixture)
	})

	tr, err := c.GetTransportRequest(t.Context(), "devk900123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET (read-only)", gotMethod)
	}
	if gotPath != "/sap/bc/adt/cts/transportrequests/DEVK900123" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "application/vnd.sap.adt.transportorganizer.v1+xml" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if gotClient != "100" {
		t.Errorf("sap-client = %q", gotClient)
	}

	if tr.Number != "DEVK900123" || tr.Owner != "JDOE" || tr.Description != "Sales order pricing fix" ||
		tr.Status != "D" || tr.Type != "K" || tr.Target != "QAS" {
		t.Errorf("unexpected header: %+v", tr)
	}
	if len(tr.Objects) != 1 || tr.Objects[0].Type != "TABU" || tr.Objects[0].Description != "Table Contents" {
		t.Errorf("unexpected request-level objects: %+v", tr.Objects)
	}
	if len(tr.Tasks) != 2 {
		t.Fatalf("want 2 tasks, got %d", len(tr.Tasks))
	}

	t1 := tr.Tasks[0]
	if t1.Number != "DEVK900124" || t1.Owner != "JDOE" || t1.Status != "D" || t1.Description != "Pricing class changes" {
		t.Errorf("unexpected task 1: %+v", t1)
	}
	wantT1 := []types.ADTTransportObject{
		{PgmID: "R3TR", Type: "CLAS", Name: "ZCL_SD_PRICING", Description: "SD pricing helper"},
		{PgmID: "LIMU", Type: "METH", Name: "ZCL_SD_PRICING                CALCULATE", Description: "Method"},
		{PgmID: "R3TR", Type: "PROG", Name: "ZSD_PRICING_REPORT", Description: "Pricing report"},
	}
	if len(t1.Objects) != len(wantT1) {
		t.Fatalf("task 1 objects: %+v", t1.Objects)
	}
	for i, o := range wantT1 {
		if t1.Objects[i] != o {
			t.Errorf("task 1 object %d = %+v, want %+v", i, t1.Objects[i], o)
		}
	}

	t2 := tr.Tasks[1]
	if t2.Number != "DEVK900125" || t2.Owner != "ASMITH" || t2.Status != "R" || t2.Description != "DDIC changes & config" {
		t.Errorf("unexpected task 2: %+v", t2)
	}
	if len(t2.Objects) != 4 {
		t.Fatalf("task 2 objects: %+v", t2.Objects)
	}
	types2 := []string{}
	for _, o := range t2.Objects {
		types2 = append(types2, o.PgmID+"/"+o.Type)
	}
	if strings.Join(types2, ",") != "R3TR/TABL,LIMU/TABD,R3TR/DDLS,R3TR/SUSH" {
		t.Errorf("task 2 object types = %v", types2)
	}
	if t2.Objects[3].Name != "81680085" || t2.Objects[3].Description != "" {
		t.Errorf("object without obj_desc/obj_info: %+v", t2.Objects[3])
	}
}

// TestGetTransportRequest_JSONShape pins the C-AI5 JSON contract.
func TestGetTransportRequest_JSONShape(t *testing.T) {
	tr, err := parseTransportRequestXML(loadFixture(t, "transportrequest_two_tasks.xml"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"number", "description", "owner", "status", "type", "target", "tasks", "objects"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %s", k, b)
		}
	}
	task := m["tasks"].([]any)[0].(map[string]any)
	for _, k := range []string{"number", "owner", "description", "status", "objects"} {
		if _, ok := task[k]; !ok {
			t.Errorf("missing task key %q", k)
		}
	}
	obj := task["objects"].([]any)[0].(map[string]any)
	for _, k := range []string{"pgmid", "type", "name", "description"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("missing object key %q", k)
		}
	}
}

func TestGetTransportRequest_EmptyTaskAndNoRequestObjects(t *testing.T) {
	const body = `<?xml version="1.0" encoding="utf-8"?>
<tm:root xmlns:tm="http://www.sap.com/cts/adt/tm">
  <tm:request tm:number="DEVK900200" tm:owner="JDOE" tm:desc="Empty" tm:type="W" tm:status="D" tm:target="QAS">
    <tm:task tm:number="DEVK900201" tm:owner="JDOE" tm:desc="Empty task" tm:status="D"/>
  </tm:request>
</tm:root>`
	tr, err := parseTransportRequestXML([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if tr.Objects != nil {
		t.Errorf("request objects should be omitted, got %+v", tr.Objects)
	}
	b, _ := json.Marshal(tr)
	if strings.Contains(string(b), `"objects":null`) {
		t.Errorf("task objects must be [] not null: %s", b)
	}
	if strings.Count(string(b), `"objects":[]`) != 1 {
		t.Errorf("want one empty task objects array: %s", b)
	}
}

func TestGetTransportRequest_InvalidNumberNoRequest(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no request expected, got %s %s", r.Method, r.URL.Path)
	})
	if _, err := c.GetTransportRequest(t.Context(), "DEVK900123/../../x"); !errors.Is(err, ErrInvalidTransportNumber) {
		t.Fatalf("want ErrInvalidTransportNumber, got %v", err)
	}
}

func TestGetTransportRequest_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`<exc:exception xmlns:exc="http://www.sap.com/abapxml/types/communicationframework"><message>Request DEVK999999 does not exist</message></exc:exception>`))
	})
	if _, err := c.GetTransportRequest(t.Context(), "DEVK999999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetTransportRequest_EmptyRootIsNotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<tm:root xmlns:tm="http://www.sap.com/cts/adt/tm"/>`))
	})
	if _, err := c.GetTransportRequest(t.Context(), "DEVK999999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetTransportRequest_ServerError(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("no authorization"))
	})
	_, err := c.GetTransportRequest(t.Context(), "DEVK900123")
	if err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("want HTTP 403 error, got %v", err)
	}
}

func TestGetTransportRequest_Unauthenticated(t *testing.T) {
	c, _ := newUnauthenticatedTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no request expected")
	})
	if _, err := c.GetTransportRequest(t.Context(), "DEVK900123"); err == nil {
		t.Fatal("expected an error")
	}
}
