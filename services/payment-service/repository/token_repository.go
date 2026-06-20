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
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// PatreonToken is a decrypted Patreon OAuth credential for one all-chat subject.
// Exactly one of UserID / ViewerID is set (ADR-0019): a connection belongs to a
// streamer users account or a viewer identity, never both.
type PatreonToken struct {
	UserID        *string
	ViewerID      *string
	PatreonUserID string
	AccessToken   string
	RefreshToken  string
	ExpiresAt     time.Time
	Scopes        []string
}

// TokenRepository persists Patreon OAuth credentials (encrypted at rest).
type TokenRepository struct {
	db     *pgxpool.Pool
	cipher *encryption.MultiKeyEncryptor
	logger *zap.Logger
}

// NewTokenRepository builds a TokenRepository.
func NewTokenRepository(db *pgxpool.Pool, cipher *encryption.MultiKeyEncryptor, logger *zap.Logger) *TokenRepository {
	return &TokenRepository{db: db, cipher: cipher, logger: logger}
}

// Upsert stores (or replaces) the Patreon connection for a subject. Re-connecting
// the same subject updates its row (ON CONFLICT on the per-subject partial unique
// index). Linking a Patreon account already tied to a different all-chat identity
// violates the patreon_user_id unique constraint and returns an error — one Patreon
// account maps to exactly one subject (ADR-0019).
func (r *TokenRepository) Upsert(ctx context.Context, t PatreonToken) error {
	encAccess, err := r.encrypt(t.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}
	encRefresh, err := r.encrypt(t.RefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt refresh token: %w", err)
	}
	kid := int(r.currentKid())

	if t.ViewerID != nil && *t.ViewerID != "" {
		_, err = r.db.Exec(ctx, `
			INSERT INTO patreon_oauth_tokens
			    (viewer_id, patreon_user_id, access_token, refresh_token, token_expires_at, granted_scopes, encryption_version, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			ON CONFLICT (viewer_id) WHERE viewer_id IS NOT NULL DO UPDATE SET
			    patreon_user_id    = EXCLUDED.patreon_user_id,
			    access_token       = EXCLUDED.access_token,
			    refresh_token      = EXCLUDED.refresh_token,
			    token_expires_at   = EXCLUDED.token_expires_at,
			    granted_scopes     = EXCLUDED.granted_scopes,
			    encryption_version = EXCLUDED.encryption_version,
			    updated_at         = NOW()`,
			*t.ViewerID, t.PatreonUserID, encAccess, encRefresh, t.ExpiresAt, t.Scopes, kid)
		if err != nil {
			return fmt.Errorf("upsert viewer patreon token: %w", err)
		}
		return nil
	}

	if t.UserID == nil || *t.UserID == "" {
		return fmt.Errorf("patreon token upsert requires a user or viewer subject")
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO patreon_oauth_tokens
		    (user_id, patreon_user_id, access_token, refresh_token, token_expires_at, granted_scopes, encryption_version, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_id) WHERE user_id IS NOT NULL DO UPDATE SET
		    patreon_user_id    = EXCLUDED.patreon_user_id,
		    access_token       = EXCLUDED.access_token,
		    refresh_token      = EXCLUDED.refresh_token,
		    token_expires_at   = EXCLUDED.token_expires_at,
		    granted_scopes     = EXCLUDED.granted_scopes,
		    encryption_version = EXCLUDED.encryption_version,
		    updated_at         = NOW()`,
		*t.UserID, t.PatreonUserID, encAccess, encRefresh, t.ExpiresAt, t.Scopes, kid)
	if err != nil {
		return fmt.Errorf("upsert patreon token: %w", err)
	}
	return nil
}

// UpdateTokens stores refreshed tokens for the connection identified by its Patreon
// user id (unique across subjects). A blank refresh token (some refresh responses
// omit it) leaves the stored refresh token untouched.
func (r *TokenRepository) UpdateTokens(ctx context.Context, patreonUserID string, token *oauth2.Token) error {
	encAccess, err := r.encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("encrypt access token: %w", err)
	}

	if token.RefreshToken == "" {
		_, err = r.db.Exec(ctx, `
			UPDATE patreon_oauth_tokens
			SET access_token = $2, token_expires_at = $3, encryption_version = $4, updated_at = NOW()
			WHERE patreon_user_id = $1`,
			patreonUserID, encAccess, token.Expiry, int(r.currentKid()))
	} else {
		var encRefresh string
		encRefresh, err = r.encrypt(token.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt refresh token: %w", err)
		}
		_, err = r.db.Exec(ctx, `
			UPDATE patreon_oauth_tokens
			SET access_token = $2, refresh_token = $3, token_expires_at = $4, encryption_version = $5, updated_at = NOW()
			WHERE patreon_user_id = $1`,
			patreonUserID, encAccess, encRefresh, token.Expiry, int(r.currentKid()))
	}
	if err != nil {
		return fmt.Errorf("update patreon tokens: %w", err)
	}
	return nil
}

// GetSubjectByPatreonUserID resolves the all-chat subject linked to a Patreon user
// id. Exactly one of the returned (userID, viewerID) is non-nil when found.
func (r *TokenRepository) GetSubjectByPatreonUserID(ctx context.Context, patreonUserID string) (userID, viewerID *string, found bool, err error) {
	err = r.db.QueryRow(ctx,
		"SELECT user_id, viewer_id FROM patreon_oauth_tokens WHERE patreon_user_id = $1", patreonUserID).
		Scan(&userID, &viewerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("lookup patreon token by patreon user id: %w", err)
	}
	return userID, viewerID, true, nil
}

// DeleteByUserID removes a user's Patreon connection (on disconnect).
func (r *TokenRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM patreon_oauth_tokens WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("delete patreon token: %w", err)
	}
	return nil
}

// DeleteByViewerID removes a viewer's Patreon connection (on disconnect).
func (r *TokenRepository) DeleteByViewerID(ctx context.Context, viewerID string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM patreon_oauth_tokens WHERE viewer_id = $1", viewerID); err != nil {
		return fmt.Errorf("delete viewer patreon token: %w", err)
	}
	return nil
}

// ListAll returns up to limit connections (decrypted) for the reconcile job,
// oldest-updated first so repeated passes cover everyone. Each token carries its
// subject (user or viewer).
func (r *TokenRepository) ListAll(ctx context.Context, limit int) ([]PatreonToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT user_id, viewer_id, patreon_user_id, access_token, refresh_token, token_expires_at, granted_scopes
		FROM patreon_oauth_tokens
		ORDER BY updated_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list patreon tokens: %w", err)
	}
	defer rows.Close()

	var out []PatreonToken
	for rows.Next() {
		var t PatreonToken
		var encAccess, encRefresh string
		if err := rows.Scan(&t.UserID, &t.ViewerID, &t.PatreonUserID, &encAccess, &encRefresh, &t.ExpiresAt, &t.Scopes); err != nil {
			r.logger.Warn("Failed to scan patreon token row", zap.Error(err))
			continue
		}
		if t.AccessToken, err = r.decrypt(encAccess); err != nil {
			r.logger.Warn("Failed to decrypt patreon access token", zap.String("patreon_user_id", t.PatreonUserID), zap.Error(err))
			continue
		}
		if t.RefreshToken, err = r.decrypt(encRefresh); err != nil {
			r.logger.Warn("Failed to decrypt patreon refresh token", zap.String("patreon_user_id", t.PatreonUserID), zap.Error(err))
			continue
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patreon token rows: %w", err)
	}
	return out, nil
}

func (r *TokenRepository) currentKid() encryption.KidByte {
	if r.cipher == nil {
		return 1
	}
	return r.cipher.CurrentKid()
}

func (r *TokenRepository) encrypt(s string) (string, error) {
	if r.cipher == nil || s == "" {
		return s, nil
	}
	return r.cipher.EncryptString(s)
}

func (r *TokenRepository) decrypt(s string) (string, error) {
	if r.cipher == nil || s == "" {
		return s, nil
	}
	return r.cipher.DecryptString(s)
}
