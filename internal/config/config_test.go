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

// --- applyEnvOverrides Tests ---

func TestApplyEnvOverrides_AllVars(t *testing.T) {
	t.Setenv("BAI_GATEWAY", "https://gw.test.com")
	t.Setenv("BAI_BFF", "bff.test.com:443")
	t.Setenv("BAI_DOMAIN", "test.com")
	t.Setenv("BAI_REALM", "testrealm")
	t.Setenv("BAI_MODEL", "testmodel")

	cfg := &Config{}
	applyEnvOverrides(cfg)

	if cfg.GatewayURL != "https://gw.test.com" {
		t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, "https://gw.test.com")
	}
	if cfg.BFFURL != "bff.test.com:443" {
		t.Errorf("BFFURL = %q, want %q", cfg.BFFURL, "bff.test.com:443")
	}
	if cfg.Domain != "test.com" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "test.com")
	}
	if cfg.Realm != "testrealm" {
		t.Errorf("Realm = %q, want %q", cfg.Realm, "testrealm")
	}
	if cfg.Defaults.Model != "testmodel" {
		t.Errorf("Model = %q, want %q", cfg.Defaults.Model, "testmodel")
	}
}

func TestApplyEnvOverrides_AccessToken(t *testing.T) {
	t.Setenv("BAI_ACCESS_TOKEN", "mytoken")

	cfg := &Config{}
	applyEnvOverrides(cfg)

	if cfg.Auth.AccessToken != "mytoken" {
		t.Errorf("AccessToken = %q, want %q", cfg.Auth.AccessToken, "mytoken")
	}
	if cfg.Auth.TokenExpiry.IsZero() {
		t.Error("expected TokenExpiry to be set when BAI_ACCESS_TOKEN is provided")
	}
}

func TestApplyEnvOverrides_NoVarsSet(t *testing.T) {
	// Unset any lingering env vars that might be set by other tests.
	for _, k := range []string{"BAI_GATEWAY", "BAI_BFF", "BAI_DOMAIN", "BAI_REALM", "BAI_MODEL", "BAI_ACCESS_TOKEN"} {
		t.Setenv(k, "")
	}
	cfg := &Config{GatewayURL: "original"}
	applyEnvOverrides(cfg)
	if cfg.GatewayURL != "original" {
		t.Errorf("expected no override, GatewayURL = %q", cfg.GatewayURL)
	}
}

// --- MergeProject Tests ---

func TestMergeProject_ModelOverride(t *testing.T) {
	cfg := &Config{Defaults: Defaults{Model: "auto"}}
	p := &ProjectConfig{Model: "anthropic"}
	cfg.mergeProject(p)
	if cfg.Defaults.Model != "anthropic" {
		t.Errorf("expected model 'anthropic', got %q", cfg.Defaults.Model)
	}
}

func TestMergeProject_EndpointOverride(t *testing.T) {
	cfg := &Config{BFFURL: "default.com:443"}
	p := &ProjectConfig{Endpoint: "project.com:443"}
	cfg.mergeProject(p)
	if cfg.BFFURL != "project.com:443" {
		t.Errorf("expected endpoint 'project.com:443', got %q", cfg.BFFURL)
	}
}

func TestMergeProject_EmptyDoesNotOverride(t *testing.T) {
	cfg := &Config{Defaults: Defaults{Model: "auto"}, BFFURL: "orig.com:443"}
	p := &ProjectConfig{}
	cfg.mergeProject(p)
	if cfg.Defaults.Model != "auto" {
		t.Errorf("empty project model should not override, got %q", cfg.Defaults.Model)
	}
	if cfg.BFFURL != "orig.com:443" {
		t.Errorf("empty project endpoint should not override, got %q", cfg.BFFURL)
	}
}

// --- FindProjectConfig Tests ---

func TestFindProjectConfig_Found(t *testing.T) {
	root := t.TempDir()
	baiDir := filepath.Join(root, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "model: gpt-4\nmax_turns: 10\n"
	if err := os.WriteFile(filepath.Join(baiDir, "settings.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := FindProjectConfig(root)
	if p == nil {
		t.Fatal("expected non-nil ProjectConfig")
	}
	if p.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got %q", p.Model)
	}
	if p.MaxTurns != 10 {
		t.Errorf("expected max_turns 10, got %d", p.MaxTurns)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := FindProjectConfig(root)
	if p != nil {
		t.Errorf("expected nil ProjectConfig, got %+v", p)
	}
}

func TestFindProjectConfig_WalksUp(t *testing.T) {
	root := t.TempDir()
	baiDir := filepath.Join(root, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baiDir, "settings.yaml"), []byte("model: parent-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "pkg", "mypackage")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	p := FindProjectConfig(sub)
	if p == nil {
		t.Fatal("expected non-nil ProjectConfig when walking up")
	}
	if p.Model != "parent-model" {
		t.Errorf("expected 'parent-model', got %q", p.Model)
	}
}

// --- Load Tests ---

func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	withTempHome(t)
	// Unset env overrides so we get pure defaults.
	for _, k := range []string{"BAI_GATEWAY", "BAI_BFF", "BAI_DOMAIN", "BAI_REALM", "BAI_MODEL", "BAI_ACCESS_TOKEN"} {
		t.Setenv(k, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL = %q, want %q", cfg.GatewayURL, DefaultGatewayURL)
	}
	if cfg.BFFURL != DefaultBFFURL {
		t.Errorf("BFFURL = %q, want %q", cfg.BFFURL, DefaultBFFURL)
	}
}

func TestLoad_BackfillsMissingFields(t *testing.T) {
	withTempHome(t)
	for _, k := range []string{"BAI_GATEWAY", "BAI_BFF", "BAI_DOMAIN", "BAI_REALM", "BAI_MODEL", "BAI_ACCESS_TOKEN"} {
		t.Setenv(k, "")
	}
	home, _ := os.UserHomeDir()
	baiDir := filepath.Join(home, ".bai")
	if err := os.MkdirAll(baiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a minimal config file with missing fields.
	if err := os.WriteFile(filepath.Join(baiDir, "config.yaml"), []byte("realm: myrealm\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	orig := keychain.Default
	keychain.SetBackend(newMockKeychain(false))
	defer keychain.SetBackend(orig)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GatewayURL != DefaultGatewayURL {
		t.Errorf("GatewayURL not backfilled: %q", cfg.GatewayURL)
	}
	if cfg.BFFURL != DefaultBFFURL {
		t.Errorf("BFFURL not backfilled: %q", cfg.BFFURL)
	}
	if cfg.Domain != DefaultDomain {
		t.Errorf("Domain not backfilled: %q", cfg.Domain)
	}
	if cfg.Realm != "myrealm" {
		t.Errorf("Realm should be preserved: %q", cfg.Realm)
	}
}

// --- RawAccessToken Tests ---

func TestRawAccessToken_Present(t *testing.T) {
	data := []byte("auth:\n  access_token: enc:abc123\n")
	got := rawAccessToken(&data)
	if got != "enc:abc123" {
		t.Errorf("rawAccessToken = %q, want %q", got, "enc:abc123")
	}
}

func TestRawAccessToken_Missing(t *testing.T) {
	data := []byte("realm: test\n")
	got := rawAccessToken(&data)
	if got != "" {
		t.Errorf("rawAccessToken = %q, want empty string", got)
	}
}
