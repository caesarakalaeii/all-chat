package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// StringCipher defines methods for encrypting and decrypting short text values.
type StringCipher interface {
	Encrypt(text string) (string, error)
	Decrypt(text string) (string, error)
}

// AESGCMCipher implements StringCipher using AES-GCM.
type AESGCMCipher struct {
	aead cipher.AEAD
}

// NewAESGCMCipher creates a new AES-GCM cipher using the provided key.
// The key must be 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256 respectively.
func NewAESGCMCipher(key string) (*AESGCMCipher, error) {
	keyBytes := []byte(key)
	keyLen := len(keyBytes)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, fmt.Errorf("invalid AES key length %d: must be 16, 24, or 32 bytes", keyLen)
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES-GCM cipher: %w", err)
	}

	return &AESGCMCipher{aead: aead}, nil
}

// Encrypt encrypts plaintext and returns a base64 encoded ciphertext containing the nonce prefix.
func (c *AESGCMCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64 encoded ciphertext that includes the nonce prefix.
func (c *AESGCMCipher) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonceSize := c.aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt ciphertext: %w", err)
	}

	return string(plaintext), nil
}
