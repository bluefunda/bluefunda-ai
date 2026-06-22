package crypto

import (
	"testing"
)

func TestEncryptDecrypt_roundTrip(t *testing.T) {
	original := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test-token-payload"
	encrypted, err := Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if !IsEncrypted(encrypted) {
		t.Fatalf("encrypted value missing prefix: %q", encrypted)
	}
	if encrypted == original {
		t.Fatal("encrypted value should differ from original")
	}

	decrypted, err := Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != original {
		t.Errorf("Decrypt = %q, want %q", decrypted, original)
	}
}

func TestDecrypt_plaintext(t *testing.T) {
	plain := "some-plaintext-token"
	result, err := Decrypt(plain)
	if err != nil {
		t.Fatalf("Decrypt plaintext: %v", err)
	}
	if result != plain {
		t.Errorf("Decrypt = %q, want %q", result, plain)
	}
}

func TestEncrypt_empty(t *testing.T) {
	enc, err := Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	if enc != "" {
		t.Errorf("Encrypt empty = %q, want empty", enc)
	}
}

func TestIsEncrypted(t *testing.T) {
	if IsEncrypted("plaintext") {
		t.Error("plaintext should not be encrypted")
	}
	if !IsEncrypted("enc:base64data") {
		t.Error("enc: prefixed should be encrypted")
	}
}

func TestEncrypt_uniqueNonce(t *testing.T) {
	plain := "same-token"
	a, _ := Encrypt(plain)
	b, _ := Encrypt(plain)
	if a == b {
		t.Error("two encryptions of the same plaintext should differ (unique nonces)")
	}
}
