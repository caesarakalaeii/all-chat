package subscription

import (
	"context"
	"fmt"

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
		return false, fmt.Errorf("failed to check overlay status: %w", err)
	}

	return isActive, nil
}
