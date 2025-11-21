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
		INSERT INTO overlays (id, user_id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		overlay.ID,
		overlay.UserID,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
	).Scan(&overlay.CreatedAt, &overlay.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create overlay: %w", err)
	}

	return nil
}

// GetByID retrieves an overlay by ID
func (r *OverlayRepository) GetByID(ctx context.Context, id string) (*models.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
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
		SELECT id, user_id, name, description, is_active, created_at, updated_at
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
		SELECT id, user_id, name, description, is_active, created_at, updated_at
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
		SET name = $1, description = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`

	err := r.pool.QueryRow(ctx, query,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
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
		SELECT id, user_id, name, created_at, updated_at
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
		if err := rows.Scan(&overlay.ID, &overlay.UserID, &overlay.Name, &overlay.CreatedAt, &overlay.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan overlay: %w", err)
		}
		overlays = append(overlays, &overlay)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlays: %w", err)
	}

	return overlays, nil
}
