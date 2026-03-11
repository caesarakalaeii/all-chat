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
