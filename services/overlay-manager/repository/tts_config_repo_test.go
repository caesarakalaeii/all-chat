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

package repository

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTTSConfigTestDB starts a Postgres container, applies the minimum
// schema needed for overlay_tts_configs (overlays PK target + the trigger
// function + the overlay_tts_configs table itself), and returns a repo + a
// cleanup callback. Mirrors the pattern in overlay_repo_test.go.
func setupTTSConfigTestDB(t *testing.T) (*TTSConfigRepository, func(), func(overlayID string) error) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Create overlays table (FK target) and the trigger function + the
	// overlay_tts_configs table we are testing.
	overlayRepo, err := NewOverlayRepository(connStr)
	require.NoError(t, err)

	_, err = overlayRepo.pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;

		CREATE TABLE IF NOT EXISTS overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT TRUE,
			is_public_for_viewers BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS overlay_tts_configs (
			id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id         UUID NOT NULL UNIQUE REFERENCES overlays(id) ON DELETE CASCADE,
			encrypted_api_key  BYTEA NOT NULL,
			voice_id           TEXT NOT NULL,
			tts_signing_secret BYTEA NOT NULL,
			created_at         TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at         TIMESTAMP NOT NULL DEFAULT NOW()
		);

		CREATE TRIGGER update_overlay_tts_configs_updated_at
			BEFORE UPDATE ON overlay_tts_configs
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
	`)
	require.NoError(t, err)

	repo := NewTTSConfigRepository(overlayRepo.pool)

	// Helper: seed a fresh overlay row and return its UUID so tests can
	// create TTS configs referencing a valid overlay.
	seedOverlay := func(overlayID string) error {
		_, ferr := overlayRepo.pool.Exec(ctx, `
			INSERT INTO overlays (id, user_id, name)
			VALUES ($1, $2, $3)
		`, overlayID, uuid.NewString(), "test-overlay")
		return ferr
	}

	cleanup := func() {
		overlayRepo.pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return repo, cleanup, seedOverlay
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// TestTTSConfigRoundtrip — CreateOrUpdate followed by GetByOverlayID
// returns byte-identical encrypted_api_key + matching voice_id.
func TestTTSConfigRoundtrip(t *testing.T) {
	repo, cleanup, seed := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := uuid.NewString()
	require.NoError(t, seed(overlayID))

	encKey := randomBytes(t, 128)
	voiceID := "21m00Tcm4TlvDq8ikWAM"

	created, err := repo.CreateOrUpdate(ctx, overlayID, encKey, voiceID)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, overlayID, created.OverlayID)
	assert.Equal(t, encKey, created.EncryptedAPIKey)
	assert.Equal(t, voiceID, created.VoiceID)
	assert.Len(t, created.SigningSecret, 32)

	got, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, encKey, got.EncryptedAPIKey)
	assert.Equal(t, voiceID, got.VoiceID)
	assert.Equal(t, created.SigningSecret, got.SigningSecret)
}

// TestTTSConfigGetNotFoundSentinel — GetByOverlayID returns
// ErrTTSConfigNotFound for a missing overlay.
func TestTTSConfigGetNotFoundSentinel(t *testing.T) {
	repo, cleanup, _ := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, err := repo.GetByOverlayID(ctx, uuid.NewString())
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrTTSConfigNotFound), "expected ErrTTSConfigNotFound, got: %v", err)
}

// TestTTSConfigCreateOrUpdateIdempotent — second call for the same overlay
// updates (no duplicate row, voice_id reflects the second call).
func TestTTSConfigCreateOrUpdateIdempotent(t *testing.T) {
	repo, cleanup, seed := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := uuid.NewString()
	require.NoError(t, seed(overlayID))

	_, err := repo.CreateOrUpdate(ctx, overlayID, randomBytes(t, 32), "voice-A")
	require.NoError(t, err)

	newKey := randomBytes(t, 48)
	second, err := repo.CreateOrUpdate(ctx, overlayID, newKey, "voice-B")
	require.NoError(t, err)
	assert.Equal(t, "voice-B", second.VoiceID)
	assert.Equal(t, newKey, second.EncryptedAPIKey)

	// Only one row should exist for this overlay.
	got, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, "voice-B", got.VoiceID)
	assert.Equal(t, newKey, got.EncryptedAPIKey)
}

// TestTTSConfigDelete — Delete removes the row; subsequent GetByOverlayID
// yields ErrTTSConfigNotFound.
func TestTTSConfigDelete(t *testing.T) {
	repo, cleanup, seed := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := uuid.NewString()
	require.NoError(t, seed(overlayID))

	_, err := repo.CreateOrUpdate(ctx, overlayID, randomBytes(t, 16), "voice-del")
	require.NoError(t, err)

	require.NoError(t, repo.Delete(ctx, overlayID))

	_, err = repo.GetByOverlayID(ctx, overlayID)
	assert.True(t, errors.Is(err, ErrTTSConfigNotFound))
}

// TestTTSConfigDeleteMissingReturnsSentinel — Delete on a non-existent row
// returns ErrTTSConfigNotFound.
func TestTTSConfigDeleteMissingReturnsSentinel(t *testing.T) {
	repo, cleanup, _ := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	err := repo.Delete(ctx, uuid.NewString())
	assert.True(t, errors.Is(err, ErrTTSConfigNotFound))
}

// TestTTSConfigRotateSigningSecret — RotateSigningSecret writes a new 32-byte
// value, returns it, and a subsequent GetByOverlayID reflects the new value.
func TestTTSConfigRotateSigningSecret(t *testing.T) {
	repo, cleanup, seed := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := uuid.NewString()
	require.NoError(t, seed(overlayID))

	created, err := repo.CreateOrUpdate(ctx, overlayID, randomBytes(t, 32), "voice-rot")
	require.NoError(t, err)

	oldSecret := created.SigningSecret

	newSecret, err := repo.RotateSigningSecret(ctx, overlayID)
	require.NoError(t, err)
	assert.Len(t, newSecret, 32)
	assert.NotEqual(t, oldSecret, newSecret, "rotation must produce a different secret")

	got, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, newSecret, got.SigningSecret)
}

// TestTTSConfigRotateMissingReturnsSentinel — rotating a non-existent row
// returns ErrTTSConfigNotFound.
func TestTTSConfigRotateMissingReturnsSentinel(t *testing.T) {
	repo, cleanup, _ := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	_, err := repo.RotateSigningSecret(ctx, uuid.NewString())
	assert.True(t, errors.Is(err, ErrTTSConfigNotFound))
}

// TestTTSConfigRoundtripRandom64Bytes — the encrypted_api_key column is
// declared BYTEA, so arbitrary binary (including embedded NULs) must
// roundtrip intact.
func TestTTSConfigRoundtripRandom64Bytes(t *testing.T) {
	repo, cleanup, seed := setupTTSConfigTestDB(t)
	defer cleanup()

	ctx := context.Background()
	overlayID := uuid.NewString()
	require.NoError(t, seed(overlayID))

	rand64 := randomBytes(t, 64)
	_, err := repo.CreateOrUpdate(ctx, overlayID, rand64, "voice-bytes")
	require.NoError(t, err)

	got, err := repo.GetByOverlayID(ctx, overlayID)
	require.NoError(t, err)
	assert.Equal(t, rand64, got.EncryptedAPIKey)
}
