package sdk

import (
	"testing"
	"time"

	"github.com/bluefunda/abaper/types"
)

func TestNew_RequiredFields(t *testing.T) {
	cases := []struct {
		name string
		opts Options
	}{
		{"missing host", Options{Username: "dev", Password: "pw", SkipConnect: true}},
		{"missing username", Options{Host: "https://h", Password: "pw", SkipConnect: true}},
		{"missing password", Options{Host: "https://h", Username: "dev", SkipConnect: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(tc.opts)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if client != nil {
				t.Errorf("expected nil client on error, got %v", client)
			}
		})
	}
}

func TestNew_SkipConnectReturnsClient(t *testing.T) {
	client, err := New(Options{
		Host:            "https://sap.example.com:44300",
		Username:        "developer",
		Password:        "secret",
		AllowSelfSigned: true,
		SkipConnect:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
	// New returns the public types.ADTClient interface directly.
	if client.IsAuthenticated() {
		t.Error("expected client to be unauthenticated when SkipConnect is set")
	}
}

func TestNewFromConfig(t *testing.T) {
	client := NewFromConfig(&types.ADTConfig{
		Host:     "https://sap.example.com:44300",
		Client:   "001",
		Username: "developer",
		Password: "secret",
	})
	if client == nil {
		t.Fatal("expected a non-nil client")
	}
}

func TestOrDefault(t *testing.T) {
	if got := orDefault("", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
	if got := orDefault("value", "fallback"); got != "value" {
		t.Errorf("expected value, got %q", got)
	}
}

func TestNew_DefaultsDoNotError(t *testing.T) {
	// Exercises the default-application path (client/language/timeouts) without
	// a live connection.
	_, err := New(Options{
		Host:           "https://sap.example.com:44300",
		Username:       "developer",
		Password:       "secret",
		ConnectTimeout: 0,
		RequestTimeout: 0,
		SkipConnect:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error applying defaults: %v", err)
	}
	// Sanity: non-zero durations are valid inputs too.
	_, err = New(Options{
		Host:           "https://sap.example.com:44300",
		Username:       "developer",
		Password:       "secret",
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 10 * time.Second,
		SkipConnect:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error with explicit timeouts: %v", err)
	}
}
