package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ViewerIdentityRepository handles database operations for viewer identity
// (cross-platform linking and cosmetics).
type ViewerIdentityRepository struct {
	db *pgxpool.Pool
}

// NewViewerIdentityRepository creates a new ViewerIdentityRepository.
func NewViewerIdentityRepository(db *pgxpool.Pool) *ViewerIdentityRepository {
	return &ViewerIdentityRepository{db: db}
}

// GetOrCreateViewerByPlatform looks up or creates a viewer record for the given
// (platform, platformUserID). Returns the durable viewer_id UUID.
//
// Flow:
//  1. SELECT viewer_id FROM viewer_platform_identities WHERE platform=$1 AND platform_user_id=$2
//  2. If found: return that viewer_id
//  3. If not found:
//     a. INSERT INTO viewers DEFAULT VALUES RETURNING id  → newViewerID
//     b. INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id)
//     c. INSERT INTO viewer_cosmetics (viewer_id) ON CONFLICT DO NOTHING
//     d. UPDATE viewer_sessions SET viewer_id=$1 WHERE platform=$2 AND platform_user_id=$3
//     e. Return newViewerID
func (r *ViewerIdentityRepository) GetOrCreateViewerByPlatform(ctx context.Context, platform, platformUserID string) (uuid.UUID, error) {
	// Step 1: try to find existing mapping
	var viewerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID,
	).Scan(&viewerID)

	if err == nil {
		// Found existing viewer
		return viewerID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, fmt.Errorf("failed to lookup viewer_platform_identity: %w", err)
	}

	// Step 3a: create new viewer record
	var newViewerID uuid.UUID
	err = r.db.QueryRow(ctx,
		`INSERT INTO viewers DEFAULT VALUES RETURNING id`,
	).Scan(&newViewerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer: %w", err)
	}

	// Step 3b: create platform identity mapping
	_, err = r.db.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, $2, $3)`,
		newViewerID, platform, platformUserID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer_platform_identity: %w", err)
	}

	// Step 3c: create cosmetics row (no-op if already exists)
	_, err = r.db.Exec(ctx,
		`INSERT INTO viewer_cosmetics (viewer_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		newViewerID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer_cosmetics: %w", err)
	}

	// Step 3d: backfill viewer_id on existing session rows for this platform user
	_, err = r.db.Exec(ctx,
		`UPDATE viewer_sessions SET viewer_id = $1 WHERE platform = $2 AND platform_user_id = $3`,
		newViewerID, platform, platformUserID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to update viewer_sessions viewer_id: %w", err)
	}

	return newViewerID, nil
}

// GetViewerCosmetics returns the name_color for a viewer, or nil if not set.
func (r *ViewerIdentityRepository) GetViewerCosmetics(ctx context.Context, viewerID uuid.UUID) (*string, error) {
	var nameColor *string
	err := r.db.QueryRow(ctx,
		`SELECT name_color FROM viewer_cosmetics WHERE viewer_id = $1`,
		viewerID,
	).Scan(&nameColor)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get viewer cosmetics: %w", err)
	}

	return nameColor, nil
}

// UpsertViewerCosmetics sets name_color for a viewer. Pass nil to clear the color.
func (r *ViewerIdentityRepository) UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, nameColor *string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO viewer_cosmetics (viewer_id, name_color, updated_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (viewer_id) DO UPDATE SET name_color = EXCLUDED.name_color, updated_at = NOW()`,
		viewerID, nameColor,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert viewer cosmetics: %w", err)
	}
	return nil
}
