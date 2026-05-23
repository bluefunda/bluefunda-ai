package grpc

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/bluefunda/bluefunda-ai/api/proto/bff"
	"github.com/bluefunda/bluefunda-ai/internal/config"
)

// DefaultTimeout for unary RPCs.
const DefaultTimeout = 30 * time.Second

// PingTimeout for health checks.
const PingTimeout = 5 * time.Second

// Ping checks gRPC connectivity to the target without authentication.
func Ping(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), PingTimeout)
	defer cancel()

	cc, err := grpc.NewClient(target, transportCreds(target))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer cc.Close()

	cc.Connect()
	for {
		state := cc.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if !cc.WaitForStateChange(ctx, state) {
			return fmt.Errorf("unreachable (state: %s)", state)
		}
	}
}

// TokenSource provides access tokens for gRPC metadata injection.
// It handles token refresh transparently.
type TokenSource struct {
	cfg         *config.Config
	refreshFunc func() (string, error)
	mu          sync.Mutex
}

// NewTokenSource creates a TokenSource that reads and refreshes tokens from config.
func NewTokenSource(cfg *config.Config, refreshFunc func() (string, error)) *TokenSource {
	return &TokenSource{cfg: cfg, refreshFunc: refreshFunc}
}

// Token returns the current access token, refreshing if expired.
func (ts *TokenSource) Token() (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.cfg.TokenValid() {
		return ts.cfg.Auth.AccessToken, nil
	}

	if ts.refreshFunc == nil {
		if ts.cfg.Auth.AccessToken != "" {
			return ts.cfg.Auth.AccessToken, nil
		}
		return "", fmt.Errorf("not signed in — run `bai login`")
	}

	newToken, err := ts.refreshFunc()
	if err != nil {
		return "", fmt.Errorf("token refresh failed (run `bai login`): %w", err)
	}
	return newToken, nil
}

// NearExpiry reports whether the token expires within the given duration.
func (ts *TokenSource) NearExpiry(within time.Duration) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.cfg.Auth.TokenExpiry.IsZero() {
		return true
	}
	return time.Until(ts.cfg.Auth.TokenExpiry) < within
}

// EnsureValidToken forces a token refresh if needed, returning only the error.
func (ts *TokenSource) EnsureValidToken() error {
	_, err := ts.Token()
	return err
}

// IsAuthError reports whether err represents an authentication/authorization failure.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	code := status.Code(err)
	if code == codes.Unauthenticated || code == codes.PermissionDenied {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "token refresh failed") ||
		strings.Contains(msg, "not authenticated")
}

// Conn wraps a gRPC client connection and provides the BFF service client.
type Conn struct {
	cc     *grpc.ClientConn
	Client pb.BFFServiceClient
	TS     *TokenSource
}

// Dial establishes a gRPC connection to the BFF service.
// It configures auth metadata injection and default timeouts.
// TLS is used automatically for remote targets; localhost uses plaintext.
func Dial(target string, ts *TokenSource, opts ...grpc.DialOption) (*Conn, error) {
	if target == "" {
		return nil, fmt.Errorf("endpoint not configured — run `bai login`")
	}

	defaults := []grpc.DialOption{
		transportCreds(target),
		grpc.WithUnaryInterceptor(authUnaryInterceptor(ts)),
		grpc.WithStreamInterceptor(authStreamInterceptor(ts)),
	}
	allOpts := append(defaults, opts...)

	cc, err := grpc.NewClient(target, allOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", target, err)
	}
	return &Conn{
		cc:     cc,
		Client: pb.NewBFFServiceClient(cc),
		TS:     ts,
	}, nil
}

// transportCreds returns TLS credentials for remote targets and
// plaintext (insecure) credentials for localhost/loopback addresses.
func transportCreds(target string) grpc.DialOption {
	host := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		host = h
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{}))
}

// Close closes the underlying gRPC connection.
func (c *Conn) Close() error {
	return c.cc.Close()
}

// authUnaryInterceptor injects the Bearer token into unary RPC metadata.
// On Unauthenticated errors, it refreshes the token and retries once.
func authUnaryInterceptor(ts *TokenSource) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx, err := attachToken(ctx, ts)
		if err != nil {
			return err
		}

		err = invoker(ctx, method, req, reply, cc, opts...)
		if err == nil {
			return nil
		}

		// Retry once on Unauthenticated if we can refresh.
		if status.Code(err) == codes.Unauthenticated && ts.refreshFunc != nil {
			ts.mu.Lock()
			newToken, refreshErr := ts.refreshFunc()
			if refreshErr == nil {
				ts.cfg.Auth.AccessToken = newToken
			}
			ts.mu.Unlock()
			if refreshErr != nil {
				return err
			}
			ctx, err = attachToken(ctx, ts)
			if err != nil {
				return err
			}
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		return err
	}
}

// authStreamInterceptor injects the Bearer token into streaming RPC metadata.
// On Unauthenticated errors, it refreshes the token and retries once.
func authStreamInterceptor(ts *TokenSource) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx, err := attachToken(ctx, ts)
		if err != nil {
			return nil, err
		}

		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err == nil {
			return cs, nil
		}

		// Retry once on Unauthenticated if we can refresh.
		if status.Code(err) == codes.Unauthenticated && ts.refreshFunc != nil {
			ts.mu.Lock()
			newToken, refreshErr := ts.refreshFunc()
			if refreshErr == nil {
				ts.cfg.Auth.AccessToken = newToken
			}
			ts.mu.Unlock()
			if refreshErr != nil {
				return nil, err
			}
			ctx, err = attachToken(ctx, ts)
			if err != nil {
				return nil, err
			}
			return streamer(ctx, desc, cc, method, opts...)
		}
		return nil, err
	}
}

// attachToken reads the current token, extracts the user ID (sub claim) and
// realm from the JWT, and attaches authorization, realm, and user ID as gRPC metadata.
func attachToken(ctx context.Context, ts *TokenSource) (context.Context, error) {
	token, err := ts.Token()
	if err != nil {
		return ctx, err
	}
	claims, err := claimsFromJWT(token)
	if err != nil {
		return ctx, fmt.Errorf("extract claims from token: %w", err)
	}
	realm := realmFromIssuer(claims.Iss)
	md := metadata.Pairs(
		"authorization", "Bearer "+token,
		"x-realm", realm,
		"x-user-id", claims.Sub,
	)
	return metadata.NewOutgoingContext(ctx, md), nil
}

// jwtClaims holds the JWT claims needed for gRPC metadata.
type jwtClaims struct {
	Sub string `json:"sub"`
	Iss string `json:"iss"`
}

// claimsFromJWT decodes the JWT payload and returns sub and iss claims.
// No signature verification — the BFF validates the token.
func claimsFromJWT(token string) (*jwtClaims, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", err)
	}
	if claims.Sub == "" {
		return nil, fmt.Errorf("JWT missing sub claim")
	}
	return &claims, nil
}

// realmFromIssuer extracts the realm name from a Keycloak issuer URL.
// Expected format: https://auth.example.com/realms/{realm}
// Falls back to DefaultRealm if parsing fails.
func realmFromIssuer(issuer string) string {
	const marker = "/realms/"
	idx := strings.Index(issuer, marker)
	if idx == -1 {
		return config.DefaultRealm
	}
	remaining := issuer[idx+len(marker):]
	if slashIdx := strings.Index(remaining, "/"); slashIdx != -1 {
		remaining = remaining[:slashIdx]
	}
	if remaining == "" {
		return config.DefaultRealm
	}
	return remaining
}

// ContextWithTimeout returns a context with the default unary timeout.
func ContextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}
