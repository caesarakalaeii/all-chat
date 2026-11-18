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

// HandleAcceptTos records that the authenticated user accepted the current
// Terms of Service and Privacy Policy (users.tos_accepted = TRUE). The frontend
// shows a blocking prompt after login while /auth/me reports tos_accepted=false.
//
// Acceptance is the user's own act, so it is refused while an admin
// impersonates them and from a personal access token.
func (h *AuthHandler) HandleAcceptTos(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if c.GetString("impersonated_by") != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Terms cannot be accepted while impersonating"})
		return
	}
	if !RefuseAPIToken(c, "Terms can only be accepted from a signed-in session, not with a token.") {
		return
	}

	if err := h.userRepo.AcceptTos(c.Request.Context(), userID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to record terms acceptance", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record terms acceptance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tos_accepted": true})
}
