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
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PgTokenStore resolves the token stored by auth-service in
// instagram_oauth_tokens (migration 096). The stored credential follows the
// same chain as Facebook's (ADR-0060): a Page token obtained from a long-lived
// user token does not expire, so expires_at is normally NULL. The column
// exists anyway because an IG re-auth flow exists and slice B stores the
// 60-day long-lived user token with an expiry when the Page-derived token is
// unavailable — an expired row must fail as UnresolvedTokenError so the source
// deactivates and the streamer re-auths, rather than hammering Graph with a
// dead token.
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

// GetToken returns the decrypted token, or an error when the user has no
// token row for this IG user, the row has expired, or decryption fails
// (fail loud: the poller logs and skips via UnresolvedTokenError).
func (s *PgTokenStore) GetToken(ctx context.Context, userID, igUserID string) (string, error) {
	var (
		encToken string
		version  int
		expires  *time.Time
	)
	err := s.db.QueryRow(ctx,
		`SELECT access_token, encryption_version, expires_at FROM instagram_oauth_tokens WHERE user_id = $1 AND ig_user_id = $2`,
		userID, igUserID,
	).Scan(&encToken, &version, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("instagram: no token stored (user %s, ig user %s)", userID, igUserID)
	}
	if err != nil {
		return "", fmt.Errorf("instagram: load token: %w", err)
	}
	if expires != nil && !expires.After(time.Now()) {
		return "", fmt.Errorf("instagram: token for ig user %s expired at %s (streamer must re-auth)", igUserID, expires.Format(time.RFC3339))
	}
	if version >= 1 {
		if s.cipher == nil {
			return "", errors.New("instagram: encrypted token but no TOKEN_ENCRYPTION_KEY_V1 configured")
		}
		plain, err := s.cipher.DecryptString(encToken)
		if err != nil {
			return "", fmt.Errorf("instagram: decrypt token: %w", err)
		}
		return plain, nil
	}
	return encToken, nil
}
