package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/user"
	"runtime"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const encPrefix = "enc:"

// IsEncrypted returns true if the value has the encryption prefix.
func IsEncrypted(s string) bool {
	return strings.HasPrefix(s, encPrefix)
}

// Encrypt encrypts plaintext using AES-256-GCM with a machine-derived key.
// Returns a string with the "enc:" prefix followed by base64-encoded ciphertext.
func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key, err := deriveKey()
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value produced by Encrypt. If the value does not have
// the "enc:" prefix, it is returned as-is (plaintext migration path).
func Decrypt(encoded string) (string, error) {
	if !IsEncrypted(encoded) {
		return encoded, nil
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encPrefix))
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	key, err := deriveKey()
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// deriveKey produces a 32-byte AES key from machine-id and username via HKDF-SHA256.
func deriveKey() ([]byte, error) {
	machineID, err := readMachineID()
	if err != nil {
		machineID = "fallback-no-machine-id"
	}
	u, err := user.Current()
	username := "unknown"
	if err == nil {
		username = u.Username
	}
	ikm := []byte(machineID + ":" + username)
	salt := []byte("bai-token-encryption-v1")
	hk := hkdf.New(sha256.New, ikm, salt, []byte("bai-config"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hk, key); err != nil {
		return nil, err
	}
	return key, nil
}

func readMachineID() (string, error) {
	switch runtime.GOOS {
	case "linux":
		data, err := os.ReadFile("/etc/machine-id")
		if err != nil {
			data, err = os.ReadFile("/var/lib/dbus/machine-id")
		}
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(data)), nil
	case "darwin":
		// macOS: use IOPlatformUUID via ioreg — but for simplicity, use hostname + kernel UUID
		data, err := os.ReadFile("/etc/machine-id")
		if err == nil {
			return strings.TrimSpace(string(data)), nil
		}
		hostname, err := os.Hostname()
		if err != nil {
			return "", err
		}
		return hostname, nil
	default:
		hostname, err := os.Hostname()
		if err != nil {
			return "", err
		}
		return hostname, nil
	}
}
