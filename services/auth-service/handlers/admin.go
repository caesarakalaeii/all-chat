package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	repo      *repository.UserRepository
	logger    *zap.Logger
	jwtSecret string
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(repo *repository.UserRepository, logger *zap.Logger, jwtSecret string) *AdminHandler {
	return &AdminHandler{
		repo:      repo,
		logger:    logger,
		jwtSecret: jwtSecret,
	}
}

// ListUsers returns all users in the system (admin only)
// GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	// Get all users from database
	users, err := h.repo.GetAllUsers(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to fetch users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch users",
		})
		return
	}

	// Return users without sensitive information
	type UserResponse struct {
		ID              string  `json:"id"`
		Username        string  `json:"username"`
		DisplayName     string  `json:"display_name"`
		AuthProvider    string  `json:"auth_provider"`
		ProfileImageURL string  `json:"profile_image_url"`
		CreatedAt       string  `json:"created_at"`
		TwitchID        *string `json:"twitch_id"`
		YouTubeID       *string `json:"youtube_id"`
		KickID          *string `json:"kick_id"`
		TikTokID        *string `json:"tiktok_id"`
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = UserResponse{
			ID:              user.ID,
			Username:        user.Username,
			DisplayName:     user.DisplayName,
			AuthProvider:    user.AuthProvider,
			ProfileImageURL: user.ProfileImageURL,
			CreatedAt:       user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			TwitchID:        user.TwitchID,
			YouTubeID:       user.GoogleID,
			KickID:          user.KickID,
			TikTokID:        user.TikTokOpenID,
		}
	}

	h.logger.Info("Listed users", zap.Int("count", len(users)))
	c.JSON(http.StatusOK, response)
}

// GetUser returns a specific user by ID (admin only)
// GET /api/v1/admin/users/:id
func (h *AdminHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")

	user, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to fetch user", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	// Return user without sensitive information
	response := gin.H{
		"id":                user.ID,
		"username":          user.Username,
		"display_name":      user.DisplayName,
		"auth_provider":     user.AuthProvider,
		"profile_image_url": user.ProfileImageURL,
		"created_at":        user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"twitch_id":         user.TwitchID,
		"youtube_id":        user.GoogleID,
		"kick_id":           user.KickID,
		"tiktok_id":         user.TikTokOpenID,
	}

	h.logger.Info("Fetched user", zap.String("user_id", userID))
	c.JSON(http.StatusOK, response)
}

// ImpersonateUser generates an impersonation token for an admin to act as another user
// POST /api/v1/admin/users/:id/impersonate
func (h *AdminHandler) ImpersonateUser(c *gin.Context) {
	// Get admin user ID from context (set by auth middleware)
	adminUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Get admin username from context
	adminUsername, _ := c.Get("username")

	// Get target user ID from URL
	targetUserID := c.Param("id")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user ID required"})
		return
	}

	// Fetch target user from database
	targetUser, err := h.repo.GetUserByID(c.Request.Context(), targetUserID)
	if err != nil {
		h.logger.Error("Failed to fetch target user for impersonation",
			zap.String("target_user_id", targetUserID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Get target's Twitch ID (if available)
	targetTwitchID := ""
	if targetUser.TwitchID != nil {
		targetTwitchID = *targetUser.TwitchID
	}

	// Generate impersonation JWT
	token, err := auth.GenerateImpersonationJWT(
		adminUserID.(string),
		adminUsername.(string),
		targetUser.ID,
		targetUser.Username,
		targetTwitchID,
		h.jwtSecret,
	)
	if err != nil {
		h.logger.Error("Failed to generate impersonation token",
			zap.String("admin_id", adminUserID.(string)),
			zap.String("target_id", targetUserID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	h.logger.Info("Admin impersonating user",
		zap.String("admin_id", adminUserID.(string)),
		zap.String("admin_username", adminUsername.(string)),
		zap.String("target_id", targetUserID),
		zap.String("target_username", targetUser.Username))

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user_id":     targetUser.ID,
		"username":    targetUser.Username,
		"expires_in":  7200, // 2 hours in seconds
		"impersonating": true,
	})
}
