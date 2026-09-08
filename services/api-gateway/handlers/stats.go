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
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/sessions"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StatsHandler serves public platform statistics.
type StatsHandler struct {
	redis  *redis.Client
	logger *zap.Logger

	// In-process memo for the connected-overlay count. The /stats endpoint is
	// unauthenticated and hit by every landing-page visit, while the SCAN it
	// needs is O(keyspace); a short TTL bounds both costs without making the
	// number meaningfully stale ("live right now" survives 30s of lag).
	overlayCount     int64
	overlayCountAt   time.Time
	overlayCountTTL  time.Duration
	overlayCountLock sync.Mutex
}

// NewStatsHandler creates a StatsHandler.
func NewStatsHandler(redis *redis.Client, logger *zap.Logger) *StatsHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StatsHandler{redis: redis, logger: logger, overlayCountTTL: 30 * time.Second}
}

// PlatformStats is the public /stats response: per-platform message volumes
// for the last 7 days plus the landing-page counters.
type PlatformStats struct {
	Platforms map[string]int64 `json:"platforms"`
	AllTime   int64            `json:"all_time"`
	Users     int64            `json:"users"`
	// OverlaysLive is the number of overlays with an active WebSocket
	// connection right now (memoized briefly, see StatsHandler).
	OverlaysLive int64 `json:"overlays_live"`
}

// GetPlatformStats returns message counts per platform for the last 7 days
// plus the all-time, user and live-overlay counters.
// GET /api/v1/stats — public, no auth required.
func (h *StatsHandler) GetPlatformStats(c *gin.Context) {
	ctx := c.Request.Context()

	platforms := []string{"twitch", "youtube", "kick", "tiktok", "discord"}
	result := make(map[string]int64, len(platforms))

	// Build list of the last 7 daily bucket suffixes (YYYY-MM-DD).
	now := time.Now().UTC()
	days := make([]string, 7)
	for i := range days {
		days[i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	for _, platform := range platforms {
		var total int64
		for _, day := range days {
			val, err := h.redis.Get(ctx, "chat:stats:daily:"+platform+":"+day).Int64()
			if err == nil {
				total += val
			}
		}
		result[platform] = total
	}

	stats := PlatformStats{
		Platforms:    result,
		AllTime:      h.redisGetInt(ctx, "chat:stats:total"),
		Users:        h.redisGetInt(ctx, "stats:users:total"),
		OverlaysLive: h.connectedOverlayCount(ctx),
	}

	c.JSON(http.StatusOK, stats)
}

// redisGetInt reads a counter key, treating missing and unreadable alike as 0:
// these are decorative landing-page numbers, and a Redis blip must not 500 the
// page over them.
func (h *StatsHandler) redisGetInt(ctx context.Context, key string) int64 {
	val, err := h.redis.Get(ctx, key).Int64()
	if err != nil {
		return 0
	}
	return val
}

// connectedOverlayCount returns the number of overlays with an active
// WebSocket connection, memoized for overlayCountTTL.
func (h *StatsHandler) connectedOverlayCount(ctx context.Context) int64 {
	h.overlayCountLock.Lock()
	defer h.overlayCountLock.Unlock()

	if time.Since(h.overlayCountAt) < h.overlayCountTTL {
		return h.overlayCount
	}

	ids, err := h.scanActiveOverlayIDs(ctx)
	if err != nil {
		// Keep the last good value on failure; before the first success that
		// is 0, which is honest ("we could not count") rather than alarming.
		h.logger.Warn("failed to scan connected overlays; keeping last count", zap.Error(err))
		return h.overlayCount
	}
	h.overlayCount = int64(len(ids))
	h.overlayCountAt = time.Now()
	return h.overlayCount
}

// ActiveOverlay describes an overlay with a live WebSocket connection.
type ActiveOverlay struct {
	OverlayID string `json:"overlay_id"`
	// ConnectedSince is when the current connection session began (RFC3339),
	// used to show admins how long an overlay has been open. Nil when no session
	// timestamp is available (e.g. the connection is in its post-disconnect
	// linger window and the session has already ended).
	ConnectedSince *time.Time `json:"connected_since,omitempty"`
}

// GetActiveOverlays returns the overlays with active WebSocket connections,
// each annotated with when its connection began so admins can spot overlays
// that are open but whose streamer is no longer live ("dead but open").
// GET /api/v1/admin/overlays/active — requires admin auth.
func (h *StatsHandler) GetActiveOverlays(c *gin.Context) {
	ctx := c.Request.Context()

	activeIDs, err := h.scanActiveOverlayIDs(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan active overlays"})
		return
	}

	overlays := make([]ActiveOverlay, 0, len(activeIDs))
	if len(activeIDs) == 0 {
		c.JSON(http.StatusOK, overlays)
		return
	}

	// Fetch each overlay's session start time in one round trip. started_at
	// lives in the session:active:{id} hash written on connect (kept alive by
	// the connection heartbeat), so no DB access is needed here.
	pipe := h.redis.Pipeline()
	startedCmds := make([]*redis.StringCmd, len(activeIDs))
	for i, id := range activeIDs {
		startedCmds[i] = pipe.HGet(ctx, sessions.SessionKeyPrefix+id, "started_at")
	}
	// started_at is an OPTIONAL enrichment ("connected for X"); the overlay:connected
	// SCAN above is the authoritative "which overlays are live" answer. A failure here
	// must NOT discard that answer — returning 500 made the admin UI (which silently
	// ignores a non-OK response) render every overlay as "not connected" even while
	// live, whenever this pipeline hit a transient blip or a single stray/mistyped
	// session key. Degrade instead: log and return connection status without durations.
	// redis.Nil merely means some session hashes are missing (expected, handled
	// per-command below); on any other error each command carries that error, so its
	// Result() call below fails and connected_since is simply omitted.
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		h.logger.Warn("failed to load overlay session start times; returning connection status without durations",
			zap.Error(err),
		)
	}

	for i, id := range activeIDs {
		overlay := ActiveOverlay{OverlayID: id}
		if startedStr, err := startedCmds[i].Result(); err == nil && startedStr != "" {
			if started, perr := time.Parse(time.RFC3339, startedStr); perr == nil {
				overlay.ConnectedSince = &started
			}
		}
		overlays = append(overlays, overlay)
	}

	c.JSON(http.StatusOK, overlays)
}

// scanActiveOverlayIDs returns the de-duplicated overlay IDs that currently
// have an overlay:connected:* key (i.e. a live WebSocket connection).
func (h *StatsHandler) scanActiveOverlayIDs(ctx context.Context) ([]string, error) {
	// SCAN guarantees full coverage but not uniqueness (a key can repeat across
	// batches during rehashing), so de-duplicate to avoid duplicate rows and
	// redundant lookups.
	var activeIDs []string
	seen := make(map[string]struct{})
	var cursor uint64
	for {
		keys, next, err := h.redis.Scan(ctx, cursor, "overlay:connected:*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			// key format: "overlay:connected:{id}"
			id := key[len("overlay:connected:"):]
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			activeIDs = append(activeIDs, id)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return activeIDs, nil
}
