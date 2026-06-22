package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bluefunda/bluefunda-ai/internal/crypto"
	"gopkg.in/yaml.v3"
)

// Platform defaults — no user input needed.
const (
	DefaultGatewayURL = "https://ai.bluefunda.com"
	DefaultBFFURL     = "cli.bluefunda.com:443"
	DefaultDomain     = "bluefunda.com"
	DefaultRealm      = "trm"
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
	GatewayURL string   `yaml:"gateway"`  // was: gateway_url
	BFFURL     string   `yaml:"endpoint"` // was: bff_url
	Domain     string   `yaml:"domain"`
	Realm      string   `yaml:"realm"`
	Auth       Auth     `yaml:"auth"`
	Defaults   Defaults `yaml:"defaults"`
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
// values read from the YAML file. CLI flags take precedence over these.
// Precedence order: CLI flags > BAI_* env vars > YAML file > compiled defaults.
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

	// Decrypt tokens if they were stored encrypted.
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

// Save writes the config to ~/.bai/config.yaml, encrypting tokens at rest.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	// Encrypt tokens before writing.
	toSave := *cfg
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
