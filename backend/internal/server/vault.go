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
	key []byte
}

func NewVault(secret string) *Vault {
	if secret == "" {
		secret = "dev-secret-key"
	}
	digest := sha256.Sum256([]byte(secret))
	return &Vault{key: digest[:]}
}

func (v *Vault) Encrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(v.key)
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

func (v *Vault) Decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce := data[:gcm.NonceSize()]
	cipherText := data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
