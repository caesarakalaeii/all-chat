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

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := ParseKey("c2VjcmV0X2tleV9zZWNyZXRfa2V5X3NlY3JldF9rZXk=")
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}

	enc, err := NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}

	plaintext := "super-secret-token"
	ciphertext, err := enc.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if ciphertext == plaintext {
		t.Fatalf("ciphertext should not equal plaintext")
	}

	roundTrip, err := enc.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if roundTrip != plaintext {
		t.Fatalf("expected %s, got %s", plaintext, roundTrip)
	}
}

func TestParseKeyRejectsInvalidLength(t *testing.T) {
	if _, err := ParseKey("short"); err == nil {
		t.Fatalf("expected error for invalid key length")
	}
}
