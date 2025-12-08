package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// Vault handles encrypting sensitive credentials before storage.
type Vault struct {
	primaryKey   []byte
	fallbackKeys [][]byte
}

func deriveKey(secret string) []byte {
	digest := sha256.Sum256([]byte(secret))
	return digest[:]
}

// NewVault accepts a primary key plus optional fallbacks used for decryption/rotation.
func NewVault(primary string, fallbacks ...string) *Vault {
	keys := make([][]byte, 0, len(fallbacks)+1)
	seen := make(map[string]struct{})
	if primary != "" {
		keys = append(keys, deriveKey(primary))
		seen[primary] = struct{}{}
	}
	for _, fb := range fallbacks {
		if fb == "" {
			continue
		}
		if _, ok := seen[fb]; ok {
			continue
		}
		seen[fb] = struct{}{}
		keys = append(keys, deriveKey(fb))
	}

	if len(keys) == 0 {
		keys = append(keys, deriveKey("dev-secret-key"))
	}

	v := &Vault{
		primaryKey: keys[0],
	}
	if len(keys) > 1 {
		v.fallbackKeys = keys[1:]
	}
	return v
}

func (v *Vault) Encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(v.primaryKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nonce, nonce, []byte(value), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// DecryptWithInfo attempts decryption using primary then fallback keys, returning whether the primary was used.
func (v *Vault) DecryptWithInfo(value string) (string, bool, error) {
	if value == "" {
		return "", true, nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", false, err
	}

	tryKey := func(key []byte) (string, bool, error) {
		block, err := aes.NewCipher(key)
		if err != nil {
			return "", false, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", false, err
		}
		if len(data) < gcm.NonceSize() {
			return "", false, errors.New("ciphertext too short")
		}
		nonce := data[:gcm.NonceSize()]
		cipherText := data[gcm.NonceSize():]
		plain, err := gcm.Open(nil, nonce, cipherText, nil)
		if err != nil {
			return "", false, err
		}
		return string(plain), true, nil
	}

	if plain, ok, err := tryKey(v.primaryKey); err == nil && ok {
		return plain, true, nil
	}

	for _, fb := range v.fallbackKeys {
		if plain, ok, err := tryKey(fb); err == nil && ok {
			return plain, false, nil
		}
	}

	return "", false, errors.New("decryption failed with all keys")
}

func (v *Vault) Decrypt(value string) (string, error) {
	plain, _, err := v.DecryptWithInfo(value)
	return plain, err
}
