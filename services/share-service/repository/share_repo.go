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

// GetAcceptedSharesByRecipient returns accepted shares where recipient_user_id = userID
// This represents "who does this user share TO" (outgoing edges in share graph for cycle detection)
func (r *ShareRepository) GetAcceptedSharesByRecipient(ctx context.Context, userID string) ([]models.ShareRequest, error) {
	query := `
		SELECT id, sender_user_id, sender_overlay_id, recipient_user_id,
		       status, created_at, responded_at, expires_at
		FROM share_requests
		WHERE recipient_user_id = $1 AND status = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID, models.StatusAccepted)
	if err != nil {
		r.logger.Error("Failed to get accepted shares by recipient",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get accepted shares: %w", err)
	}
	defer rows.Close()

	var shares []models.ShareRequest
	for rows.Next() {
		var share models.ShareRequest
		if err := rows.Scan(&share.ID, &share.SenderUserID, &share.SenderOverlayID,
			&share.RecipientUserID, &share.Status, &share.CreatedAt,
			&share.RespondedAt, &share.ExpiresAt); err != nil {
			r.logger.Error("Failed to scan share request", zap.Error(err))
			continue
		}
		shares = append(shares, share)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating accepted shares", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	r.logger.Debug("Retrieved accepted shares by recipient",
		zap.String("user_id", userID),
		zap.Int("count", len(shares)))

	return shares, nil
}

// GetUnseenAcceptances returns accepted share requests where user is sender and has_seen_acceptance = false
// Includes recipient_display_name from users table (the accepting user's name)
func (r *ShareRepository) GetUnseenAcceptances(ctx context.Context, senderUserID string) ([]models.ShareRequest, error) {
	query := `
		SELECT sr.id, sr.sender_user_id, sr.sender_overlay_id, sr.recipient_user_id,
		       sr.status, sr.created_at, sr.responded_at, sr.expires_at, sr.has_seen_acceptance,
		       u.display_name as sender_display_name
		FROM share_requests sr
		JOIN users u ON u.id = sr.recipient_user_id
		WHERE sr.sender_user_id = $1
		  AND sr.status = $2
		  AND sr.has_seen_acceptance = false
		ORDER BY sr.responded_at DESC
	`

	rows, err := r.db.Query(ctx, query, senderUserID, models.StatusAccepted)
	if err != nil {
		r.logger.Error("Failed to get unseen acceptances",
			zap.String("sender_user_id", senderUserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get unseen acceptances: %w", err)
	}
	defer rows.Close()

	var requests []models.ShareRequest
	for rows.Next() {
		var req models.ShareRequest
		err := rows.Scan(&req.ID, &req.SenderUserID, &req.SenderOverlayID, &req.RecipientUserID,
			&req.Status, &req.CreatedAt, &req.RespondedAt, &req.ExpiresAt, &req.HasSeenAcceptance,
			&req.SenderDisplayName)
		if err != nil {
			r.logger.Error("Failed to scan share request", zap.Error(err))
			continue
		}
		requests = append(requests, req)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating unseen acceptances", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	r.logger.Debug("Retrieved unseen acceptances",
		zap.String("sender_user_id", senderUserID),
		zap.Int("count", len(requests)))

	return requests, nil
}

// AcceptedShareDetail contains the accepted share info needed for the "add source" UI
type AcceptedShareDetail struct {
	ShareID           string `json:"share_id"`
	SenderOverlayID   string `json:"sender_overlay_id"`
	SenderOverlayName string `json:"sender_overlay_name"`
	SenderDisplayName string `json:"sender_display_name"`
	ShareStatus       string `json:"share_status"`
}

// GetAcceptedShareDetails returns accepted shares where the current user is recipient,
// joined with overlay name and sender display name for the add-source UI.
func (r *ShareRepository) GetAcceptedShareDetails(ctx context.Context, recipientUserID string) ([]*AcceptedShareDetail, error) {
	query := `
		SELECT
			sr.id AS share_id,
			sr.sender_overlay_id,
			o.name AS sender_overlay_name,
			u.display_name AS sender_display_name,
			sr.status AS share_status
		FROM share_requests sr
		JOIN overlays o ON o.id = sr.sender_overlay_id
		JOIN users u ON u.id = sr.sender_user_id
		WHERE sr.recipient_user_id = $1
		  AND sr.status = 'accepted'
		ORDER BY sr.created_at DESC
	`
	rows, err := r.db.Query(ctx, query, recipientUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query accepted shares: %w", err)
	}
	defer rows.Close()

	details := make([]*AcceptedShareDetail, 0)
	for rows.Next() {
		var d AcceptedShareDetail
		if err := rows.Scan(&d.ShareID, &d.SenderOverlayID, &d.SenderOverlayName,
			&d.SenderDisplayName, &d.ShareStatus); err != nil {
			return nil, fmt.Errorf("failed to scan accepted share: %w", err)
		}
		details = append(details, &d)
	}
	return details, rows.Err()
}

// ExpireAcceptedShare atomically sets a single accepted share to 'expired' and deactivates
// its overlay_chat_sources entries. Mirrors RevokeShareRequest pattern.
// Idempotent: if the share is not in 'accepted' state, this is a no-op.
func (r *ShareRepository) ExpireAcceptedShare(ctx context.Context, shareID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	result, err := tx.Exec(ctx,
		`UPDATE share_requests SET status = $1, responded_at = NOW()
		 WHERE id = $2 AND status = $3`,
		models.StatusExpired, shareID, models.StatusAccepted)
	if err != nil {
		return fmt.Errorf("failed to expire share: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil // Already expired or not found — idempotent
	}

	_, err = tx.Exec(ctx,
		`UPDATE overlay_chat_sources SET is_active = false
		 WHERE channel_id = $1 AND platform = 'shared_overlay'`,
		shareID)
	if err != nil {
		return fmt.Errorf("failed to deactivate sources: %w", err)
	}

	return tx.Commit(ctx)
}

// ExpireTimedAcceptedShares expires accepted shares whose share_expires_at has passed.
// Uses a transaction per share to atomically deactivate overlay_chat_sources.
// Returns the count of shares expired.
func (r *ShareRepository) ExpireTimedAcceptedShares(ctx context.Context) (int, error) {
	// Find IDs of expired accepted shares with custom time-based expiry
	rows, err := r.db.Query(ctx, `
		SELECT id FROM share_requests
		WHERE status = $1
		  AND expiry_option = 'custom'
		  AND share_expires_at IS NOT NULL
		  AND share_expires_at < NOW()
	`, models.StatusAccepted)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired shares: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			r.logger.Error("Failed to scan share ID", zap.Error(err))
			continue
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to iterate share IDs: %w", err)
	}

	count := 0
	for _, id := range ids {
		if err := r.ExpireAcceptedShare(ctx, id); err != nil {
			r.logger.Error("Failed to expire timed share",
				zap.String("share_id", id),
				zap.Error(err))
			continue
		}
		count++
	}

	if count > 0 {
		r.logger.Info("Expired timed accepted shares", zap.Int("count", count))
	}
	return count, nil
}

// MarkAcceptanceSeen sets has_seen_acceptance = true for share request
func (r *ShareRepository) MarkAcceptanceSeen(ctx context.Context, id string) error {
	query := `UPDATE share_requests SET has_seen_acceptance = true WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to mark acceptance seen",
			zap.String("id", id),
			zap.Error(err))
		return fmt.Errorf("failed to mark acceptance seen: %w", err)
	}

	r.logger.Debug("Marked acceptance seen",
		zap.String("id", id))

	return nil
}
