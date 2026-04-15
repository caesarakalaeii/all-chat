// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrEmptyKey        = errors.New("encryption key cannot be empty")
	ErrInvalidKeyBytes = errors.New("encryption key must be 16, 24, or 32 bytes")
)

type AESEncryptor struct {
	gcm       cipher.AEAD
	nonceSize int
}

func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeyBytes, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &AESEncryptor{gcm: gcm, nonceSize: gcm.NonceSize()}, nil
}

func ParseKey(key string) ([]byte, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	if decoded, err := base64.StdEncoding.DecodeString(key); err == nil {
		if len(decoded) == 16 || len(decoded) == 24 || len(decoded) == 32 {
			return decoded, nil
		}
	}

	raw := []byte(key)
	if len(raw) == 16 || len(raw) == 24 || len(raw) == 32 {
		return raw, nil
	}

	return nil, fmt.Errorf("%w: got %d bytes", ErrInvalidKeyBytes, len(raw))
}

func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
	nonce := make([]byte, e.nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	if len(data) < e.nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, payload := data[:e.nonceSize], data[e.nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// Encrypt is an alias for EncryptString to match crypto.StringCipher interface
func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
	return e.EncryptString(plaintext)
}

// Decrypt is an alias for DecryptString to match crypto.StringCipher interface
func (e *AESEncryptor) Decrypt(ciphertext string) (string, error) {
	return e.DecryptString(ciphertext)
}
