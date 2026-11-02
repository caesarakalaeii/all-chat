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
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OverlayRepository handles overlay persistence
type OverlayRepository struct {
	pool *pgxpool.Pool
}

// NewOverlayRepository creates a new overlay repository
func NewOverlayRepository(connString string) (*OverlayRepository, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &OverlayRepository{pool: pool}, nil
}

// Create creates a new overlay
func (r *OverlayRepository) Create(ctx context.Context, overlay *models.Overlay) error {
	// Validate before creating
	if err := overlay.Validate(); err != nil {
		return err
	}

	// Generate ID if not provided
	if overlay.ID == "" {
		overlay.ID = uuid.New().String()
	}

	query := `
		INSERT INTO overlays (id, user_id, name, description, is_active, is_public_for_viewers, overlay_type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		overlay.ID,
		overlay.UserID,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
		overlay.IsPublicForViewers,
		overlay.OverlayType,
	).Scan(&overlay.CreatedAt, &overlay.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create overlay: %w", err)
	}

	return nil
}

// GetByID retrieves an overlay by ID
func (r *OverlayRepository) GetByID(ctx context.Context, id string) (*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, is_public_for_viewers, overlay_type, created_at, updated_at
		FROM overlays
		WHERE id = $1
	`

	overlay := &models.Overlay{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&overlay.ID,
		&overlay.UserID,
		&overlay.Name,
		&overlay.Description,
		&overlay.IsActive,
		&overlay.IsPublicForViewers,
		&overlay.OverlayType,
		&overlay.CreatedAt,
		&overlay.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("overlay not found")
		}
		return nil, fmt.Errorf("failed to get overlay: %w", err)
	}

	return overlay, nil
}

// GetByIDAndUserID retrieves an overlay by ID and user ID (authorization check)
func (r *OverlayRepository) GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, is_public_for_viewers, overlay_type, created_at, updated_at
		FROM overlays
		WHERE id = $1 AND user_id = $2
	`

	overlay := &models.Overlay{}
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&overlay.ID,
		&overlay.UserID,
		&overlay.Name,
		&overlay.Description,
		&overlay.IsActive,
		&overlay.IsPublicForViewers,
		&overlay.OverlayType,
		&overlay.CreatedAt,
		&overlay.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("overlay not found or unauthorized")
		}
		return nil, fmt.Errorf("failed to get overlay: %w", err)
	}

	return overlay, nil
}

// ListByUserID retrieves all overlays for a user
func (r *OverlayRepository) ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, is_public_for_viewers, overlay_type, created_at, updated_at
		FROM overlays
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list overlays: %w", err)
	}
	defer rows.Close()

	overlays := []*models.Overlay{}
	for rows.Next() {
		overlay := &models.Overlay{}
		err := rows.Scan(
			&overlay.ID,
			&overlay.UserID,
			&overlay.Name,
			&overlay.Description,
			&overlay.IsActive,
			&overlay.IsPublicForViewers,
			&overlay.OverlayType,
			&overlay.CreatedAt,
			&overlay.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan overlay: %w", err)
		}
		overlays = append(overlays, overlay)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlays: %w", err)
	}

	return overlays, nil
}

// Update updates an existing overlay
func (r *OverlayRepository) Update(ctx context.Context, overlay *models.Overlay) error {
	// Validate before updating
	if err := overlay.Validate(); err != nil {
		return err
	}

	query := `
		UPDATE overlays
		SET name = $1, description = $2, is_active = $3, is_public_for_viewers = $4, overlay_type = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
		overlay.IsPublicForViewers,
		overlay.OverlayType,
		overlay.ID,
	).Scan(&overlay.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("overlay not found")
		}
		return fmt.Errorf("failed to update overlay: %w", err)
	}

	return nil
}

// UnsetAllPublicForUser sets is_public_for_viewers = false for all overlays owned by the user
// except the one with excludeID. Used to enforce the one-public-overlay-per-user constraint.
func (r *OverlayRepository) UnsetAllPublicForUser(ctx context.Context, userID, excludeID string) error {
	query := `
		UPDATE overlays
		SET is_public_for_viewers = false, updated_at = NOW()
		WHERE user_id = $1 AND id != $2 AND is_public_for_viewers = true
	`
	_, err := r.pool.Exec(ctx, query, userID, excludeID)
	if err != nil {
		return fmt.Errorf("failed to unset public overlays: %w", err)
	}
	return nil
}

// Delete deletes an overlay by ID
func (r *OverlayRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM overlays WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete overlay: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("overlay not found")
	}

	return nil
}

// GetAllOverlays returns all overlays (admin only)
func (r *OverlayRepository) GetAllOverlays(ctx context.Context) ([]*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, overlay_type, created_at, updated_at
		FROM overlays
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query overlays: %w", err)
	}
	defer rows.Close()

	var overlays []*models.Overlay
	for rows.Next() {
		var overlay models.Overlay
		if err := rows.Scan(&overlay.ID, &overlay.UserID, &overlay.Name, &overlay.OverlayType, &overlay.CreatedAt, &overlay.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan overlay: %w", err)
		}
		overlays = append(overlays, &overlay)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlays: %w", err)
	}

	return overlays, nil
}

// OverlayWithSourceCount is an overlay paired with its number of chat sources and its
// owner's identity (LEFT-joined from users; empty strings when the owner row is absent).
type OverlayWithSourceCount struct {
	models.Overlay
	SourcesCount     int
	OwnerUsername    string
	OwnerDisplayName string
}

// likeContains turns s into an ILIKE pattern that matches it as a literal
// substring ("" stays "", meaning no search): usernames are full of
// underscores, which LIKE would otherwise read as a single-character wildcard.
func likeContains(s string) string {
	if s == "" {
		return ""
	}
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
}

// OverlayListFilter narrows the admin overlay listing. Query is a
// case-insensitive literal substring match on the overlay's name and id and its
// owner's id and username; IDs, when non-nil, restricts it to those overlays
// (the connected-only view and deep links).
type OverlayListFilter struct {
	Query string
	IDs   []string
}

// ListOverlaysWithSourceCount returns one page of overlays matching filter,
// newest first, each with its source count, plus the total number of matches.
func (r *OverlayRepository) ListOverlaysWithSourceCount(ctx context.Context, filter OverlayListFilter, limit, offset int) ([]*OverlayWithSourceCount, int, error) {
	// ids compare as text so a malformed id matches nothing instead of failing
	// the uuid cast; id breaks created_at ties so pages neither repeat nor skip.
	query := `
		SELECT o.id, o.user_id, o.name, o.overlay_type, o.created_at, o.updated_at,
		       (SELECT COUNT(*) FROM overlay_chat_sources s WHERE s.overlay_id = o.id) AS sources_count,
		       u.username, u.display_name, COUNT(*) OVER () AS total
		FROM overlays o
		LEFT JOIN users u ON u.id = o.user_id
		WHERE ($1 = '' OR o.name ILIKE $1 OR o.id::text ILIKE $1 OR o.user_id::text ILIKE $1 OR u.username ILIKE $1)
		  AND ($2::text[] IS NULL OR o.id::text = ANY($2))
		ORDER BY o.created_at DESC, o.id
		LIMIT $3 OFFSET $4
	`

	rows, err := r.pool.Query(ctx, query, likeContains(filter.Query), filter.IDs, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query overlays: %w", err)
	}
	defer rows.Close()

	overlays := []*OverlayWithSourceCount{}
	total := 0
	for rows.Next() {
		var o OverlayWithSourceCount
		var username, displayName sql.NullString
		if err := rows.Scan(&o.ID, &o.UserID, &o.Name, &o.OverlayType, &o.CreatedAt, &o.UpdatedAt, &o.SourcesCount, &username, &displayName, &total); err != nil {
			return nil, 0, fmt.Errorf("failed to scan overlay: %w", err)
		}
		o.OwnerUsername = username.String
		o.OwnerDisplayName = displayName.String
		overlays = append(overlays, &o)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating overlays: %w", err)
	}

	return overlays, total, nil
}

// ListByUserIDWithSourceCount returns a single user's overlays with their source
// counts in one aggregated query, replacing a per-overlay ListByOverlayID loop.
func (r *OverlayRepository) ListByUserIDWithSourceCount(ctx context.Context, userID string) ([]*OverlayWithSourceCount, error) {
	query := `
		SELECT o.id, o.user_id, o.name, o.overlay_type, o.created_at, o.updated_at, COUNT(s.id) AS sources_count,
		       u.username, u.display_name
		FROM overlays o
		LEFT JOIN overlay_chat_sources s ON s.overlay_id = o.id
		LEFT JOIN users u ON u.id = o.user_id
		WHERE o.user_id = $1
		GROUP BY o.id, u.id, u.username, u.display_name
		ORDER BY o.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list overlays: %w", err)
	}
	defer rows.Close()

	overlays := []*OverlayWithSourceCount{}
	for rows.Next() {
		var o OverlayWithSourceCount
		var username, displayName sql.NullString
		if err := rows.Scan(&o.ID, &o.UserID, &o.Name, &o.OverlayType, &o.CreatedAt, &o.UpdatedAt, &o.SourcesCount, &username, &displayName); err != nil {
			return nil, fmt.Errorf("failed to scan overlay: %w", err)
		}
		o.OwnerUsername = username.String
		o.OwnerDisplayName = displayName.String
		overlays = append(overlays, &o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlays: %w", err)
	}

	return overlays, nil
}
