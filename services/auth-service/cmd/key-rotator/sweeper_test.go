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

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// keyBytes produces a 32-byte key from a short string for testing.
func keyBytes(s string) []byte {
	b := make([]byte, 32)
	copy(b, []byte(s))
	return b
}

// makeTestEncryptor builds a MultiKeyEncryptor with one versioned key (kid=0x01)
// and an optional legacy key.
func makeTestEncryptor(t *testing.T, legacyKeyBytes []byte) *encryption.MultiKeyEncryptor {
	t.Helper()
	cipher1, err := encryption.NewAESEncryptor(keyBytes("test-key-v1-32bytes-padding-xxx"))
	require.NoError(t, err)

	var legacyKeys []*encryption.AESEncryptor
	if legacyKeyBytes != nil {
		legacyEnc, err := encryption.NewAESEncryptor(legacyKeyBytes)
		require.NoError(t, err)
		legacyKeys = append(legacyKeys, legacyEnc)
	}

	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: cipher1}},
		legacyKeys,
	)
	require.NoError(t, err)
	return enc
}

// makeTwoKeyEncryptor builds a MultiKeyEncryptor with two versioned keys;
// kid=0x01 is "old", kid=0x02 is "current".
func makeTwoKeyEncryptor(t *testing.T) (*encryption.MultiKeyEncryptor, *encryption.MultiKeyEncryptor) {
	t.Helper()
	cipher1, err := encryption.NewAESEncryptor(keyBytes("test-key-v1-32bytes-padding-xxx"))
	require.NoError(t, err)
	cipher2, err := encryption.NewAESEncryptor(keyBytes("test-key-v2-32bytes-padding-xxx"))
	require.NoError(t, err)

	// oldEnc can only encrypt/decrypt with kid=0x01
	oldEnc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: cipher1}},
		nil,
	)
	require.NoError(t, err)

	// newEnc can decrypt both kids, writes with kid=0x02
	newEnc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{
			{Kid: 0x01, Cipher: cipher1},
			{Kid: 0x02, Cipher: cipher2},
		},
		nil,
	)
	require.NoError(t, err)
	return oldEnc, newEnc
}

// --- testcontainer setup ---

const testSchema = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL DEFAULT '',
    refresh_token TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS viewer_sessions (
    id TEXT PRIMARY KEY,
    platform VARCHAR(50) NOT NULL DEFAULT 'twitch',
    platform_user_id VARCHAR(100) NOT NULL DEFAULT 'uid',
    username VARCHAR(100) NOT NULL DEFAULT 'user',
    display_name VARCHAR(200) NOT NULL DEFAULT 'User',
    token_expires_at TIMESTAMP NOT NULL DEFAULT NOW() + INTERVAL '1 hour',
    access_token TEXT NOT NULL DEFAULT '',
    refresh_token TEXT,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS youtube_oauth_tokens (
    user_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    access_token TEXT NOT NULL DEFAULT '',
    refresh_token TEXT NOT NULL DEFAULT '',
    encryption_version SMALLINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW(),
    PRIMARY KEY (user_id, channel_id)
);

CREATE TABLE IF NOT EXISTS overlay_tts_configs (
    id TEXT PRIMARY KEY,
    encrypted_api_key BYTEA NOT NULL,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS kick_oauth_tokens (
    id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL DEFAULT '',
    refresh_token TEXT NOT NULL DEFAULT '',
    encryption_version SMALLINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tiktok_oauth_tokens (
    id TEXT PRIMARY KEY,
    access_token TEXT NOT NULL DEFAULT '',
    refresh_token TEXT NOT NULL DEFAULT '',
    encryption_version SMALLINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT NOW()
);
`

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "start postgres container")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, testSchema)
	require.NoError(t, err, "create schema")

	cleanup := func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
	return pool, cleanup
}

// --- encryptIfNotCurrentKid helper tests ---

// TestSweeper_EncryptIfNotCurrentKid_AlreadyCurrent: blob with kid==CurrentKid() → returns (input, false, nil).
func TestSweeper_EncryptIfNotCurrentKid_AlreadyCurrent(t *testing.T) {
	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)
	s := NewSweeper(nil, enc, logger)

	plaintext := "my-secret-token"
	encrypted, err := enc.EncryptString(plaintext)
	require.NoError(t, err)

	result, changed, err := s.encryptIfNotCurrentKid(encrypted)
	require.NoError(t, err)
	assert.False(t, changed, "should not be re-encrypted when already at current kid")
	assert.Equal(t, encrypted, result)
}

// TestSweeper_EncryptIfNotCurrentKid_OldKid: blob with kid==0x01, CurrentKid()==0x02 → re-encrypted.
func TestSweeper_EncryptIfNotCurrentKid_OldKid(t *testing.T) {
	oldEnc, newEnc := makeTwoKeyEncryptor(t)
	logger := zaptest.NewLogger(t)
	s := NewSweeper(nil, newEnc, logger)

	plaintext := "my-secret-token"
	// Encrypt with old encryptor (kid=0x01)
	oldBlob, err := oldEnc.EncryptString(plaintext)
	require.NoError(t, err)

	// Verify it has kid=0x01
	raw, err := base64.StdEncoding.DecodeString(oldBlob)
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), raw[0], "old blob should have kid=0x01")

	// Sweep with newEnc (current kid=0x02)
	result, changed, err := s.encryptIfNotCurrentKid(oldBlob)
	require.NoError(t, err)
	assert.True(t, changed)

	// Result should have kid=0x02
	rawNew, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.Equal(t, byte(0x02), rawNew[0], "re-encrypted blob should have kid=0x02")

	// Decryption with newEnc should return original plaintext
	decrypted, err := newEnc.DecryptString(result)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestSweeper_EncryptIfNotCurrentKid_LegacyKidless: kid-less legacy blob → re-encrypted.
func TestSweeper_EncryptIfNotCurrentKid_LegacyKidless(t *testing.T) {
	legacyKey := keyBytes("legacy-key-32bytes-padding-xxxx")
	enc := makeTestEncryptor(t, legacyKey)
	logger := zaptest.NewLogger(t)
	s := NewSweeper(nil, enc, logger)

	// Build a kid-less legacy blob using AESEncryptor directly
	legacyCipher, err := encryption.NewAESEncryptor(legacyKey)
	require.NoError(t, err)
	plaintext := "legacy-token-value"
	legacyBlob, err := legacyCipher.EncryptString(plaintext)
	require.NoError(t, err)

	result, changed, err := s.encryptIfNotCurrentKid(legacyBlob)
	require.NoError(t, err)
	assert.True(t, changed, "legacy blob should be re-encrypted")

	// Result should decrypt to original plaintext
	decrypted, err := enc.DecryptString(result)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestSweeper_EncryptIfNotCurrentKid_LegacyKidlessCollidingFirstByte guards
// audit #12: a kid-less legacy blob whose first decoded byte coincidentally
// equals the current kid (the 1/256 case) MUST still be re-encrypted, not
// skipped. The old byte-only fast-path mis-classified it as "already current"
// and left it un-migrated, becoming undecryptable after legacy-key retirement.
func TestSweeper_EncryptIfNotCurrentKid_LegacyKidlessCollidingFirstByte(t *testing.T) {
	legacyKey := keyBytes("legacy-key-32bytes-padding-xxxx")
	enc := makeTestEncryptor(t, legacyKey) // CurrentKid() == 0x01
	logger := zaptest.NewLogger(t)
	s := NewSweeper(nil, enc, logger)

	legacyCipher, err := encryption.NewAESEncryptor(legacyKey)
	require.NoError(t, err)
	plaintext := "legacy-token-colliding-first-byte"

	// The legacy nonce is random, so retry until the first decoded byte collides
	// with the current kid (deterministic in practice: P(hit)=1/256 per attempt).
	var legacyBlob string
	for i := 0; i < 100000; i++ {
		b, err := legacyCipher.EncryptString(plaintext)
		require.NoError(t, err)
		raw, derr := base64.StdEncoding.DecodeString(b)
		require.NoError(t, derr)
		if raw[0] == enc.CurrentKid() {
			legacyBlob = b
			break
		}
	}
	require.NotEmpty(t, legacyBlob, "failed to construct a colliding-first-byte legacy blob")

	result, changed, err := s.encryptIfNotCurrentKid(legacyBlob)
	require.NoError(t, err)
	assert.True(t, changed, "audit #12: colliding-first-byte legacy blob MUST be re-encrypted, not skipped")
	assert.NotEqual(t, legacyBlob, result)

	decrypted, err := enc.DecryptString(result)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestSweeper_EncryptIfNotCurrentKid_DecryptFails: garbage blob → returns ("", false, error).
func TestSweeper_EncryptIfNotCurrentKid_DecryptFails(t *testing.T) {
	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)
	s := NewSweeper(nil, enc, logger)

	// base64-encoded garbage that doesn't decrypt
	garblage := base64.StdEncoding.EncodeToString([]byte("thisisnotvalidciphertextxxxxxx"))
	_, changed, err := s.encryptIfNotCurrentKid(garblage)
	assert.Error(t, err, "should return error for garbage ciphertext")
	assert.False(t, changed)
}

// --- per-table sweep tests using testcontainers ---

// TestSweeper_SweepUsers_Idempotent: 1 current, 1 old-kid, 1 legacy → exactly 2 updates; second sweep touches 0.
func TestSweeper_SweepUsers_Idempotent(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Build three-layer encryptor: kid=0x01 (old), kid=0x02 (current), plus a legacy key.
	legacyKeyBytes := keyBytes("legacy-key-for-idempotent-test-")
	cipher1, err := encryption.NewAESEncryptor(keyBytes("test-key-v1-32bytes-padding-xxx"))
	require.NoError(t, err)
	cipher2, err := encryption.NewAESEncryptor(keyBytes("test-key-v2-32bytes-padding-xxx"))
	require.NoError(t, err)
	legacyCipher, err := encryption.NewAESEncryptor(legacyKeyBytes)
	require.NoError(t, err)

	// oldEnc can encrypt/decrypt with kid=0x01 only (no legacy)
	oldEnc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: cipher1}},
		nil,
	)
	require.NoError(t, err)

	// sweepEnc can decrypt kid=0x01, kid=0x02, and legacy; writes with kid=0x02
	sweepEnc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{
			{Kid: 0x01, Cipher: cipher1},
			{Kid: 0x02, Cipher: cipher2},
		},
		[]*encryption.AESEncryptor{legacyCipher},
	)
	require.NoError(t, err)

	logger := zaptest.NewLogger(t)

	// Row 1: already at current kid (kid=0x02 via sweepEnc)
	currentBlob, err := sweepEnc.EncryptString("current-token-1")
	require.NoError(t, err)
	// Row 2: old kid (kid=0x01 via oldEnc)
	oldBlob, err := oldEnc.EncryptString("old-token-2")
	require.NoError(t, err)
	// Row 3: legacy kid-less blob produced by legacyCipher (AESEncryptor, no kid prefix)
	legacyBlob, err := legacyCipher.EncryptString("legacy-token-3")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ($1,$2,$2),($3,$4,$4),($5,$6,$6)`,
		"user-1", currentBlob, "user-2", oldBlob, "user-3", legacyBlob)
	require.NoError(t, err)

	s := NewSweeper(pool, sweepEnc, logger, WithBatchDelay(0))
	err = s.sweepUsers(ctx)
	require.NoError(t, err)

	// user-1 was already current → skipped; user-2 and user-3 re-encrypted
	assert.Equal(t, int64(3), s.metrics.RowsScanned["users"])
	assert.Equal(t, int64(2), s.metrics.RowsReEncrypted["users"])
	assert.Equal(t, int64(1), s.metrics.RowsSkipped["users"])

	// Second sweep: all rows should now be at current kid → 0 re-encrypted
	s2 := NewSweeper(pool, sweepEnc, logger, WithBatchDelay(0))
	err = s2.sweepUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), s2.metrics.RowsScanned["users"])
	assert.Equal(t, int64(0), s2.metrics.RowsReEncrypted["users"])
	assert.Equal(t, int64(3), s2.metrics.RowsSkipped["users"])
}

// TestSweeper_SkipsCurrentKid: sweeper does not re-encrypt a row already at CurrentKid.
func TestSweeper_SkipsCurrentKid(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)

	blob, err := enc.EncryptString("my-token")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('u1', $1, $1)`, blob)
	require.NoError(t, err)

	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepUsers(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.metrics.RowsScanned["users"])
	assert.Equal(t, int64(0), s.metrics.RowsReEncrypted["users"])
	assert.Equal(t, int64(1), s.metrics.RowsSkipped["users"])
}

// TestSweeper_SweepUsers_DryRun: dry-run=true; no UPDATEs issued; metrics still report counts.
func TestSweeper_SweepUsers_DryRun(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	oldEnc, newEnc := makeTwoKeyEncryptor(t)
	logger := zaptest.NewLogger(t)

	oldBlob, err := oldEnc.EncryptString("token-to-rotate")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('dry-user', $1, $1)`, oldBlob)
	require.NoError(t, err)

	s := NewSweeper(pool, newEnc, logger, WithDryRun(true), WithBatchDelay(0))
	err = s.sweepUsers(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["users"], "dry-run still counts would-update rows")

	// Verify the DB was NOT mutated
	var storedAt string
	err = pool.QueryRow(ctx, `SELECT access_token FROM users WHERE id='dry-user'`).Scan(&storedAt)
	require.NoError(t, err)
	assert.Equal(t, oldBlob, storedAt, "dry-run must not mutate the DB")
}

// TestSweeper_DryRun is an alias covering the broader SweepAll dry-run contract.
func TestSweeper_DryRun(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	oldEnc, newEnc := makeTwoKeyEncryptor(t)
	logger := zaptest.NewLogger(t)

	oldBlob, err := oldEnc.EncryptString("token-value")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('u-dry', $1, $1)`, oldBlob)
	require.NoError(t, err)
	// Add kick row at encryption_version=1 with old blob
	_, err = pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (id, access_token, refresh_token, encryption_version) VALUES ('k-dry', $1, $1, 1)`, oldBlob)
	require.NoError(t, err)

	s := NewSweeper(pool, newEnc, logger, WithDryRun(true), WithBatchDelay(0))
	err = s.SweepAll(ctx)
	require.NoError(t, err)

	// No DB changes
	var ua, ka string
	_ = pool.QueryRow(ctx, `SELECT access_token FROM users WHERE id='u-dry'`).Scan(&ua)
	_ = pool.QueryRow(ctx, `SELECT access_token FROM kick_oauth_tokens WHERE id='k-dry'`).Scan(&ka)
	assert.Equal(t, oldBlob, ua, "users row not mutated in dry-run")
	assert.Equal(t, oldBlob, ka, "kick row not mutated in dry-run")
}

// TestSweeper_TTSBytea: BYTEA encrypted_api_key is re-encrypted correctly.
func TestSweeper_TTSBytea(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	// Legacy AESEncryptor produces a base64 string; when Phase 13 stored it in BYTEA
	// it stored the raw bytes of that string (i.e., []byte(base64string)).
	legacyKey := keyBytes("tts-legacy-key-32bytes-paddingx")
	legacyCipher, err := encryption.NewAESEncryptor(legacyKey)
	require.NoError(t, err)
	legacyBlob, err := legacyCipher.EncryptString("elevenlabs-api-key-secret")
	require.NoError(t, err)
	// Phase 13 stored string bytes into BYTEA
	byteaValue := []byte(legacyBlob)

	// Our current encryptor has legacyKey as fallback
	currentCipher, err := encryption.NewAESEncryptor(keyBytes("test-key-v1-32bytes-padding-xxx"))
	require.NoError(t, err)
	enc, err := encryption.NewMultiKeyEncryptor(
		[]encryption.KeyEntry{{Kid: 0x01, Cipher: currentCipher}},
		[]*encryption.AESEncryptor{legacyCipher},
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO overlay_tts_configs (id, encrypted_api_key) VALUES ('tts-1', $1)`, byteaValue)
	require.NoError(t, err)

	logger := zaptest.NewLogger(t)
	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepOverlayTTSConfigs(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.metrics.RowsScanned["overlay_tts_configs"])
	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["overlay_tts_configs"])

	// Verify updated BYTEA decrypts to the original plaintext
	var updatedBytes []byte
	err = pool.QueryRow(ctx, `SELECT encrypted_api_key FROM overlay_tts_configs WHERE id='tts-1'`).Scan(&updatedBytes)
	require.NoError(t, err)
	decrypted, err := enc.DecryptString(string(updatedBytes))
	require.NoError(t, err)
	assert.Equal(t, "elevenlabs-api-key-secret", decrypted)
}

// TestSweeper_HandlesDecryptError: corrupted row is skipped, error counted, sweep continues.
func TestSweeper_HandlesDecryptError(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)

	// Good row
	goodBlob, err := enc.EncryptString("valid-token")
	require.NoError(t, err)
	// Corrupted row: valid base64 but not valid ciphertext for our key
	corruptedBlob := base64.StdEncoding.EncodeToString([]byte("corruption-that-wont-decrypt-ok"))

	// Insert good first, then corrupted (order matters for metrics check)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('good', $1, $1), ('bad', $2, $2)`,
		goodBlob, corruptedBlob)
	require.NoError(t, err)

	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepUsers(ctx)
	require.NoError(t, err, "sweep should continue past decrypt errors")

	assert.Equal(t, int64(2), s.metrics.RowsScanned["users"])
	assert.GreaterOrEqual(t, s.metrics.Errors["users"], int64(1), "should count decrypt error")
}

// TestSweeper_Telemetry: verify metrics populated correctly across multiple tables.
func TestSweeper_Telemetry(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	oldEnc, newEnc := makeTwoKeyEncryptor(t)
	logger := zaptest.NewLogger(t)

	// Insert rows in users and kick_oauth_tokens
	oldBlob, err := oldEnc.EncryptString("telemetry-token")
	require.NoError(t, err)
	currentBlob, err := newEnc.EncryptString("current-token")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('t1', $1, $1), ('t2', $2, $2)`,
		oldBlob, currentBlob)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (id, access_token, refresh_token, encryption_version) VALUES ('k1', $1, $1, 1)`,
		oldBlob)
	require.NoError(t, err)

	s := NewSweeper(pool, newEnc, logger, WithBatchDelay(0))
	err = s.SweepAll(ctx)
	require.NoError(t, err)

	// users: 2 scanned, 1 re-encrypted, 1 skipped
	assert.Equal(t, int64(2), s.metrics.RowsScanned["users"])
	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["users"])
	assert.Equal(t, int64(1), s.metrics.RowsSkipped["users"])

	// kick_oauth_tokens: 1 scanned, 1 re-encrypted (it's at kid=0x01, needs kid=0x02)
	assert.Equal(t, int64(1), s.metrics.RowsScanned["kick_oauth_tokens"])
	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["kick_oauth_tokens"])
}

// TestSweeper_SkipsTikTokV0: tiktok_oauth_tokens rows with encryption_version=0 are skipped.
func TestSweeper_SkipsTikTokV0(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)

	// Insert a v0 row (plaintext — Node.js wrote this before Phase 14)
	_, err := pool.Exec(ctx, `INSERT INTO tiktok_oauth_tokens (id, access_token, refresh_token, encryption_version) VALUES ('tt1', 'plaintext-access', 'plaintext-refresh', 0)`)
	require.NoError(t, err)

	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepTikTokOAuthTokens(ctx)
	require.NoError(t, err)

	// v0 row is not in the SQL result (WHERE encryption_version >= 1), so scanned = 0
	assert.Equal(t, int64(0), s.metrics.RowsScanned["tiktok_oauth_tokens"])
	assert.Equal(t, int64(0), s.metrics.RowsReEncrypted["tiktok_oauth_tokens"])

	// Verify DB row is untouched
	var at string
	err = pool.QueryRow(ctx, `SELECT access_token FROM tiktok_oauth_tokens WHERE id='tt1'`).Scan(&at)
	require.NoError(t, err)
	assert.Equal(t, "plaintext-access", at, "v0 tiktok row must not be touched")
}

// TestSweeper_KickV0EncryptsDirect: kick v0 (plaintext) rows are encrypted directly without decrypt step.
func TestSweeper_KickV0EncryptsDirect(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)

	// Insert a v0 kick row (plaintext)
	plainAt := "kick-plaintext-access-token"
	plainRt := "kick-plaintext-refresh-token"
	_, err := pool.Exec(ctx, `INSERT INTO kick_oauth_tokens (id, access_token, refresh_token, encryption_version) VALUES ('k-v0', $1, $2, 0)`,
		plainAt, plainRt)
	require.NoError(t, err)

	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepKickOAuthTokens(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.metrics.RowsScanned["kick_oauth_tokens"])
	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["kick_oauth_tokens"])

	// Verify the stored tokens decrypt to the plaintext values
	var storedAt, storedRt string
	var version int
	err = pool.QueryRow(ctx, `SELECT access_token, refresh_token, encryption_version FROM kick_oauth_tokens WHERE id='k-v0'`).
		Scan(&storedAt, &storedRt, &version)
	require.NoError(t, err)

	decAt, err := enc.DecryptString(storedAt)
	require.NoError(t, err)
	assert.Equal(t, plainAt, decAt)

	decRt, err := enc.DecryptString(storedRt)
	require.NoError(t, err)
	assert.Equal(t, plainRt, decRt)

	assert.Equal(t, 1, version, "encryption_version should be updated to 1 after direct encryption")
}

// TestSweeper_YouTubeV0EncryptsDirect guards audit #13: youtube v0 (plaintext)
// rows must be encrypted directly (no Decrypt step) and bumped to
// encryption_version=1 — previously the sweep ran encryptIfNotCurrentKid
// unconditionally, so every v0 row failed Decrypt, was counted as an error, and
// stayed plaintext at rest.
func TestSweeper_YouTubeV0EncryptsDirect(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)

	// Insert a v0 youtube row (plaintext).
	plainAt := "yt-plaintext-access-token"
	plainRt := "yt-plaintext-refresh-token"
	_, err := pool.Exec(ctx, `INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, encryption_version) VALUES ('u-v0', 'c-v0', $1, $2, 0)`,
		plainAt, plainRt)
	require.NoError(t, err)

	s := NewSweeper(pool, enc, logger, WithBatchDelay(0))
	err = s.sweepYouTubeOAuthTokens(ctx)
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.metrics.RowsScanned["youtube_oauth_tokens"])
	assert.Equal(t, int64(1), s.metrics.RowsReEncrypted["youtube_oauth_tokens"])
	assert.Equal(t, int64(0), s.metrics.Errors["youtube_oauth_tokens"], "v0 row must not be counted as an error")

	// Verify the stored tokens are now encrypted and decrypt back to plaintext.
	var storedAt, storedRt string
	var version int
	err = pool.QueryRow(ctx, `SELECT access_token, refresh_token, encryption_version FROM youtube_oauth_tokens WHERE user_id='u-v0' AND channel_id='c-v0'`).
		Scan(&storedAt, &storedRt, &version)
	require.NoError(t, err)
	assert.NotEqual(t, plainAt, storedAt, "access_token must no longer be plaintext")

	decAt, err := enc.DecryptString(storedAt)
	require.NoError(t, err)
	assert.Equal(t, plainAt, decAt)

	decRt, err := enc.DecryptString(storedRt)
	require.NoError(t, err)
	assert.Equal(t, plainRt, decRt)

	assert.Equal(t, 1, version, "encryption_version should be 1 after direct encryption")
}

// TestSweeper_Idempotent runs SweepAll twice and verifies the second run touches 0 rows.
func TestSweeper_Idempotent(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	oldEnc, newEnc := makeTwoKeyEncryptor(t)
	logger := zaptest.NewLogger(t)

	// Seed a mix of rows
	oldBlob, _ := oldEnc.EncryptString("token-a")
	currentBlob, _ := newEnc.EncryptString("token-b")
	_, err := pool.Exec(ctx, `INSERT INTO users (id, access_token, refresh_token) VALUES ('ia', $1, $1), ('ib', $2, $2)`,
		oldBlob, currentBlob)
	require.NoError(t, err)

	s1 := NewSweeper(pool, newEnc, logger, WithBatchDelay(0))
	err = s1.SweepAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), s1.metrics.RowsReEncrypted["users"])

	// Second run: all rows at current kid now
	s2 := NewSweeper(pool, newEnc, logger, WithBatchDelay(0))
	err = s2.SweepAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), s2.metrics.RowsReEncrypted["users"], "second sweep must touch 0 rows")
}

// TestSweeper_SkipTable: WithSkipTable prevents a table from being swept.
func TestSweeper_SkipTable(t *testing.T) {
	enc := makeTestEncryptor(t, nil)
	logger := zaptest.NewLogger(t)
	observerLogger, _ := zap.NewNop(), zap.NewNop()
	_ = observerLogger

	// Use nil pool — if sweep is called it will panic; skip should prevent any call
	s := NewSweeper(nil, enc, logger, WithSkipTable("users"))
	assert.True(t, s.skipTables["users"])
}
