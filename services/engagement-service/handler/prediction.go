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

type createPredictionReq struct {
	Title       string   `json:"title"`
	Outcomes    []string `json:"outcomes"`
	AutoLockSec int      `json:"auto_lock_seconds"`
}

// CreatePrediction (owner) opens an All-Chat-native prediction.
func (h *Handler) CreatePrediction(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	var req createPredictionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Title == "" || len(req.Outcomes) < 2 || len(req.Outcomes) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prediction needs a title and 2–10 outcomes"})
		return
	}
	// Twitch precedence: don't open an All-Chat prediction while a mirrored Twitch
	// prediction is live on this overlay.
	if live, err := h.repo.HasLiveNativePrediction(c.Request.Context(), overlayID); err != nil {
		h.log.Error("check live native prediction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create prediction"})
		return
	} else if live {
		c.JSON(http.StatusConflict, gin.H{"error": "a Twitch prediction is currently live on this overlay"})
		return
	}
	var autoLockAt *time.Time
	if req.AutoLockSec > 0 {
		t := time.Now().Add(time.Duration(req.AutoLockSec) * time.Second)
		autoLockAt = &t
	}
	pred, err := h.repo.CreatePrediction(c.Request.Context(), overlayID, req.Title, req.Outcomes, autoLockAt)
	if err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "an active prediction already exists for this overlay"})
			return
		}
		h.log.Error("create prediction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create prediction"})
		return
	}
	h.markActive(c, overlayID, pred.ID)
	h.pub.PublishPrediction(c.Request.Context(), pred)
	h.announcer.AnnouncePrediction(overlayID, pred) // opt-in chat announce (H4-2); best-effort, non-blocking
	c.JSON(http.StatusCreated, pred)
}

// LockPrediction (owner) stops accepting wagers and clears the active flag.
func (h *Handler) LockPrediction(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	pid, ok := uuidParam(c, "pid")
	if !ok {
		return
	}
	pred, err := h.repo.LockPrediction(c.Request.Context(), pid, overlayID)
	if err != nil {
		h.log.Error("lock prediction", zap.Error(err))
		c.JSON(statusForRepoErr(err), gin.H{"error": "could not lock prediction"})
		return
	}
	h.clearActive(c, pred.ID) // locked → no more wagers accepted
	h.pub.PublishPrediction(c.Request.Context(), pred)
	c.JSON(http.StatusOK, pred)
}

type resolveReq struct {
	WinningOutcomeID string `json:"winning_outcome_id"`
}

// ResolvePrediction (owner) settles a locked prediction and pays out winners.
func (h *Handler) ResolvePrediction(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	pid, ok := uuidParam(c, "pid")
	if !ok {
		return
	}
	var req resolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	winning, err := uuid.Parse(req.WinningOutcomeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "winning_outcome_id must be a valid uuid"})
		return
	}
	pred, err := h.repo.ResolvePrediction(c.Request.Context(), pid, winning, overlayID)
	if err != nil {
		h.log.Error("resolve prediction", zap.Error(err))
		c.JSON(statusForRepoErr(err), gin.H{"error": "could not resolve prediction"})
		return
	}
	h.clearActive(c, pred.ID)
	h.pub.PublishPrediction(c.Request.Context(), pred)
	c.JSON(http.StatusOK, pred)
}

// CancelPrediction (owner) voids a prediction and refunds all stakes.
func (h *Handler) CancelPrediction(c *gin.Context) {
	userID, ok := requireUser(c)
	if !ok {
		return
	}
	overlayID, ok := h.requireOwnedOverlay(c, userID)
	if !ok {
		return
	}
	pid, ok := uuidParam(c, "pid")
	if !ok {
		return
	}
	pred, err := h.repo.CancelPrediction(c.Request.Context(), pid, overlayID)
	if err != nil {
		h.log.Error("cancel prediction", zap.Error(err))
		c.JSON(statusForRepoErr(err), gin.H{"error": "could not cancel prediction"})
		return
	}
	h.clearActive(c, pred.ID)
	h.pub.PublishPrediction(c.Request.Context(), pred)
	c.JSON(http.StatusOK, pred)
}

// GetActivePrediction (public) returns the overlay's live prediction.
//
// This is a public UNAUTHENTICATED render seam keyed by the overlay UUID. The
// OBS/render path holds no token, so we cannot distinguish the owner from a viewer
// and therefore do NOT apply the is_public_for_viewers gate here — gating would black
// out the streamer's OWN OBS widget the moment they set is_public_for_viewers=false
// (which defaults to true). The overlay UUID is itself a bearer capability that
// auth-service withholds from viewers, so this discloses only the live title + tally
// to a holder of the private overlay id — not a cross-tenant leak. The
// is_public_for_viewers gate IS enforced on every viewer-authored surface
// (vote/wager/balance/heartbeat via requireViewerParticipation) and on the
// username-keyed GetStreamerActive read. See PR #524 review finding #3.
func (h *Handler) GetActivePrediction(c *gin.Context) {
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return
	}
	pred, err := h.repo.GetActiveDisplayPrediction(c.Request.Context(), overlayID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "no active prediction"})
			return
		}
		h.log.Error("get active prediction", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not load prediction"})
		return
	}
	c.JSON(http.StatusOK, pred)
}

type wagerReq struct {
	OutcomeIdx int   `json:"outcome_idx"`
	Amount     int64 `json:"amount"`
}

// WebWager (viewer) places a points wager from the web page / extension. The
// overlay :id path segment scopes which economy the balance is debited from.
func (h *Handler) WebWager(c *gin.Context) {
	viewerID, ok := requireViewer(c)
	if !ok {
		return
	}
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return
	}
	pid, ok := uuidParam(c, "pid")
	if !ok {
		return
	}
	if !h.requireViewerParticipation(c, overlayID) {
		return
	}
	var req wagerReq
	if err := c.ShouldBindJSON(&req); err != nil || req.OutcomeIdx < 1 || req.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "outcome_idx (>=1) and positive amount required"})
		return
	}
	platform := c.GetString("platform")
	res, err := h.repo.Wager(c.Request.Context(), pid, viewerID, overlayID, req.OutcomeIdx, req.Amount, platform, nil, "")
	if err != nil {
		h.log.Error("web wager", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not place wager"})
		return
	}
	if !res.Accepted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wager not accepted", "reason": res.Reason})
		return
	}
	if pred, err := h.repo.GetPrediction(c.Request.Context(), pid); err == nil {
		h.pub.PublishPrediction(c.Request.Context(), pred)
		c.JSON(http.StatusOK, gin.H{"balance": res.NewBalance, "prediction": pred})
		return
	}
	c.JSON(http.StatusOK, gin.H{"balance": res.NewBalance})
}
