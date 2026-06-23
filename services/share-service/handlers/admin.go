// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// userEntitlementWriter is the narrow repository surface the admin handler needs.
// Satisfied by *repository.PremiumRepository in production and a mock in tests.
type userEntitlementWriter interface {
	UpdateUserPremium(ctx context.Context, userID string, isPremium bool) error
	SetUserBetaTester(ctx context.Context, userID string, isBetaTester bool) error
}

type AdminHandler struct {
	premiumRepo userEntitlementWriter
	logger      *zap.Logger
}

func NewAdminHandler(premiumRepo userEntitlementWriter, logger *zap.Logger) *AdminHandler {
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

		// Repo returns "user not found: <id>" (fmt.Errorf, no sentinel) — match on prefix.
		if strings.HasPrefix(err.Error(), "user not found") {
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

// SetUserBetaTester handles POST /api/v1/admin/beta-tester/users/:id
// Body: {"is_beta_tester": true}
// Grants/revokes the ADR-0020 beta-tester role (all premium + early-access
// features). This is the ongoing grandfathering mechanism — there is no data
// migration.
func (h *AdminHandler) SetUserBetaTester(c *gin.Context) {
	adminUserID := c.GetString("user_id") // From JWTAuth middleware
	targetUserID := c.Param("id")

	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "user ID required",
		})
		return
	}

	var req struct {
		IsBetaTester bool `json:"is_beta_tester"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "is_beta_tester field required",
		})
		return
	}

	err := h.premiumRepo.SetUserBetaTester(c.Request.Context(), targetUserID, req.IsBetaTester)
	if err != nil {
		h.logger.Error("Failed to update beta-tester status",
			zap.String("admin_id", adminUserID),
			zap.String("target_id", targetUserID),
			zap.Error(err))

		// Repo returns "user not found: <id>" (fmt.Errorf, no sentinel) — match on prefix.
		if strings.HasPrefix(err.Error(), "user not found") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "user not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update beta-tester status",
		})
		return
	}

	h.logger.Info("Beta-tester status updated by admin",
		zap.String("admin_id", adminUserID),
		zap.String("target_id", targetUserID),
		zap.Bool("is_beta_tester", req.IsBetaTester))

	c.JSON(http.StatusOK, gin.H{
		"message":        "beta-tester status updated",
		"user_id":        targetUserID,
		"is_beta_tester": req.IsBetaTester,
	})
}
