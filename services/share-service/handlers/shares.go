package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ShareHandler handles share request operations
type ShareHandler struct {
	shareRepo *repository.ShareRepository
	userRepo  *repository.UserSearchRepository
	db        *pgxpool.Pool
	logger    *zap.Logger
}

// NewShareHandler creates a new share handler
func NewShareHandler(
	shareRepo *repository.ShareRepository,
	userRepo *repository.UserSearchRepository,
	db *pgxpool.Pool,
	logger *zap.Logger,
) *ShareHandler {
	return &ShareHandler{
		shareRepo: shareRepo,
		userRepo:  userRepo,
		db:        db,
		logger:    logger,
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

// AcceptRequest handles POST /api/v1/shares/:id/accept
func (h *ShareHandler) AcceptRequest(c *gin.Context) {
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

	// Update status to accepted
	err = h.shareRepo.UpdateStatus(c.Request.Context(), requestID, models.StatusAccepted)
	if err != nil {
		h.logger.Error("Failed to accept share request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to accept request",
		})
		return
	}

	h.logger.Info("Share request accepted",
		zap.String("request_id", requestID),
		zap.String("user_id", userID))

	c.JSON(http.StatusOK, gin.H{
		"status": "accepted",
	})
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
