// Package keychain stores sensitive tokens in the OS-native credential store.
//
// On macOS, the system Keychain is used via the `security` CLI.
// On Linux, the Secret Service (GNOME Keyring / KWallet) is used via `secret-tool`.
// When neither is available, or when BAI_NO_KEYCHAIN=1 is set, the caller should
// fall back to the file-based crypto.Encrypt path.
package keychain

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const service = "bai"

// ErrNotFound is returned when a requested key does not exist in the keychain.
var ErrNotFound = errors.New("keychain: entry not found")

// Backend abstracts OS keychain operations so tests can inject a mock.
type Backend interface {
	Available() bool
	Set(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
}

// Default is the active backend. Replace via SetBackend in tests.
var Default Backend = &osBackend{}

// SetBackend replaces the active backend. Used by tests to inject a mock.
func SetBackend(b Backend) { Default = b }

// Available reports whether the active backend is usable on this machine.
func Available() bool { return Default.Available() }

// Set stores value under key in the OS keychain.
func Set(key, value string) error { return Default.Set(key, value) }

// Get retrieves the value for key. Returns ErrNotFound if absent.
func Get(key string) (string, error) { return Default.Get(key) }

// Delete removes key from the OS keychain. Returns nil if the key was absent.
func Delete(key string) error { return Default.Delete(key) }

// ---- OS backend ----

type osBackend struct{}

func (o *osBackend) Available() bool {
	if os.Getenv("BAI_NO_KEYCHAIN") == "1" {
		return false
	}
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("security")
		return err == nil
	case "linux":
		_, err := exec.LookPath("secret-tool")
		return err == nil
	default:
		return false
	}
}

func (o *osBackend) Set(key, value string) error {
	switch runtime.GOOS {
	case "darwin":
		return darwinSet(key, value)
	case "linux":
		return linuxSet(key, value)
	default:
		return fmt.Errorf("keychain: unsupported platform %s", runtime.GOOS)
	}
}

func (o *osBackend) Get(key string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return darwinGet(key)
	case "linux":
		return linuxGet(key)
	default:
		return "", ErrNotFound
	}
}

func (o *osBackend) Delete(key string) error {
	switch runtime.GOOS {
	case "darwin":
		return darwinDelete(key)
	case "linux":
		return linuxDelete(key)
	default:
		return nil
	}
}

// ---- macOS: security(1) ----

func darwinSet(key, value string) error {
	// -U updates an existing entry; creates a new one if absent.
	cmd := exec.Command("security", "add-generic-password",
		"-s", service, "-a", key, "-w", value, "-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("security add-generic-password: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func darwinGet(key string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("security", "find-generic-password",
		"-s", service, "-a", key, "-w")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", ErrNotFound
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

func darwinDelete(key string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-s", service, "-a", key)
	_ = cmd.Run() // ignore error if entry does not exist
	return nil
}

// ---- Linux: secret-tool(1) ----

func linuxSet(key, value string) error {
	cmd := exec.Command("secret-tool", "store",
		"--label", "bai "+key,
		"service", service,
		"account", key)
	cmd.Stdin = strings.NewReader(value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("secret-tool store: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func linuxGet(key string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("secret-tool", "lookup",
		"service", service,
		"account", key)
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", ErrNotFound
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

func linuxDelete(key string) error {
	cmd := exec.Command("secret-tool", "clear",
		"service", service,
		"account", key)
	_ = cmd.Run()
	return nil
}
