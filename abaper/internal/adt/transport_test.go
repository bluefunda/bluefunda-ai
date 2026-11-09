package adt

import (
	"net/http"
	"strings"
	"testing"
)

// transportInfoXML has no XMLName field, so encoding/xml matches whatever
// the root element is called and then navigates its "DATA" child via the
// "DATA>OBJECTNAME" field paths — the root itself must not be named DATA.
const transportInfoXMLBody = `<?xml version="1.0" encoding="utf-8"?>
<abap><DATA><OBJECTNAME>ZFOO</OBJECTNAME><DEVCLASS>ZPKG</DEVCLASS>
<TRANSPORTS><item><TRKORR>Q4HK900123</TRKORR><AS4TEXT>Test transport</AS4TEXT><AS4USER>DEVELOPER</AS4USER><TRSTATUS>D</TRSTATUS><TRFUNCTION>K</TRFUNCTION><TARSYSTEM/><AS4DATE>20260101</AS4DATE></item></TRANSPORTS>
</DATA></abap>`

func TestGetTransportInfo(t *testing.T) {
	var gotPath string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(transportInfoXMLBody))
	})

	info, err := c.GetTransportInfo(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/sap/bc/adt/programs/programs/zfoo/transportinfo" {
		t.Errorf("unexpected path: %q", gotPath)
	}
	if info.Package != "ZPKG" || len(info.Transports) != 1 {
		t.Fatalf("unexpected result: %+v", info)
	}
	if info.Transports[0].Number != "Q4HK900123" {
		t.Errorf("unexpected transport entry: %+v", info.Transports[0])
	}
}

// TestGetTransportInfo_FallsBackToCTS covers SAP versions where the
// per-object /transportinfo endpoint doesn't exist (404): GetTransportInfo
// must fall back to POST /cts/transportchecks rather than erroring out.
func TestGetTransportInfo_FallsBackToCTS(t *testing.T) {
	const ctsResponseXML = `<?xml version="1.0" encoding="utf-8"?>
<asx:abap xmlns:asx="http://www.sap.com/abapxml"><asx:values><DATA>
<DEVCLASS>ZPKG</DEVCLASS>
<REQUESTS><CTS_REQUEST><REQ_HEADER><TRKORR>Q4HK900124</TRKORR><AS4TEXT>CTS fallback transport</AS4TEXT></REQ_HEADER></CTS_REQUEST></REQUESTS>
</DATA></asx:values></asx:abap>`

	var ctsCalled bool
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/transportinfo") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.Contains(r.URL.Path, "/cts/transportchecks") {
			ctsCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(ctsResponseXML))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	info, err := c.GetTransportInfo(t.Context(), "program", "ZFOO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctsCalled {
		t.Fatal("expected the CTS transportchecks fallback to be called")
	}
	if info.Package != "ZPKG" || len(info.Transports) != 1 || info.Transports[0].Number != "Q4HK900124" {
		t.Errorf("unexpected fallback result: %+v", info)
	}
}

// TestCreateTransport_EscapesDescription is a regression test: description
// and packageName used to be embedded raw (unescaped) into the request URL's
// query string, so any description containing a space or reserved query
// character (here "&") produced a malformed request line — Go's net/http
// server rejects that outright with its own 400 Bad Request, never reaching
// application logic at all.
func TestCreateTransport_EscapesDescription(t *testing.T) {
	var gotDescription string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotDescription = r.URL.Query().Get("description")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("Q4HK900127"))
	})

	if _, err := c.CreateTransport(t.Context(), "program", "ZFOO", "Fix bug & add feature", "ZPKG"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDescription != "Fix bug & add feature" {
		t.Errorf("expected the description to survive round-trip through query escaping, got %q", gotDescription)
	}
}

func TestCreateTransport(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("Q4HK900125"))
	})

	num, err := c.CreateTransport(t.Context(), "program", "ZFOO", "My change", "ZPKG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "/programs/programs/zfoo/transports") {
		t.Errorf("unexpected URL: %q", gotURL)
	}
	if num != "Q4HK900125" {
		t.Errorf("expected transport number from body, got %q", num)
	}
}

func TestCreateTransport_NumberFromLocationHeader(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/sap/bc/adt/cts/transportrequests/Q4HK900126")
		w.WriteHeader(http.StatusOK)
	})

	num, err := c.CreateTransport(t.Context(), "program", "ZFOO", "My change", "ZPKG")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != "Q4HK900126" {
		t.Errorf("expected transport number from Location header, got %q", num)
	}
}

func TestCreateTransport_Failure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("no authorization to create transports"))
	})
	if _, err := c.CreateTransport(t.Context(), "program", "ZFOO", "My change", "ZPKG"); err == nil {
		t.Fatal("expected an error")
	}
}
