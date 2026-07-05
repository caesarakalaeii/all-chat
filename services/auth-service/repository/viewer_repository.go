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

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/shared/premium"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ViewerRepository handles database operations for viewer sessions
type ViewerRepository struct {
	db         *pgxpool.Pool
	cipher     StringCipher
	recomputer *premium.Recomputer
}

// NewViewerRepository creates a new ViewerRepository. recomputer derives
// viewers.is_premium after an admin override change (ADR-0019).
func NewViewerRepository(db *pgxpool.Pool, cipher StringCipher, recomputer *premium.Recomputer) *ViewerRepository {
	return &ViewerRepository{
		db:         db,
		cipher:     cipher,
		recomputer: recomputer,
	}
}

// GetByPlatformUserID retrieves a viewer session by platform and platform_user_id
func (r *ViewerRepository) GetByPlatformUserID(ctx context.Context, platform, platformUserID string) (*models.ViewerSession, error) {
	query := `
		SELECT id, platform, platform_user_id, username, display_name, avatar_url,
		       access_token, refresh_token, token_expires_at,
		       last_message_at, message_count_1min, message_count_1hour,
		       rate_limit_reset_1min, rate_limit_reset_1hour,
		       is_banned, banned_at, banned_reason,
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
		&session.IsBanned,
		&session.BannedAt,
		&session.BannedReason,
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
		       is_banned, banned_at, banned_reason,
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
		&session.IsBanned,
		&session.BannedAt,
		&session.BannedReason,
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

// MigratePlatformUserID updates the platform_user_id for an existing session identified by
// (platform, oldPlatformUserID) to newPlatformUserID.  This is used when a viewer previously
// authenticated with a Google account ID but we now have their proper YouTube channel ID.
// The update is applied to viewer_sessions only; caller must separately update
// viewer_platform_identities via ViewerIdentityRepository.MigratePlatformUserID.
func (r *ViewerRepository) MigratePlatformUserID(ctx context.Context, platform, oldPlatformUserID, newPlatformUserID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE viewer_sessions SET platform_user_id = $3, updated_at = NOW() WHERE platform = $1 AND platform_user_id = $2`,
		platform, oldPlatformUserID, newPlatformUserID,
	)
	if err != nil {
		return fmt.Errorf("failed to migrate viewer_sessions platform_user_id: %w", err)
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

// ListAll returns all viewer sessions with pagination
func (r *ViewerRepository) ListAll(ctx context.Context, limit, offset int) ([]models.ViewerSession, error) {
	query := `
		SELECT vs.id, vs.platform, vs.platform_user_id, vs.username, vs.display_name, vs.avatar_url,
		       vs.access_token, vs.refresh_token, vs.token_expires_at,
		       vs.last_message_at, vs.message_count_1min, vs.message_count_1hour,
		       vs.rate_limit_reset_1min, vs.rate_limit_reset_1hour,
		       COALESCE(v.is_premium, false) AS is_premium,
		       v.premium_admin_override_expires_at,
		       vs.viewer_id,
		       vs.is_banned, vs.banned_at, vs.banned_reason,
		       vs.created_at, vs.updated_at
		FROM viewer_sessions vs
		LEFT JOIN viewers v ON vs.viewer_id = v.id
		ORDER BY vs.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list viewer sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]models.ViewerSession, 0)
	for rows.Next() {
		var session models.ViewerSession
		err := rows.Scan(
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
			&session.IsPremium,
			&session.PremiumExpiresAt,
			&session.ViewerID,
			&session.IsBanned,
			&session.BannedAt,
			&session.BannedReason,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan viewer session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// BanViewer bans a viewer from sending messages
func (r *ViewerRepository) BanViewer(ctx context.Context, sessionID uuid.UUID, reason string) error {
	query := `
		UPDATE viewer_sessions
		SET is_banned = true,
		    banned_at = NOW(),
		    banned_reason = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, sessionID, reason)
	if err != nil {
		return fmt.Errorf("failed to ban viewer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("viewer session not found")
	}

	return nil
}

// UnbanViewer removes the ban from a viewer
func (r *ViewerRepository) UnbanViewer(ctx context.Context, sessionID uuid.UUID) error {
	query := `
		UPDATE viewer_sessions
		SET is_banned = false,
		    banned_at = NULL,
		    banned_reason = NULL,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.Exec(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to unban viewer: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("viewer session not found")
	}

	return nil
}

// SetViewerPremium records the admin's viewer-premium decision as a tri-state
// override (ADR-0019, mirroring users.premium_admin_override) on the viewer linked
// to the session, then re-derives viewers.is_premium via shared/premium. Granting
// maps to a force-grant (override TRUE) that survives a subscription lapse; revoking
// clears the override (NULL) so premium then follows any active viewer subscription
// or linked-streamer inheritance. A hard viewer-premium ban (override FALSE) is
// reserved for a future explicit admin action.
//
// ttl makes the grant time-limited (ADR-0027): non-nil grants viewer premium only
// until NOW()+ttl, after which RecomputeViewer (and the payment-service sweep) treat
// the override as absent. ttl is only meaningful when granting; a nil ttl (or any
// revoke) clears the expiry. The deadline is computed from the database clock.
func (r *ViewerRepository) SetViewerPremium(ctx context.Context, sessionID uuid.UUID, isPremium bool, ttl *time.Duration) error {
	var viewerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_sessions WHERE id = $1 AND viewer_id IS NOT NULL`, sessionID).Scan(&viewerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("viewer not found or no linked viewer identity")
	}
	if err != nil {
		return fmt.Errorf("failed to resolve viewer for session: %w", err)
	}

	var override *bool
	if isPremium {
		v := true
		override = &v
	}
	var ttlSeconds *float64
	if isPremium && ttl != nil {
		s := ttl.Seconds()
		ttlSeconds = &s
	}
	if _, err := r.db.Exec(ctx, `
		UPDATE viewers
		SET premium_admin_override = $2,
		    premium_admin_override_expires_at = CASE
		        WHEN $3::double precision IS NULL THEN NULL
		        ELSE NOW() + make_interval(secs => $3::double precision)
		    END
		WHERE id = $1`, viewerID, override, ttlSeconds); err != nil {
		return fmt.Errorf("failed to update viewer premium override: %w", err)
	}

	if _, err := r.recomputer.RecomputeViewer(ctx, viewerID.String()); err != nil {
		return fmt.Errorf("failed to recompute viewer premium: %w", err)
	}

	return nil
}
