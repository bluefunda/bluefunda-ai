package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bluefunda/bluefunda-ai/internal/auth"
	"github.com/bluefunda/bluefunda-ai/internal/config"
	caigrpc "github.com/bluefunda/bluefunda-ai/internal/grpc"
	"github.com/bluefunda/bluefunda-ai/internal/ui"
)

// loadContextFiles returns the combined contents of .bai/context.md (searching
// upward from cwd to the git root) and ~/.bai/instructions.md (user-level).
// Also checks AGENTS.md at each level for backward compatibility.
// Returns an empty string if no context files are found.
func loadContextFiles(cwd string) string {
	var parts []string

	if project := findContextFile(cwd); project != "" {
		parts = append(parts, project)
	}

	if home, err := os.UserHomeDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(home, ".bai", "instructions.md")); err == nil && len(b) > 0 {
			parts = append(parts, string(b))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

// findContextFile walks upward from dir until a git root, returning the contents
// of the first .bai/context.md or AGENTS.md it finds, or "" if none exist.
func findContextFile(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if b, err := os.ReadFile(filepath.Join(abs, ".bai", "context.md")); err == nil && len(b) > 0 {
			return string(b)
		}
		if b, err := os.ReadFile(filepath.Join(abs, "AGENTS.md")); err == nil && len(b) > 0 {
			return string(b)
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break // stop at git root
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break // filesystem root
		}
		abs = parent
	}
	return ""
}

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
		return nil, cfg, fmt.Errorf("not signed in — run `bai login`")
	}

	refreshFunc := func() (string, error) {
		tok, err := auth.Refresh(cfg.Domain, cfg.Realm, cfg.Auth.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("token refresh failed — run `bai login`: %w", err)
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

// gitRepoName returns the basename of the git repository root, or "" if not in a git repo.
func gitRepoName() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return filepath.Base(strings.TrimSpace(string(out)))
}

// printer returns a Printer configured from flags and config.
func printer(cfg *config.Config) *ui.Printer {
	return &ui.Printer{
		Out:    os.Stdout,
		Err:    os.Stderr,
		Format: outputFormat(cfg),
	}
}
