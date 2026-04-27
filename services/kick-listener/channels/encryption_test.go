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

package channels

// Tests for kick_oauth_tokens decryption (Task 3, plan 14-05).
// These tests verify the decryptKickToken helper which is called by getKickAuthToken.
// RED phase: written before implementation, will compile after cipher field is added.

import (
	"testing"

	"github.com/caesar/all-chat/shared/encryption"
)

// newTestMultiKeyEncryptor builds a single-version encryptor for use in tests.
// Uses a 32-byte raw string key (test only — not a real secret).
func newTestMultiKeyEncryptor(t *testing.T) *encryption.MultiKeyEncryptor {
	t.Helper()
	// exactly 32 ASCII bytes — valid AES-256 key for test purposes
	key := "test-key-32-bytes-for-unit-tests"
	parsed, err := encryption.ParseKey(key)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	aes, err := encryption.NewAESEncryptor(parsed)
	if err != nil {
		t.Fatalf("NewAESEncryptor: %v", err)
	}
	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: aes}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewMultiKeyEncryptor: %v", err)
	}
	return enc
}

// TestDecryptKickToken_VersionedDecrypts proves that decryptKickToken decrypts
// a versioned ciphertext (encryption_version=1) when a cipher is set.
func TestDecryptKickToken_VersionedDecrypts(t *testing.T) {
	enc := newTestMultiKeyEncryptor(t)

	// Encrypt the plaintext access token the same way overlay-manager does on write.
	plaintext := "test-access-token-12345"
	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("EncryptString: %v", err)
	}

	m := &Manager{cipher: enc}
	got, err := m.decryptKickToken(ciphertext, 1)
	if err != nil {
		t.Fatalf("decryptKickToken(version=1) error: %v", err)
	}
	if got != plaintext {
		t.Errorf("decryptKickToken(version=1) = %q, want %q", got, plaintext)
	}
}

// TestDecryptKickToken_PlaintextLegacy proves that decryptKickToken passes
// through the token unchanged when encryption_version=0 (legacy rows).
func TestDecryptKickToken_PlaintextLegacy(t *testing.T) {
	enc := newTestMultiKeyEncryptor(t)
	m := &Manager{cipher: enc}

	raw := "plain-oauth-token-no-encryption"
	got, err := m.decryptKickToken(raw, 0)
	if err != nil {
		t.Fatalf("decryptKickToken(version=0) error: %v", err)
	}
	if got != raw {
		t.Errorf("decryptKickToken(version=0) = %q, want %q", got, raw)
	}
}

// TestDecryptKickToken_NilCipherVersioned proves that decryptKickToken returns
// an error when encryption_version>=1 but no cipher is configured.
func TestDecryptKickToken_NilCipherVersioned(t *testing.T) {
	m := &Manager{cipher: nil}
	_, err := m.decryptKickToken("some-ciphertext", 1)
	if err == nil {
		t.Error("expected error when cipher is nil and version=1, got nil")
	}
}
