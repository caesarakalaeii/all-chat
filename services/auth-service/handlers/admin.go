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
	"fmt"
	"net/http"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// AdminHandler handles admin-specific endpoints
type AdminHandler struct {
	repo         *repository.UserRepository
	db           *pgxpool.Pool
	logger       *zap.Logger
	userKeyChain *auth.KeyChain
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(repo *repository.UserRepository, db *pgxpool.Pool, logger *zap.Logger, userKeyChain *auth.KeyChain) *AdminHandler {
	return &AdminHandler{
		repo:         repo,
		db:           db,
		logger:       logger,
		userKeyChain: userKeyChain,
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
		IsPremium       bool    `json:"is_premium"`
		IsBetaTester    bool    `json:"is_beta_tester"`
		IsBanned        bool    `json:"is_banned"`
		BannedAt        *string `json:"banned_at,omitempty"`
		BannedReason    *string `json:"banned_reason,omitempty"`
		BannedBy        *string `json:"banned_by,omitempty"`
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		var bannedAt *string
		if user.BannedAt != nil {
			formatted := user.BannedAt.Format("2006-01-02T15:04:05Z07:00")
			bannedAt = &formatted
		}

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
			IsPremium:       user.IsPremium,
			IsBetaTester:    user.IsBetaTester,
			IsBanned:        user.IsBanned,
			BannedAt:        bannedAt,
			BannedReason:    user.BannedReason,
			BannedBy:        user.BannedBy,
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
	token, err := auth.GenerateImpersonationJWTWithKid(
		h.userKeyChain.LatestKid(),
		adminUserID.(string),
		adminUsername.(string),
		targetUser.ID,
		targetUser.Username,
		targetTwitchID,
		string(h.userKeyChain.LatestSecret()),
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

	// DSGVO Art. 5(2) accountability: persist audit record
	_, auditErr := h.db.Exec(c.Request.Context(),
		`INSERT INTO impersonation_audit_log (admin_user_id, admin_username, target_user_id, target_username)
		 VALUES ($1, $2, $3, $4)`,
		adminUserID.(string), adminUsername.(string), targetUserID, targetUser.Username)
	if auditErr != nil {
		h.logger.Error("Failed to write impersonation audit log",
			zap.String("admin_id", adminUserID.(string)),
			zap.String("target_id", targetUserID),
			zap.Error(auditErr))
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         token,
		"user_id":       targetUser.ID,
		"username":      targetUser.Username,
		"expires_in":    7200, // 2 hours in seconds
		"impersonating": true,
	})
}

// BanUser bans a user account (admin only)
// POST /api/v1/admin/users/:id/ban
func (h *AdminHandler) BanUser(c *gin.Context) {
	adminID := c.GetString("user_id") // from JWT
	userID := c.Param("id")

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reason is required"})
		return
	}

	// Get user to ban their platform IDs too
	user, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Don't allow banning yourself
	if userID == adminID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot ban yourself"})
		return
	}

	// Don't allow banning other admins
	if user.IsAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot ban another admin"})
		return
	}

	// Ban user account (transaction handles overlays/sources)
	if err := h.repo.BanUser(c.Request.Context(), userID, adminID, req.Reason); err != nil {
		h.logger.Error("Failed to ban user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban user"})
		return
	}

	// Also ban all platform IDs for this user
	if user.TwitchID != nil {
		h.repo.BanPlatformID(c.Request.Context(), "twitch", *user.TwitchID, adminID, "Auto-ban: "+req.Reason)
	}
	if user.GoogleID != nil {
		h.repo.BanPlatformID(c.Request.Context(), "youtube", *user.GoogleID, adminID, "Auto-ban: "+req.Reason)
	}
	if user.KickID != nil {
		h.repo.BanPlatformID(c.Request.Context(), "kick", *user.KickID, adminID, "Auto-ban: "+req.Reason)
	}

	h.logger.Info("User banned",
		zap.String("user_id", userID),
		zap.String("username", user.Username),
		zap.String("admin_id", adminID),
		zap.String("reason", req.Reason))

	c.JSON(http.StatusOK, gin.H{"message": "user banned successfully"})
}

// UnbanUser removes ban from user account (admin only)
// POST /api/v1/admin/users/:id/unban
func (h *AdminHandler) UnbanUser(c *gin.Context) {
	adminID := c.GetString("user_id")
	userID := c.Param("id")

	// Get user for logging
	user, err := h.repo.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if err := h.repo.UnbanUser(c.Request.Context(), userID); err != nil {
		h.logger.Error("Failed to unban user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban user"})
		return
	}

	h.logger.Info("User unbanned",
		zap.String("user_id", userID),
		zap.String("username", user.Username),
		zap.String("admin_id", adminID))

	c.JSON(http.StatusOK, gin.H{"message": "user unbanned successfully"})
}

// ListBannedUsers returns all banned users (admin only)
// GET /api/v1/admin/users/banned
func (h *AdminHandler) ListBannedUsers(c *gin.Context) {
	limit := 50
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil {
			offset = parsed
		}
	}

	users, err := h.repo.GetBannedUsers(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("Failed to fetch banned users", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch banned users"})
		return
	}

	h.logger.Info("Listed banned users", zap.Int("count", len(users)))
	c.JSON(http.StatusOK, gin.H{"banned_users": users})
}

// GetDashboardStats returns aggregated statistics for admin dashboard
// GET /api/v1/admin/stats
func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	type AdminStats struct {
		TotalUsers     int            `json:"total_users"`
		BannedUsers    int            `json:"banned_users"`
		ActiveOverlays int            `json:"active_overlays"`
		TotalSources   map[string]int `json:"total_sources"`
	}

	var stats AdminStats
	stats.TotalSources = make(map[string]int)

	// Total users
	err := h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)
	if err != nil {
		h.logger.Error("Failed to count users", zap.Error(err))
	}

	// Banned users
	err = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM users WHERE is_banned = true").Scan(&stats.BannedUsers)
	if err != nil {
		h.logger.Error("Failed to count banned users", zap.Error(err))
	}

	// Active overlays
	err = h.db.QueryRow(c.Request.Context(), "SELECT COUNT(*) FROM overlays WHERE is_active = true").Scan(&stats.ActiveOverlays)
	if err != nil {
		h.logger.Error("Failed to count active overlays", zap.Error(err))
	}

	// Sources by platform
	rows, err := h.db.Query(c.Request.Context(), `
		SELECT platform, COUNT(*)
		FROM overlay_chat_sources
		GROUP BY platform
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var platform string
			var count int
			if err := rows.Scan(&platform, &count); err == nil {
				stats.TotalSources[platform] = count
			}
		}
	}

	c.JSON(http.StatusOK, stats)
}

// Helper function to parse integers safely
func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
}
