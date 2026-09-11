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

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PgTokenStore resolves the Page access token stored by auth-service in
// facebook_oauth_tokens (migration 094). Page tokens from a long-lived user
// token do not carry an expiry (ADR-0060), so there is no refresh path here:
// an invalid token surfaces as a Graph API error and the operator reconnects.
type PgTokenStore struct {
	db     *pgxpool.Pool
	cipher *encryption.MultiKeyEncryptor
	logger *zap.Logger
}

// NewPgTokenStore builds the store. cipher may be nil for plaintext rows
// (encryption_version = 0), matching the other listeners' behaviour.
func NewPgTokenStore(db *pgxpool.Pool, cipher *encryption.MultiKeyEncryptor, logger *zap.Logger) *PgTokenStore {
	return &PgTokenStore{db: db, cipher: cipher, logger: logger}
}

// GetPageToken returns the decrypted Page token, or an error when the user has
// no token row for this Page (fail loud: the poller logs and skips).
func (s *PgTokenStore) GetPageToken(ctx context.Context, userID, pageID string) (string, error) {
	var (
		encToken string
		version  int
	)
	err := s.db.QueryRow(ctx,
		`SELECT access_token, encryption_version FROM facebook_oauth_tokens WHERE user_id = $1 AND page_id = $2`,
		userID, pageID,
	).Scan(&encToken, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("facebook: no page token stored (user %s, page %s)", userID, pageID)
	}
	if err != nil {
		return "", fmt.Errorf("facebook: load page token: %w", err)
	}
	if version >= 1 {
		if s.cipher == nil {
			return "", errors.New("facebook: encrypted page token but no TOKEN_ENCRYPTION_KEY_V1 configured")
		}
		plain, err := s.cipher.DecryptString(encToken)
		if err != nil {
			return "", fmt.Errorf("facebook: decrypt page token: %w", err)
		}
		return plain, nil
	}
	return encToken, nil
}
