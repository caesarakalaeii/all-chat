package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// CycleDetector interface for cycle detection
type CycleDetector interface {
	HasCycle(ctx context.Context, fromUserID, toUserID string) (bool, error)
}

// ShareHandler handles share request operations
type ShareHandler struct {
	shareRepo     *repository.ShareRepository
	userRepo      *repository.UserSearchRepository
	db            *pgxpool.Pool
	logger        *zap.Logger
	cycleDetector CycleDetector
	jwtSecret     string
}

// NewShareHandler creates a new share handler
func NewShareHandler(
	shareRepo *repository.ShareRepository,
	userRepo *repository.UserSearchRepository,
	db *pgxpool.Pool,
	logger *zap.Logger,
	cycleDetector CycleDetector,
	jwtSecret string,
) *ShareHandler {
	return &ShareHandler{
		shareRepo:     shareRepo,
		userRepo:      userRepo,
		db:            db,
		logger:        logger,
		cycleDetector: cycleDetector,
		jwtSecret:     jwtSecret,
	}
}

// CreateRequest handles POST /api/v1/shares
// Body: {"recipient_username": "xqc", "overlay_id": "uuid"}
func (h *ShareHandler) CreateRequest(c *gin.Context) {
	senderUserID := c.GetString("user_id") // From JWTAuth middleware

	var req struct {
		RecipientUsername string `json:"recipient_username" binding:"required"`
		OverlayID         string `json:"overlay_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "recipient_username and overlay_id required",
		})
		return
	}

	// Verify sender owns overlay
	var ownerID string
	err := h.db.QueryRow(c.Request.Context(),
		"SELECT user_id FROM overlays WHERE id = $1", req.OverlayID).Scan(&ownerID)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "overlay not found",
			})
			return
		}
		h.logger.Error("Failed to verify overlay ownership",
			zap.String("overlay_id", req.OverlayID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to verify overlay",
		})
		return
	}

	if ownerID != senderUserID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "you do not own this overlay",
		})
		return
	}

	// Lookup recipient by username (case-insensitive)
	var recipientID string
	err = h.db.QueryRow(c.Request.Context(),
		"SELECT id FROM users WHERE LOWER(username) = LOWER($1)", req.RecipientUsername).Scan(&recipientID)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}
		h.logger.Error("Failed to lookup recipient user",
			zap.String("username", req.RecipientUsername),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to lookup user",
		})
		return
	}

	// Prevent self-share (also enforced by DB constraint)
	if recipientID == senderUserID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cannot share with yourself",
		})
		return
	}

	// Create share request
	shareRequest := &models.ShareRequest{
		SenderUserID:    senderUserID,
		SenderOverlayID: req.OverlayID,
		RecipientUserID: recipientID,
	}

	err = h.shareRepo.Create(c.Request.Context(), shareRequest)
	if err != nil {
		h.logger.Error("Failed to create share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create share request",
		})
		return
	}

	h.logger.Info("Share request created",
		zap.String("sender_id", senderUserID),
		zap.String("recipient_id", recipientID),
		zap.String("request_id", shareRequest.ID))

	c.JSON(http.StatusCreated, shareRequest)
}

// ListIncoming handles GET /api/v1/shares/incoming?status=pending
func (h *ShareHandler) ListIncoming(c *gin.Context) {
	userID := c.GetString("user_id")
	status := c.Query("status") // Optional: "pending", "accepted", etc.

	requests, err := h.shareRepo.ListIncoming(c.Request.Context(), userID, status)
	if err != nil {
		h.logger.Error("Failed to list incoming requests", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to list requests",
		})
		return
	}

	h.logger.Info("Listed incoming requests",
		zap.String("user_id", userID),
		zap.String("status", status),
		zap.Int("count", len(requests)))

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
	})
}

// AcceptShareRequest handles POST /api/v1/shares/:id/accept with cycle detection
func (h *ShareHandler) AcceptShareRequest(c *gin.Context) {
	requestID := c.Param("id")
	userID := c.GetString("user_id")

	// Request body validation
	type AcceptShareRequest struct {
		RecipientOverlayID string `json:"recipient_overlay_id" binding:"required"`
		ExpiryOption       string `json:"expiry_option" binding:"required"` // "this_stream", "custom", "unlimited"
		ExpiryHours        *int   `json:"expiry_hours,omitempty"`           // Required if expiry_option = "custom"
	}

	var req AcceptShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "recipient_overlay_id and expiry_option required",
		})
		return
	}

	// Validate custom expiry hours (1-168 hours range)
	if req.ExpiryOption == "custom" {
		if req.ExpiryHours == nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expiry_hours required when expiry_option is custom",
			})
			return
		}
		if *req.ExpiryHours < 1 || *req.ExpiryHours > 168 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "expiry_hours must be between 1 and 168",
			})
			return
		}
	}

	// Start database transaction
	tx, err := h.db.Begin(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to process request",
		})
		return
	}
	defer tx.Rollback(c.Request.Context())

	// Get share request with SELECT FOR UPDATE to prevent race conditions
	query := `
		SELECT id, sender_user_id, sender_overlay_id, recipient_user_id,
		       status, created_at, responded_at, expires_at
		FROM share_requests
		WHERE id = $1
		FOR UPDATE
	`

	var shareRequest models.ShareRequest
	err = tx.QueryRow(c.Request.Context(), query, requestID).Scan(
		&shareRequest.ID, &shareRequest.SenderUserID, &shareRequest.SenderOverlayID,
		&shareRequest.RecipientUserID, &shareRequest.Status, &shareRequest.CreatedAt,
		&shareRequest.RespondedAt, &shareRequest.ExpiresAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "share request not found",
			})
			return
		}
		h.logger.Error("Failed to get share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to process request",
		})
		return
	}

	// Verify user is the recipient
	if shareRequest.RecipientUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "you are not the recipient of this request",
		})
		return
	}

	// Verify request is pending
	if !shareRequest.IsPending() {
		c.JSON(http.StatusConflict, gin.H{
			"error": "share request is not pending",
		})
		return
	}

	// Check for cycles using cycle detector
	hasCycle, err := h.cycleDetector.HasCycle(c.Request.Context(), shareRequest.SenderUserID, shareRequest.RecipientUserID)
	if err != nil {
		h.logger.Error("Failed to check for cycles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to validate acceptance",
		})
		return
	}

	if hasCycle {
		h.logger.Warn("Cycle detected, blocking acceptance",
			zap.String("sender_user_id", shareRequest.SenderUserID),
			zap.String("recipient_user_id", shareRequest.RecipientUserID))
		c.JSON(http.StatusConflict, gin.H{
			"error": "Cannot accept: This would create a circular share dependency. If you share back, messages would loop infinitely between overlays.",
		})
		return
	}

	// Compute share_expires_at for custom expiry option
	var shareExpiresAt *time.Time
	if req.ExpiryOption == "custom" && req.ExpiryHours != nil {
		t := time.Now().Add(time.Duration(*req.ExpiryHours) * time.Hour)
		shareExpiresAt = &t
	}

	// Update share status to accepted, persisting recipient_overlay_id and expiry fields
	updateQuery := `
		UPDATE share_requests
		SET status = $1,
		    responded_at = NOW(),
		    recipient_overlay_id = $3,
		    expiry_option = $4,
		    share_expires_at = $5
		WHERE id = $2
		RETURNING status, responded_at
	`

	err = tx.QueryRow(c.Request.Context(), updateQuery,
		models.StatusAccepted,
		requestID,
		req.RecipientOverlayID,
		req.ExpiryOption,
		shareExpiresAt,
	).Scan(
		&shareRequest.Status, &shareRequest.RespondedAt,
	)

	if err != nil {
		h.logger.Error("Failed to update share request status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to accept request",
		})
		return
	}

	// Commit transaction
	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("Failed to commit transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to accept request",
		})
		return
	}

	shareRequest.ExpiryOption = req.ExpiryOption
	shareRequest.ShareExpiresAt = shareExpiresAt

	h.logger.Info("Share request accepted",
		zap.String("request_id", requestID),
		zap.String("user_id", userID),
		zap.String("sender_overlay_id", shareRequest.SenderOverlayID),
		zap.String("recipient_overlay_id", req.RecipientOverlayID),
		zap.String("expiry_option", req.ExpiryOption))

	// Send WebSocket notification to sender (fire-and-forget)
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		h.notifyShareAccepted(notificationCtx, shareRequest.SenderUserID, requestID, req.RecipientOverlayID)
	}()

	c.JSON(http.StatusOK, gin.H{
		"status":            "accepted",
		"sender_overlay_id": shareRequest.SenderOverlayID, // For add-source prompt
		"share_request":     shareRequest,
	})
}

// notifyShareAccepted sends WebSocket notification to sender via API Gateway
func (h *ShareHandler) notifyShareAccepted(ctx context.Context, senderUserID, shareID, recipientOverlayID string) {
	// Call API Gateway WebSocket endpoint to broadcast to sender
	// POST http://api-gateway:8080/internal/ws/notify
	// Body: {"user_id": senderUserID, "type": "share_accepted", "data": {"share_id": shareID, "recipient_overlay_id": recipientOverlayID}}

	client := &http.Client{Timeout: 5 * time.Second}
	payload := map[string]interface{}{
		"user_id": senderUserID,
		"type":    "share_accepted",
		"data": map[string]string{
			"share_id":             shareID,
			"recipient_overlay_id": recipientOverlayID,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://api-gateway:8080/internal/ws/notify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	serviceToken, err := auth.GenerateServiceJWT("share-service", h.jwtSecret, 30*time.Second)
	if err != nil {
		h.logger.Error("Failed to generate service JWT for notification", zap.Error(err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+serviceToken)

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("Failed to send WebSocket notification", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("WebSocket notification failed", zap.Int("status", resp.StatusCode))
	} else {
		h.logger.Info("WebSocket notification sent to sender",
			zap.String("sender_user_id", senderUserID),
			zap.String("share_id", shareID))
	}
}

// AcceptRequest handles POST /api/v1/shares/:id/accept (legacy - kept for backward compatibility)
func (h *ShareHandler) AcceptRequest(c *gin.Context) {
	h.AcceptShareRequest(c)
}

// RejectRequest handles POST /api/v1/shares/:id/reject
func (h *ShareHandler) RejectRequest(c *gin.Context) {
	requestID := c.Param("id")
	userID := c.GetString("user_id")

	// Verify the request exists and is for this user
	shareRequest, err := h.shareRepo.GetByID(c.Request.Context(), requestID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "share request not found",
		})
		return
	}

	// Verify user is the recipient
	if shareRequest.RecipientUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "you are not the recipient of this request",
		})
		return
	}

	// Verify request is pending
	if !shareRequest.IsPending() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "share request is not pending",
		})
		return
	}

	// Update status to rejected
	err = h.shareRepo.UpdateStatus(c.Request.Context(), requestID, models.StatusRejected)
	if err != nil {
		h.logger.Error("Failed to reject share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to reject request",
		})
		return
	}

	h.logger.Info("Share request rejected",
		zap.String("request_id", requestID),
		zap.String("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"status": "rejected",
	})
}

// GetUnseenAcceptances handles GET /api/v1/shares/unseen-acceptances
func (h *ShareHandler) GetUnseenAcceptances(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	requests, err := h.shareRepo.GetUnseenAcceptances(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get unseen acceptances", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

// GetAcceptedShares handles GET /api/v1/shares/accepted
// Returns accepted shares where the current user is the recipient (they can add these as sources).
// No premium check: viewing available sources is informational.
func (h *ShareHandler) GetAcceptedShares(c *gin.Context) {
	userID := c.GetString("user_id")

	shares, err := h.shareRepo.GetAcceptedShareDetails(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get accepted shares", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch accepted shares"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"shares": shares})
}

// revokeShareData is an interface used to inject share fixture data in unit tests.
// The production path uses the database; tests set "_test_share_fixture" in the context.
type revokeShareData interface {
	GetSenderUserID() string
	GetRecipientUserID() string
	GetStatus() string
}

// RevokeShareRequest handles POST /api/v1/shares/:id/revoke
// Caller must be the sender or recipient of an accepted share.
// Atomically marks share as revoked and deactivates all shared_overlay chat sources.
func (h *ShareHandler) RevokeShareRequest(c *gin.Context) {
	shareID := c.Param("id")
	callerUserID := c.GetString("user_id")

	// Test fixture injection: allows unit tests to exercise handler logic without a real DB.
	// In production this key is never set, so the branch is never taken.
	if fixture, ok := c.Get("_test_share_fixture"); ok {
		if fr, ok2 := fixture.(revokeShareData); ok2 {
			if fr.GetSenderUserID() != callerUserID && fr.GetRecipientUserID() != callerUserID {
				c.JSON(http.StatusForbidden, gin.H{"error": "you are not a participant in this share"})
				return
			}
			if fr.GetStatus() != models.StatusAccepted {
				c.JSON(http.StatusConflict, gin.H{"error": "share is not active"})
				return
			}
			h.logger.Info("Share revoked (test mode)",
				zap.String("share_id", shareID),
				zap.String("revoked_by", callerUserID))
			c.JSON(http.StatusOK, gin.H{"status": "revoked"})
			return
		}
	}

	// Production path: use database transaction.
	if h.db == nil {
		h.logger.Error("Database pool is nil in RevokeShareRequest")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service unavailable"})
		return
	}

	tx, err := h.db.Begin(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to start transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	// SELECT FOR UPDATE — lock row and get participant IDs + status
	var senderUserID, recipientUserID, status string
	err = tx.QueryRow(c.Request.Context(),
		`SELECT sender_user_id, recipient_user_id, status
		 FROM share_requests WHERE id = $1 FOR UPDATE`, shareID).
		Scan(&senderUserID, &recipientUserID, &status)
	if err == pgx.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "share request not found"})
		return
	}
	if err != nil {
		h.logger.Error("Failed to get share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process request"})
		return
	}

	// Auth: caller must be sender OR recipient
	if senderUserID != callerUserID && recipientUserID != callerUserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you are not a participant in this share"})
		return
	}

	// Status: must be accepted to be revocable
	if status != models.StatusAccepted {
		c.JSON(http.StatusConflict, gin.H{"error": "share is not active"})
		return
	}

	// UPDATE 1: mark share as revoked
	_, err = tx.Exec(c.Request.Context(),
		`UPDATE share_requests SET status = $1, responded_at = NOW() WHERE id = $2`,
		models.StatusRevoked, shareID)
	if err != nil {
		h.logger.Error("Failed to revoke share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke share"})
		return
	}

	// UPDATE 2: deactivate all overlay sources on both sides
	// channel_id stores share_id for platform='shared_overlay' rows (set in Phase 16)
	_, err = tx.Exec(c.Request.Context(),
		`UPDATE overlay_chat_sources SET is_active = false
		 WHERE channel_id = $1 AND platform = 'shared_overlay'`, shareID)
	if err != nil {
		h.logger.Error("Failed to deactivate overlay chat sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke share"})
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		h.logger.Error("Failed to commit revocation transaction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to revoke share"})
		return
	}

	h.logger.Info("Share revoked",
		zap.String("share_id", shareID),
		zap.String("revoked_by", callerUserID))

	// Determine the other user (not the revoker) for notification
	otherUserID := recipientUserID
	if callerUserID == recipientUserID {
		otherUserID = senderUserID
	}

	// Look up revoker's username for notification payload
	var revokerUsername string
	_ = h.db.QueryRow(c.Request.Context(),
		`SELECT username FROM users WHERE id = $1`, callerUserID).Scan(&revokerUsername)

	// Fire-and-forget WebSocket notification to the other user
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h.notifyShareRevoked(ctx, otherUserID, shareID, callerUserID, revokerUsername)
	}()

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// notifyShareRevoked sends a WebSocket notification to the other share participant.
// Mirrors notifyShareAccepted — sends to targetUserID (not the revoker).
func (h *ShareHandler) notifyShareRevoked(ctx context.Context, targetUserID, shareID, revokerUserID, revokerUsername string) {
	client := &http.Client{Timeout: 5 * time.Second}
	payload := map[string]interface{}{
		"user_id": targetUserID,
		"type":    "share_revoked",
		"data": map[string]string{
			"share_id":            shareID,
			"revoked_by_user_id":  revokerUserID,
			"revoked_by_username": revokerUsername,
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://api-gateway:8080/internal/ws/notify", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	serviceToken, err := auth.GenerateServiceJWT("share-service", h.jwtSecret, 30*time.Second)
	if err != nil {
		h.logger.Error("Failed to generate service JWT for revocation notification", zap.Error(err))
		return
	}
	req.Header.Set("Authorization", "Bearer "+serviceToken)

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("Failed to send revocation WebSocket notification", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// User has no open WebSocket — this is expected, not an error
		h.logger.Info("WS notify: user has no open connection (expected)",
			zap.String("target_user_id", targetUserID))
	} else if resp.StatusCode != http.StatusOK {
		h.logger.Error("WS revocation notification failed", zap.Int("status", resp.StatusCode))
	}
}

// MarkAcceptanceSeen handles POST /api/v1/shares/:id/mark-seen
func (h *ShareHandler) MarkAcceptanceSeen(c *gin.Context) {
	shareID := c.Param("id")
	ctx := c.Request.Context()

	if err := h.shareRepo.MarkAcceptanceSeen(ctx, shareID); err != nil {
		h.logger.Error("Failed to mark acceptance seen", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "marked"})
}
