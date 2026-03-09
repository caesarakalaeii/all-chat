package repository

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ShareRepository handles share request persistence
type ShareRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewShareRepository creates a new share repository
func NewShareRepository(db *pgxpool.Pool, logger *zap.Logger) *ShareRepository {
	return &ShareRepository{db: db, logger: logger}
}

// Create creates a new share request with 7-day expiry
func (r *ShareRepository) Create(ctx context.Context, req *models.ShareRequest) error {
	// Validate before insert
	if err := req.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	query := `
		INSERT INTO share_requests (
			sender_user_id, sender_overlay_id, recipient_user_id,
			status, created_at, expires_at
		) VALUES ($1, $2, $3, $4, NOW(), NOW() + INTERVAL '7 days')
		RETURNING id, created_at, expires_at
	`

	err := r.db.QueryRow(ctx, query,
		req.SenderUserID,
		req.SenderOverlayID,
		req.RecipientUserID,
		models.StatusPending,
	).Scan(&req.ID, &req.CreatedAt, &req.ExpiresAt)

	if err != nil {
		r.logger.Error("Failed to create share request",
			zap.String("sender_user_id", req.SenderUserID),
			zap.String("recipient_user_id", req.RecipientUserID),
			zap.Error(err))
		return fmt.Errorf("failed to create share request: %w", err)
	}

	req.Status = models.StatusPending
	r.logger.Info("Share request created",
		zap.String("id", req.ID),
		zap.String("sender_user_id", req.SenderUserID),
		zap.String("recipient_user_id", req.RecipientUserID))

	return nil
}

// GetByID retrieves a share request by ID
func (r *ShareRepository) GetByID(ctx context.Context, id string) (*models.ShareRequest, error) {
	query := `
		SELECT id, sender_user_id, sender_overlay_id, recipient_user_id,
		       status, created_at, responded_at, expires_at
		FROM share_requests
		WHERE id = $1
	`

	var req models.ShareRequest
	err := r.db.QueryRow(ctx, query, id).Scan(
		&req.ID, &req.SenderUserID, &req.SenderOverlayID, &req.RecipientUserID,
		&req.Status, &req.CreatedAt, &req.RespondedAt, &req.ExpiresAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("share request not found")
		}
		r.logger.Error("Failed to get share request by ID",
			zap.String("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get share request: %w", err)
	}

	return &req, nil
}

// ListIncoming lists share requests received by a user
func (r *ShareRepository) ListIncoming(ctx context.Context, recipientUserID string, status string) ([]models.ShareRequest, error) {
	query := `
		SELECT id, sender_user_id, sender_overlay_id, recipient_user_id,
		       status, created_at, responded_at, expires_at
		FROM share_requests
		WHERE recipient_user_id = $1
	`

	args := []interface{}{recipientUserID}

	// Optional status filtering for Pending vs History tabs
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to list incoming requests",
			zap.String("recipient_user_id", recipientUserID),
			zap.String("status", status),
			zap.Error(err))
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}
	defer rows.Close()

	var requests []models.ShareRequest
	for rows.Next() {
		var req models.ShareRequest
		if err := rows.Scan(&req.ID, &req.SenderUserID, &req.SenderOverlayID,
			&req.RecipientUserID, &req.Status, &req.CreatedAt,
			&req.RespondedAt, &req.ExpiresAt); err != nil {
			r.logger.Error("Failed to scan share request", zap.Error(err))
			continue
		}
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating share requests", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	r.logger.Debug("Listed incoming share requests",
		zap.String("recipient_user_id", recipientUserID),
		zap.String("status", status),
		zap.Int("count", len(requests)))

	return requests, nil
}

// UpdateStatus updates the status of a share request and sets responded_at timestamp
func (r *ShareRepository) UpdateStatus(ctx context.Context, id string, newStatus string) error {
	// Validate status
	validStatuses := map[string]bool{
		models.StatusAccepted: true,
		models.StatusRejected: true,
		models.StatusExpired:  true,
	}
	if !validStatuses[newStatus] {
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	query := `
		UPDATE share_requests
		SET status = $1, responded_at = NOW()
		WHERE id = $2
		RETURNING status, responded_at
	`

	var status string
	var respondedAt *string
	err := r.db.QueryRow(ctx, query, newStatus, id).Scan(&status, &respondedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("share request not found")
		}
		r.logger.Error("Failed to update share request status",
			zap.String("id", id),
			zap.String("new_status", newStatus),
			zap.Error(err))
		return fmt.Errorf("failed to update status: %w", err)
	}

	r.logger.Info("Share request status updated",
		zap.String("id", id),
		zap.String("new_status", newStatus))

	return nil
}

// ListByOverlay lists all share requests for a specific overlay
func (r *ShareRepository) ListByOverlay(ctx context.Context, overlayID string) ([]models.ShareRequest, error) {
	query := `
		SELECT id, sender_user_id, sender_overlay_id, recipient_user_id,
		       status, created_at, responded_at, expires_at
		FROM share_requests
		WHERE sender_overlay_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, overlayID)
	if err != nil {
		r.logger.Error("Failed to list requests by overlay",
			zap.String("overlay_id", overlayID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to list requests: %w", err)
	}
	defer rows.Close()

	var requests []models.ShareRequest
	for rows.Next() {
		var req models.ShareRequest
		if err := rows.Scan(&req.ID, &req.SenderUserID, &req.SenderOverlayID,
			&req.RecipientUserID, &req.Status, &req.CreatedAt,
			&req.RespondedAt, &req.ExpiresAt); err != nil {
			r.logger.Error("Failed to scan share request", zap.Error(err))
			continue
		}
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating share requests", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	return requests, nil
}

// ExpirePendingRequests marks pending requests with expired dates as 'expired'
// Returns the number of requests that were expired
func (r *ShareRepository) ExpirePendingRequests(ctx context.Context) (int, error) {
	query := `
		UPDATE share_requests
		SET status = $1, responded_at = NOW()
		WHERE status = $2 AND expires_at < NOW()
	`

	result, err := r.db.Exec(ctx, query, models.StatusExpired, models.StatusPending)
	if err != nil {
		r.logger.Error("Failed to expire pending requests", zap.Error(err))
		return 0, fmt.Errorf("failed to expire requests: %w", err)
	}

	rowsAffected := result.RowsAffected()
	return int(rowsAffected), nil
}
