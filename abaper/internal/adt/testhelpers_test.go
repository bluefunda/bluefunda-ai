package adt

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluefunda/abaper/types"
	"go.uber.org/zap"
)

// newTestADTClient builds an ADTClientImpl already marked authenticated
// (authenticated=true, a non-empty csrfToken) and pointed at an
// httptest.Server, bypassing the full Authenticate() handshake so each test
// can focus on the method under test. Authenticate() itself is tested
// separately against an unauthenticated client.
func newTestADTClient(t *testing.T, handler http.HandlerFunc) (*ADTClientImpl, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &ADTClientImpl{
		config: &types.ADTConfig{
			Host:     ts.URL,
			Client:   "100",
			Language: "EN",
			Username: "testuser",
			Password: "testpass",
		},
		httpClient:    ts.Client(),
		logger:        zap.NewNop(),
		baseURL:       ts.URL + "/sap/bc/adt",
		authenticated: true,
		csrfToken:     "test-csrf-token",
		sessionType:   string(types.SessionStateful),
	}
	return c, ts
}

// newUnauthenticatedTestADTClient is like newTestADTClient but leaves
// authenticated=false and csrfToken empty, for testing the Authenticate()
// flow itself and the "not authenticated" guard on every public method.
func newUnauthenticatedTestADTClient(t *testing.T, handler http.HandlerFunc) (*ADTClientImpl, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	c := &ADTClientImpl{
		config: &types.ADTConfig{
			Host:     ts.URL,
			Client:   "100",
			Language: "EN",
			Username: "testuser",
			Password: "testpass",
		},
		httpClient:  ts.Client(),
		logger:      zap.NewNop(),
		baseURL:     ts.URL + "/sap/bc/adt",
		sessionType: string(types.SessionStateful),
	}
	return c, ts
}

// lockResponseXML is the literal ABAP-XML lock response shape (see
// parseLockResponse / LockResponse) SAP ADT returns from a LOCK action.
func lockResponseXML(lockHandle, corrNr string) string {
	return `<?xml version="1.0" encoding="utf-8"?><abap><values><DATA><LOCK_HANDLE>` +
		lockHandle + `</LOCK_HANDLE><CORR_NR>` + corrNr + `</CORR_NR></DATA></values></abap>`
}

// activationMessagesXML is the literal <messages><msg type=.../></messages>
// shape ActivateObject parses (distinct from the chkl:messages checklist
// format used by activateWithRetry/postActivationPayload).
func activationMessagesXML(severity, text string) string {
	if severity == "" {
		return `<?xml version="1.0" encoding="utf-8"?><messages/>`
	}
	return `<?xml version="1.0" encoding="utf-8"?><messages><msg type="` + severity +
		`"><shortText><txt>` + text + `</txt></shortText></msg></messages>`
}
