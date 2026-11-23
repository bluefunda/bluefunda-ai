package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// deviceServer answers the device endpoint, rejecting any scope in reject
// with invalid_scope, and records the scopes it was asked for.
func deviceServer(t *testing.T, reject string) (*httptest.Server, *[]string) {
	t.Helper()
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		scope := r.PostForm.Get("scope")
		asked = append(asked, scope)
		if reject != "" && strings.Contains(scope, reject) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_scope","error_description":"Invalid scopes: ` + scope + `"}`))
			return
		}
		_, _ = w.Write([]byte(`{"device_code":"d","user_code":"U-1","verification_uri":"https://x","expires_in":60,"interval":5}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

func TestRequestDeviceCode_AsksForOrganization(t *testing.T) {
	srv, asked := deviceServer(t, "")
	dev, err := requestDeviceCode(srv.URL)
	if err != nil || dev.UserCode != "U-1" {
		t.Fatalf("dev = %+v, err = %v", dev, err)
	}
	if len(*asked) != 1 || (*asked)[0] != loginScope || !strings.Contains(loginScope, "organization") {
		t.Errorf("asked %q", *asked)
	}
}

// A client that isn't allowed the extra scopes still logs in, with openid.
func TestRequestDeviceCode_FallsBackOnInvalidScope(t *testing.T) {
	srv, asked := deviceServer(t, "organization")
	dev, err := requestDeviceCode(srv.URL)
	if err != nil || dev == nil {
		t.Fatalf("err = %v", err)
	}
	if len(*asked) != 2 || (*asked)[1] != minimalScope {
		t.Errorf("asked %q, want the full scope then openid", *asked)
	}
}

// Other failures are reported, not retried.
func TestRequestDeviceCode_OtherErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized_client"}`))
	}))
	t.Cleanup(srv.Close)
	if _, err := requestDeviceCode(srv.URL); err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Errorf("err = %v", err)
	}
}
