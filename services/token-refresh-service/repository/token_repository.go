package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

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
	TokenType    string // "user", "viewer", "youtube_channel"
	ChannelID    string // For YouTube channel tokens
	SessionID    string // For viewer sessions
}

// TokenRepository handles database operations for OAuth tokens
type TokenRepository struct {
	db     *pgxpool.Pool
	cipher crypto.StringCipher
	logger *zap.Logger
}

// NewTokenRepository creates a new token repository
func NewTokenRepository(db *pgxpool.Pool, cipher crypto.StringCipher, logger *zap.Logger) *TokenRepository {
	return &TokenRepository{
		db:     db,
		cipher: cipher,
		logger: logger,
	}
}

// GetExpiringUserTokens returns user tokens expiring within the specified duration
func (r *TokenRepository) GetExpiringUserTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	query := `
		SELECT id, auth_provider, username, display_name,
		       access_token, refresh_token, token_expires_at
		FROM users
		WHERE token_expires_at < $1
		  AND token_expires_at > NOW()
		  AND refresh_token IS NOT NULL
		  AND refresh_token != ''
		  AND is_banned = false
		ORDER BY token_expires_at ASC
		LIMIT 100
	`

	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, query, expiryThreshold)
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

// GetExpiringViewerTokens returns viewer session tokens expiring within the specified duration
func (r *TokenRepository) GetExpiringViewerTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	query := `
		SELECT id, platform, username, display_name,
		       access_token, refresh_token, token_expires_at
		FROM viewer_sessions
		WHERE token_expires_at < $1
		  AND token_expires_at > NOW()
		  AND refresh_token IS NOT NULL
		  AND refresh_token != ''
		ORDER BY token_expires_at ASC
		LIMIT 100
	`

	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, query, expiryThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to query expiring viewer tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*ExpiringToken
	for rows.Next() {
		var token ExpiringToken
		var encAccessToken, encRefreshToken string

		err := rows.Scan(
			&token.SessionID,
			&token.Platform,
			&token.Username,
			&token.DisplayName,
			&encAccessToken,
			&encRefreshToken,
			&token.ExpiresAt,
		)
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

// GetExpiringYouTubeTokens returns YouTube channel tokens expiring within the specified duration
func (r *TokenRepository) GetExpiringYouTubeTokens(ctx context.Context, expiresWithin time.Duration) ([]*ExpiringToken, error) {
	query := `
		SELECT user_id, channel_id, access_token, refresh_token, expiry
		FROM youtube_oauth_tokens
		WHERE expiry < $1
		  AND expiry > NOW()
		  AND refresh_token IS NOT NULL
		  AND refresh_token != ''
		ORDER BY expiry ASC
		LIMIT 100
	`

	expiryThreshold := time.Now().Add(expiresWithin)
	rows, err := r.db.Query(ctx, query, expiryThreshold)
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

// encryptToken encrypts a token string
func (r *TokenRepository) encryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Encrypt(token)
}

// decryptToken decrypts a token string
func (r *TokenRepository) decryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Decrypt(token)
}
