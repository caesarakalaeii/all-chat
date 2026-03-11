package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminHandler struct {
	premiumRepo *repository.PremiumRepository
	logger      *zap.Logger
}

func NewAdminHandler(premiumRepo *repository.PremiumRepository, logger *zap.Logger) *AdminHandler {
	return &AdminHandler{
		premiumRepo: premiumRepo,
		logger:      logger,
	}
}

// SetUserPremium handles POST /api/v1/admin/users/:id/premium
// Body: {"is_premium": true}
// User decision: Dedicated endpoint for clarity (not generic PATCH)
func (h *AdminHandler) SetUserPremium(c *gin.Context) {
	adminUserID := c.GetString("user_id") // From JWTAuth middleware
	targetUserID := c.Param("id")

	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "user ID required",
		})
		return
	}

	var req struct {
		IsPremium bool `json:"is_premium"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "is_premium field required",
		})
		return
	}

	// Update premium status
	err := h.premiumRepo.UpdateUserPremium(c.Request.Context(), targetUserID, req.IsPremium)
	if err != nil {
		h.logger.Error("Failed to update premium status",
			zap.String("admin_id", adminUserID),
			zap.String("target_id", targetUserID),
			zap.Error(err))

		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update premium status",
		})
		return
	}

	h.logger.Info("Premium status updated by admin",
		zap.String("admin_id", adminUserID),
		zap.String("target_id", targetUserID),
		zap.Bool("is_premium", req.IsPremium))

	c.JSON(http.StatusOK, gin.H{
		"message":    "premium status updated",
		"user_id":    targetUserID,
		"is_premium": req.IsPremium,
	})
}
