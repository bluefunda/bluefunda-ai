package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		cfg.Defaults.Model = "openai"
	}

	// Persist the migrated config so old field names are not re-read next time.
	if needsSave {
		_ = Save(&cfg)
	}

	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		GatewayURL: DefaultGatewayURL,
		BFFURL:     DefaultBFFURL,
		Domain:     DefaultDomain,
		Realm:      DefaultRealm,
		Defaults:   Defaults{Model: "openai", Output: "text"},
	}
}

// Save writes the config to ~/.bai/config.yaml.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// TokenValid returns true if the access token exists and has not expired.
func (c *Config) TokenValid() bool {
	return c.Auth.AccessToken != "" && time.Now().Before(c.Auth.TokenExpiry)
}
