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
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// permanentFailSuppressDuration is pushed onto token_expires_at when a
// non-retryable refresh error is detected. The token will be excluded from
// all future batches until the user re-authenticates (which resets the column
// to a real expiry). 30 days is long enough that it is effectively permanent
// without being infinite.
const permanentFailSuppressDuration = 30 * 24 * time.Hour

// QueryGetExpiringUserTokens is exported so that unit tests can assert the
// SQL contains a bounded recovery window instead of the unbounded form.
const QueryGetExpiringUserTokens = `
	SELECT id, auth_provider, username, display_name,
	       access_token, refresh_token, token_expires_at
	FROM users
	WHERE (
	    (token_expires_at < $1 AND token_expires_at > NOW())
	    OR (token_expires_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW())
	  )
	  AND refresh_token IS NOT NULL
	  AND refresh_token != ''
	  AND is_banned = false
	ORDER BY token_expires_at ASC
	LIMIT 100
`

// QueryGetExpiringViewerTokens is exported for test assertions.
const QueryGetExpiringViewerTokens = `
	SELECT id, platform, username, display_name,
	       access_token, refresh_token, token_expires_at, user_id
	FROM viewer_sessions
	WHERE (
	    (token_expires_at < $1 AND token_expires_at > NOW())
	    OR (token_expires_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW())
	  )
	  AND refresh_token IS NOT NULL
	  AND refresh_token != ''
	ORDER BY token_expires_at ASC
	LIMIT 100
`

// QueryGetExpiringYouTubeTokens is exported for test assertions.
const QueryGetExpiringYouTubeTokens = `
	SELECT user_id, channel_id, access_token, refresh_token, expiry
	FROM youtube_oauth_tokens
	WHERE (
	    (expiry < $1 AND expiry > NOW())
	    OR (expiry BETWEEN NOW() - INTERVAL '48 hours' AND NOW())
	  )
	  AND refresh_token IS NOT NULL
	  AND refresh_token != ''
	ORDER BY expiry ASC
	LIMIT 100
`

// QueryGetExpiringTwitchLinkTokens selects expiring linked Twitch credentials
// (twitch_oauth_tokens, ADR-0016) — the chat grants of accounts whose login
// provider is not Twitch. Same bounded 48-hour recovery window as the other
// token sources. Exported for test assertions.
const QueryGetExpiringTwitchLinkTokens = `
	SELECT user_id, twitch_login, access_token, refresh_token, token_expires_at
	FROM twitch_oauth_tokens
	WHERE (
	    (token_expires_at < $1 AND token_expires_at > NOW())
	    OR (token_expires_at BETWEEN NOW() - INTERVAL '48 hours' AND NOW())
	  )
	  AND refresh_token IS NOT NULL
	  AND refresh_token != ''
	ORDER BY token_expires_at ASC
	LIMIT 100
`

// ExpiringToken represents a token that needs to be refreshed
type ExpiringToken struct {
	// Common fields
	ID           string
	Platform     string // "twitch", "youtube", "kick"
	Username     string
	DisplayName  string
	RefreshToken string
	AccessToken  string
	ExpiresAt    time.Time

	// Type-specific fields
	TokenType string // "user", "viewer", "youtube_channel", "twitch_link"
	ChannelID string // For YouTube channel tokens (channel_id) and linked Twitch credentials (twitch_login)
	SessionID string // For viewer sessions
}

// TokenRepository handles database operations for OAuth tokens
type TokenRepository struct {
	db     *pgxpool.Pool
	cipher *encryption.MultiKeyEncryptor
	logger *zap.Logger
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *pgxpool.Pool, cipher *encryption.MultiKeyEncryptor, logger *zap.Logger) *TokenRepository {
	return &TokenRepository{
		db:     db,
		cipher: cipher,
		logger: logger,
	}
}

// GetExpiringUserTokens returns user tokens expiring within the specified duration.
// Also recovers already-expired tokens within the last 48 hours to handle transient
// failures. Tokens older than 48 hours are excluded — they are considered permanently
// failed and should have been suppressed by MarkUserTokenPermanentlyFailed.
func (r *TokenRepository) GetExpiringUserTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, QueryGetExpiringUserTokens, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring user tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*ExpiringToken
	for rows.Next() {
		var token ExpiringToken
		var encAccessToken, encRefreshToken string

		err := rows.Scan(
			&token.ID,
			&token.Platform,
			&token.Username,
			&token.DisplayName,
			&encAccessToken,
			&encRefreshToken,
			&token.ExpiresAt,
		)
		if err != nil {
			r.logger.Warn("Failed to scan user token row", zap.Error(err))
			continue
		}

		// Decrypt tokens
		token.AccessToken, err = r.decryptToken(encAccessToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt access token",
				zap.String("user_id", token.ID),
				zap.Error(err),
			)
			continue
		}

		token.RefreshToken, err = r.decryptToken(encRefreshToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt refresh token",
				zap.String("user_id", token.ID),
				zap.Error(err),
			)
			continue
		}

		token.TokenType = "user"
		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user token rows: %w", err)
	}

	return tokens, nil
}

// GetExpiringViewerTokens returns viewer session tokens expiring within the specified duration.
// Only recovers tokens that expired within the last 48 hours. Older expired tokens are
// assumed to have been permanently failed and suppressed via MarkViewerTokenPermanentlyFailed.
func (r *TokenRepository) GetExpiringViewerTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, QueryGetExpiringViewerTokens, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring viewer tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*ExpiringToken
	for rows.Next() {
		var token ExpiringToken
		var encAccessToken, encRefreshToken string
		var userID *string

		err := rows.Scan(
			&token.SessionID,
			&token.Platform,
			&token.Username,
			&token.DisplayName,
			&encAccessToken,
			&encRefreshToken,
			&token.ExpiresAt,
			&userID,
		)
		if err == nil && userID != nil {
			token.ID = *userID
		}
		if err != nil {
			r.logger.Warn("Failed to scan viewer token row", zap.Error(err))
			continue
		}

		// Decrypt tokens
		token.AccessToken, err = r.decryptToken(encAccessToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt viewer access token",
				zap.String("session_id", token.SessionID),
				zap.Error(err),
			)
			continue
		}

		token.RefreshToken, err = r.decryptToken(encRefreshToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt viewer refresh token",
				zap.String("session_id", token.SessionID),
				zap.Error(err),
			)
			continue
		}

		token.TokenType = "viewer"
		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating viewer token rows: %w", err)
	}

	return tokens, nil
}

// GetExpiringYouTubeTokens returns YouTube channel tokens expiring within the specified duration.
// Only recovers tokens that expired within the last 48 hours. Older expired tokens are
// assumed to have been permanently failed and suppressed via MarkYouTubeTokenPermanentlyFailed.
func (r *TokenRepository) GetExpiringYouTubeTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, QueryGetExpiringYouTubeTokens, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring YouTube tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*ExpiringToken
	for rows.Next() {
		var token ExpiringToken
		var encAccessToken, encRefreshToken string

		err := rows.Scan(
			&token.ID, // user_id
			&token.ChannelID,
			&encAccessToken,
			&encRefreshToken,
			&token.ExpiresAt,
		)
		if err != nil {
			r.logger.Warn("Failed to scan YouTube token row", zap.Error(err))
			continue
		}

		// Decrypt tokens
		token.AccessToken, err = r.decryptToken(encAccessToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt YouTube access token",
				zap.String("user_id", token.ID),
				zap.String("channel_id", token.ChannelID),
				zap.Error(err),
			)
			continue
		}

		token.RefreshToken, err = r.decryptToken(encRefreshToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt YouTube refresh token",
				zap.String("user_id", token.ID),
				zap.String("channel_id", token.ChannelID),
				zap.Error(err),
			)
			continue
		}

		token.TokenType = "youtube_channel"
		token.Platform = "youtube"
		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating YouTube token rows: %w", err)
	}

	return tokens, nil
}

// UpdateUserTokens updates the OAuth tokens for a user after refresh
func (r *TokenRepository) UpdateUserTokens(ctx context.Context, userID string, token *oauth2.Token) error {
	encAccessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encRefreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
		UPDATE users
		SET access_token = $2,
		    refresh_token = $3,
		    token_expires_at = $4,
		    updated_at = $5
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, userID, encAccessToken, encRefreshToken, token.Expiry, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update user tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	return nil
}

// UpdateViewerTokens updates the OAuth tokens for a viewer session after refresh
func (r *TokenRepository) UpdateViewerTokens(ctx context.Context, sessionID string, token *oauth2.Token) error {
	encAccessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encRefreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
		UPDATE viewer_sessions
		SET access_token = $2,
		    refresh_token = $3,
		    token_expires_at = $4,
		    updated_at = $5
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, sessionID, encAccessToken, encRefreshToken, token.Expiry, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update viewer tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("viewer session not found: %s", sessionID)
	}

	return nil
}

// UpdateYouTubeTokens updates the OAuth tokens for a YouTube channel after refresh
func (r *TokenRepository) UpdateYouTubeTokens(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	encAccessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encRefreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
		UPDATE youtube_oauth_tokens
		SET access_token = $3,
		    refresh_token = $4,
		    expiry = $5,
		    updated_at = $6
		WHERE user_id = $1 AND channel_id = $2
	`

	result, err := r.db.Exec(ctx, query, userID, channelID, encAccessToken, encRefreshToken, token.Expiry, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update YouTube tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("YouTube token not found: user_id=%s, channel_id=%s", userID, channelID)
	}

	return nil
}

// GetUserOverlays returns all active overlay IDs for a user
func (r *TokenRepository) GetUserOverlays(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT id
		FROM overlays
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user overlays: %w", err)
	}
	defer rows.Close()

	var overlayIDs []string
	for rows.Next() {
		var overlayID string
		if err := rows.Scan(&overlayID); err != nil {
			r.logger.Warn("Failed to scan overlay ID", zap.Error(err))
			continue
		}
		overlayIDs = append(overlayIDs, overlayID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlay rows: %w", err)
	}

	return overlayIDs, nil
}

// MarkUserTokenPermanentlyFailed pushes the user's token_expires_at forward by
// suppressDuration (typically 30 days). This removes the token from future refresh
// batches. The user must re-authenticate through the normal login flow to reset the
// expiry to a real value.
func (r *TokenRepository) MarkUserTokenPermanentlyFailed(ctx context.Context, userID string, suppressDuration time.Duration) error {
	query := `
		UPDATE users
		SET token_expires_at = NOW() + $2::interval,
		    updated_at = NOW()
		WHERE id = $1
	`
	interval := fmt.Sprintf("%d seconds", int(suppressDuration.Seconds()))
	result, err := r.db.Exec(ctx, query, userID, interval)
	if err != nil {
		return fmt.Errorf("failed to mark user token permanently failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}

// MarkViewerTokenPermanentlyFailed pushes the viewer session's token_expires_at
// forward by suppressDuration, removing it from future refresh batches.
func (r *TokenRepository) MarkViewerTokenPermanentlyFailed(ctx context.Context, sessionID string, suppressDuration time.Duration) error {
	query := `
		UPDATE viewer_sessions
		SET token_expires_at = NOW() + $2::interval,
		    updated_at = NOW()
		WHERE id = $1
	`
	interval := fmt.Sprintf("%d seconds", int(suppressDuration.Seconds()))
	result, err := r.db.Exec(ctx, query, sessionID, interval)
	if err != nil {
		return fmt.Errorf("failed to mark viewer token permanently failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("viewer session not found: %s", sessionID)
	}
	return nil
}

// MarkYouTubeTokenPermanentlyFailed pushes the YouTube channel token's expiry
// forward by suppressDuration, removing it from future refresh batches.
func (r *TokenRepository) MarkYouTubeTokenPermanentlyFailed(ctx context.Context, userID, channelID string, suppressDuration time.Duration) error {
	query := `
		UPDATE youtube_oauth_tokens
		SET expiry = NOW() + $3::interval,
		    updated_at = NOW()
		WHERE user_id = $1 AND channel_id = $2
	`
	interval := fmt.Sprintf("%d seconds", int(suppressDuration.Seconds()))
	result, err := r.db.Exec(ctx, query, userID, channelID, interval)
	if err != nil {
		return fmt.Errorf("failed to mark YouTube token permanently failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("YouTube token not found: user_id=%s, channel_id=%s", userID, channelID)
	}
	return nil
}

// GetExpiringTwitchLinkTokens returns linked Twitch credentials (ADR-0016,
// twitch_oauth_tokens) expiring within the specified duration. These rows
// carry the EventSub chat grant of accounts whose login provider is not
// Twitch; letting them lapse silently demotes the channel to the IRC listener.
func (r *TokenRepository) GetExpiringTwitchLinkTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, QueryGetExpiringTwitchLinkTokens, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring linked Twitch tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*ExpiringToken
	for rows.Next() {
		var token ExpiringToken
		var encAccessToken, encRefreshToken string

		err := rows.Scan(
			&token.ID,        // user_id
			&token.ChannelID, // twitch_login
			&encAccessToken,
			&encRefreshToken,
			&token.ExpiresAt,
		)
		if err != nil {
			r.logger.Warn("Failed to scan linked Twitch token row", zap.Error(err))
			continue
		}

		token.AccessToken, err = r.decryptToken(encAccessToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt linked Twitch access token",
				zap.String("user_id", token.ID),
				zap.String("twitch_login", token.ChannelID),
				zap.Error(err),
			)
			continue
		}

		token.RefreshToken, err = r.decryptToken(encRefreshToken)
		if err != nil {
			r.logger.Warn("Failed to decrypt linked Twitch refresh token",
				zap.String("user_id", token.ID),
				zap.String("twitch_login", token.ChannelID),
				zap.Error(err),
			)
			continue
		}

		token.Username = token.ChannelID
		token.TokenType = "twitch_link"
		token.Platform = "twitch"
		tokens = append(tokens, &token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating linked Twitch token rows: %w", err)
	}

	return tokens, nil
}

// UpdateTwitchLinkTokens updates a linked Twitch credential after refresh.
func (r *TokenRepository) UpdateTwitchLinkTokens(ctx context.Context, userID, twitchLogin string, token *oauth2.Token) error {
	encAccessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encRefreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
		UPDATE twitch_oauth_tokens
		SET access_token = $3,
		    refresh_token = $4,
		    token_expires_at = $5,
		    updated_at = $6
		WHERE user_id = $1 AND twitch_login = $2
	`

	result, err := r.db.Exec(ctx, query, userID, twitchLogin, encAccessToken, encRefreshToken, token.Expiry, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update linked Twitch tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("linked Twitch token not found: user_id=%s, twitch_login=%s", userID, twitchLogin)
	}

	return nil
}

// MarkTwitchLinkTokenPermanentlyFailed pushes the linked Twitch credential's
// expiry forward by suppressDuration, removing it from future refresh batches.
// The channel then falls back to the IRC listener until the user re-consents.
func (r *TokenRepository) MarkTwitchLinkTokenPermanentlyFailed(ctx context.Context, userID, twitchLogin string, suppressDuration time.Duration) error {
	query := `
		UPDATE twitch_oauth_tokens
		SET token_expires_at = NOW() + $3::interval,
		    updated_at = NOW()
		WHERE user_id = $1 AND twitch_login = $2
	`
	interval := fmt.Sprintf("%d seconds", int(suppressDuration.Seconds()))
	result, err := r.db.Exec(ctx, query, userID, twitchLogin, interval)
	if err != nil {
		return fmt.Errorf("failed to mark linked Twitch token permanently failed: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("linked Twitch token not found: user_id=%s, twitch_login=%s", userID, twitchLogin)
	}
	return nil
}

// encryptToken encrypts a token string
func (r *TokenRepository) encryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.EncryptString(token)
}

// decryptToken decrypts a token string
func (r *TokenRepository) decryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.DecryptString(token)
}
