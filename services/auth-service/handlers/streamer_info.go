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
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// streamerInfoDB is a minimal interface over *pgxpool.Pool so the lookup
// branches can be unit-tested without a live database.
type streamerInfoDB interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

// StreamerInfoHandler handles requests for streamer information
type StreamerInfoHandler struct {
	log      *zap.Logger
	userRepo UserRepositoryInterface
	db       streamerInfoDB
}

// NewStreamerInfoHandler creates a new streamer info handler
func NewStreamerInfoHandler(log *zap.Logger, userRepo UserRepositoryInterface, db streamerInfoDB) *StreamerInfoHandler {
	return &StreamerInfoHandler{
		log:      log.Named("streamer-info"),
		userRepo: userRepo,
		db:       db,
	}
}

// PlatformInfo represents information about a platform for a streamer
type PlatformInfo struct {
	Platform    string `json:"platform"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	IsActive    bool   `json:"is_active"`
}

// StreamerInfoResponse is the response for streamer info
// Note: overlay_id is intentionally NOT included to prevent viewers from
// accessing the secret overlay ID or triggering YouTube listener polling
type StreamerInfoResponse struct {
	Username    string         `json:"username"`
	DisplayName string         `json:"display_name,omitempty"`
	Platforms   []PlatformInfo `json:"platforms"`

	// ViewerPublic reports whether a viewer WebSocket connection to this
	// streamer would be accepted — i.e. whether the api-gateway's
	// GetPublicOverlayByUsername would resolve an overlay for them.
	//
	// Why this exists: the gateway rejects a non-public streamer *before* the
	// WebSocket upgrade, so the browser only ever sees close code 1006 with an
	// empty reason — indistinguishable from DNS failure, a cold-start proxy or
	// a gateway rolling mid-connect. Clients used to guess from "1006 on the
	// first attempt", which silenced them on any transient blip. This flag lets
	// them ask the question over HTTP instead and only stop retrying on an
	// explicit false.
	//
	// It reveals nothing the viewer would not learn a moment later by
	// attempting the connection, and in particular exposes neither the overlay
	// ID nor anything that triggers listener polling, so the constraint in the
	// note above is preserved.
	//
	// Name is a cross-repo contract (all-chat-extension reads `viewer_public`);
	// deliberately not `omitempty` so `false` is always on the wire.
	ViewerPublic bool `json:"viewer_public"`
}

// viewerPublicQuery mirrors the api-gateway's GetPublicOverlayByUsername
// (services/api-gateway/subscription/repository.go) predicate exactly, so the
// flag this handler reports and the decision the gateway actually makes on a
// viewer WebSocket upgrade cannot disagree. It selects existence only — the
// overlay ID never leaves the database.
const viewerPublicQuery = `
	SELECT EXISTS (
		SELECT 1
		FROM overlays o
		JOIN users u ON o.user_id = u.id
		WHERE u.id = $1
		  AND o.is_active = true
		  AND o.is_public_for_viewers = true
		  AND u.is_banned = false
	)
`

// hasPublicOverlay answers "would the gateway accept a viewer connection for
// this user?". A query failure resolves to false rather than failing the whole
// request: the platform list is the primary payload, and a client that reads a
// missing/false flag from a degraded response must treat it as a transport
// failure and keep retrying (never as a policy "stop").
func (h *StreamerInfoHandler) hasPublicOverlay(ctx context.Context, userID string) bool {
	var viewerPublic bool
	if err := h.db.QueryRow(ctx, viewerPublicQuery, userID).Scan(&viewerPublic); err != nil {
		h.log.Warn("Failed to resolve viewer_public flag; reporting false",
			zap.Error(err),
			zap.String("user_id", userID))
		return false
	}
	return viewerPublic
}

// HandleGetStreamerInfo returns information about a streamer and their active platforms
// Supports lookup by:
// - Username (case-insensitive) - works for Twitch, Kick, TikTok
// - Channel handle/ID - works for YouTube (@handle or channel ID)
func (h *StreamerInfoHandler) HandleGetStreamerInfo(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username is required"})
		return
	}

	ctx := c.Request.Context()

	// Try Method 1: Get user by username (case-insensitive)
	user, err := h.userRepo.GetByUsername(ctx, username)

	// If not found by username, try Method 2: Look up by channel_id in overlay_chat_sources
	// This handles YouTube handles/channel IDs which aren't stored as usernames
	if err != nil {
		h.log.Debug("User not found by username, trying channel_id lookup",
			zap.String("username", username))

		// Query to find user by channel_id OR channel_handle (case-insensitive)
		// Note: Don't filter by is_active - we want to find streamers who have
		// All-Chat configured even if they're not currently live
		channelQuery := `
			SELECT DISTINCT u.id, u.username
			FROM users u
			INNER JOIN overlays o ON o.user_id = u.id
			INNER JOIN overlay_chat_sources ocs ON ocs.overlay_id = o.id
			WHERE (
				LOWER(ocs.channel_id) = LOWER($1)
				OR LOWER(LTRIM(ocs.channel_handle, '@')) = LOWER(LTRIM($1, '@'))
			)
			LIMIT 1
		`

		var userID, foundUsername string
		err = h.db.QueryRow(ctx, channelQuery, username).Scan(&userID, &foundUsername)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Expected: a viewer/extension looked up a streamer or channel
				// that isn't configured in All-Chat. This is a normal 404, not an
				// error — log at debug to avoid flooding error logs (and the
				// stacktrace the production logger attaches at error level).
				h.log.Debug("Streamer not found by username or channel_id",
					zap.String("lookup_value", username))
				c.JSON(http.StatusNotFound, gin.H{"error": "streamer not found"})
				return
			}
			// Unexpected database error — surface as 500 rather than a misleading 404.
			h.log.Error("Channel_id lookup query failed",
				zap.Error(err),
				zap.String("lookup_value", username))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up streamer"})
			return
		}

		// Now get the full user object
		user, err = h.userRepo.GetByUsername(ctx, foundUsername)
		if err != nil {
			h.log.Error("Failed to get user after channel_id lookup", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch user"})
			return
		}
	}

	// Query all configured sources from the PUBLIC overlay only
	// Note: Return all configured sources regardless of is_active status
	// so the extension can show the badge even when the streamer isn't live
	query := `
		SELECT DISTINCT
			ocs.platform,
			ocs.channel_id,
			ocs.channel_name,
			ocs.is_active
		FROM overlay_chat_sources ocs
		INNER JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.user_id = $1 AND o.is_public_for_viewers = true
		ORDER BY ocs.platform
	`

	rows, err := h.db.Query(ctx, query, user.ID)
	if err != nil {
		h.log.Error("Failed to query active sources", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch platform info"})
		return
	}
	defer rows.Close()

	platforms := make([]PlatformInfo, 0)
	for rows.Next() {
		var p PlatformInfo
		if err := rows.Scan(&p.Platform, &p.ChannelID, &p.ChannelName, &p.IsActive); err != nil {
			h.log.Error("Failed to scan platform info", zap.Error(err))
			continue
		}
		platforms = append(platforms, p)
	}

	// If no public overlay sources found, return 404 so the extension
	// knows not to inject UI. A user with no public overlay is effectively
	// "not found" from the viewer's perspective.
	if len(platforms) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "streamer not found"})
		return
	}

	// Return response (overlay_id intentionally excluded for security)
	response := StreamerInfoResponse{
		Username:     user.Username,
		DisplayName:  user.Username,
		Platforms:    platforms,
		ViewerPublic: h.hasPublicOverlay(ctx, user.ID),
	}

	c.JSON(http.StatusOK, response)
}
