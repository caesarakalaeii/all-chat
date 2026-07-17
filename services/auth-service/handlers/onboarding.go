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
	"errors"
	"net/http"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// updateOnboardingRequest is the PATCH /me/onboarding body. Completed is a
// pointer so a missing field is rejected instead of defaulting to false
// (which would silently re-arm the setup guide).
type updateOnboardingRequest struct {
	Completed *bool `json:"completed" binding:"required"`
}

// HandleUpdateOnboarding sets or clears users.onboarding_completed_at for the
// authenticated user. completed=true is sent when the user finishes or
// dismisses the first-run setup guide; completed=false re-arms it (the
// "restart onboarding" action in Settings).
//
// Impersonation sessions are rejected: an admin browsing as a user must not
// silently mutate that user's onboarding state.
func (h *AuthHandler) HandleUpdateOnboarding(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	if ib, ok := c.Get("impersonated_by"); ok {
		if ibStr, ok := ib.(string); ok && ibStr != "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Onboarding state cannot be changed while impersonating"})
			return
		}
	}

	var req updateOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Body must be {\"completed\": true|false}"})
		return
	}

	completedAt, err := h.userRepo.SetOnboardingCompleted(c.Request.Context(), userID.(string), *req.Completed)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to update onboarding state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update onboarding state"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"onboarding_completed_at": completedAt})
}
