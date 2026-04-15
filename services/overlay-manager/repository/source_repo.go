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

// ListByOverlayID retrieves all sources for an overlay.
// For shared_overlay sources, it JOINs share_requests to populate ShareStatus.
// Non-shared_overlay sources have ShareStatus == nil (omitted from JSON).
func (r *SourceRepository) ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error) {
	query := `
		SELECT ocs.id, ocs.overlay_id, ocs.platform, ocs.channel_id, ocs.channel_name,
		       ocs.channel_handle, ocs.auth_required, ocs.config, ocs.is_active,
		       ocs.created_at, ocs.updated_at,
		       sr.status AS share_status
		FROM overlay_chat_sources ocs
		LEFT JOIN share_requests sr
		    ON ocs.platform = 'shared_overlay'
		    AND sr.id::text = ocs.channel_id
		WHERE ocs.overlay_id = $1
		ORDER BY ocs.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, overlayID)
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
