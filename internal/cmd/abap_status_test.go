package cmd

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/bluefunda/bluefunda-ai/internal/abap"
)

// statusGateway answers the health check without a token and SAP connects
// only with one, like abaper-gw.
type statusGateway struct {
	mu         sync.Mutex
	healthAuth string
	connects   int
}

func (g *statusGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch r.URL.Path {
	case "/abaper/health":
		g.healthAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	case "/abaper/api/v1/system/connect":
		g.connects++
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func TestCollectABAPStatus_SessionStates(t *testing.T) {
	reg := &abap.Systems{Systems: []abap.SAPSystem{{ID: "s1", Name: "dev", Host: "https://sap.example.com"}}, Active: "s1"}
	valid := func() (string, error) { return "tok", nil }
	expired := func() (string, error) { return "", errors.New("token refresh failed — run `bai login`") }

	for _, tc := range []struct {
		name                 string
		token                func() (string, error) // nil → no stored session
		signedIn, expired    bool
		healthAuth           string
		systemChecked, sysUp bool
	}{
		{name: "signed in", token: valid, signedIn: true, healthAuth: "Bearer tok", systemChecked: true, sysUp: true},
		{name: "expired", token: expired, expired: true},
		{name: "not signed in"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &statusGateway{}
			srv := httptest.NewServer(g)
			t.Cleanup(srv.Close)
			var c *abap.Client
			if tc.token != nil {
				c = abap.NewClient(srv.URL, "individual", tc.token, reg.ActiveSystem())
			}

			got := collectABAPStatus(c, srv.URL, "individual", reg)
			if got.SignedIn != tc.signedIn || got.SessionExpired != tc.expired {
				t.Errorf("signedIn=%v sessionExpired=%v, want %v/%v", got.SignedIn, got.SessionExpired, tc.signedIn, tc.expired)
			}
			if !got.GatewayReachable || got.GatewayStatus != "ok" {
				t.Errorf("gateway reachable=%v status=%q — the health check needs no token", got.GatewayReachable, got.GatewayStatus)
			}
			if g.healthAuth != tc.healthAuth {
				t.Errorf("health Authorization = %q, want %q", g.healthAuth, tc.healthAuth)
			}
			if checked := g.connects > 0; checked != tc.systemChecked {
				t.Errorf("SAP system checked = %v, want %v", checked, tc.systemChecked)
			}
			if len(got.Systems) != 1 || got.Systems[0].Name != "dev" || !got.Systems[0].Active || got.Systems[0].Reachable != tc.sysUp {
				t.Errorf("systems = %+v", got.Systems)
			}
		})
	}
}
