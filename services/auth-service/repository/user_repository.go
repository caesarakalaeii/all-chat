package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/shared/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// UserRepository handles user data persistence
type UserRepository struct {
	db     *pgxpool.Pool
	cipher crypto.StringCipher
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, cipher crypto.StringCipher) *UserRepository {
	return &UserRepository{db: db, cipher: cipher}
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.ID = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	accessToken, err := r.encryptToken(user.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	refreshToken, err := r.encryptToken(user.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
INSERT INTO users (
id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
access_token, refresh_token, token_expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
`

	_, err = r.db.Exec(ctx, query,
		user.ID, user.TwitchID, user.GoogleID, user.TikTokOpenID, user.KickID, user.AuthProvider, user.Username, user.DisplayName,
		user.ProfileImageURL, accessToken, refreshToken,
		user.TokenExpiresAt, user.CreatedAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetByTwitchID retrieves a user by Twitch ID
func (r *UserRepository) GetByTwitchID(ctx context.Context, twitchID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE twitch_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, twitchID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Twitch ID: %w", err)
	}

	return user, nil
}

// GetByGoogleID retrieves a user by Google ID
func (r *UserRepository) GetByGoogleID(ctx context.Context, googleID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE google_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, googleID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Google ID: %w", err)
	}

	return user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now()
	accessToken, err := r.encryptToken(user.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(user.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	query := `
UPDATE users
SET username = $2, display_name = $3, profile_image_url = $4,
	access_token = $5, refresh_token = $6, token_expires_at = $7, updated_at = $8
WHERE id = $1
`

	result, err := r.db.Exec(ctx, query,
		user.ID, user.Username, user.DisplayName, user.ProfileImageURL,
		accessToken, refreshToken, user.TokenExpiresAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// Delete removes a user by ID
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("user id is required")
	}

	result, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// UpdateTokens updates only the OAuth tokens for a user
func (r *UserRepository) UpdateTokens(ctx context.Context, userID, accessToken, refreshToken string, expiresAt time.Time) error {
	encAccessToken, err := r.encryptToken(accessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	encRefreshToken, err := r.encryptToken(refreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}
	query := `
UPDATE users
SET access_token = $2, refresh_token = $3, token_expires_at = $4, updated_at = $5
WHERE id = $1
`

	result, err := r.db.Exec(ctx, query, userID, encAccessToken, encRefreshToken, expiresAt, time.Now())
	if err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// GetByKickID retrieves a user by Kick ID
func (r *UserRepository) GetByKickID(ctx context.Context, kickID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE kick_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, kickID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by Kick ID: %w", err)
	}

	return user, nil
}

// GetByTikTokID retrieves a user by TikTok Open ID
func (r *UserRepository) GetByTikTokID(ctx context.Context, tiktokOpenID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, tiktok_open_id, kick_id, auth_provider, username, display_name, profile_image_url,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE tiktok_open_id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, tiktokOpenID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by TikTok ID: %w", err)
	}

	return user, nil
}

// StoreYouTubeToken stores YouTube OAuth token in youtube_oauth_tokens table keyed by channel ID
func (r *UserRepository) StoreYouTubeToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	if channelID == "" {
		return fmt.Errorf("channel_id is required for storing YouTube tokens")
	}

	query := `
		INSERT INTO youtube_oauth_tokens (
			user_id, channel_id, access_token, refresh_token, token_type, expiry, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	tokenType := token.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	accessToken, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}
	refreshToken, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	_, err = r.db.Exec(ctx, query,
		userID, channelID, accessToken, refreshToken, tokenType,
		token.Expiry, now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to store YouTube token: %w", err)
	}

	return nil
}

func (r *UserRepository) scanUser(row pgx.Row) (*models.User, error) {
	user := &models.User{}
	var encryptedAccessToken, encryptedRefreshToken string

	err := row.Scan(
		&user.ID, &user.TwitchID, &user.GoogleID, &user.TikTokOpenID, &user.KickID, &user.AuthProvider, &user.Username, &user.DisplayName,
		&user.ProfileImageURL, &encryptedAccessToken, &encryptedRefreshToken,
		&user.TokenExpiresAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	user.AccessToken, err = r.decryptToken(encryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	user.RefreshToken, err = r.decryptToken(encryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	return user, nil
}

func (r *UserRepository) encryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Encrypt(token)
}

func (r *UserRepository) decryptToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Decrypt(token)
}
