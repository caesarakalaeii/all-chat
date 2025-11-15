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
