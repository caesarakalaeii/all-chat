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

	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SearchHandler handles user search operations
type SearchHandler struct {
	repo   *repository.UserSearchRepository
	logger *zap.Logger
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(repo *repository.UserSearchRepository, logger *zap.Logger) *SearchHandler {
	return &SearchHandler{repo: repo, logger: logger}
}

// SearchUsers handles GET /api/v1/users/search?platform=twitch&query=xqc
func (h *SearchHandler) SearchUsers(c *gin.Context) {
	platform := c.Query("platform")
	query := c.Query("query")

	// Validation
	if platform == "" || query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "platform and query parameters required",
		})
		return
	}

	validPlatforms := map[string]bool{
		"twitch": true, "youtube": true, "kick": true, "tiktok": true,
	}
	if !validPlatforms[platform] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid platform, must be twitch, youtube, kick, or tiktok",
		})
		return
	}

	// Perform search with 10 result limit
	users, err := h.repo.SearchUsersByPlatform(c.Request.Context(), platform, query, 10)
	if err != nil {
		h.logger.Error("Search failed",
			zap.String("platform", platform),
			zap.String("query", query),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "search failed",
		})
		return
	}

	h.logger.Info("User search completed",
		zap.String("platform", platform),
		zap.String("query", query),
		zap.Int("result_count", len(users)))

	c.JSON(http.StatusOK, gin.H{
		"users": users,
	})
}
