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
	"database/sql"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SourceRepository handles overlay chat source persistence
type SourceRepository struct {
	pool *pgxpool.Pool
}

// NewSourceRepository creates a new source repository
func NewSourceRepository(connString string) (*SourceRepository, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &SourceRepository{pool: pool}, nil
}

// Create creates a new chat source for an overlay
func (r *SourceRepository) Create(ctx context.Context, source *models.ChatSource) error {
	// Generate ID if not provided
	if source.ID == "" {
		source.ID = uuid.New().String()
	}

	query := `
		INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, channel_name, channel_handle, auth_required, config, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		source.ID,
		source.OverlayID,
		source.Platform,
		source.ChannelID,
		source.ChannelName,
		source.ChannelHandle,
		source.AuthRequired,
		source.Config,
		source.IsActive,
	).Scan(&source.CreatedAt, &source.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create source: %w", err)
	}

	return nil
}

// CreateOrUpdateAuto inserts a chat source or, if one already exists for the same
// (overlay_id, platform, channel_id), updates its display fields and re-activates it —
// returning the canonical row either way. Used by the internal add-source-auto endpoint so
// the OAuth flow is idempotent: re-connecting an already-configured channel (e.g. to grant
// the EventSub chat scopes) succeeds instead of failing on the UNIQUE constraint. The
// existing config (e.g. TTS/relay settings) is intentionally preserved.
func (r *SourceRepository) CreateOrUpdateAuto(ctx context.Context, source *models.ChatSource) error {
	if source.ID == "" {
		source.ID = uuid.New().String()
	}

	query := `
		INSERT INTO overlay_chat_sources (id, overlay_id, platform, channel_id, channel_name, channel_handle, auth_required, config, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
		ON CONFLICT (overlay_id, platform, channel_id) DO UPDATE SET
			channel_name   = EXCLUDED.channel_name,
			channel_handle = EXCLUDED.channel_handle,
			is_active      = true,
			updated_at     = NOW()
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		source.ID,
		source.OverlayID,
		source.Platform,
		source.ChannelID,
		source.ChannelName,
		source.ChannelHandle,
		source.AuthRequired,
		source.Config,
		source.IsActive,
	).Scan(&source.ID, &source.CreatedAt, &source.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert source: %w", err)
	}

	return nil
}

// ListByOverlayID retrieves all sources for an overlay without per-user ownership
// context (IsOwnChannel is always false). Use ListByOverlayIDForUser when the caller
// is an authenticated user that needs the re-consent nudge flag.
func (r *SourceRepository) ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error) {
	return r.ListByOverlayIDForUser(ctx, overlayID, "")
}

// ListByOverlayIDForUser retrieves all sources for an overlay, computing IsOwnChannel
// relative to requestingUserID (pass "" for public/admin callers — ownership is then
// always false).
// For shared_overlay sources, it JOINs share_requests to populate ShareStatus.
// Non-shared_overlay sources have ShareStatus == nil (omitted from JSON).
func (r *SourceRepository) ListByOverlayIDForUser(ctx context.Context, overlayID, requestingUserID string) ([]*models.ChatSource, error) {
	// chat_via_eventsub mirrors the IRC/EventSub partition predicate (see
	// twitch-eventsub-listener/channels/manager.go and twitch-listener/channels/repository.go):
	// a Twitch source is read via EventSub when its channel owner granted user:read:chat
	// and still has a valid token — either on their Twitch-login users row, or via
	// linked Twitch credentials in twitch_oauth_tokens (ADR-0016: YouTube/Kick-login
	// accounts that completed the Twitch add-source consent). The frontend uses it to
	// show a badge / reconnect CTA.
	//
	// is_own_channel is whether requestingUserID ($2) owns this Twitch channel and can
	// re-consent to enable EventSub chat. Unlike chat_via_eventsub it ignores scope/expiry
	// (re-consent is exactly what fixes an expired/narrow grant) and is matched per-user,
	// so it also covers non-Twitch-login owners via twitch_oauth_tokens (ADR-0016) — the
	// case the old frontend username heuristic silently missed. NULLIF keeps an empty
	// requesting user from matching (NULL = no row owns it).
	query := `
		SELECT ocs.id, ocs.overlay_id, ocs.platform, ocs.channel_id, ocs.channel_name,
		       ocs.channel_handle, ocs.auth_required, ocs.config, ocs.is_active,
		       ocs.created_at, ocs.updated_at,
		       sr.status AS share_status,
		       (ocs.platform = 'twitch' AND (
		           EXISTS (
		               SELECT 1 FROM users u
		               WHERE LOWER(u.username) = LOWER(ocs.channel_id)
		                 AND u.auth_provider = 'twitch'
		                 AND 'user:read:chat' = ANY(u.granted_scopes)
		                 AND u.token_expires_at > NOW()
		           )
		           OR EXISTS (
		               SELECT 1 FROM twitch_oauth_tokens t
		               WHERE LOWER(t.twitch_login) = LOWER(ocs.channel_id)
		                 AND 'user:read:chat' = ANY(t.granted_scopes)
		                 AND t.token_expires_at > NOW()
		           )
		       )) AS chat_via_eventsub,
		       (ocs.platform = 'twitch' AND (
		           EXISTS (
		               SELECT 1 FROM users u
		               WHERE u.id = NULLIF($2, '')::uuid
		                 AND u.auth_provider = 'twitch'
		                 AND LOWER(u.username) = LOWER(ocs.channel_id)
		           )
		           OR EXISTS (
		               SELECT 1 FROM twitch_oauth_tokens t
		               WHERE t.user_id = NULLIF($2, '')::uuid
		                 AND LOWER(t.twitch_login) = LOWER(ocs.channel_id)
		           )
		       )) AS is_own_channel
		FROM overlay_chat_sources ocs
		LEFT JOIN share_requests sr
		    ON ocs.platform = 'shared_overlay'
		    AND sr.id::text = ocs.channel_id
		WHERE ocs.overlay_id = $1
		ORDER BY ocs.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, overlayID, requestingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	defer rows.Close()

	sources := []*models.ChatSource{}
	for rows.Next() {
		source := &models.ChatSource{}
		err := rows.Scan(
			&source.ID,
			&source.OverlayID,
			&source.Platform,
			&source.ChannelID,
			&source.ChannelName,
			&source.ChannelHandle,
			&source.AuthRequired,
			&source.Config,
			&source.IsActive,
			&source.CreatedAt,
			&source.UpdatedAt,
			&source.ShareStatus,
			&source.ChatViaEventSub,
			&source.IsOwnChannel,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sources = append(sources, source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}

// GetByID retrieves a source by ID
func (r *SourceRepository) GetByID(ctx context.Context, id string) (*models.ChatSource, error) {
	query := `
		SELECT id, overlay_id, platform, channel_id, channel_name, channel_handle, auth_required, config, is_active, created_at, updated_at
		FROM overlay_chat_sources
		WHERE id = $1
	`

	source := &models.ChatSource{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&source.ID,
		&source.OverlayID,
		&source.Platform,
		&source.ChannelID,
		&source.ChannelName,
		&source.ChannelHandle,
		&source.AuthRequired,
		&source.Config,
		&source.IsActive,
		&source.CreatedAt,
		&source.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("source not found")
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// Delete deletes a source by ID
func (r *SourceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM overlay_chat_sources WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("source not found")
	}

	return nil
}

// UpdateConfig updates the config JSONB field for a source by ID.
func (r *SourceRepository) UpdateConfig(ctx context.Context, id string, config map[string]interface{}) error {
	query := `UPDATE overlay_chat_sources SET config = $2, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id, config)
	return err
}

// GetAllSources returns all sources across all overlays (admin only)
func (r *SourceRepository) GetAllSources(ctx context.Context) ([]*models.ChatSource, error) {
	query := `
		SELECT id, overlay_id, platform, channel_id, channel_name, channel_handle, is_active, created_at, updated_at
		FROM overlay_chat_sources
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sources: %w", err)
	}
	defer rows.Close()

	var sources []*models.ChatSource
	for rows.Next() {
		var source models.ChatSource
		if err := rows.Scan(&source.ID, &source.OverlayID, &source.Platform, &source.ChannelID, &source.ChannelName, &source.ChannelHandle, &source.IsActive, &source.CreatedAt, &source.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}

// SourceWithOverlay is a chat source joined with metadata from its owning overlay,
// including the overlay owner's identity (LEFT-joined from users; empty strings when the
// owner row is absent).
type SourceWithOverlay struct {
	models.ChatSource
	OverlayName      string
	UserID           string
	OwnerUsername    string
	OwnerDisplayName string
}

// GetAllSourcesWithOverlay returns every source joined with its overlay's name and
// owner in a single query. Sources whose overlay has been deleted are excluded
// (INNER JOIN), matching the previous per-source "skip if overlay not found" behavior.
// This replaces an N+1 lookup that fetched each source's overlay individually.
func (r *SourceRepository) GetAllSourcesWithOverlay(ctx context.Context) ([]*SourceWithOverlay, error) {
	query := `
		SELECT s.id, s.overlay_id, s.platform, s.channel_id, s.channel_name, s.channel_handle,
		       s.is_active, s.created_at, s.updated_at,
		       o.name, o.user_id,
		       u.username, u.display_name
		FROM overlay_chat_sources s
		JOIN overlays o ON o.id = s.overlay_id
		LEFT JOIN users u ON u.id = o.user_id
		ORDER BY s.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query sources: %w", err)
	}
	defer rows.Close()

	var sources []*SourceWithOverlay
	for rows.Next() {
		var sw SourceWithOverlay
		var username, displayName sql.NullString
		if err := rows.Scan(&sw.ID, &sw.OverlayID, &sw.Platform, &sw.ChannelID, &sw.ChannelName, &sw.ChannelHandle, &sw.IsActive, &sw.CreatedAt, &sw.UpdatedAt, &sw.OverlayName, &sw.UserID, &username, &displayName); err != nil {
			return nil, fmt.Errorf("failed to scan source: %w", err)
		}
		sw.OwnerUsername = username.String
		sw.OwnerDisplayName = displayName.String
		sources = append(sources, &sw)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	return sources, nil
}
