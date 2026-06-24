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

// Package encryption provides versioned AES-GCM encryption primitives.
//
// # Ciphertext wire format (D-01)
//
// New ciphertexts produced by MultiKeyEncryptor use the format:
//
//	base64( [kid(1B)] [nonce(12B)] [ciphertext] [tag(16B)] )
//
// Legacy ciphertexts (produced before Phase 14) have no kid prefix:
//
//	base64( [nonce(12B)] [ciphertext] [tag(16B)] )
//
// # Kid-byte disambiguation (D-05)
//
// DecryptString tries the versioned path first: if blob[0] is a registered kid
// AND len(decoded) >= 1+12+16, it strips the kid byte and attempts AEAD decrypt.
// If AEAD authentication fails (e.g. a legacy blob whose first byte coincidentally
// equals a registered kid — probability 1/256), it falls through to each legacy key
// in order. This eliminates false-positive mis-routing without requiring any schema
// change or format sentinel.
//
// # Kid namespace (T-14-01-02)
//
// 0x00 is reserved as "legacy / no kid" and is never written by EncryptString.
// 0x01..0x7F are allocated monotonically by Phase planners.
// 0x80..0xFF are reserved for future use.
// Constructor returns ErrReservedKid for kid 0x00 or kid > MaxKid.
package encryption

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
)

// KidByte is a 1-byte key identifier prepended to every versioned ciphertext.
// 0x00 is reserved as "legacy / kid-less" and is never written by EncryptString.
// 0x01..0x7F are allocated by planners; each allocation must be sequential and
// monotonically increasing. See docs/architecture/05-SECURITY.md for the registry.
type KidByte = byte

const (
	// LegacyKid is the sentinel for kid-less (pre-Phase-14) ciphertext.
	// It is never written as a prefix by MultiKeyEncryptor.EncryptString.
	LegacyKid KidByte = 0x00

	// MaxKid is the upper bound of the valid kid range (inclusive).
	// 0x80..0xFF are reserved for future use.
	MaxKid KidByte = 0x7F

	// minVersionedBlobLen is the minimum byte length of a decoded versioned blob:
	// 1 (kid) + 12 (nonce) + 0 (min ciphertext) + 16 (GCM tag).
	minVersionedBlobLen = 1 + 12 + 16
)

var (
	// ErrNoEncryptionKeys is returned when no versioned key (TOKEN_ENCRYPTION_KEY_V1
	// etc.) is configured. At least one Vn key is required.
	ErrNoEncryptionKeys = errors.New(
		"no encryption keys configured: set TOKEN_ENCRYPTION_KEY_V1 " +
			"(and optional TOKEN_ENCRYPTION_KEY for legacy fallback)")

	// ErrReservedKid is returned when a caller attempts to register kid 0x00 (reserved
	// as LegacyKid) or any kid > MaxKid (0x7F).
	ErrReservedKid = errors.New("kid 0x00 reserved as legacy; valid range 0x01..0x7F")

	// ErrUnknownKid is returned by internal helpers when a blob's kid byte is not in
	// the registered map. DecryptString does NOT surface this — it falls back to
	// legacy keys transparently.
	ErrUnknownKid = errors.New("ciphertext kid byte not registered")
)

// KeyEntry maps a KidByte to its AES-GCM cipher primitive.
type KeyEntry struct {
	Kid    KidByte
	Cipher *AESEncryptor
}

// MultiKeyEncryptor encrypts with the latest registered key and decrypts with any
// registered versioned key plus an optional legacy (kid-less) fallback chain.
//
// Thread-safe: the registered key maps are immutable after construction.
//
// Unified key chain (D-04): TOKEN_ENCRYPTION_KEY and YOUTUBE_TOKEN_ENCRYPTION_KEY are
// both accepted as legacy fallback keys so that YouTube OAuth tokens encrypted before
// D-04 migration still decrypt without a code change.
type MultiKeyEncryptor struct {
	latest     *KeyEntry
	byKid      map[KidByte]*AESEncryptor
	legacyKeys []*AESEncryptor // order: TOKEN_ENCRYPTION_KEY first, then YOUTUBE_TOKEN_ENCRYPTION_KEY (D-04)
}

// NewMultiKeyEncryptor constructs a MultiKeyEncryptor from explicit KeyEntry slices.
// Intended for tests and the key-rotator sweeper (plan 14-06).
//
// entries must be non-empty. entries[len(entries)-1] is the latest (write) key.
// legacyKeys may be nil or empty when no legacy data exists (e.g. Kick/TikTok
// new columns where the versioned scheme is used from day one).
//
// Returns ErrReservedKid if any entry has kid 0x00 or kid > MaxKid.
// Returns an error on duplicate kid registrations.
func NewMultiKeyEncryptor(entries []KeyEntry, legacyKeys []*AESEncryptor) (*MultiKeyEncryptor, error) {
	if len(entries) == 0 {
		return nil, ErrNoEncryptionKeys
	}
	byKid := make(map[KidByte]*AESEncryptor, len(entries))
	for i := range entries {
		kid := entries[i].Kid
		if kid == LegacyKid || kid > MaxKid {
			return nil, fmt.Errorf("%w: got 0x%02x", ErrReservedKid, kid)
		}
		if _, dup := byKid[kid]; dup {
			return nil, fmt.Errorf("duplicate kid 0x%02x", kid)
		}
		byKid[kid] = entries[i].Cipher
	}
	latest := &entries[len(entries)-1]
	return &MultiKeyEncryptor{latest: latest, byKid: byKid, legacyKeys: legacyKeys}, nil
}

// NewMultiKeyEncryptorFromEnv constructs a MultiKeyEncryptor from environment variables.
//
// Versioned keys (D-02):
//
//	TOKEN_ENCRYPTION_KEY_V1, _V2, ...  → kid 0x01, 0x02, ...
//
// Discovery stops at the first missing Vn. Returns ErrNoEncryptionKeys if V1 is absent.
// The last present Vn becomes the write key (CurrentKid).
//
// Legacy fallback keys (D-04, D-05):
//
//	TOKEN_ENCRYPTION_KEY          → first legacy key (original rollout, Phase 13)
//	YOUTUBE_TOKEN_ENCRYPTION_KEY  → second legacy key (YouTube OAuth, unified in D-04)
//
// Legacy keys are optional. If neither is set, DecryptString cannot decrypt kid-less blobs.
//
// Constructor never panics; all parse/construction errors are returned as wrapped errors
// (T-14-01-05).
//
// A warning is logged when a key appears to be all-zero or low-entropy (audit L5/L7).
// ParseKey's base64-vs-raw ambiguity is documented in encryption.go (audit L8): if
// the input is valid base64 and decodes to a valid key length, it is treated as base64;
// otherwise the raw bytes are used.
//
// NewMultiKeyEncryptorFromEnv uses a no-op logger (backward compat). Production callers
// should use NewMultiKeyEncryptorFromEnvWithLogger to actually surface weak-key
// warnings (audit L5 — warnIfWeakKey was previously dead code, always called with
// zap.NewNop(), so warnings were never emitted).
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error) {
	return NewMultiKeyEncryptorFromEnvWithLogger(zap.NewNop())
}

// NewMultiKeyEncryptorFromEnvWithLogger is identical to NewMultiKeyEncryptorFromEnv
// but threads a real logger so weak-key warnings are actually emitted (audit L5).
// A nil logger is treated as a no-op (safe to call with nil in tests).
func NewMultiKeyEncryptorFromEnvWithLogger(logger *zap.Logger) (*MultiKeyEncryptor, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	var entries []KeyEntry
	for n := 1; n <= int(MaxKid); n++ {
		envName := "TOKEN_ENCRYPTION_KEY_V" + strconv.Itoa(n)
		v := os.Getenv(envName)
		if v == "" {
			break
		}
		parsed, err := ParseKey(v)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", envName, err)
		}
		warnIfWeakKey(logger, envName, parsed)
		cipher, err := NewAESEncryptor(parsed)
		if err != nil {
			return nil, fmt.Errorf("create cipher %s: %w", envName, err)
		}
		entries = append(entries, KeyEntry{Kid: KidByte(n), Cipher: cipher})
	}
	if len(entries) == 0 {
		return nil, ErrNoEncryptionKeys
	}

	var legacyKeys []*AESEncryptor

	// TOKEN_ENCRYPTION_KEY — legacy fallback, first in chain (D-04, D-05)
	if v := os.Getenv("TOKEN_ENCRYPTION_KEY"); v != "" {
		parsed, err := ParseKey(v)
		if err != nil {
			return nil, fmt.Errorf("parse TOKEN_ENCRYPTION_KEY: %w", err)
		}
		warnIfWeakKey(logger, "TOKEN_ENCRYPTION_KEY", parsed)
		cipher, err := NewAESEncryptor(parsed)
		if err != nil {
			return nil, fmt.Errorf("create cipher TOKEN_ENCRYPTION_KEY: %w", err)
		}
		legacyKeys = append(legacyKeys, cipher)
	}

	// YOUTUBE_TOKEN_ENCRYPTION_KEY — second legacy fallback for YouTube OAuth tokens (D-04)
	if v := os.Getenv("YOUTUBE_TOKEN_ENCRYPTION_KEY"); v != "" {
		parsed, err := ParseKey(v)
		if err != nil {
			return nil, fmt.Errorf("parse YOUTUBE_TOKEN_ENCRYPTION_KEY: %w", err)
		}
		warnIfWeakKey(logger, "YOUTUBE_TOKEN_ENCRYPTION_KEY", parsed)
		cipher, err := NewAESEncryptor(parsed)
		if err != nil {
			return nil, fmt.Errorf("create cipher YOUTUBE_TOKEN_ENCRYPTION_KEY: %w", err)
		}
		legacyKeys = append(legacyKeys, cipher)
	}

	return NewMultiKeyEncryptor(entries, legacyKeys)
}

// CurrentKid returns the KidByte used for new writes (the latest registered key).
func (m *MultiKeyEncryptor) CurrentKid() KidByte { return m.latest.Kid }

// EncryptString encrypts plaintext using the latest registered key.
//
// Wire format: base64( [kid(1B)] [nonce(12B)] [ciphertext] [tag(16B)] )
//
// Implementation: delegates to the underlying AESEncryptor to produce the legacy
// blob (nonce||ct||tag), base64-decodes it, prepends the kid byte, and base64-encodes
// the result. The kid byte is always non-zero (LegacyKid is never written).
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error) {
	// Delegate to the underlying AESEncryptor which produces base64(nonce||ct||tag).
	legacyBlob, err := m.latest.Cipher.EncryptString(plaintext)
	if err != nil {
		return "", err
	}
	rawLegacy, err := base64.StdEncoding.DecodeString(legacyBlob)
	if err != nil {
		return "", fmt.Errorf("re-decode for kid prefix: %w", err)
	}
	// Prepend the kid byte: [kid][nonce||ct||tag]
	out := make([]byte, 0, len(rawLegacy)+1)
	out = append(out, m.latest.Kid)
	out = append(out, rawLegacy...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptString auto-detects the ciphertext format and decrypts accordingly.
//
// Versioned path (D-01): tried first when both conditions hold:
//  1. len(decoded) >= 1+12+16 (minimum valid versioned blob size)
//  2. decoded[0] is a registered kid byte
//
// If AEAD authentication fails on the versioned path (e.g. false-positive kid byte —
// a legacy blob whose first decoded byte coincidentally equals a registered kid;
// probability 1/256 per kid), the error is silently discarded and the fallback
// chain is tried (D-05 / T-14-01-01).
//
// Legacy fallback path (D-05): each legacy key is tried in order. TOKEN_ENCRYPTION_KEY
// is tried before YOUTUBE_TOKEN_ENCRYPTION_KEY so that the common case is handled
// without an extra AEAD attempt.
//
// Returns an error only when all keys in the chain (versioned + all legacy) fail AEAD
// authentication.
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}

	// Versioned path: attempt if blob is long enough and kid is registered.
	if len(decoded) >= minVersionedBlobLen {
		kid := decoded[0]
		if cipher, ok := m.byKid[kid]; ok {
			// Reconstitute the legacy-shaped blob (nonce||ct||tag) for the underlying
			// AESEncryptor, which expects base64(nonce||ct||tag) without the kid byte.
			legacyShaped := base64.StdEncoding.EncodeToString(decoded[1:])
			if pt, aerr := cipher.DecryptString(legacyShaped); aerr == nil {
				return pt, nil
			}
			// AEAD authentication failed: this is a false-positive kid byte on a legacy
			// blob. Fall through to the legacy key chain (D-05 / T-14-01-01).
		}
	}

	// Legacy fallback: try each legacy key in order (TOKEN_ENCRYPTION_KEY first).
	// The original (pre-kid-prefix) ciphertext string is passed as-is.
	for _, lk := range m.legacyKeys {
		if pt, lerr := lk.DecryptString(ciphertext); lerr == nil {
			return pt, nil
		}
	}

	return "", fmt.Errorf(
		"decrypt: no key in chain (versioned kid map + %d legacy key(s)) authenticated the ciphertext",
		len(m.legacyKeys))
}

// Encrypt is a StringCipher-compatible alias for EncryptString.
func (m *MultiKeyEncryptor) Encrypt(s string) (string, error) { return m.EncryptString(s) }

// Decrypt is a StringCipher-compatible alias for DecryptString.
func (m *MultiKeyEncryptor) Decrypt(s string) (string, error) { return m.DecryptString(s) }

// warnIfWeakKey logs a warning when a key appears all-zero (audit L5/L7). This is
// advisory only — the key is still accepted to avoid breaking existing deployments
// during rotation. Callers must pass a real (non-no-op) logger for warnings to be
// emitted; NewMultiKeyEncryptorFromEnvWithLogger threads the service logger through.
func warnIfWeakKey(logger *zap.Logger, envName string, key []byte) {
	if logger == nil {
		return
	}
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		logger.Warn("encryption key is all zeros — this is insecure for production",
			zap.String("env_var", envName))
	}
}
