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
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type createPollReq struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	AllowChange *bool    `json:"allow_change"`
	DurationSec int      `json:"duration_seconds"`
}

// CreatePoll (owner) opens an All-Chat-native poll and flags the overlay's source
// channels active so chat votes are forwarded.
func (h *Handler) CreatePoll(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	var req createPollReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Question == "" || len(req.Options) < 2 || len(req.Options) > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll needs a question and 2–5 options"})
		return
	}
	allowChange := true
	if req.AllowChange != nil {
		allowChange = *req.AllowChange
	}
	var endsAt *time.Time
	if req.DurationSec > 0 {
		t := time.Now().Add(time.Duration(req.DurationSec) * time.Second)
		endsAt = &t
	}

	poll, err := h.repo.CreatePoll(c.Request.Context(), overlayID, req.Question, req.Options, allowChange, endsAt)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "an active poll already exists for this overlay"})
			return
		}
		h.log.Error("create poll", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create poll"})
		return
	}

	h.markActive(c, overlayID, poll.ID)
	h.pub.PublishPoll(c.Request.Context(), poll)
	c.JSON(http.StatusCreated, poll)
}

// ClosePoll (owner) ends a poll, clears active flags, and broadcasts the final state.
func (h *Handler) ClosePoll(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	pollID, ok := uuidParam(c, "pollId")
	if !ok {
		return
	}
	poll, err := h.repo.ClosePoll(c.Request.Context(), pollID)
	if err != nil {
		h.log.Error("close poll", zap.Error(err))
		c.JSON(statusForRepoErr(err), gin.H{"error": "could not close poll"})
		return
	}
	h.clearActive(c, overlayID, poll.ID)
	h.pub.PublishPoll(c.Request.Context(), poll)
	c.JSON(http.StatusOK, poll)
}

// GetActivePoll (public) returns the overlay's active poll for OBS/web rendering.
func (h *Handler) GetActivePoll(c *gin.Context) {
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return
	}
	poll, err := h.repo.GetActivePoll(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active poll"})
			return
		}
		h.log.Error("get active poll", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load poll"})
		return
	}
	c.JSON(http.StatusOK, poll)
}

type voteReq struct {
	OptionIdx int `json:"option_idx"`
}

// WebVote (viewer) records a click-to-vote from the web page / extension.
func (h *Handler) WebVote(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	pollID, ok := uuidParam(c, "pollId")
	if !ok {
		return
	}
	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OptionIdx < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option_idx (>=1) required"})
		return
	}
	platform := c.GetString("platform")
	accepted, err := h.repo.RecordVote(c.Request.Context(), pollID, viewerID, req.OptionIdx, platform, nil)
	if err != nil {
		h.log.Error("web vote", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record vote"})
		return
	}
	if !accepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vote not accepted (poll closed or invalid option)"})
		return
	}
	// Reload and broadcast the updated tally.
	poll, err := h.repo.GetPoll(c.Request.Context(), pollID)
	if err == nil {
		h.pub.PublishPoll(c.Request.Context(), poll)
		c.JSON(http.StatusOK, poll)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// markActive flags the overlay's source channels as running an engagement, so the
// message-processor hot path forwards chat commands from those channels.
func (h *Handler) markActive(c *gin.Context, overlayID, engagementID uuid.UUID) {
	channels, err := h.repo.SourceChannelsForOverlay(c.Request.Context(), overlayID)
	if err != nil {
		h.log.Warn("load source channels for active flag", zap.Error(err))
		return
	}
	h.pub.SetActive(c.Request.Context(), engagementID, channels)
}

// clearActive removes an engagement's active flag from the overlay's channels.
func (h *Handler) clearActive(c *gin.Context, overlayID, engagementID uuid.UUID) {
	channels, err := h.repo.SourceChannelsForOverlay(c.Request.Context(), overlayID)
	if err != nil {
		h.log.Warn("load source channels for active flag", zap.Error(err))
		return
	}
	h.pub.ClearActive(c.Request.Context(), engagementID, channels)
}
