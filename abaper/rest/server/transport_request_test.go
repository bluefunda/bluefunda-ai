package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bluefunda/bluefunda-ai/abaper/internal/adt"
	"github.com/bluefunda/bluefunda-ai/abaper/types"
	"go.uber.org/zap"
)

func TestTransportRequestHandler(t *testing.T) {
	t.Run("returns request with tasks and objects", func(t *testing.T) {
		var gotKey string
		rs := newTestServer(t, &fakeADTClient{
			getTransportRequestFn: func(_ context.Context, trkey string) (*types.ADTTransportRequest, error) {
				gotKey = trkey
				return &types.ADTTransportRequest{
					Number: trkey, Description: "Fix", Owner: "JDOE", Status: "D", Type: "K", Target: "QAS",
					Tasks: []types.ADTTransportTask{
						{Number: "DEVK900124", Owner: "JDOE", Description: "t1", Status: "D",
							Objects: []types.ADTTransportObject{{PgmID: "R3TR", Type: "CLAS", Name: "ZCL_X"}}},
						{Number: "DEVK900125", Owner: "ASMITH", Description: "t2", Status: "R",
							Objects: []types.ADTTransportObject{{PgmID: "R3TR", Type: "TABL", Name: "ZT"}, {PgmID: "LIMU", Type: "TABD", Name: "ZT"}}},
					},
				}, nil
			},
		})
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/transports/devk900123", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotKey != "DEVK900123" {
			t.Errorf("trkey passed to client = %q, want upper-cased", gotKey)
		}
		data := decodeSuccess[types.ADTTransportRequest](t, rec)
		if data.Number != "DEVK900123" || data.Target != "QAS" || len(data.Tasks) != 2 || len(data.Tasks[1].Objects) != 2 {
			t.Errorf("unexpected data: %+v", data)
		}
	})

	t.Run("bad trkey is 400 without calling SAP", func(t *testing.T) {
		for _, key := range []string{"NOTATRKEY", "DEVK12345", "DEVK9001234"} {
			called := false
			rs := newTestServer(t, &fakeADTClient{
				getTransportRequestFn: func(context.Context, string) (*types.ADTTransportRequest, error) {
					called = true
					return nil, nil
				},
			})
			rec := doJSON(t, rs, http.MethodGet, "/api/v1/transports/"+key, nil)
			if rec.Code != http.StatusBadRequest || called {
				t.Errorf("%s: got %d (called=%v), want 400 without SAP call", key, rec.Code, called)
			}
		}
	})

	t.Run("not found is 404", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{
			getTransportRequestFn: func(_ context.Context, trkey string) (*types.ADTTransportRequest, error) {
				return nil, fmt.Errorf("%w: transport request %s", adt.ErrNotFound, trkey)
			},
		})
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/transports/DEVK999999", nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("SAP error is 502", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{
			getTransportRequestFn: func(context.Context, string) (*types.ADTTransportRequest, error) {
				return nil, errors.New("get transport request failed: HTTP 403")
			},
		})
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/transports/DEVK900123", nil)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", rec.Code)
		}
	})

	t.Run("read-only: POST is 405", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/transports/DEVK900123", map[string]string{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("existing transport routes still win", func(t *testing.T) {
		rs := newTestServer(t, &fakeADTClient{})
		rec := doJSON(t, rs, http.MethodPost, "/api/v1/transports/info", map[string]string{
			"object_type": "program", "object_name": "zfoo",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("transports/info: expected 200, got %d", rec.Code)
		}
	})

	t.Run("no credentials and no static client is 401", func(t *testing.T) {
		rs := NewRestServerWithPool(&Config{}, zap.NewNop(), NewPool(0, zap.NewNop()), nil)
		rec := doJSON(t, rs, http.MethodGet, "/api/v1/transports/DEVK900123", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("uses the per-request X-SAP-* client", func(t *testing.T) {
		var sawTransportGET bool
		sap := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/sap/bc/adt/cts/transportrequests/DEVK900123" {
				sawTransportGET = r.Method == http.MethodGet
				_, _ = w.Write([]byte(`<tm:root xmlns:tm="http://www.sap.com/cts/adt/tm"><tm:request tm:number="DEVK900123" tm:owner="DEV" tm:desc="x" tm:status="D" tm:type="K" tm:target="QAS"/></tm:root>`))
				return
			}
			w.Header().Set("X-CSRF-Token", "tok")
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(sap.Close)
		// The static fallback must not be used when X-SAP-* headers are present.
		fallback := &fakeADTClient{getTransportRequestFn: func(context.Context, string) (*types.ADTTransportRequest, error) {
			t.Error("static fallback client used despite X-SAP-* headers")
			return nil, errors.New("fallback")
		}}
		rs := NewRestServerWithPool(&Config{}, zap.NewNop(), NewPool(0, zap.NewNop()), fallback)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/transports/DEVK900123", nil)
		req.Header.Set("X-SAP-Host", sap.URL)
		req.Header.Set("X-SAP-User", "DEV")
		req.Header.Set("X-SAP-Password", "pw")
		rec := httptest.NewRecorder()
		rs.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !sawTransportGET {
			t.Fatalf("status = %d (GET seen=%v): %s", rec.Code, sawTransportGET, rec.Body.String())
		}
	})
}
