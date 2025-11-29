package server

import (
	"testing"
)

func TestVaultEncryptDecrypt(t *testing.T) {
	secretKey := "super-secret-key-for-testing-32-bytes"
	vault := NewVault(secretKey)

	original := "my-secret-password"
	encrypted, err := vault.Encrypt(original)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("encrypted string is empty")
	}
	if encrypted == original {
		t.Fatal("encrypted string matches original")
	}

	decrypted, err := vault.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if decrypted != original {
		t.Fatalf("decrypted value %q does not match original %q", decrypted, original)
	}
}

func TestVaultDecryptInvalid(t *testing.T) {
	vault := NewVault("some-key")
	_, err := vault.Decrypt("invalid-base64-data")
	if err == nil {
		t.Fatal("expected error for invalid data")
	}
}

func TestVaultStableKey(t *testing.T) {
	key := "stable-key"
	vault1 := NewVault(key)
	vault2 := NewVault(key)

	msg := "hello"
	enc, _ := vault1.Encrypt(msg)
	
	dec, err := vault2.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt with same key failed: %v", err)
	}
	if dec != msg {
		t.Fatalf("cross-instance decryption mismatch")
	}
}
