package adt

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bluefunda/abaper/types"
)

// TestCreateServiceBinding_FullSequence: unlike Domain/DataElement,
// CreateServiceBinding sends the full payload in one create-metadata POST
// (no separate shell+lock+PUT) and activates directly afterward — only two
// requests total.
func TestCreateServiceBinding_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/businessservices/bindings":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistSuccessBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateServiceBinding(t.Context(), "ZFOO_BINDING", types.ServiceBindingProperties{
		Description:           "Test binding",
		ServiceDefinitionName: "ZFOO_SRV",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 2 {
		t.Fatalf("expected exactly 2 requests (create + activate, no lock/PUT/unlock), got %d: %v", len(log.calls), log.methods())
	}
}

func TestCreateServiceBinding_ActivationFailure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/businessservices/bindings":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistErrorBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.CreateServiceBinding(t.Context(), "ZFOO_BINDING", types.ServiceBindingProperties{
		ServiceDefinitionName: "ZFOO_SRV",
	})
	if err == nil {
		t.Fatal("expected an error for a failed activation checklist")
	}
}

// TestUpdateServiceBinding_FullSequence: unlike Create, Update goes through
// setPropertiesAndActivate (lock -> PUT -> unlock -> activate), the same
// pattern as Domain/DataElement updates.
func TestUpdateServiceBinding_FullSequence(t *testing.T) {
	log := &requestLog{t: t}
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch {
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "LOCK":
			_, _ = w.Write([]byte(lockResponseXML("lock-handle-1", "")))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Query().Get("_action") == "UNLOCK":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/sap/bc/adt/activation":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(activationChecklistSuccessBody))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	err := c.UpdateServiceBinding(t.Context(), "ZFOO_BINDING", types.ServiceBindingProperties{
		ServiceDefinitionName: "ZFOO_SRV",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(log.calls) != 4 {
		t.Fatalf("expected 4 requests (lock, PUT, unlock, activate), got %d: %v", len(log.calls), log.methods())
	}
}

const serviceBindingReadXML = `<?xml version="1.0" encoding="utf-8"?>
<serviceBinding xmlns="http://www.sap.com/adt/ddic/ServiceBindings" name="ZFOO_BINDING" description="Test binding" version="active" published="true">
<services><content version="1"><serviceDefinition name="ZFOO_SRV"/></content></services>
<binding category="0" version="V4"/>
</serviceBinding>`

func TestGetServiceBinding(t *testing.T) {
	var gotAccept string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(serviceBindingReadXML))
	})

	binding, err := c.GetServiceBinding(t.Context(), "zfoo_binding")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAccept != "application/vnd.sap.adt.businessservices.servicebinding.v2+xml" {
		t.Errorf("unexpected Accept header: %q", gotAccept)
	}
	if binding.Description != "Test binding" || binding.ServiceDefinitionName != "ZFOO_SRV" {
		t.Errorf("unexpected parsed binding: %+v", binding)
	}
	if !binding.Published {
		t.Error("expected Published=true")
	}
	if binding.BindingCategory != "0" || binding.BindingVersion != "V4" {
		t.Errorf("unexpected binding category/version: %+v", binding)
	}
}

func TestGetServiceBinding_NotFound(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := c.GetServiceBinding(t.Context(), "ZMISSING"); err == nil {
		t.Fatal("expected an error for a missing service binding")
	}
}

const servicePublishSuccessXML = `<?xml version="1.0" encoding="utf-8"?><abap><values><DATA>` +
	`<SEVERITY>OK</SEVERITY><SHORT_TEXT>Service published successfully</SHORT_TEXT><LONG_TEXT></LONG_TEXT>` +
	`</DATA></values></abap>`

func TestPublishServiceBinding(t *testing.T) {
	var gotURL string
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(servicePublishSuccessXML))
	})

	result, err := c.PublishServiceBinding(t.Context(), "zfoo_binding")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotURL, "/businessservices/odatav4/publishjobs") {
		t.Errorf("expected the publishjobs endpoint, got %q", gotURL)
	}
	if result.Severity != "OK" || result.ShortText != "Service published successfully" {
		t.Errorf("unexpected publish result: %+v", result)
	}
}

func TestPublishServiceBinding_Failure(t *testing.T) {
	c, _ := newTestADTClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("binding is not active"))
	})
	if _, err := c.PublishServiceBinding(t.Context(), "ZFOO_BINDING"); err == nil {
		t.Fatal("expected an error when the binding isn't active yet")
	}
}
