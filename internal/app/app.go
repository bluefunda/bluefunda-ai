// Package app holds the identity of the product binary being run. bai and
// abaper are built from the same platform core; everything that differs
// between them at the platform level — config directory, env-var prefix,
// OAuth client, backend, keychain service, branding — is read from Current().
//
// A product's main sets its identity once with Set before any other package
// is used. The zero configuration is bai.
package app

import (
	"os"
	"path/filepath"
	"sync"
)

// Identity describes one product built on the platform core.
type Identity struct {
	// Name is the binary name used in user-facing hints ("run `<Name> login`").
	Name string
	// DisplayName is the brand shown in the TUI header.
	DisplayName string
	// Dir is the dot-directory used under $HOME and at project roots (".bai").
	Dir string
	// EnvPrefix prefixes environment overrides ("BAI_" → BAI_MODEL).
	EnvPrefix string
	// ClientID is the Keycloak client used for the device login flow.
	ClientID string
	// KeychainService is the OS keychain service name tokens are stored under.
	KeychainService string
	// HomebrewCask is the cask name used by `<Name> update`.
	HomebrewCask string
	// Placeholder is the empty-input hint in the chat TUI.
	Placeholder string
	// BFFURL is the product's default gRPC backend. Empty uses
	// config.DefaultBFFURL; config files, env vars and flags still override.
	// Every product signs in to the same Keycloak realm (config.DefaultRealm).
	BFFURL string
	// Accent / AccentBold override the TUI accent colours (hex, e.g. "#FF5F1F").
	// Empty keeps the default palette.
	Accent     string
	AccentBold string
}

// Bai is the identity of the bai CLI.
var Bai = Identity{
	Name:            "bai",
	DisplayName:     "BlueFunda AI",
	Dir:             ".bai",
	EnvPrefix:       "BAI_",
	ClientID:        "bai",
	KeychainService: "bai",
	HomebrewCask:    "bai",
	Placeholder:     "Ask BlueFunda AI...",
}

var (
	mu      sync.RWMutex
	current = Bai
)

// Current returns the identity of the running product.
func Current() Identity {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set replaces the identity of the running product. Call it once from main.
func Set(id Identity) {
	mu.Lock()
	defer mu.Unlock()
	current = id
}

// Getenv reads <EnvPrefix><key>, e.g. Getenv("MODEL") → $BAI_MODEL.
func Getenv(key string) string {
	return os.Getenv(Current().EnvPrefix + key)
}

// HomeDir returns ~/<Dir> without creating it.
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, Current().Dir), nil
}

// HomePath joins elem onto ~/<Dir>.
func HomePath(elem ...string) (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{dir}, elem...)...), nil
}
