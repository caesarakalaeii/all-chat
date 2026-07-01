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

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetBalance (viewer) returns the caller's point balance in one overlay economy.
func (h *Handler) GetBalance(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := overlayIDQuery(c)
	if !ok {
		return
	}
	bal, err := h.repo.GetBalance(c.Request.Context(), viewerID, overlayID)
	if err != nil {
		h.log.Error("get balance", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load balance"})
		return
	}
	cfg, _ := h.repo.GetEarnConfig(c.Request.Context(), overlayID)
	c.JSON(http.StatusOK, gin.H{"balance": bal, "points_name": cfg.PointsName})
}

// GetEngagement (viewer) returns the pull-first private snapshot: balance plus the
// caller's current vote and wager on the overlay's live poll/prediction. This is
// how non-broadcast per-viewer state reaches the web page / extension (v1).
func (h *Handler) GetEngagement(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := overlayIDQuery(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	bal, err := h.repo.GetBalance(ctx, viewerID, overlayID)
	if err != nil {
		h.log.Error("engagement balance", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load engagement"})
		return
	}
	cfg, _ := h.repo.GetEarnConfig(ctx, overlayID)
	out := models.ViewerEngagement{PointsName: cfg.PointsName, Balance: bal}

	if poll, err := h.repo.GetActivePoll(ctx, overlayID); err == nil {
		if optID, err := h.repo.GetViewerVote(ctx, poll.ID, viewerID); err == nil {
			out.VotedOptionID = optID
		}
	}
	if pred, err := h.repo.GetActivePrediction(ctx, overlayID); err == nil {
		if outcomeID, amount, err := h.repo.GetViewerEntry(ctx, pred.ID, viewerID); err == nil && outcomeID != nil {
			out.WagerOutcome = outcomeID
			out.WagerAmount = amount
		}
	}
	c.JSON(http.StatusOK, out)
}

// GetConfig (owner) returns the overlay's points earning configuration.
func (h *Handler) GetConfig(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	cfg, err := h.repo.GetEarnConfig(c.Request.Context(), overlayID)
	if err != nil {
		h.log.Error("get config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load config"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// PutConfig (owner) updates the overlay's points earning configuration.
func (h *Handler) PutConfig(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	var cfg models.EarnConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid config body"})
		return
	}
	cfg.OverlayID = overlayID // never trust the body's id
	if cfg.PointsName == "" {
		cfg.PointsName = "Points"
	}
	if err := h.repo.UpsertEarnConfig(c.Request.Context(), cfg); err != nil {
		h.log.Error("put config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save config"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type heartbeatReq struct {
	OverlayID string `json:"overlay_id"`
}

// Heartbeat (viewer) awards watch-time points, deduped to once per minute-bucket
// per (viewer, overlay). Sent by the web page / extension roughly every 60s while
// focused. The dedup key makes concurrent/duplicate beats idempotent.
func (h *Handler) Heartbeat(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	var req heartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id required"})
		return
	}
	overlayID, err := uuid.Parse(req.OverlayID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id must be a valid uuid"})
		return
	}
	ctx := c.Request.Context()
	cfg, err := h.repo.GetEarnConfig(ctx, overlayID)
	if err != nil {
		h.log.Error("heartbeat config", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load config"})
		return
	}
	if !cfg.Enabled || cfg.WatchPerMinute <= 0 {
		bal, _ := h.repo.GetBalance(ctx, viewerID, overlayID)
		c.JSON(http.StatusOK, gin.H{"balance": bal})
		return
	}
	bucket := time.Now().Unix() / 60
	dedup := fmt.Sprintf("watch:%s:%s:%d", viewerID, overlayID, bucket)
	if _, err := h.repo.AwardPoints(ctx, viewerID, overlayID, cfg.WatchPerMinute, "earn_watch", "heartbeat", nil, dedup); err != nil {
		if !errors.Is(err, repository.ErrInsufficientBalance) { // never happens on credit
			h.log.Warn("award watch points", zap.Error(err))
		}
	}
	bal, _ := h.repo.GetBalance(ctx, viewerID, overlayID)
	c.JSON(http.StatusOK, gin.H{"balance": bal})
}
