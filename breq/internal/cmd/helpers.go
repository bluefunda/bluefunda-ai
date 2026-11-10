package cmd

import (
	"fmt"
	"os"

	"github.com/bluefunda/bluerequests/internal/auth"
	"github.com/bluefunda/bluerequests/internal/config"
	trmgrpc "github.com/bluefunda/bluerequests/internal/grpc"
	"github.com/bluefunda/bluerequests/internal/ui"
)

func saveAuthTokens(cfg *config.Config, tok *auth.TokenResponse) error {
	cfg.Auth.AccessToken = tok.AccessToken
	cfg.Auth.RefreshToken = tok.RefreshToken
	cfg.Auth.TokenExpiry = tok.Expiry()
	return config.Save(cfg)
}

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

	p.Success("Re-authenticated successfully.")
	return nil
}

func bffConn() (*trmgrpc.Conn, *config.Config, error) {
	cfg := loadConfig()
	if cfg.Auth.AccessToken == "" {
		return nil, cfg, fmt.Errorf("not authenticated; run 'breq login'")
	}

	refreshFunc := func() (string, error) {
		tok, err := auth.Refresh(cfg.Domain, cfg.Realm, cfg.Auth.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("token refresh failed (run 'breq login'): %w", err)
		}
		if err := saveAuthTokens(cfg, tok); err != nil {
			return "", fmt.Errorf("save tokens: %w", err)
		}
		return tok.AccessToken, nil
	}

	ts := trmgrpc.NewTokenSource(cfg, refreshFunc)
	conn, err := trmgrpc.Dial(cfg.BFFURL, ts)
	if err != nil {
		return nil, cfg, err
	}
	return conn, cfg, nil
}

func printer(cfg *config.Config) *ui.Printer {
	return &ui.Printer{
		Out:    os.Stdout,
		Err:    os.Stderr,
		Format: outputFormat(cfg),
	}
}
