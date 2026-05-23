package cmd

import (
	"fmt"
	"os"

	"github.com/bluefunda/bluefunda-ai/internal/auth"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

// saveAuthTokens persists the token response into cfg and saves to disk.
func saveAuthTokens(cfg *config.Config, tok *auth.TokenResponse) error {
	cfg.Auth.AccessToken = tok.AccessToken
	cfg.Auth.RefreshToken = tok.RefreshToken
	cfg.Auth.TokenExpiry = tok.Expiry()
	return config.Save(cfg)
}

// reAuthenticate performs an inline device-code login, updating cfg in place.
// Because cfg is shared with the TokenSource, the existing gRPC connection
// picks up the new tokens automatically — no reconnection needed.
func reAuthenticate(cfg *config.Config, p *ui.Printer) error {
	p.Warn("Session expired. Starting re-authentication...")
	p.Info("You will need to approve login in your browser.")

	tok, err := auth.LoginWithDevice(cfg.Domain, cfg.Realm)
	if err != nil {
		return fmt.Errorf("re-authentication failed: %w", err)
	}

	if err := saveAuthTokens(cfg, tok); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}

	p.Success("Re-authenticated successfully. Resuming chat.")
	return nil
}

// bffConn establishes an authenticated gRPC connection to the BFF.
// Caller must defer conn.Close().
func bffConn() (*caigrpc.Conn, *config.Config, error) {
	cfg := loadConfig()
	if cfg.Auth.AccessToken == "" {
		return nil, cfg, fmt.Errorf("not authenticated; run 'ai login'")
	}

	refreshFunc := func() (string, error) {
		tok, err := auth.Refresh(cfg.Domain, cfg.Realm, cfg.Auth.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("token refresh failed (run 'ai login'): %w", err)
		}
		if err := saveAuthTokens(cfg, tok); err != nil {
			return "", fmt.Errorf("save tokens: %w", err)
		}
		return tok.AccessToken, nil
	}

	ts := caigrpc.NewTokenSource(cfg, refreshFunc)
	conn, err := caigrpc.Dial(cfg.BFFURL, ts)
	if err != nil {
		return nil, cfg, err
	}
	return conn, cfg, nil
}

// printer returns a Printer configured from flags and config.
func printer(cfg *config.Config) *ui.Printer {
	return &ui.Printer{
		Out:    os.Stdout,
		Err:    os.Stderr,
		Format: outputFormat(cfg),
	}
}
