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

package oauth

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// TokenStore defines the interface for storing and retrieving OAuth tokens
type TokenStore interface {
	UpdateToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error
	GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error)
}

// PostgresTokenStore implements TokenStore using PostgreSQL
type PostgresTokenStore struct {
	db     *pgxpool.Pool
	enc    *encryption.MultiKeyEncryptor
	logger *zap.Logger
}

// NewPostgresTokenStore creates a new PostgreSQL token store
func NewPostgresTokenStore(db *pgxpool.Pool, enc *encryption.MultiKeyEncryptor, logger *zap.Logger) *PostgresTokenStore {
	return &PostgresTokenStore{
		db:     db,
		enc:    enc,
		logger: logger,
	}
}

// UpdateToken writes a refreshed token back to its row. It never inserts: rows are
// created by auth-service on consent and deleted when the streamer disconnects
// YouTube, so a refresh that was in flight during a disconnect must not recreate the
// row. A row that is gone is reported as an error.
func (s *PostgresTokenStore) UpdateToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	encAccess, err := s.enc.EncryptString(token.AccessToken)
	if err != nil {
		s.logger.Error("Failed to encrypt access token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("encrypt access token: %w", err)
	}

	encRefresh, err := s.enc.EncryptString(token.RefreshToken)
	if err != nil {
		s.logger.Error("Failed to encrypt refresh token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("encrypt refresh token: %w", err)
	}

	tag, err := s.db.Exec(ctx, `
UPDATE youtube_oauth_tokens
SET access_token = $3, refresh_token = $4, token_type = $5, expiry = $6, encryption_version = 1, updated_at = NOW()
WHERE user_id = $1 AND channel_id = $2
`,
		userID,
		channelID,
		encAccess,
		encRefresh,
		token.TokenType,
		token.Expiry,
	)
	if err == nil && tag.RowsAffected() == 0 {
		err = fmt.Errorf("token row no longer exists (YouTube disconnected)")
	}

	if err != nil {
		s.logger.Error("Failed to save OAuth token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to save token: %w", err)
	}

	s.logger.Info("Updated OAuth token",
		zap.String("user_id", userID),
		zap.String("channel_id", channelID),
	)

	return nil
}

// GetToken retrieves an OAuth token from the database
func (s *PostgresTokenStore) GetToken(ctx context.Context, userID, channelID string) (*oauth2.Token, error) {
	query := `
SELECT access_token, refresh_token, token_type, expiry, encryption_version
FROM youtube_oauth_tokens
WHERE user_id = $1 AND channel_id = $2
`

	var token oauth2.Token
	var expiry time.Time
	var encryptionVersion int16
	var storedAccess, storedRefresh string

	err := s.db.QueryRow(ctx, query, userID, channelID).Scan(
		&storedAccess,
		&storedRefresh,
		&token.TokenType,
		&expiry,
		&encryptionVersion,
	)

	if err != nil {
		s.logger.Error("Failed to get OAuth token",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if encryptionVersion >= 1 {
		decryptedAccess, decryptErr := s.enc.DecryptString(storedAccess)
		if decryptErr != nil {
			s.logger.Error("Failed to decrypt access token",
				zap.String("user_id", userID),
				zap.String("channel_id", channelID),
				zap.Error(decryptErr),
			)
			return nil, fmt.Errorf("decrypt access token: %w", decryptErr)
		}

		decryptedRefresh, decryptErr := s.enc.DecryptString(storedRefresh)
		if decryptErr != nil {
			s.logger.Error("Failed to decrypt refresh token",
				zap.String("user_id", userID),
				zap.String("channel_id", channelID),
				zap.Error(decryptErr),
			)
			return nil, fmt.Errorf("decrypt refresh token: %w", decryptErr)
		}

		token.AccessToken = decryptedAccess
		token.RefreshToken = decryptedRefresh
	} else {
		s.logger.Warn("Found legacy plaintext tokens; returning without decryption",
			zap.String("user_id", userID),
			zap.String("channel_id", channelID),
		)
		token.AccessToken = storedAccess
		token.RefreshToken = storedRefresh
	}

	token.Expiry = expiry

	return &token, nil
}
