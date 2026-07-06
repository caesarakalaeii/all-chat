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

// Streamer-keyed viewer participation (issue #523, ADR-0031). The browser
// extension and no-install viewer page know only a streamer's username — the
// overlay id is a bearer capability auth-service withholds from viewers. These
// endpoints resolve username -> the streamer's public overlay server-side (the
// same resolution /ws/chat/{streamer} uses) and then reuse the exact overlay-keyed
// vote/wager/balance primitives, so the overlay id never crosses the wire. Every
// mutating primitive re-binds to the resolved overlay (RecordVote/Wager reject a
// poll/prediction from another overlay), so username resolution grants no
// cross-overlay authority.
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// resolveStreamerOverlay reads the :username path segment and resolves it to the
// streamer's public overlay id. It aborts with 400 on an empty username and 404 when
// the streamer has no active, public-for-viewers overlay (mirroring the "not public"
// close the viewer WebSocket returns), so callers can `if !ok { return }`.
func (h *Handler) resolveStreamerOverlay(c *gin.Context) (uuid.UUID, bool) {
	username := strings.TrimSpace(c.Param("username"))
	if username == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "streamer username required"})
		return uuid.Nil, false
	}
	overlayID, ok, err := h.repo.PublicOverlayForStreamer(c.Request.Context(), username)
	if err != nil {
		h.log.Error("resolve streamer overlay", zap.String("username", username), zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not resolve streamer"})
		return uuid.Nil, false
	}
	if !ok {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "streamer not found or overlay not public for viewers"})
		return uuid.Nil, false
	}
	return overlayID, true
}

// streamerActiveResponse is the public aggregate the extension / viewer page renders:
// the overlay's live poll and prediction (either may be null) plus the economy's
// display name. It is the streamer-keyed sibling of GET /overlays/:id/active-{poll,
// prediction} and applies the same All-Chat-over-native display precedence + grace
// window (GetActiveDisplayPoll / GetActiveDisplayPrediction).
type streamerActiveResponse struct {
	PointsName string             `json:"points_name"`
	Poll       *models.Poll       `json:"poll"`
	Prediction *models.Prediction `json:"prediction"`
}

// GetStreamerActive (public) returns the streamer's live poll/prediction for the
// extension and no-install viewer page. No per-viewer data, so no auth.
func (h *Handler) GetStreamerActive(c *gin.Context) {
	overlayID, ok := h.resolveStreamerOverlay(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	cfg, _ := h.repo.GetEarnConfig(ctx, overlayID)
	resp := streamerActiveResponse{PointsName: cfg.PointsName}

	if poll, err := h.repo.GetActiveDisplayPoll(ctx, overlayID); err == nil {
		resp.Poll = poll
	} else if !errors.Is(err, repository.ErrNotFound) {
		h.log.Error("streamer active poll", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load poll"})
		return
	}
	if pred, err := h.repo.GetActiveDisplayPrediction(ctx, overlayID); err == nil {
		resp.Prediction = pred
	} else if !errors.Is(err, repository.ErrNotFound) {
		h.log.Error("streamer active prediction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load prediction"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// StreamerEngagement (viewer) returns the pull-first private snapshot — balance plus
// the caller's current vote/wager on the resolved overlay's live round. Streamer-keyed
// sibling of GET /viewers/me/engagement.
func (h *Handler) StreamerEngagement(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := h.resolveStreamerOverlay(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	bal, err := h.repo.GetBalance(ctx, viewerID, overlayID)
	if err != nil {
		h.log.Error("streamer engagement balance", zap.Error(err))
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

type streamerVoteReq struct {
	PollID    string `json:"poll_id"`
	OptionIdx int    `json:"option_idx"`
}

// StreamerVote (viewer) records a click-to-vote from the extension / viewer page. The
// poll id is validated against the resolved overlay inside RecordVote, so a stale or
// foreign poll id is silently rejected as "not accepted".
func (h *Handler) StreamerVote(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := h.resolveStreamerOverlay(c)
	if !ok {
		return
	}
	var req streamerVoteReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OptionIdx < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll_id and option_idx (>=1) required"})
		return
	}
	pollID, err := uuid.Parse(req.PollID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "poll_id must be a valid uuid"})
		return
	}
	platform := c.GetString("platform")
	accepted, err := h.repo.RecordVote(c.Request.Context(), pollID, viewerID, overlayID, req.OptionIdx, platform, nil)
	if err != nil {
		h.log.Error("streamer vote", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not record vote"})
		return
	}
	if !accepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vote not accepted (poll closed or invalid option)"})
		return
	}
	poll, err := h.repo.GetPoll(c.Request.Context(), pollID)
	if err == nil {
		h.pub.PublishPoll(c.Request.Context(), poll)
		c.JSON(http.StatusOK, poll)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type streamerWagerReq struct {
	PredictionID string `json:"prediction_id"`
	OutcomeIdx   int    `json:"outcome_idx"`
	Amount       int64  `json:"amount"`
}

// StreamerWager (viewer) places a points wager from the extension / viewer page. The
// prediction id is bound to the resolved overlay inside Wager (a foreign id reports
// "not_found"); the balance debited is the resolved overlay's economy.
func (h *Handler) StreamerWager(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := h.resolveStreamerOverlay(c)
	if !ok {
		return
	}
	var req streamerWagerReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OutcomeIdx < 1 || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prediction_id, outcome_idx (>=1) and positive amount required"})
		return
	}
	predID, err := uuid.Parse(req.PredictionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prediction_id must be a valid uuid"})
		return
	}
	platform := c.GetString("platform")
	res, err := h.repo.Wager(c.Request.Context(), predID, viewerID, overlayID, req.OutcomeIdx, req.Amount, platform, nil)
	if err != nil {
		h.log.Error("streamer wager", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not place wager"})
		return
	}
	if !res.Accepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wager not accepted", "reason": res.Reason})
		return
	}
	if pred, err := h.repo.GetPrediction(c.Request.Context(), predID); err == nil {
		h.pub.PublishPrediction(c.Request.Context(), pred)
		c.JSON(http.StatusOK, gin.H{"balance": res.NewBalance, "prediction": pred})
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": res.NewBalance})
}

// StreamerHeartbeat (viewer) awards watch-time points on the resolved overlay, deduped
// once per minute-bucket per (viewer, overlay). Streamer-keyed sibling of
// POST /viewers/me/heartbeat.
func (h *Handler) StreamerHeartbeat(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := h.resolveStreamerOverlay(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	cfg, err := h.repo.GetEarnConfig(ctx, overlayID)
	if err != nil {
		h.log.Error("streamer heartbeat config", zap.Error(err))
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
