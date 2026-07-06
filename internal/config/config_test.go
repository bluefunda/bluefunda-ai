package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/keychain"
)

func TestTokenValid_Valid(t *testing.T) {
	cfg := &Config{
		Auth: Auth{
			AccessToken: "token",
			TokenExpiry: time.Now().Add(1 * time.Hour),
		},
	}
	if !cfg.TokenValid() {
		t.Error("expected token to be valid")
	}
}

func TestTokenValid_Expired(t *testing.T) {
	cfg := &Config{
		Auth: Auth{
			AccessToken: "token",
			TokenExpiry: time.Now().Add(-1 * time.Hour),
		},
	}
	if cfg.TokenValid() {
		t.Error("expected token to be invalid (expired)")
	}
}

func TestTokenValid_Empty(t *testing.T) {
	cfg := &Config{}
	if cfg.TokenValid() {
		t.Error("expected token to be invalid (empty)")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("expected gateway %s, got %s", DefaultGatewayURL, cfg.GatewayURL)
	}
	if cfg.BFFURL != DefaultBFFURL {
		t.Errorf("expected bff %s, got %s", DefaultBFFURL, cfg.BFFURL)
	}
	if cfg.Domain != DefaultDomain {
		t.Errorf("expected domain %s, got %s", DefaultDomain, cfg.Domain)
	}
	if cfg.Defaults.Model != "auto" {
		t.Errorf("expected model auto, got %s", cfg.Defaults.Model)
	}
	if cfg.Defaults.Output != "text" {
		t.Errorf("expected output text, got %s", cfg.Defaults.Output)
	}
}

func TestAuthURL_DefaultRealm(t *testing.T) {
	url := AuthURL("example.com", "")
	expected := "https://auth.example.com/realms/individual/protocol/openid-connect"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestAuthURL_CustomRealm(t *testing.T) {
	url := AuthURL("example.com", "individual")
	expected := "https://auth.example.com/realms/individual/protocol/openid-connect"
	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestDefaultConfig_Realm(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Realm != DefaultRealm {
		t.Errorf("expected realm %s, got %s", DefaultRealm, cfg.Realm)
	}
}

// ---- keychain integration tests ----

// mockKeychain is an in-memory keychain for config tests.
type mockKeychain struct {
	store     map[string]string
	available bool
}

func newMockKeychain(available bool) *mockKeychain {
	return &mockKeychain{store: make(map[string]string), available: available}
}

func (m *mockKeychain) Available() bool { return m.available }
func (m *mockKeychain) Set(k, v string) error {
	m.store[k] = v
	return nil
}
func (m *mockKeychain) Get(k string) (string, error) {
	v, ok := m.store[k]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (m *mockKeychain) Delete(k string) error {
	delete(m.store, k)
	return nil
}

func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestSave_KeychainAvailable(t *testing.T) {
	withTempHome(t)
	orig := keychain.Default
	mock := newMockKeychain(true)
	keychain.SetBackend(mock)
	defer keychain.SetBackend(orig)

	cfg := defaultConfig()
	cfg.Auth.AccessToken = "access-abc"
	cfg.Auth.RefreshToken = "refresh-xyz"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Tokens must be in keychain.
	if got, _ := mock.Get("access_token"); got != "access-abc" {
		t.Errorf("keychain access_token = %q, want %q", got, "access-abc")
	}
	if got, _ := mock.Get("refresh_token"); got != "refresh-xyz" {
		t.Errorf("keychain refresh_token = %q, want %q", got, "refresh-xyz")
	}

	// YAML file must not contain the tokens.
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".bai", "config.yaml"))
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if got := string(data); containsToken(got) {
		t.Errorf("config.yaml contains token data:\n%s", got)
	}
}

func TestSave_KeychainUnavailable(t *testing.T) {
	withTempHome(t)
	orig := keychain.Default
	mock := newMockKeychain(false)
	keychain.SetBackend(mock)
	defer keychain.SetBackend(orig)

	cfg := defaultConfig()
	cfg.Auth.AccessToken = "access-abc"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Keychain must be untouched.
	if _, err := mock.Get("access_token"); err == nil {
		t.Error("expected keychain to be empty, but found access_token")
	}

	// YAML must have the enc: prefixed token.
	home, _ := os.UserHomeDir()
	data, _ := os.ReadFile(filepath.Join(home, ".bai", "config.yaml"))
	if !containsEncPrefix(string(data)) {
		t.Errorf("config.yaml missing enc: prefix:\n%s", string(data))
	}
}

func TestClearTokens(t *testing.T) {
	cfg := &Config{
		Auth: Auth{
			AccessToken:  "tok",
			RefreshToken: "ref",
			TokenExpiry:  time.Now().Add(time.Hour),
		},
	}
	cfg.ClearTokens()
	if cfg.Auth.AccessToken != "" || cfg.Auth.RefreshToken != "" {
		t.Error("ClearTokens did not zero auth fields")
	}
	if !cfg.Auth.TokenExpiry.IsZero() {
		t.Error("ClearTokens did not zero TokenExpiry")
	}
}

func containsToken(s string) bool {
	return len(s) > 0 &&
		(contains(s, "access-abc") || contains(s, "refresh-xyz"))
}

func containsEncPrefix(s string) bool {
	return contains(s, "enc:")
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}()
}
