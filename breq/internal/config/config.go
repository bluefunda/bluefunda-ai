package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultBFFURL   = "requests.bluefunda.com:443"
	DefaultDomain   = "bluefunda.com"
	DefaultRealm    = "trm"
	DefaultClientID = "bluerequests"
)

func AuthURL(domain, realm string) string {
	if realm == "" {
		realm = DefaultRealm
	}
	return fmt.Sprintf("https://auth.%s/realms/%s/protocol/openid-connect", domain, realm)
}

type Config struct {
	BFFURL   string   `yaml:"bff_url"`
	Domain   string   `yaml:"domain"`
	Realm    string   `yaml:"realm"`
	Auth     Auth     `yaml:"auth"`
	Defaults Defaults `yaml:"defaults"`
}

type Auth struct {
	AccessToken  string    `yaml:"access_token"`
	RefreshToken string    `yaml:"refresh_token"`
	TokenExpiry  time.Time `yaml:"token_expiry"`
}

type Defaults struct {
	Output string `yaml:"output"`
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	dir := filepath.Join(home, ".breq")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
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
	return &cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		BFFURL:   DefaultBFFURL,
		Domain:   DefaultDomain,
		Realm:    DefaultRealm,
		Defaults: Defaults{Output: "table"},
	}
}

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

func (c *Config) TokenValid() bool {
	return c.Auth.AccessToken != "" && time.Now().Before(c.Auth.TokenExpiry)
}
