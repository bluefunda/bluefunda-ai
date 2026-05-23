package grpc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTokenSource_ValidToken(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: "valid-token",
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}
	ts := NewTokenSource(cfg, nil)

	token, err := ts.Token()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "valid-token" {
		t.Fatalf("expected 'valid-token', got %q", token)
	}
}

func TestTokenSource_ExpiredToken_NoRefresh(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: "expired-token",
			TokenExpiry: time.Now().Add(-1 * time.Hour),
		},
	}
	ts := NewTokenSource(cfg, nil)

	// With no refresh func but token exists, it should still return it
	// (the interceptor handles the retry on 401).
	token, err := ts.Token()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "expired-token" {
		t.Fatalf("expected 'expired-token', got %q", token)
	}
}

func TestTokenSource_ExpiredToken_WithRefresh(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: "expired-token",
			TokenExpiry: time.Now().Add(-1 * time.Hour),
		},
	}
	refreshCalled := false
	refreshFunc := func() (string, error) {
		refreshCalled = true
		cfg.Auth.AccessToken = "refreshed-token"
		cfg.Auth.TokenExpiry = time.Now().Add(1 * time.Hour)
		return "refreshed-token", nil
	}
	ts := NewTokenSource(cfg, refreshFunc)

	token, err := ts.Token()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !refreshCalled {
		t.Fatal("refresh func was not called")
	}
	if token != "refreshed-token" {
		t.Fatalf("expected 'refreshed-token', got %q", token)
	}
}

func TestTokenSource_NoToken(t *testing.T) {
	cfg := &config.Config{}
	ts := NewTokenSource(cfg, nil)

	_, err := ts.Token()
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestAttachToken(t *testing.T) {
	// Fake JWT: header.payload.signature
	// Payload: {"sub":"test-user-123"}
	testJWT := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ0ZXN0LXVzZXItMTIzIn0.dGVzdC1zaWc"

	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: testJWT,
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}
	ts := NewTokenSource(cfg, nil)

	ctx, err := attachToken(context.Background(), ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("no outgoing metadata found")
	}

	authValues := md.Get("authorization")
	if len(authValues) == 0 || authValues[0] != "Bearer "+testJWT {
		t.Fatalf("expected 'Bearer %s', got %q", testJWT, authValues)
	}

	realmValues := md.Get("x-realm")
	if len(realmValues) == 0 || realmValues[0] != "trm" {
		t.Fatalf("expected x-realm 'trm', got %q", realmValues)
	}

	userIDValues := md.Get("x-user-id")
	if len(userIDValues) == 0 || userIDValues[0] != "test-user-123" {
		t.Fatalf("expected x-user-id 'test-user-123', got %q", userIDValues)
	}
}

func TestDial_EmptyTarget(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: "token",
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}
	ts := NewTokenSource(cfg, nil)

	_, err := Dial("", ts)
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestContextWithTimeout(t *testing.T) {
	ctx, cancel := ContextWithTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected context to have deadline")
	}
	remaining := time.Until(deadline)
	if remaining < 29*time.Second || remaining > 31*time.Second {
		t.Fatalf("expected ~30s timeout, got %v", remaining)
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"unauthenticated", status.Error(codes.Unauthenticated, "bad token"), true},
		{"permission denied", status.Error(codes.PermissionDenied, "forbidden"), true},
		{"not found", status.Error(codes.NotFound, "not found"), false},
		{"internal", status.Error(codes.Internal, "internal"), false},
		{"token refresh failed message", fmt.Errorf("token refresh failed (run 'ai login'): expired"), true},
		{"not authenticated message", fmt.Errorf("not authenticated; run 'ai login'"), true},
		{"generic error", fmt.Errorf("connection refused"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAuthError(tt.err)
			if got != tt.want {
				t.Errorf("IsAuthError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestTokenSource_NearExpiry(t *testing.T) {
	tests := []struct {
		name   string
		expiry time.Time
		within time.Duration
		want   bool
	}{
		{"well before expiry", time.Now().Add(1 * time.Hour), 2 * time.Minute, false},
		{"near expiry", time.Now().Add(1 * time.Minute), 2 * time.Minute, true},
		{"already expired", time.Now().Add(-1 * time.Minute), 2 * time.Minute, true},
		{"zero expiry", time.Time{}, 2 * time.Minute, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Auth: config.Auth{
					AccessToken: "tok",
					TokenExpiry: tt.expiry,
				},
			}
			ts := NewTokenSource(cfg, nil)
			got := ts.NearExpiry(tt.within)
			if got != tt.want {
				t.Errorf("NearExpiry(%v) = %v, want %v", tt.within, got, tt.want)
			}
		})
	}
}

func TestStreamInterceptor_RetryOnUnauthenticated(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			// Fake JWT with sub claim: {"sub":"u1"}
			AccessToken: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1MSJ9.c2ln",
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}

	refreshCalled := false
	refreshFunc := func() (string, error) {
		refreshCalled = true
		cfg.Auth.AccessToken = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1MSJ9.c2ln"
		cfg.Auth.TokenExpiry = time.Now().Add(1 * time.Hour)
		return cfg.Auth.AccessToken, nil
	}
	ts := NewTokenSource(cfg, refreshFunc)
	interceptor := authStreamInterceptor(ts)

	callCount := 0
	fakeStreamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		callCount++
		if callCount == 1 {
			return nil, status.Error(codes.Unauthenticated, "token expired")
		}
		return nil, nil // success on retry
	}

	_, _ = interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/test", fakeStreamer)

	if !refreshCalled {
		t.Fatal("expected refresh to be called")
	}
	if callCount != 2 {
		t.Fatalf("expected streamer to be called 2 times, got %d", callCount)
	}
}

func TestEnsureValidToken(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			AccessToken: "tok",
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}
	ts := NewTokenSource(cfg, nil)

	if err := ts.EnsureValidToken(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With no token and no refresh, should fail
	cfg2 := &config.Config{}
	ts2 := NewTokenSource(cfg2, nil)
	if err := ts2.EnsureValidToken(); err == nil {
		t.Fatal("expected error for missing token")
	}
}
