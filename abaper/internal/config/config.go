package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultBaseURL = "https://api.bluefunda.com"
	DefaultRealm   = "individual"
	ClientID       = "cai-cli"
	ConfigDir      = ".abaper"
	ConfigFile     = "config"
	TokenFile      = "tokens.yaml"
)

type Config struct {
	BaseURL string `mapstructure:"base_url"`
	Org     string `mapstructure:"org"`
	Realm   string `mapstructure:"realm"`
}

type Tokens struct {
	AccessToken  string `mapstructure:"access_token"`
	RefreshToken string `mapstructure:"refresh_token"`
	ExpiresAt    int64  `mapstructure:"expires_at"`
}

func ConfigDirPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ConfigDir)
}

func Init() {
	configDir := ConfigDirPath()

	viper.SetConfigName(ConfigFile)
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	viper.SetDefault("base_url", DefaultBaseURL)
	viper.SetDefault("org", "default")
	viper.SetDefault("realm", DefaultRealm)

	viper.SetEnvPrefix("ABAPER")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()
}

func Load() *Config {
	return &Config{
		BaseURL: viper.GetString("base_url"),
		Org:     viper.GetString("org"),
		Realm:   viper.GetString("realm"),
	}
}

func EnsureConfigDir() error {
	dir := ConfigDirPath()
	return os.MkdirAll(dir, 0700)
}

func SaveTokens(tokens *Tokens) error {
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.Set("access_token", tokens.AccessToken)
	v.Set("refresh_token", tokens.RefreshToken)
	v.Set("expires_at", tokens.ExpiresAt)

	path := filepath.Join(ConfigDirPath(), TokenFile)
	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write tokens: %w", err)
	}

	return os.Chmod(path, 0600)
}

func LoadTokens() (*Tokens, error) {
	v := viper.New()
	v.SetConfigName("tokens")
	v.SetConfigType("yaml")
	v.AddConfigPath(ConfigDirPath())

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	tokens := &Tokens{}
	if err := v.Unmarshal(tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func ClearTokens() error {
	path := filepath.Join(ConfigDirPath(), TokenFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
