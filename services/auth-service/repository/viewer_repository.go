package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ViewerRepository handles database operations for viewer sessions
type ViewerRepository struct {
	db     *pgxpool.Pool
	cipher StringCipher
}

// NewViewerRepository creates a new ViewerRepository
func NewViewerRepository(db *pgxpool.Pool, cipher StringCipher) *ViewerRepository {
	return &ViewerRepository{
		db:     db,
		cipher: cipher,
	}
}

// GetByPlatformUserID retrieves a viewer session by platform and platform_user_id
func (r *ViewerRepository) GetByPlatformUserID(ctx context.Context, platform, platformUserID string) (*models.ViewerSession, error) {
	query := `
		SELECT id, platform, platform_user_id, username, display_name, avatar_url,
		       access_token, refresh_token, token_expires_at,
		       last_message_at, message_count_1min, message_count_1hour,
		       rate_limit_reset_1min, rate_limit_reset_1hour,
		       created_at, updated_at
		FROM viewer_sessions
		WHERE platform = $1 AND platform_user_id = $2
	`

	var session models.ViewerSession
	err := r.db.QueryRow(ctx, query, platform, platformUserID).Scan(
		&session.ID,
		&session.Platform,
		&session.PlatformUserID,
		&session.Username,
		&session.DisplayName,
		&session.AvatarURL,
		&session.AccessToken,
		&session.RefreshToken,
		&session.TokenExpiresAt,
		&session.LastMessageAt,
		&session.MessageCount1Min,
		&session.MessageCount1Hour,
		&session.RateLimitReset1Min,
		&session.RateLimitReset1Hour,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get viewer session: %w", err)
	}

	return &session, nil
}

// GetByID retrieves a viewer session by ID
func (r *ViewerRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ViewerSession, error) {
	query := `
		SELECT id, platform, platform_user_id, username, display_name, avatar_url,
		       access_token, refresh_token, token_expires_at,
		       last_message_at, message_count_1min, message_count_1hour,
		       rate_limit_reset_1min, rate_limit_reset_1hour,
		       created_at, updated_at
		FROM viewer_sessions
		WHERE id = $1
	`

	var session models.ViewerSession
	err := r.db.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.Platform,
		&session.PlatformUserID,
		&session.Username,
		&session.DisplayName,
		&session.AvatarURL,
		&session.AccessToken,
		&session.RefreshToken,
		&session.TokenExpiresAt,
		&session.LastMessageAt,
		&session.MessageCount1Min,
		&session.MessageCount1Hour,
		&session.RateLimitReset1Min,
		&session.RateLimitReset1Hour,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("viewer session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get viewer session: %w", err)
	}

	return &session, nil
}

// Create creates a new viewer session
func (r *ViewerRepository) Create(ctx context.Context, session *models.ViewerSession) error {
	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}

	// Tokens are already encrypted by the handler
	query := `
		INSERT INTO viewer_sessions (
			id, platform, platform_user_id, username, display_name, avatar_url,
			access_token, refresh_token, token_expires_at,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		session.ID,
		session.Platform,
		session.PlatformUserID,
		session.Username,
		session.DisplayName,
		session.AvatarURL,
		session.AccessToken,
		session.RefreshToken,
		session.TokenExpiresAt,
	).Scan(&session.CreatedAt, &session.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create viewer session: %w", err)
	}

	return nil
}

// Update updates an existing viewer session
func (r *ViewerRepository) Update(ctx context.Context, session *models.ViewerSession) error {
	query := `
		UPDATE viewer_sessions
		SET username = $2,
		    display_name = $3,
		    avatar_url = $4,
		    access_token = $5,
		    refresh_token = $6,
		    token_expires_at = $7,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(ctx, query,
		session.ID,
		session.Username,
		session.DisplayName,
		session.AvatarURL,
		session.AccessToken,
		session.RefreshToken,
		session.TokenExpiresAt,
	).Scan(&session.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to update viewer session: %w", err)
	}

	return nil
}

// UpdateRateLimits updates the rate limiting counters
func (r *ViewerRepository) UpdateRateLimits(ctx context.Context, sessionID uuid.UUID, count1Min, count1Hour int, reset1Min, reset1Hour time.Time) error {
	query := `
		UPDATE viewer_sessions
		SET message_count_1min = $2,
		    message_count_1hour = $3,
		    rate_limit_reset_1min = $4,
		    rate_limit_reset_1hour = $5,
		    last_message_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
	`

	_, err := r.db.Exec(ctx, query,
		sessionID,
		count1Min,
		count1Hour,
		reset1Min,
		reset1Hour,
	)

	if err != nil {
		return fmt.Errorf("failed to update rate limits: %w", err)
	}

	return nil
}

// Delete deletes a viewer session (logout)
func (r *ViewerRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM viewer_sessions WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete viewer session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("viewer session not found")
	}

	return nil
}

// LogMessage logs a message sent by a viewer
func (r *ViewerRepository) LogMessage(ctx context.Context, log *models.ViewerMessageLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}

	query := `
		INSERT INTO viewer_message_history (
			id, viewer_session_id, streamer_user_id, overlay_id,
			platform, channel_id, channel_name, message_text,
			sent_at, success, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
	`

	_, err := r.db.Exec(ctx, query,
		log.ID,
		log.ViewerSessionID,
		log.StreamerUserID,
		log.OverlayID,
		log.Platform,
		log.ChannelID,
		log.ChannelName,
		log.MessageText,
		log.SentAt,
		log.Success,
		log.ErrorMessage,
	)

	if err != nil {
		return fmt.Errorf("failed to log message: %w", err)
	}

	return nil
}

// DecryptAccessToken decrypts the access token
func (r *ViewerRepository) DecryptAccessToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Decrypt(token)
}

// DecryptRefreshToken decrypts the refresh token
func (r *ViewerRepository) DecryptRefreshToken(token string) (string, error) {
	if r.cipher == nil || token == "" {
		return token, nil
	}
	return r.cipher.Decrypt(token)
}
