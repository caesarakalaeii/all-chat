package repository

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MaintenanceRepository handles persistence for maintenance windows.
type MaintenanceRepository struct {
	pool *pgxpool.Pool
}

// NewMaintenanceRepository creates a new MaintenanceRepository.
func NewMaintenanceRepository(pool *pgxpool.Pool) *MaintenanceRepository {
	return &MaintenanceRepository{pool: pool}
}

// Create inserts a new maintenance window and returns the created record.
func (r *MaintenanceRepository) Create(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error) {
	query := `
		INSERT INTO maintenance_windows (title, description, starts_at, ends_at, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, description, starts_at, ends_at, created_by, created_at
	`

	row := r.pool.QueryRow(ctx, query, req.Title, req.Description, req.StartsAt, req.EndsAt, createdBy)

	var mw models.MaintenanceWindow
	if err := row.Scan(
		&mw.ID,
		&mw.Title,
		&mw.Description,
		&mw.StartsAt,
		&mw.EndsAt,
		&mw.CreatedBy,
		&mw.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to create maintenance window: %w", err)
	}

	return &mw, nil
}

// ListAll returns all maintenance windows ordered by start time (ascending).
func (r *MaintenanceRepository) ListAll(ctx context.Context) ([]models.MaintenanceWindow, error) {
	query := `
		SELECT id, title, description, starts_at, ends_at, created_by, created_at
		FROM maintenance_windows
		ORDER BY starts_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list maintenance windows: %w", err)
	}
	defer rows.Close()

	windows := make([]models.MaintenanceWindow, 0)
	for rows.Next() {
		var mw models.MaintenanceWindow
		if err := rows.Scan(
			&mw.ID,
			&mw.Title,
			&mw.Description,
			&mw.StartsAt,
			&mw.EndsAt,
			&mw.CreatedBy,
			&mw.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan maintenance window: %w", err)
		}
		windows = append(windows, mw)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate maintenance windows: %w", err)
	}

	return windows, nil
}

// ListUpcoming returns maintenance windows that have not yet ended (ends_at > NOW()),
// ordered by start time ascending. This is the user-facing query.
func (r *MaintenanceRepository) ListUpcoming(ctx context.Context) ([]models.MaintenanceWindow, error) {
	query := `
		SELECT id, title, description, starts_at, ends_at, created_by, created_at
		FROM maintenance_windows
		WHERE ends_at > NOW()
		ORDER BY starts_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list upcoming maintenance windows: %w", err)
	}
	defer rows.Close()

	windows := make([]models.MaintenanceWindow, 0)
	for rows.Next() {
		var mw models.MaintenanceWindow
		if err := rows.Scan(
			&mw.ID,
			&mw.Title,
			&mw.Description,
			&mw.StartsAt,
			&mw.EndsAt,
			&mw.CreatedBy,
			&mw.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan upcoming maintenance window: %w", err)
		}
		windows = append(windows, mw)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate upcoming maintenance windows: %w", err)
	}

	return windows, nil
}

// Delete removes a maintenance window by ID.
func (r *MaintenanceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM maintenance_windows WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete maintenance window: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("maintenance window not found: %s", id)
	}

	return nil
}
