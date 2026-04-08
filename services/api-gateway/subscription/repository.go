package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database queries for subscription management
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new subscription repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// VerifyOverlayOwnership verifies that a user owns an overlay
func (r *Repository) VerifyOverlayOwnership(ctx context.Context, overlayID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM overlays
			WHERE id = $1 AND user_id = $2
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, overlayID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify overlay ownership: %w", err)
	}

	return exists, nil
}

// IsOverlayActive checks if an overlay is active
func (r *Repository) IsOverlayActive(ctx context.Context, overlayID string) (bool, error) {
	query := `
		SELECT is_active FROM overlays
		WHERE id = $1
	`

	var isActive bool
	err := r.db.QueryRow(ctx, query, overlayID).Scan(&isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check overlay status: %w", err)
	}

	return isActive, nil
}

// GetPublicOverlayByUsername gets the public overlay ID for a streamer by username
// Returns the overlay ID if the streamer has a public overlay, empty string if not found
func (r *Repository) GetPublicOverlayByUsername(ctx context.Context, username string) (string, error) {
	query := `
		SELECT o.id
		FROM overlays o
		JOIN users u ON o.user_id = u.id
		WHERE u.username = $1
		  AND o.is_active = true
		  AND o.is_public_for_viewers = true
		  AND u.is_banned = false
		ORDER BY o.created_at ASC
		LIMIT 1
	`

	var overlayID string
	err := r.db.QueryRow(ctx, query, username).Scan(&overlayID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("failed to get public overlay: %w", err)
	}

	return overlayID, nil
}

// ActivateSourcesForOverlay activates all sources for an overlay
// This is called when a WebSocket connection is established
// Skips shared_overlay sources whose share_request is revoked or expired.
func (r *Repository) ActivateSourcesForOverlay(ctx context.Context, overlayID string) (int, error) {
	query := `
		UPDATE overlay_chat_sources ocs
		SET is_active = true,
		    updated_at = NOW()
		WHERE ocs.overlay_id = $1
		  AND ocs.is_active = false
		  AND NOT (
		    ocs.platform = 'shared_overlay'
		    AND EXISTS (
		      SELECT 1 FROM share_requests sr
		      WHERE sr.id::text = ocs.channel_id
		        AND sr.status IN ('revoked', 'expired')
		    )
		  )
	`

	result, err := r.db.Exec(ctx, query, overlayID)
	if err != nil {
		return 0, fmt.Errorf("failed to activate sources: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// OverlaySource represents a single chat source for an overlay
type OverlaySource struct {
	Platform    string
	ChannelID   string
	ChannelName string
}

// GetOverlaySources returns all platform+channel_id pairs configured for an overlay
func (r *Repository) GetOverlaySources(ctx context.Context, overlayID string) ([]OverlaySource, error) {
	query := `
		SELECT platform, channel_id, channel_name
		FROM overlay_chat_sources
		WHERE overlay_id = $1
	`

	rows, err := r.db.Query(ctx, query, overlayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay sources: %w", err)
	}
	defer rows.Close()

	sources := make([]OverlaySource, 0)
	for rows.Next() {
		var src OverlaySource
		if err := rows.Scan(&src.Platform, &src.ChannelID, &src.ChannelName); err != nil {
			return nil, fmt.Errorf("failed to scan overlay source: %w", err)
		}
		sources = append(sources, src)
	}

	return sources, nil
}

// DeactivateSourcesForOverlay deactivates all sources for an overlay
// This is called when the last WebSocket connection for an overlay disconnects
func (r *Repository) DeactivateSourcesForOverlay(ctx context.Context, overlayID string) (int, error) {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = false,
		    updated_at = NOW()
		WHERE overlay_id = $1
		  AND is_active = true
	`

	result, err := r.db.Exec(ctx, query, overlayID)
	if err != nil {
		return 0, fmt.Errorf("failed to deactivate sources: %w", err)
	}

	return int(result.RowsAffected()), nil
}
