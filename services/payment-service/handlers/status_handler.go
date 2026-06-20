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
	"net/http"

	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/caesar/all-chat/shared/premium"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StatusHandler serves the user's premium/Patreon status and disconnect.
type StatusHandler struct {
	subRepo    *repository.SubscriptionRepository
	tokenRepo  *repository.TokenRepository
	recomputer *premium.Recomputer
	logger     *zap.Logger
}

// NewStatusHandler builds a StatusHandler.
func NewStatusHandler(subRepo *repository.SubscriptionRepository, tokenRepo *repository.TokenRepository, recomputer *premium.Recomputer, logger *zap.Logger) *StatusHandler {
	return &StatusHandler{subRepo: subRepo, tokenRepo: tokenRepo, recomputer: recomputer, logger: logger}
}

// Status returns the caller's Patreon connection / subscription state.
func (h *StatusHandler) Status(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	sub, found, err := h.subRepo.GetByUserID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to load subscription", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return
	}
	if !found {
		c.JSON(http.StatusOK, gin.H{"connected": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"connected":  true,
		"status":     sub.Status,
		"tier_id":    sub.TierID,
		"cents":      sub.Cents,
		"renews_at":  sub.CurrentPeriodEnd,
		"is_premium": sub.Status == patreon.StatusActive,
	})
}

// Disconnect unlinks the caller's Patreon account, marks their subscriptions former,
// and recomputes premium (which revokes the subscription-derived grant; an admin
// override, if any, is preserved).
func (h *StatusHandler) Disconnect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	ctx := c.Request.Context()

	if err := h.tokenRepo.Delete(ctx, userID); err != nil {
		h.logger.Error("Failed to delete Patreon token", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}
	if err := h.subRepo.MarkFormerByUserID(ctx, userID); err != nil {
		h.logger.Error("Failed to mark subscriptions former", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}
	if _, err := h.recomputer.Recompute(ctx, userID); err != nil {
		h.logger.Error("Failed to recompute premium after disconnect", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disconnect"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Patreon disconnected"})
}
