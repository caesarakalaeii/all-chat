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
	"strconv"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AdminViewerHandler handles admin operations for viewer management
type AdminViewerHandler struct {
	log        *zap.Logger
	viewerRepo *repository.ViewerRepository
}

// NewAdminViewerHandler creates a new admin viewer handler
func NewAdminViewerHandler(log *zap.Logger, viewerRepo *repository.ViewerRepository) *AdminViewerHandler {
	return &AdminViewerHandler{
		log:        log.Named("admin-viewers"),
		viewerRepo: viewerRepo,
	}
}

// BanRequest is the request body for banning a viewer
type BanRequest struct {
	Reason string `json:"reason"`
}

// HandleListViewers lists all viewer sessions with pagination
func (h *AdminViewerHandler) HandleListViewers(c *gin.Context) {
	// Parse pagination parameters
	limit := 50
	offset := 0

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	ctx := c.Request.Context()
	sessions, err := h.viewerRepo.ListAll(ctx, limit, offset)
	if err != nil {
		h.log.Error("Failed to list viewers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list viewers"})
		return
	}

	// Don't expose tokens in response
	for i := range sessions {
		sessions[i].AccessToken = ""
		sessions[i].RefreshToken = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"viewers": sessions,
		"limit":   limit,
		"offset":  offset,
	})
}

// HandleBanViewer bans a viewer from sending messages
func (h *AdminViewerHandler) HandleBanViewer(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	var req BanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = "No reason provided"
	}

	ctx := c.Request.Context()
	if err := h.viewerRepo.BanViewer(ctx, sessionID, req.Reason); err != nil {
		h.log.Error("Failed to ban viewer", zap.Error(err), zap.String("session_id", sessionIDStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to ban viewer"})
		return
	}

	h.log.Info("Viewer banned", zap.String("session_id", sessionIDStr), zap.String("reason", req.Reason))
	c.JSON(http.StatusOK, gin.H{"message": "Viewer banned successfully"})
}

// HandleUnbanViewer removes the ban from a viewer
func (h *AdminViewerHandler) HandleUnbanViewer(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	ctx := c.Request.Context()
	if err := h.viewerRepo.UnbanViewer(ctx, sessionID); err != nil {
		h.log.Error("Failed to unban viewer", zap.Error(err), zap.String("session_id", sessionIDStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unban viewer"})
		return
	}

	h.log.Info("Viewer unbanned", zap.String("session_id", sessionIDStr))
	c.JSON(http.StatusOK, gin.H{"message": "Viewer unbanned successfully"})
}

// SetPremiumRequest is the request body for setting viewer premium status.
// DurationSeconds (optional, positive) makes the grant time-limited (ADR-0027);
// omit it for a permanent grant. It is ignored when revoking.
type SetPremiumRequest struct {
	IsPremium       bool   `json:"is_premium"`
	DurationSeconds *int64 `json:"duration_seconds"`
}

// maxViewerPremiumGrantSeconds caps a time-limited viewer premium grant (ADR-0027)
// at ~10 years, mirroring the streamer side (share-service).
const maxViewerPremiumGrantSeconds int64 = 10 * 365 * 24 * 60 * 60

// HandleSetViewerPremium grants or revokes premium status for a viewer
func (h *AdminViewerHandler) HandleSetViewerPremium(c *gin.Context) {
	sessionIDStr := c.Param("session_id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	var req SetPremiumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	var ttl *time.Duration
	if req.DurationSeconds != nil {
		if *req.DurationSeconds <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration_seconds must be positive"})
			return
		}
		if *req.DurationSeconds > maxViewerPremiumGrantSeconds {
			c.JSON(http.StatusBadRequest, gin.H{"error": "duration_seconds exceeds maximum"})
			return
		}
		d := time.Duration(*req.DurationSeconds) * time.Second
		ttl = &d
	}

	ctx := c.Request.Context()
	if err := h.viewerRepo.SetViewerPremium(ctx, sessionID, req.IsPremium, ttl); err != nil {
		h.log.Error("Failed to set viewer premium", zap.Error(err), zap.String("session_id", sessionIDStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update viewer premium status"})
		return
	}

	action := "granted"
	if !req.IsPremium {
		action = "revoked"
	}
	h.log.Info("Viewer premium status updated",
		zap.String("session_id", sessionIDStr),
		zap.Bool("is_premium", req.IsPremium),
		zap.Bool("time_limited", ttl != nil),
	)
	resp := gin.H{
		"message":    "Viewer premium " + action + " successfully",
		"session_id": sessionIDStr,
		"is_premium": req.IsPremium,
	}
	if ttl != nil && req.IsPremium {
		resp["duration_seconds"] = int64(ttl.Seconds())
	}
	c.JSON(http.StatusOK, resp)
}
