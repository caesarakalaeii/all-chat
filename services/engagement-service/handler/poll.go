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
	// Don't open an All-Chat poll while a mirrored Twitch poll is live on this
	// overlay (Twitch owns that round). The reverse ordering — a Twitch poll
	// beginning while an All-Chat poll runs — is handled at read time by
	// GetActiveDisplayPoll, which keeps the All-Chat poll on display so the owner
	// can still close it.
	if live, err := h.repo.HasLiveNativePoll(c.Request.Context(), overlayID); err != nil {
		h.log.Error("check live native poll", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create poll"})
		return
	} else if live {
		c.JSON(http.StatusConflict, gin.H{"error": "a Twitch poll is currently live on this overlay"})
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
	h.announcer.AnnouncePoll(overlayID, poll) // opt-in chat announce (H4-2); best-effort, non-blocking
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
	poll, err := h.repo.ClosePoll(c.Request.Context(), pollID, overlayID)
	if err != nil {
		h.log.Error("close poll", zap.Error(err))
		c.JSON(statusForRepoErr(err), gin.H{"error": "could not close poll"})
		return
	}
	h.clearActive(c, poll.ID)
	h.pub.PublishPoll(c.Request.Context(), poll)
	c.JSON(http.StatusOK, poll)
}

// GetActivePoll (public) returns the overlay's active poll for OBS/web rendering.
//
// This is a public UNAUTHENTICATED render seam keyed by the overlay UUID. The
// OBS/render path holds no token, so we cannot distinguish the owner from a viewer
// and therefore do NOT apply the is_public_for_viewers gate here — gating would black
// out the streamer's OWN OBS widget the moment they set is_public_for_viewers=false
// (which defaults to true). The overlay UUID is itself a bearer capability that
// auth-service withholds from viewers, so this discloses only the live question + tally
// to a holder of the private overlay id — not a cross-tenant leak. The
// is_public_for_viewers gate IS enforced on every viewer-authored surface
// (vote/wager/balance/heartbeat via requireViewerParticipation) and on the
// username-keyed GetStreamerActive read. See PR #524 review finding #3.
func (h *Handler) GetActivePoll(c *gin.Context) {
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return
	}
	poll, err := h.repo.GetActiveDisplayPoll(c.Request.Context(), overlayID)
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
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return
	}
	pollID, ok := uuidParam(c, "pollId")
	if !ok {
		return
	}
	if !h.requireViewerParticipation(c, overlayID) {
		return
	}
	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OptionIdx < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "option_idx (>=1) required"})
		return
	}
	platform := c.GetString("platform")
	// Bind the vote to the :id overlay in the path so a viewer cannot cast/change a
	// vote on a poll owned by another overlay (cross-tenant tally integrity). Mirrors
	// WebWager's overlay binding.
	accepted, err := h.repo.RecordVote(c.Request.Context(), pollID, viewerID, overlayID, req.OptionIdx, platform, nil, time.Now().UnixMilli())
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

// clearActive removes an engagement's active flag from exactly the channels it was
// set on (resolved from the reverse index inside the publisher, not re-derived here).
func (h *Handler) clearActive(c *gin.Context, engagementID uuid.UUID) {
	h.pub.ClearActive(c.Request.Context(), engagementID)
}
