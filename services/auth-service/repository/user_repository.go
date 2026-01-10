package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

// StringCipher interface for encryption/decryption
type StringCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// UserRepository handles user data persistence
type UserRepository struct {
	db     *pgxpool.Pool
	cipher StringCipher
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, cipher StringCipher) *UserRepository {
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
id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
is_admin, access_token, refresh_token, token_expires_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
`

	_, err = r.db.Exec(ctx, query,
		user.ID, user.TwitchID, user.GoogleID, user.KickID, user.AuthProvider, user.Username, user.DisplayName,
		user.ProfileImageURL, user.IsAdmin, accessToken, refreshToken,
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
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_banned, banned_at, banned_reason, banned_by,
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
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_banned, banned_at, banned_reason, banned_by,
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
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, access_token, refresh_token, token_expires_at, created_at, updated_at
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

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE username = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, username))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
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
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_banned, banned_at, banned_reason, banned_by,
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

// StoreYouTubeToken stores YouTube OAuth token in youtube_oauth_tokens table keyed by channel ID
func (r *UserRepository) StoreYouTubeToken(ctx context.Context, userID, channelID string, token *oauth2.Token) error {
	if channelID == "" {
		return fmt.Errorf("channel_id is required for storing YouTube tokens")
	}

	query := `
		INSERT INTO youtube_oauth_tokens (
			user_id, channel_id, access_token, refresh_token, token_type, expiry, encryption_version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, channel_id)
		DO UPDATE SET
			access_token = EXCLUDED.access_token,
			refresh_token = EXCLUDED.refresh_token,
			token_type = EXCLUDED.token_type,
			expiry = EXCLUDED.expiry,
			encryption_version = EXCLUDED.encryption_version,
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
		token.Expiry, 1, now, now,
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
		&user.ID, &user.TwitchID, &user.GoogleID, &user.KickID, &user.AuthProvider, &user.Username, &user.DisplayName,
		&user.ProfileImageURL, &user.IsAdmin, &user.IsBanned, &user.BannedAt, &user.BannedReason, &user.BannedBy,
		&encryptedAccessToken, &encryptedRefreshToken,
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

// GetAllUsers retrieves all users (admin only)
func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
ORDER BY created_at DESC
`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user, err := r.scanUserFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// GetUserByID retrieves a user by their ID (admin only)
func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	query := `
SELECT id, twitch_id, google_id, kick_id, auth_provider, username, display_name, profile_image_url,
           is_admin, is_banned, banned_at, banned_reason, banned_by,
           access_token, refresh_token, token_expires_at, created_at, updated_at
FROM users
WHERE id = $1
`

	user, err := r.scanUser(r.db.QueryRow(ctx, query, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return user, nil
}

// scanUserFromRows scans a user from pgx.Rows
func (r *UserRepository) scanUserFromRows(rows pgx.Rows) (*models.User, error) {
	var user models.User
	var encryptedAccessToken, encryptedRefreshToken string

	err := rows.Scan(
		&user.ID, &user.TwitchID, &user.GoogleID, &user.KickID, &user.AuthProvider,
		&user.Username, &user.DisplayName, &user.ProfileImageURL,
		&user.IsAdmin, &user.IsBanned, &user.BannedAt, &user.BannedReason, &user.BannedBy,
		&encryptedAccessToken, &encryptedRefreshToken,
		&user.TokenExpiresAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	accessToken, err := r.decryptToken(encryptedAccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}
	user.AccessToken = accessToken

	refreshToken, err := r.decryptToken(encryptedRefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}
	user.RefreshToken = refreshToken

	return &user, nil
}

// BanUser bans a user account and deactivates their overlays/sources
func (r *UserRepository) BanUser(ctx context.Context, userID, adminID, reason string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update user ban status
	_, err = tx.Exec(ctx, `
		UPDATE users
		SET is_banned = true,
			banned_at = NOW(),
			banned_reason = $1,
			banned_by = $2
		WHERE id = $3
	`, reason, adminID, userID)
	if err != nil {
		return fmt.Errorf("failed to ban user: %w", err)
	}

	// Deactivate all overlays for this user
	_, err = tx.Exec(ctx, `
		UPDATE overlays
		SET is_active = false
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate overlays: %w", err)
	}

	// Deactivate all sources for this user's overlays
	_, err = tx.Exec(ctx, `
		UPDATE overlay_chat_sources
		SET is_active = false
		WHERE overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to deactivate sources: %w", err)
	}

	return tx.Commit(ctx)
}

// UnbanUser removes ban from user account (overlays remain inactive - manual reactivation)
func (r *UserRepository) UnbanUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users
		SET is_banned = false,
			banned_at = NULL,
			banned_reason = NULL,
			banned_by = NULL
		WHERE id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to unban user: %w", err)
	}
	return nil
}

// GetBannedUsers returns list of banned users with pagination
func (r *UserRepository) GetBannedUsers(ctx context.Context, limit, offset int) ([]*models.User, error) {
	query := `
		SELECT id, username, display_name, is_banned, banned_at, banned_reason, banned_by
		FROM users
		WHERE is_banned = true
		ORDER BY banned_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query banned users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var u models.User
		err := rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.IsBanned, &u.BannedAt, &u.BannedReason, &u.BannedBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan banned user: %w", err)
		}
		users = append(users, &u)
	}
	return users, nil
}

// BanPlatformID adds platform ID to ban list
func (r *UserRepository) BanPlatformID(ctx context.Context, platform, platformID, adminID, reason string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO banned_platform_ids (platform, platform_id, banned_by, reason)
		VALUES ($1, $2, $3, $4)
	`, platform, platformID, adminID, reason)
	if err != nil {
		return fmt.Errorf("failed to ban platform ID: %w", err)
	}
	return nil
}

// IsPlatformIDBanned checks if a platform ID is banned
func (r *UserRepository) IsPlatformIDBanned(ctx context.Context, platform, platformID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM banned_platform_ids
			WHERE platform = $1 AND platform_id = $2 AND is_active = true
		)
	`, platform, platformID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check platform ban: %w", err)
	}
	return exists, nil
}
