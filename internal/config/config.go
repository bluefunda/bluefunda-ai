package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/crypto"
	"github.com/bluefunda/bluefunda-ai/internal/keychain"
	"gopkg.in/yaml.v3"
)

// Platform defaults — no user input needed.
const (
	DefaultGatewayURL = "https://ai.bluefunda.com"
	DefaultBFFURL     = "cli.bluefunda.com:443"
	DefaultDomain     = "bluefunda.com"
	DefaultRealm      = "individual"
	DefaultClientID   = "bai"
)

// AuthURL returns the Keycloak OpenID Connect base URL for the given realm.
func AuthURL(domain, realm string) string {
	if realm == "" {
		realm = DefaultRealm
	}
	return fmt.Sprintf("https://auth.%s/realms/%s/protocol/openid-connect", domain, realm)
}

// Config represents the CLI configuration stored in ~/.bai/config.yaml.
type Config struct {
	GatewayURL     string             `yaml:"gateway"`  // was: gateway_url
	BFFURL         string             `yaml:"endpoint"` // was: bff_url
	Domain         string             `yaml:"domain"`
	Realm          string             `yaml:"realm"`
	Auth           Auth               `yaml:"auth"`
	Defaults       Defaults           `yaml:"defaults"`
	Profiles       map[string]Profile `yaml:"profiles,omitempty"`
	DefaultProfile string             `yaml:"default_profile,omitempty"`
}

// Profile is one named backend environment under the top-level `profiles:`
// map (e.g. dev / staging / prod). Any field left empty falls back to the
// top-level Config value it would otherwise override.
type Profile struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	Gateway  string `yaml:"gateway,omitempty"`
	Domain   string `yaml:"domain,omitempty"`
	Model    string `yaml:"model,omitempty"`
}

// profileOverride activates a profile for the current process, taking
// precedence over BAI_PROFILE and the YAML file's default_profile. Wired to
// the --profile CLI flag; set it (via SetProfileOverride) before calling
// Load().
var profileOverride string

// SetProfileOverride sets the profile Load will activate, overriding
// BAI_PROFILE and default_profile. Pass "" to clear it.
func SetProfileOverride(name string) {
	profileOverride = name
}

// ProfileNames returns the configured profile names, sorted.
func (cfg *Config) ProfileNames() []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// activeProfile resolves which profile applies — profileOverride, then
// BAI_PROFILE, then cfg.DefaultProfile, in that order — and returns it, or
// nil if none is active. Returns an error if an explicitly named profile
// (from any of those three sources) doesn't exist.
//
// This never mutates cfg. Config's own persisted fields (GatewayURL, BFFURL,
// Domain, Defaults.Model) are always the plain base values from
// ~/.bai/config.yaml — Save writes them back untouched. A profile is instead
// an overlay applied at read time by the Effective* accessors below, so that
// an unrelated Save (e.g. a token refresh mid-session) can never bake a
// temporarily active profile's values into the persisted base config.
func (cfg *Config) activeProfile() (*Profile, error) {
	name := profileOverride
	if name == "" {
		name = os.Getenv("BAI_PROFILE")
	}
	if name == "" {
		name = cfg.DefaultProfile
	}
	if name == "" {
		return nil, nil
	}
	p, ok := cfg.Profiles[name]
	if !ok {
		available := cfg.ProfileNames()
		if len(available) == 0 {
			return nil, fmt.Errorf("profile %q requested but no profiles are configured in ~/.bai/config.yaml", name)
		}
		return nil, fmt.Errorf("unknown profile %q — available: %s", name, strings.Join(available, ", "))
	}
	return &p, nil
}

// EffectiveBFFURL returns the active profile's endpoint if set, else BFFURL.
func (cfg *Config) EffectiveBFFURL() string {
	if p, err := cfg.activeProfile(); err == nil && p != nil && p.Endpoint != "" {
		return p.Endpoint
	}
	return cfg.BFFURL
}

// EffectiveGatewayURL returns the active profile's gateway if set, else GatewayURL.
func (cfg *Config) EffectiveGatewayURL() string {
	if p, err := cfg.activeProfile(); err == nil && p != nil && p.Gateway != "" {
		return p.Gateway
	}
	return cfg.GatewayURL
}

// EffectiveDomain returns the active profile's domain if set, else Domain.
func (cfg *Config) EffectiveDomain() string {
	if p, err := cfg.activeProfile(); err == nil && p != nil && p.Domain != "" {
		return p.Domain
	}
	return cfg.Domain
}

// EffectiveModel returns the active profile's model if set, else Defaults.Model.
func (cfg *Config) EffectiveModel() string {
	if p, err := cfg.activeProfile(); err == nil && p != nil && p.Model != "" {
		return p.Model
	}
	return cfg.Defaults.Model
}

// legacyConfig holds the old YAML field names for one-time migration.
type legacyConfig struct {
	BFFURLOld  string `yaml:"bff_url"`
	GatewayOld string `yaml:"gateway_url"`
}

// Auth holds persisted tokens.
type Auth struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	TokenExpiry  time.Time `yaml:"token_expiry"`
}

// Defaults holds default CLI settings.
type Defaults struct {
	Model  string `yaml:"model"`
	Output string `yaml:"output"`
}

// configDir returns ~/.bai, creating it if needed.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	dir := filepath.Join(home, ".bai")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// configPath returns the full path to the config file.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// applyEnvOverrides copies BAI_* environment variables into cfg, overriding
// values read from the YAML file (including any profile already applied).
// CLI flags take precedence over these.
// Precedence order: CLI flags > BAI_* env vars > active profile > YAML file > compiled defaults.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("BAI_GATEWAY"); v != "" {
		cfg.GatewayURL = v
	}
	if v := os.Getenv("BAI_BFF"); v != "" {
		cfg.BFFURL = v
	}
	if v := os.Getenv("BAI_DOMAIN"); v != "" {
		cfg.Domain = v
	}
	if v := os.Getenv("BAI_REALM"); v != "" {
		cfg.Realm = v
	}
	if v := os.Getenv("BAI_MODEL"); v != "" {
		cfg.Defaults.Model = v
	}
	// BAI_ACCESS_TOKEN lets CI/CD authenticate without running `bai login`.
	// Set TokenExpiry far in the future so the token source does not attempt
	// a device-flow refresh on a token it didn't issue.
	if v := os.Getenv("BAI_ACCESS_TOKEN"); v != "" {
		cfg.Auth.AccessToken = v
		cfg.Auth.TokenExpiry = time.Now().Add(24 * time.Hour)
	}
}

// MCPServerConfig configures one local MCP server started via stdio transport.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// ProjectConfig is the subset of Config that can be overridden per-project
// via .bai/settings.yaml at the git root.
type ProjectConfig struct {
	Model       string                     `yaml:"model"`
	MaxTurns    int                        `yaml:"max_turns"`
	Endpoint    string                     `yaml:"endpoint"`
	MCPServers  map[string]MCPServerConfig `yaml:"mcp_servers"`
	Permissions struct {
		Allow []string `yaml:"allow"`
		Deny  []string `yaml:"deny"`
	} `yaml:"permissions"`
}

// mergeProject applies non-zero values from p over cfg.
func (cfg *Config) mergeProject(p *ProjectConfig) {
	if p.Model != "" {
		cfg.Defaults.Model = p.Model
	}
	if p.Endpoint != "" {
		cfg.BFFURL = p.Endpoint
	}
}

// FindProjectConfig walks upward from cwd until a .git directory, returning
// the first .bai/settings.yaml found, or nil if none exists.
func FindProjectConfig(cwd string) *ProjectConfig {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return nil
	}
	for {
		candidate := filepath.Join(abs, ".bai", "settings.yaml")
		if data, err := os.ReadFile(candidate); err == nil {
			var p ProjectConfig
			if yaml.Unmarshal(data, &p) == nil {
				return &p
			}
		}
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}
	return nil
}

// Load reads the config from ~/.bai/config.yaml.
// Returns defaults if the file does not exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := defaultConfig()
			if _, err := cfg.activeProfile(); err != nil {
				return nil, err
			}
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// One-time migration: read old field names if new ones are absent.
	var needsSave bool
	var legacy legacyConfig
	if err := yaml.Unmarshal(data, &legacy); err == nil {
		if cfg.BFFURL == "" && legacy.BFFURLOld != "" {
			cfg.BFFURL = legacy.BFFURLOld
			needsSave = true
		}
		if cfg.GatewayURL == "" && legacy.GatewayOld != "" {
			cfg.GatewayURL = legacy.GatewayOld
			needsSave = true
		}
	}

	// Validate the active profile (--profile / BAI_PROFILE / default_profile)
	// exists. It's applied at read time by the Effective* accessors, not here
	// — see activeProfile's doc comment for why.
	if _, err := cfg.activeProfile(); err != nil {
		return nil, err
	}

	// Backfill defaults for missing fields.
	if cfg.GatewayURL == "" {
		cfg.GatewayURL = DefaultGatewayURL
	}
	if cfg.BFFURL == "" {
		cfg.BFFURL = DefaultBFFURL
	}
	if cfg.Domain == "" {
		cfg.Domain = DefaultDomain
	}
	if cfg.Realm == "" {
		cfg.Realm = DefaultRealm
	}
	if cfg.Defaults.Model == "" {
		cfg.Defaults.Model = "auto"
	}

	// Persist the migrated config so old field names are not re-read next time.
	if needsSave {
		_ = Save(&cfg)
	}

	// Load tokens: prefer OS keychain, fall back to file-based encryption.
	if keychain.Available() {
		if tok, err := keychain.Get("access_token"); err == nil {
			cfg.Auth.AccessToken = tok
		}
		if tok, err := keychain.Get("refresh_token"); err == nil {
			cfg.Auth.RefreshToken = tok
		}
		// Migrate enc: tokens from the YAML file into the keychain on first use.
		if cfg.Auth.AccessToken == "" {
			raw := rawAccessToken(&data)
			if raw != "" {
				if dec, err := crypto.Decrypt(raw); err == nil && dec != "" {
					cfg.Auth.AccessToken = dec
					_ = Save(&cfg) // re-save moves the token into the keychain
				}
			}
		}
	} else {
		// File-based path: decrypt enc: prefixed values.
		if cfg.Auth.AccessToken != "" {
			if dec, err := crypto.Decrypt(cfg.Auth.AccessToken); err == nil {
				cfg.Auth.AccessToken = dec
			}
		}
		if cfg.Auth.RefreshToken != "" {
			if dec, err := crypto.Decrypt(cfg.Auth.RefreshToken); err == nil {
				cfg.Auth.RefreshToken = dec
			}
		}
		// Migrate plaintext tokens to encrypted on first load.
		if cfg.Auth.AccessToken != "" && !crypto.IsEncrypted(rawAccessToken(&data)) {
			_ = Save(&cfg)
		}
	}

	applyEnvOverrides(&cfg)

	// Project config: walk cwd upward and merge .bai/settings.yaml.
	if cwd, err := os.Getwd(); err == nil {
		if p := FindProjectConfig(cwd); p != nil {
			cfg.mergeProject(p)
		}
	}

	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		GatewayURL: DefaultGatewayURL,
		BFFURL:     DefaultBFFURL,
		Domain:     DefaultDomain,
		Realm:      DefaultRealm,
		Defaults:   Defaults{Model: "auto", Output: "text"},
	}
}

// Save writes the config to ~/.bai/config.yaml.
// When the OS keychain is available, tokens are stored there and omitted from
// the YAML file. Otherwise, tokens are AES-256-GCM encrypted before writing.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	toSave := *cfg

	if keychain.Available() {
		// Store tokens in the OS keychain; write empty strings to YAML.
		if cfg.Auth.AccessToken != "" {
			_ = keychain.Set("access_token", cfg.Auth.AccessToken)
		}
		if cfg.Auth.RefreshToken != "" {
			_ = keychain.Set("refresh_token", cfg.Auth.RefreshToken)
		}
		toSave.Auth.AccessToken = ""
		toSave.Auth.RefreshToken = ""
	} else {
		// File-based path: encrypt tokens before writing.
		if toSave.Auth.AccessToken != "" {
			if enc, err := crypto.Encrypt(toSave.Auth.AccessToken); err == nil {
				toSave.Auth.AccessToken = enc
			}
		}
		if toSave.Auth.RefreshToken != "" {
			if enc, err := crypto.Encrypt(toSave.Auth.RefreshToken); err == nil {
				toSave.Auth.RefreshToken = enc
			}
		}
	}

	data, err := yaml.Marshal(&toSave)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// rawAccessToken extracts the raw access_token value from YAML bytes
// without decryption, to detect whether migration is needed.
func rawAccessToken(data *[]byte) string {
	var raw struct {
		Auth struct {
			AccessToken string `yaml:"access_token"`
		} `yaml:"auth"`
	}
	if yaml.Unmarshal(*data, &raw) == nil {
		return raw.Auth.AccessToken
	}
	return ""
}

// TokenValid returns true if the access token exists and has not expired.
func (c *Config) TokenValid() bool {
	return c.Auth.AccessToken != "" && time.Now().Before(c.Auth.TokenExpiry)
}

// ClearTokens zeroes all auth fields. Call Save after this to persist.
func (c *Config) ClearTokens() {
	c.Auth.AccessToken = ""
	c.Auth.RefreshToken = ""
	c.Auth.TokenExpiry = time.Time{}
}
