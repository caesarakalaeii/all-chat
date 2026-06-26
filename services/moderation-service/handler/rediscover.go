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
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RediscoverPublisher triggers a YouTube live-stream re-discovery for a channel.
// It returns published=false (without error) when the channel is within its
// cooldown window, so the handler can answer 429 instead of churning the stream.
type RediscoverPublisher interface {
	Publish(ctx context.Context, overlayID, channelID string) (bool, error)
}

// SetRediscoverPublisher wires the YouTube rediscovery publisher. Optional; when
// unset, HandleYouTubeRediscover reports the feature unavailable (503).
func (h *Handler) SetRediscoverPublisher(p RediscoverPublisher) { h.rediscover = p }

// HandleYouTubeRediscover forces the youtube-listener to re-discover the live
// stream for every YouTube source on the overlay. It recovers the "platform shows
// connected but no chat" case where YouTube keeps reporting an ended/crashed stream
// as live and the listener stays pinned to the dead video.
//
// Owner-only — but, unlike delete/timeout/ban, available to ANY overlay owner
// (it is a reliability recovery, not a moderation action, so it is not behind the
// premium moderation cohort gate). Ownership is verified here; the route carries no
// premium middleware. The body is ignored: the channel(s) are resolved server-side.
func (h *Handler) HandleYouTubeRediscover(c *gin.Context) {
	cl := newCaller(c)
	if cl.userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	if h.rediscover == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "rediscovery unavailable"})
		return
	}

	ctx := c.Request.Context()
	owns, err := h.repo.VerifyOverlayOwnership(ctx, cl.overlayID, cl.userID)
	if err != nil {
		h.logger.Error("rediscover ownership check failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !owns {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for this overlay"})
		return
	}

	sources, err := h.repo.ListModeratableSources(ctx, cl.overlayID)
	if err != nil {
		h.logger.Error("rediscover source lookup failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	var youtube, triggered, cooled int
	for _, s := range sources {
		if s.Platform != "youtube" {
			continue
		}
		youtube++
		published, perr := h.rediscover.Publish(ctx, cl.overlayID, s.ChannelID)
		if perr != nil {
			h.logger.Warn("failed to publish rediscover command",
				zap.String("channel_id", s.ChannelID), zap.Error(perr))
			continue
		}
		if published {
			triggered++
		} else {
			cooled++
		}
	}

	switch {
	case youtube == 0:
		c.JSON(http.StatusBadRequest, gin.H{"error": "no youtube source on this overlay"})
	case triggered == 0 && cooled > 0:
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "please wait a moment before retrying"})
	case triggered == 0:
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not trigger rediscovery"})
	default:
		h.logger.Info("YouTube rediscovery triggered",
			zap.String("overlay_id", cl.overlayID),
			zap.String("user_id", cl.userID),
			zap.Int("sources", triggered))
		c.JSON(http.StatusOK, gin.H{"status": "ok", "rediscovered": triggered})
	}
}
