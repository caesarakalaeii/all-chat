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
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrTTSConfigNotFound is returned by the TTS config repository when no row
// exists for the requested overlay ID.
var ErrTTSConfigNotFound = errors.New("tts config not found")

// TTSConfigRepository owns CRUD on the overlay_tts_configs table. The
// encrypted_api_key column holds ciphertext produced by shared/encryption at
// the handler layer — this repo is intentionally encryption-agnostic and only
// moves opaque byte slices.
type TTSConfigRepository struct {
	pool *pgxpool.Pool
}

// NewTTSConfigRepository constructs a repo around an existing pgxpool.Pool.
// The pool is owned by the caller (overlay-manager/cmd/main.go) and shared
// with the other repositories in this service.
func NewTTSConfigRepository(pool *pgxpool.Pool) *TTSConfigRepository {
	return &TTSConfigRepository{pool: pool}
}

// GetByOverlayID returns the TTS config row keyed on overlay_id. Returns
// ErrTTSConfigNotFound (not a wrapped pgx.ErrNoRows) when absent so callers
// can errors.Is against a stable sentinel.
func (r *TTSConfigRepository) GetByOverlayID(ctx context.Context, overlayID string) (*models.TTSConfig, error) {
	const q = `
		SELECT id, overlay_id, encrypted_api_key, voice_id, tts_signing_secret, created_at, updated_at
		FROM overlay_tts_configs
		WHERE overlay_id = $1
	`
	cfg := &models.TTSConfig{}
	err := r.pool.QueryRow(ctx, q, overlayID).Scan(
		&cfg.ID, &cfg.OverlayID, &cfg.EncryptedAPIKey, &cfg.VoiceID,
		&cfg.SigningSecret, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTTSConfigNotFound
		}
		return nil, fmt.Errorf("tts_config_repo: scan: %w", err)
	}
	return cfg, nil
}

// CreateOrUpdate upserts keyed on overlay_id. On first insert a fresh 32-byte
// tts_signing_secret is generated. On update the signing secret is preserved
// (rotation is an explicit operation via RotateSigningSecret). This matches
// D-10: rotation is a separate, deliberate user action.
func (r *TTSConfigRepository) CreateOrUpdate(ctx context.Context, overlayID string, encryptedKey []byte, voiceID string) (*models.TTSConfig, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("tts_config_repo: generate signing secret: %w", err)
	}
	const q = `
		INSERT INTO overlay_tts_configs (overlay_id, encrypted_api_key, voice_id, tts_signing_secret)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (overlay_id) DO UPDATE SET
			encrypted_api_key = EXCLUDED.encrypted_api_key,
			voice_id = EXCLUDED.voice_id,
			updated_at = NOW()
		RETURNING id, overlay_id, encrypted_api_key, voice_id, tts_signing_secret, created_at, updated_at
	`
	cfg := &models.TTSConfig{}
	err := r.pool.QueryRow(ctx, q, overlayID, encryptedKey, voiceID, secret).Scan(
		&cfg.ID, &cfg.OverlayID, &cfg.EncryptedAPIKey, &cfg.VoiceID,
		&cfg.SigningSecret, &cfg.CreatedAt, &cfg.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("tts_config_repo: upsert: %w", err)
	}
	return cfg, nil
}

// Delete removes the config row for overlayID. Returns ErrTTSConfigNotFound
// if no row matched.
func (r *TTSConfigRepository) Delete(ctx context.Context, overlayID string) error {
	const q = `DELETE FROM overlay_tts_configs WHERE overlay_id = $1`
	tag, err := r.pool.Exec(ctx, q, overlayID)
	if err != nil {
		return fmt.Errorf("tts_config_repo: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTTSConfigNotFound
	}
	return nil
}

// RotateSigningSecret generates a new 32-byte secret, writes it, and returns
// it. All previously-issued tts_tokens signed with the old secret will fail
// verification after this call (D-10). Returns ErrTTSConfigNotFound if the
// overlay has no existing config row.
func (r *TTSConfigRepository) RotateSigningSecret(ctx context.Context, overlayID string) ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("tts_config_repo: generate new secret: %w", err)
	}
	const q = `
		UPDATE overlay_tts_configs
		   SET tts_signing_secret = $1, updated_at = NOW()
		 WHERE overlay_id = $2
	 RETURNING tts_signing_secret
	`
	var stored []byte
	err := r.pool.QueryRow(ctx, q, secret, overlayID).Scan(&stored)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTTSConfigNotFound
		}
		return nil, fmt.Errorf("tts_config_repo: rotate: %w", err)
	}
	return stored, nil
}
