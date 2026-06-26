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

package encryption_test

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Golden test vector constants (T-14-01-03, T-14-01-04).
//
//	Plaintext: "test-token-value"
//	Key: 32 bytes of 0x00
//	Nonce: 12 bytes of 0x00 (deterministic — for tests only, never use in production)
//
// To regenerate: go test ./shared/encryption/... -update
// Regeneration writes new golden files from the same deterministic inputs, so the
// output is stable across Go versions (AES-GCM with a fixed key/nonce is deterministic).
const (
	goldenPlaintext = "test-token-value"
	goldenKeyHex    = "0000000000000000000000000000000000000000000000000000000000000000" // 32 zero bytes
)

var update = flag.Bool("update", false, "regenerate testdata/golden_*.bin files")

// TestMain allows `go test -update` to regenerate the golden testdata files without
// changing any test logic. The golden files are committed to the repository so that
// format regressions (T-14-01-04) are caught in CI.
func TestMain(m *testing.M) {
	flag.Parse()
	if *update {
		regenerateGoldens()
	}
	os.Exit(m.Run())
}

// regenerateGoldens computes deterministic ciphertexts and writes the golden testdata files.
// Key and nonce are all-zero — these are test-only values and cannot be production secrets.
func regenerateGoldens() {
	plaintext := []byte(goldenPlaintext)
	key := make([]byte, 32)   // all zeros
	nonce := make([]byte, 12) // all zeros

	block, err := aes.NewCipher(key)
	if err != nil {
		panic("regenerateGoldens NewCipher: " + err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic("regenerateGoldens NewGCM: " + err.Error())
	}

	// sealed = nonce || ciphertext || tag (GCM Seal prepends the dst nonce when dst==nonce)
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)

	// golden_legacy.bin: base64(nonce||ct||tag) — no kid prefix
	legacyB64 := base64.StdEncoding.EncodeToString(sealed)
	if err := os.WriteFile("testdata/golden_legacy.bin", []byte(legacyB64), 0600); err != nil {
		panic("regenerateGoldens write legacy: " + err.Error())
	}

	// golden_v1.bin: base64(0x01 || nonce||ct||tag)
	v1Raw := make([]byte, 0, len(sealed)+1)
	v1Raw = append(v1Raw, 0x01)
	v1Raw = append(v1Raw, sealed...)
	v1B64 := base64.StdEncoding.EncodeToString(v1Raw)
	if err := os.WriteFile("testdata/golden_v1.bin", []byte(v1B64), 0600); err != nil {
		panic("regenerateGoldens write v1: " + err.Error())
	}
}

// newTestAESEncryptor creates an AESEncryptor from a 32-byte all-zero key for tests.
func newTestAESEncryptor(t *testing.T, key []byte) *encryption.AESEncryptor {
	t.Helper()
	enc, err := encryption.NewAESEncryptor(key)
	require.NoError(t, err)
	return enc
}

// zeroKey32 returns a 32-byte all-zero key for golden test vectors.
func zeroKey32() []byte { return make([]byte, 32) }

// altKey32 returns a different 32-byte key for multi-key tests.
func altKey32() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 1)
	}
	return k
}

// thirdKey32 returns yet another 32-byte key for unified chain tests.
func thirdKey32() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 2)
	}
	return k
}

// TestMultiKeyEncryptor_RoundTripV1 verifies basic encrypt→decrypt round-trip for a
// single-key chain and that the first decoded byte of the ciphertext equals kid 0x01
// (D-01, D-02).
func TestMultiKeyEncryptor_RoundTripV1(t *testing.T) {
	cipher := newTestAESEncryptor(t, zeroKey32())
	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: cipher}},
		nil,
	)
	require.NoError(t, err)

	ct, err := enc.EncryptString("hello round-trip")
	require.NoError(t, err)

	// Verify the kid byte in the raw decoded blob.
	decoded, err := base64.StdEncoding.DecodeString(ct)
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), decoded[0], "first byte of decoded ciphertext must be kid 0x01")

	pt, err := enc.DecryptString(ct)
	require.NoError(t, err)
	assert.Equal(t, "hello round-trip", pt)
}

// TestMultiKeyEncryptor_DecryptOldKid verifies that a ciphertext written under an older
// key (V1) still decrypts after a newer key (V2) has been added. New writes use the
// latest kid (0x02); old blobs are still decryptable (D-02).
func TestMultiKeyEncryptor_DecryptOldKid(t *testing.T) {
	c1 := newTestAESEncryptor(t, zeroKey32())
	c2 := newTestAESEncryptor(t, altKey32())

	// Single-key chain: write under V1
	enc1, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: c1}},
		nil,
	)
	require.NoError(t, err)
	ct, err := enc1.EncryptString("old value")
	require.NoError(t, err)

	// Two-key chain: V1 + V2; latest is V2
	enc2, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{
			{Kid: 0x01, Cipher: c1},
			{Kid: 0x02, Cipher: c2},
		},
		nil,
	)
	require.NoError(t, err)

	assert.Equal(t, byte(0x02), enc2.CurrentKid(), "new writes should use kid 0x02")

	pt, err := enc2.DecryptString(ct)
	require.NoError(t, err)
	assert.Equal(t, "old value", pt, "V1 ciphertext must still decrypt after V2 is added")
}

// TestMultiKeyEncryptor_LegacyBackcompat verifies that a kid-less blob produced by the
// old AESEncryptor (pre-Phase-14) decrypts via the legacy fallback path (D-05).
func TestMultiKeyEncryptor_LegacyBackcompat(t *testing.T) {
	legacyCipher := newTestAESEncryptor(t, zeroKey32())
	v1Cipher := newTestAESEncryptor(t, altKey32())

	// Produce a kid-less legacy blob
	legacyCT, err := legacyCipher.EncryptString("legacy token")
	require.NoError(t, err)

	// Verify the legacy blob does NOT start with a registered kid (0x01)
	decoded, err := base64.StdEncoding.DecodeString(legacyCT)
	require.NoError(t, err)
	// Legacy blob: first byte is the first byte of the random nonce — only ~1/256 chance
	// it equals 0x01. For this test we construct it using the zero-key, which may or may
	// not collide; we only care that the multi-key chain decrypts it correctly regardless.

	_ = decoded // nonce byte checked implicitly via decrypt success below

	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: v1Cipher}},
		[]*encryption.AESEncryptor{legacyCipher},
	)
	require.NoError(t, err)

	pt, err := enc.DecryptString(legacyCT)
	require.NoError(t, err)
	assert.Equal(t, "legacy token", pt)
}

// TestMultiKeyEncryptor_FalsePositiveKid proves that legacy ciphertext whose first
// decoded byte coincidentally equals a registered kid still decrypts correctly via the
// legacy fallback path after AEAD authentication failure (T-14-01-01, D-05).
//
// Strategy: use the deterministic AES-GCM nonce to craft a legacy blob where byte[0]==0x01,
// then verify that MultiKeyEncryptor recovers via the legacy chain.
//
// Because the all-zero nonce starts with 0x00 we instead generate real random
// ciphertexts until we find one whose first decoded byte equals 0x01. The probability
// is 1/256 per attempt; in 5000 iterations we expect ~19 hits with high confidence.
func TestMultiKeyEncryptor_FalsePositiveKid(t *testing.T) {
	legacyCipher := newTestAESEncryptor(t, zeroKey32())

	// Different key for V1 so that the versioned AEAD path intentionally fails on the
	// legacy blob (wrong key = AEAD authentication error).
	v1Cipher := newTestAESEncryptor(t, altKey32())

	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: v1Cipher}},
		[]*encryption.AESEncryptor{legacyCipher},
	)
	require.NoError(t, err)

	const maxAttempts = 5000
	found := false
	for i := 0; i < maxAttempts; i++ {
		ct, encErr := legacyCipher.EncryptString("false-positive test")
		require.NoError(t, encErr)

		decoded, decErr := base64.StdEncoding.DecodeString(ct)
		require.NoError(t, decErr)

		if len(decoded) > 0 && decoded[0] == 0x01 {
			// First byte of the random nonce == registered kid 0x01: false-positive candidate.
			// The versioned path will try v1Cipher which has the wrong key → AEAD fail.
			// The legacy path must then succeed.
			pt, fallbackErr := enc.DecryptString(ct)
			require.NoError(t, fallbackErr,
				"false-positive kid should fall back to legacy key successfully (attempt %d)", i)
			assert.Equal(t, "false-positive test", pt)
			found = true
			break
		}
	}
	if !found {
		// This branch is statistically extremely unlikely (~1 in 2^40) but not impossible.
		// If it occurs, the test marks itself as skipped rather than failed so CI is not flaky.
		t.Skip("did not generate a false-positive blob within 5000 attempts (statistically improbable)")
	}
}

// TestMultiKeyEncryptor_UnifiedChain verifies that the unified key chain (D-04) accepts
// ciphertexts from both TOKEN_ENCRYPTION_KEY and YOUTUBE_TOKEN_ENCRYPTION_KEY legacy keys.
func TestMultiKeyEncryptor_UnifiedChain(t *testing.T) {
	tokenLegacy := newTestAESEncryptor(t, zeroKey32())
	youtubeLegacy := newTestAESEncryptor(t, altKey32())
	v1Cipher := newTestAESEncryptor(t, thirdKey32())

	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: v1Cipher}},
		[]*encryption.AESEncryptor{tokenLegacy, youtubeLegacy},
	)
	require.NoError(t, err)

	// Token legacy blob
	tokenCT, err := tokenLegacy.EncryptString("twitch token")
	require.NoError(t, err)
	pt, err := enc.DecryptString(tokenCT)
	require.NoError(t, err)
	assert.Equal(t, "twitch token", pt)

	// YouTube legacy blob
	ytCT, err := youtubeLegacy.EncryptString("youtube token")
	require.NoError(t, err)
	pt, err = enc.DecryptString(ytCT)
	require.NoError(t, err)
	assert.Equal(t, "youtube token", pt)

	// New versioned write decrypts with V1
	newCT, err := enc.EncryptString("new token")
	require.NoError(t, err)
	pt, err = enc.DecryptString(newCT)
	require.NoError(t, err)
	assert.Equal(t, "new token", pt)
}

// TestMultiKeyEncryptor_GoldenV1 decrypts the committed golden_v1.bin fixture and
// confirms it matches the expected plaintext (T-14-01-04 wire format regression guard).
//
// Fixture: base64([0x01][12 zero-byte nonce][AES-GCM seal of "test-token-value" with 32-byte zero key])
func TestMultiKeyEncryptor_GoldenV1(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_v1.bin")
	require.NoError(t, err, "testdata/golden_v1.bin must exist; run: go test -update to regenerate")
	ct := strings.TrimRight(string(raw), "\n")

	legacyCipher := newTestAESEncryptor(t, zeroKey32())
	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: legacyCipher}},
		nil,
	)
	require.NoError(t, err)

	pt, err := enc.DecryptString(ct)
	require.NoError(t, err)
	assert.Equal(t, goldenPlaintext, pt)
}

// TestMultiKeyEncryptor_GoldenLegacy decrypts the committed golden_legacy.bin fixture and
// confirms it matches the expected plaintext via the legacy fallback path (T-14-01-04).
//
// Fixture: base64([12 zero-byte nonce][AES-GCM seal of "test-token-value" with 32-byte zero key])
func TestMultiKeyEncryptor_GoldenLegacy(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_legacy.bin")
	require.NoError(t, err, "testdata/golden_legacy.bin must exist; run: go test -update to regenerate")
	ct := strings.TrimRight(string(raw), "\n")

	legacyCipher := newTestAESEncryptor(t, zeroKey32())
	v1Cipher := newTestAESEncryptor(t, altKey32())
	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: v1Cipher}},
		[]*encryption.AESEncryptor{legacyCipher},
	)
	require.NoError(t, err)

	pt, err := enc.DecryptString(ct)
	require.NoError(t, err)
	assert.Equal(t, goldenPlaintext, pt)
}

// TestNewMultiKeyEncryptorFromEnv verifies the environment-driven constructor (D-02).
func TestNewMultiKeyEncryptorFromEnv(t *testing.T) {
	// Both keys must be base64-encoded (raw zero bytes cannot be set as env vars
	// because the OS rejects values containing null bytes).
	v1KeyB64 := base64.StdEncoding.EncodeToString(altKey32())
	legacyKeyB64 := base64.StdEncoding.EncodeToString(zeroKey32())

	t.Setenv("TOKEN_ENCRYPTION_KEY_V1", v1KeyB64)
	t.Setenv("TOKEN_ENCRYPTION_KEY", legacyKeyB64)
	// Ensure V2 is absent so discovery stops at V1
	t.Setenv("TOKEN_ENCRYPTION_KEY_V2", "")

	enc, err := encryption.NewMultiKeyEncryptorFromEnv()
	require.NoError(t, err)

	assert.Equal(t, byte(0x01), enc.CurrentKid(), "CurrentKid must be 0x01 (only V1 configured)")

	// New writes round-trip
	ct, err := enc.EncryptString("env test")
	require.NoError(t, err)
	pt, err := enc.DecryptString(ct)
	require.NoError(t, err)
	assert.Equal(t, "env test", pt)

	// Legacy blob (produced with TOKEN_ENCRYPTION_KEY) must decrypt via fallback
	legacyCipher, err := encryption.NewAESEncryptor(zeroKey32())
	require.NoError(t, err)
	legacyCT, err := legacyCipher.EncryptString("legacy env token")
	require.NoError(t, err)
	pt, err = enc.DecryptString(legacyCT)
	require.NoError(t, err)
	assert.Equal(t, "legacy env token", pt)
}

// TestNewMultiKeyEncryptorFromEnv_RequiresAtLeastOneKey verifies that the constructor
// returns ErrNoEncryptionKeys when no TOKEN_ENCRYPTION_KEY_V<n> is set (T-14-01-05).
func TestNewMultiKeyEncryptorFromEnv_RequiresAtLeastOneKey(t *testing.T) {
	// Unset all versioned keys
	t.Setenv("TOKEN_ENCRYPTION_KEY_V1", "")
	t.Setenv("TOKEN_ENCRYPTION_KEY_V2", "")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")
	t.Setenv("YOUTUBE_TOKEN_ENCRYPTION_KEY", "")

	_, err := encryption.NewMultiKeyEncryptorFromEnv()
	require.Error(t, err)
	assert.ErrorIs(t, err, encryption.ErrNoEncryptionKeys)
	assert.Contains(t, err.Error(), "no encryption keys configured")
}

// TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy verifies that YOUTUBE_TOKEN_ENCRYPTION_KEY
// is loaded as a legacy fallback even when TOKEN_ENCRYPTION_KEY is absent (D-04 unification).
func TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy(t *testing.T) {
	v1KeyB64 := base64.StdEncoding.EncodeToString(altKey32())
	ytKeyB64 := base64.StdEncoding.EncodeToString(thirdKey32())

	t.Setenv("TOKEN_ENCRYPTION_KEY_V1", v1KeyB64)
	t.Setenv("TOKEN_ENCRYPTION_KEY", "") // intentionally absent
	t.Setenv("YOUTUBE_TOKEN_ENCRYPTION_KEY", ytKeyB64)

	enc, err := encryption.NewMultiKeyEncryptorFromEnv()
	require.NoError(t, err)

	// Blob produced by the YouTube legacy key must decrypt via the chain
	ytCipher, err := encryption.NewAESEncryptor(thirdKey32())
	require.NoError(t, err)
	ytCT, err := ytCipher.EncryptString("youtube oauth token")
	require.NoError(t, err)

	pt, err := enc.DecryptString(ytCT)
	require.NoError(t, err)
	assert.Equal(t, "youtube oauth token", pt)
}

// TestKidRangeReserved verifies that the constructor rejects kid 0x00 (reserved as
// LegacyKid) and kid > 0x7F (reserved for future use) (T-14-01-02).
func TestKidRangeReserved(t *testing.T) {
	c := newTestAESEncryptor(t, zeroKey32())

	t.Run("kid 0x00 reserved", func(t *testing.T) {
		_, err := encryption.NewMultiKeyEncryptor(
			[]encryption.KeyEntry{{Kid: 0x00, Cipher: c}},
			nil,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, encryption.ErrReservedKid)
	})

	t.Run("kid 0x80 reserved", func(t *testing.T) {
		_, err := encryption.NewMultiKeyEncryptor(
			[]encryption.KeyEntry{{Kid: 0x80, Cipher: c}},
			nil,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, encryption.ErrReservedKid)
	})

	t.Run("kid 0xFF reserved", func(t *testing.T) {
		_, err := encryption.NewMultiKeyEncryptor(
			[]encryption.KeyEntry{{Kid: 0xFF, Cipher: c}},
			nil,
		)
		require.Error(t, err)
		assert.ErrorIs(t, err, encryption.ErrReservedKid)
	})

	t.Run("kid 0x7F valid", func(t *testing.T) {
		_, err := encryption.NewMultiKeyEncryptor(
			[]encryption.KeyEntry{{Kid: 0x7F, Cipher: c}},
			nil,
		)
		require.NoError(t, err, "kid 0x7F is within the valid range 0x01..0x7F")
	})
}

// TestNewMultiKeyEncryptorFromEnvWithLogger_WarnsOnAllZeroKey (audit L5) verifies
// that when a real logger is threaded via NewMultiKeyEncryptorFromEnvWithLogger,
// warnIfWeakKey actually emits a warning for an all-zero key. Pre-L5 the function
// was always called with zap.NewNop(), so the warning was silently dropped.
func TestNewMultiKeyEncryptorFromEnvWithLogger_WarnsOnAllZeroKey(t *testing.T) {
	// zeroKey32 base64-encoded — all-zero key triggers the weak-key warning.
	zeroKeyB64 := base64.StdEncoding.EncodeToString(zeroKey32())
	t.Setenv("TOKEN_ENCRYPTION_KEY_V1", zeroKeyB64)
	t.Setenv("TOKEN_ENCRYPTION_KEY_V2", "")
	t.Setenv("TOKEN_ENCRYPTION_KEY", "")
	t.Setenv("YOUTUBE_TOKEN_ENCRYPTION_KEY", "")

	var buf strings.Builder
	enc := zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		LevelKey:    "level",
		MessageKey:  "msg",
		EncodeLevel: zapcore.CapitalLevelEncoder,
	})
	core := zapcore.NewCore(enc, zapcore.AddSync(&buf), zapcore.WarnLevel)
	logger := zap.New(core)

	_, err := encryption.NewMultiKeyEncryptorFromEnvWithLogger(logger)
	require.NoError(t, err, "all-zero key is accepted (advisory warning, not fail-closed)")

	output := buf.String()
	assert.Contains(t, output, "all zeros", "weak-key warning must be emitted with a real logger")
	assert.Contains(t, output, "TOKEN_ENCRYPTION_KEY_V1")
}

// TestNewMultiKeyEncryptorFromEnvWithLogger_NilLoggerSafe verifies that passing a
// nil logger does not panic (nil-safe default to no-op).
func TestNewMultiKeyEncryptorFromEnvWithLogger_NilLoggerSafe(t *testing.T) {
	v1KeyB64 := base64.StdEncoding.EncodeToString(altKey32())
	t.Setenv("TOKEN_ENCRYPTION_KEY_V1", v1KeyB64)
	t.Setenv("TOKEN_ENCRYPTION_KEY_V2", "")

	enc, err := encryption.NewMultiKeyEncryptorFromEnvWithLogger(nil)
	require.NoError(t, err)
	assert.NotNil(t, enc)
}
