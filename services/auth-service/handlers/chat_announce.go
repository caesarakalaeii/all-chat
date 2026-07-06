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
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// InternalAnnounceRequest is the body of POST /internal/chat/announce. Called
// service-to-service (service JWT) by engagement-service to post an opt-in round-start
// message to chat AS the overlay owner (ADR-0027, H4-2). user_id is the resolved
// overlay owner; platforms is the overlay's sendable source platforms.
type InternalAnnounceRequest struct {
	UserID    string   `json:"user_id" binding:"required"`
	Message   string   `json:"message" binding:"required"`
	Platforms []string `json:"platforms" binding:"required"`
}

// HandleInternalAnnounce posts a round-start announcement to each requested platform
// the streamer is connected on, reusing the tested streamer-send path (and its scope,
// quota, and echo-dedup handling). Best-effort per platform: a platform that fails
// (missing scope, not live, quota) is reported in results but never fails the others.
// This endpoint takes the target user from the (service-authenticated) body rather
// than a user JWT — it is reachable only over the internal service route.
func (h *ChatSendHandler) HandleInternalAnnounce(c *gin.Context) {
	var req InternalAnnounceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, message and platforms are required"})
		return
	}
	if len(req.Message) == 0 || len(req.Message) > maxMessageLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("message must be between 1 and %d characters", maxMessageLength)})
		return
	}

	ctx := c.Request.Context()
	user, err := h.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		h.log.Warn("announce: failed to load user", zap.Error(err), zap.String("user_id", req.UserID))
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.IsBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "account banned"})
		return
	}
	if err := h.refreshStreamerTokenIfNeeded(ctx, user); err != nil {
		h.log.Warn("announce: token refresh failed", zap.Error(err), zap.String("user_id", req.UserID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	// Intersect the requested platforms with the ones the streamer actually has a
	// sendable identity for. senderID is the platform-native author id (for echo dedup).
	requested := map[string]bool{}
	for _, p := range req.Platforms {
		requested[strings.ToLower(p)] = true
	}
	idsByPlatform := map[string]string{}
	if requested["twitch"] && user.TwitchID != nil && *user.TwitchID != "" {
		idsByPlatform["twitch"] = *user.TwitchID
	}
	if requested["kick"] && user.KickID != nil && *user.KickID != "" {
		idsByPlatform["kick"] = *user.KickID
	}
	if requested["youtube"] {
		if ytChannelID, ytErr := h.getActiveYouTubeChannelID(ctx, user.ID); ytErr == nil && ytChannelID != "" {
			idsByPlatform["youtube"] = ytChannelID
		}
	}
	if len(idsByPlatform) == 0 {
		// No requested platform is connected/sendable — a no-op for a best-effort announce.
		c.JSON(http.StatusOK, gin.H{"success": false, "results": []streamerSendResult{}})
		return
	}

	// Rate-limit against the streamer's own send budget (one unit per platform).
	if ok, retryAfter := h.reserveStreamerRate(ctx, user.ID, len(idsByPlatform)); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "retry_after_seconds": retryAfter})
		return
	}

	// Deterministic order so the collapsed echo pill is stable.
	platforms := make([]string, 0, len(idsByPlatform))
	for _, p := range []string{"twitch", "youtube", "kick"} {
		if _, ok := idsByPlatform[p]; ok {
			platforms = append(platforms, p)
		}
	}

	// Pre-register the echoes for a multi-platform announce so the message-processor
	// collapses the streamer's own echoed announcement into one overlay message,
	// reconciled below to the platforms that actually succeeded (see handleStreamerSendToAll).
	groupID := ""
	if len(platforms) >= 2 {
		groupID = uuid.NewString()
		h.writeSendAllKeys(ctx, idsByPlatform, req.Message, platforms, groupID)
	}

	results := make([]streamerSendResult, 0, len(platforms))
	anySuccess := false
	for _, platform := range platforms {
		var sendErr error
		switch platform {
		case "twitch":
			sendErr = h.sendStreamerTwitchMessage(ctx, user, req.Message)
		case "kick":
			sendErr = h.sendStreamerKickMessage(ctx, user, req.Message)
		case "youtube":
			sendErr = h.sendStreamerYouTubeMessage(ctx, user, req.Message)
		}
		if sendErr != nil {
			kind := sendResultErrorKind(sendErr)
			results = append(results, streamerSendResult{Platform: platform, Success: false, Error: string(kind), ErrorKind: string(kind)})
			h.log.Warn("announce platform failed", zap.String("platform", platform), zap.String("error_kind", string(kind)), zap.Error(sendErr))
		} else {
			anySuccess = true
			results = append(results, streamerSendResult{Platform: platform, Success: true})
		}
	}

	if groupID != "" {
		switch d := decideSendAllPill(platforms, results); d.action {
		case sendAllRewrite:
			h.writeSendAllKeys(ctx, idsByPlatform, req.Message, d.platforms, groupID)
		case sendAllDelete:
			h.deleteSendAllKeys(ctx, idsByPlatform, req.Message)
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": anySuccess, "results": results})
}
