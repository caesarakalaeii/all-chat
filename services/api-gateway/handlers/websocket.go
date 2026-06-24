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
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/replay"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/auth"
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// parseSinceQuery parses the ?since=<ms-epoch> query parameter. Returns 0 if
// the value is missing, empty, malformed, or non-positive — meaning "replay
// the entire buffer".
func parseSinceQuery(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// wsBearerPrefix is the subprotocol prefix used to carry a JWT over the
// WebSocket handshake (audit H5). Clients pass ['bearer.<token>'] as the
// WebSocket subprotocol; the gateway extracts the token and echoes the
// subprotocol back so the browser accepts the connection. This keeps the
// token out of the URL query string (and therefore out of access logs).
const wsBearerPrefix = "bearer."

// extractWSAuthToken resolves the JWT with subprotocol-first precedence
// (audit H5):
//  1. Sec-WebSocket-Protocol subprotocol of the form `bearer.<token>`
//  2. Fallback to ?token= query param (backward compat during client rollout)
//  3. Fallback to the httpOnly access_token cookie (audit H3). The owner
//     overlay WS handshake is same-origin, so the browser sends the httpOnly
//     cookie automatically — lets the streamer's monitor view authenticate
//     without a JS-readable token. No echo header needed (cookie path, not
//     subprotocol negotiation).
//
// When the token comes from the subprotocol, a response header is returned
// that echoes the negotiated subprotocol back to the client — the WebSocket
// spec requires the server to echo one of the offered subprotocols.
func extractWSAuthToken(r *http.Request) (token string, echoHeader http.Header) {
	// 1. Try Sec-WebSocket-Protocol subprotocol.
	for _, raw := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, proto := range strings.Split(raw, ",") {
			proto = strings.TrimSpace(proto)
			if strings.HasPrefix(proto, wsBearerPrefix) {
				candidate := strings.TrimPrefix(proto, wsBearerPrefix)
				if candidate == "" {
					continue
				}
				token = candidate
				echoHeader = http.Header{}
				echoHeader.Set("Sec-WebSocket-Protocol", proto)
				return token, echoHeader
			}
		}
	}
	// 2. Fall back to query param (backward compat during client rollout).
	token = r.URL.Query().Get("token")
	if token != "" {
		return token, nil
	}
	// 3. Fall back to the httpOnly access_token cookie (audit H3). The owner
	// overlay WS handshake is same-origin, so the browser sends the httpOnly
	// cookie automatically — lets the streamer's monitor view authenticate
	// without a JS-readable token. No echo header needed (cookie path, not
	// subprotocol negotiation).
	if ck, err := r.Cookie(auth.CookieAccessToken); err == nil && ck.Value != "" {
		return ck.Value, nil
	}
	return "", nil
}

// WebSocketHandler handles WebSocket connections for overlays
type WebSocketHandler struct {
	wsManager         *wsconn.Manager
	subscriber        *subscription.Subscriber
	repo              *subscription.Repository
	statusSubscriber  *subscription.StatusSubscriber
	userKeyChain      *auth.KeyChain
	replayBuffer      replay.DeletionReplayBuffer
	chatReplayBuffer  replay.ChatReplayBuffer
	logger            *zap.Logger
	upgrader          websocket.Upgrader
	allowedOriginList []string
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	wsManager *wsconn.Manager,
	subscriber *subscription.Subscriber,
	repo *subscription.Repository,
	statusSubscriber *subscription.StatusSubscriber,
	userKeyChain *auth.KeyChain,
	replayBuffer replay.DeletionReplayBuffer,
	chatReplayBuffer replay.ChatReplayBuffer,
	logger *zap.Logger,
) *WebSocketHandler {
	allowedOriginList := loadAllowedOrigins()
	h := &WebSocketHandler{
		wsManager:         wsManager,
		subscriber:        subscriber,
		repo:              repo,
		statusSubscriber:  statusSubscriber,
		userKeyChain:      userKeyChain,
		replayBuffer:      replayBuffer,
		chatReplayBuffer:  chatReplayBuffer,
		logger:            logger,
		allowedOriginList: allowedOriginList,
	}

	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	if sharedmiddleware.OriginAllowed(allowedOriginList, "*") {
		logger.Info("WebSocket origin allowlist disabled; allowing all origins")
	} else {
		logger.Info("Configured WebSocket origin allowlist",
			zap.Int("count", len(allowedOriginList)),
		)
	}

	return h
}

// HandleOverlayConnection handles WebSocket connection requests for overlays
func (h *WebSocketHandler) HandleOverlayConnection(c *gin.Context) {
	overlayID := c.Param("overlay_id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id is required"})
		return
	}

	// Resolve JWT from Sec-WebSocket-Protocol subprotocol first, then fall
	// back to ?token= query param (backward compat during client rollout).
	// The subprotocol path keeps the token out of access logs (audit H5).
	token, echoHeader := extractWSAuthToken(c.Request)
	var userID string
	var username string

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// If token provided, validate and check ownership
	if token != "" {
		claims, err := auth.ValidateJWTWithKeyChain(token, h.userKeyChain)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		userID = claims.UserID
		username = claims.Username

		// Verify user owns the overlay
		owns, err := h.repo.VerifyOverlayOwnership(ctx, overlayID, claims.UserID)
		if err != nil {
			h.logger.Error("Failed to verify overlay ownership",
				zap.String("overlay_id", overlayID),
				zap.String("user_id", claims.UserID),
				zap.Error(err),
			)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify ownership"})
			return
		}

		if !owns {
			c.JSON(http.StatusForbidden, gin.H{"error": "you do not own this overlay"})
			return
		}
	} else {
		// No token provided (OBS mode) - use anonymous connection
		userID = "obs"
		username = "OBS"
	}

	// Check if overlay is active
	isActive, err := h.repo.IsOverlayActive(ctx, overlayID)
	if err != nil {
		h.logger.Error("Failed to check overlay status",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check overlay status"})
		return
	}

	if !isActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "overlay is not active"})
		return
	}

	// Upgrade HTTP connection to WebSocket. echoHeader (when non-nil) echoes
	// the bearer.<token> subprotocol back so the browser accepts the handshake.
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, echoHeader)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		return
	}

	// Create WebSocket connection wrapper
	wsConn := wsconn.NewConnection(conn, overlayID, userID, h.replayBuffer, h.logger)

	// Activate all sources for this overlay (auto-activation on connect)
	activatedCount, err := h.repo.ActivateSourcesForOverlay(ctx, overlayID)
	if err != nil {
		h.logger.Error("Failed to activate sources for overlay",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		// Don't fail the connection - continue even if activation fails
	} else if activatedCount > 0 {
		h.logger.Info("Auto-activated sources for overlay",
			zap.String("overlay_id", overlayID),
			zap.Int("count", activatedCount),
		)
	}

	// Subscribe to overlay's Redis Pub/Sub channel
	// Use background context - subscription must outlive the HTTP request
	if err := h.subscriber.Subscribe(context.Background(), overlayID); err != nil {
		h.logger.Error("Failed to subscribe to overlay channel",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to subscribe"})
		return
	}

	// Add connection to manager
	// Use background context here too since connection lives beyond HTTP request
	h.wsManager.AddConnection(context.Background(), wsConn)

	// Start the read/write pumps BEFORE the initial burst below. The writePump
	// is what drains the send channel; if we enqueue the connect burst
	// (connected frame + status snapshot + replay, up to ~500 messages) before
	// it runs, the burst overflows the 256-slot channel and self-closes the
	// socket mid-replay — which made busy overlays flap.
	// Use background context, not the HTTP request context (which is cancelled
	// when this handler returns).
	wsCtx := context.Background()
	wsConn.Start(wsCtx)

	// Send connected message. The burst uses SendBlocking (backpressure) rather
	// than Send (drop-and-close) so a large replay can't tear down the socket.
	connectedMsg := models.NewConnected(overlayID)
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.SendBlocking(connectedJSON)

	// Send status snapshot for all configured sources so indicators are populated immediately.
	// Without this, indicators remain blank until the next status-change event fires.
	if h.statusSubscriber != nil {
		sources, err := h.repo.GetOverlaySources(context.Background(), overlayID)
		if err != nil {
			h.logger.Warn("Failed to get overlay sources for status snapshot",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		} else {
			for _, src := range sources {
				if statusData, ok := h.statusSubscriber.GetPlatformStatus(src.Platform, src.ChannelID); ok {
					wsMsg := models.NewPlatformStatus(*statusData)
					if msgJSON, err := wsMsg.ToJSON(); err == nil {
						wsConn.SendBlocking(msgJSON)
					}
				}
			}
		}
	}

	// Replay any chat messages buffered while no WebSocket was connected.
	// Clients can pass ?since=<ms-epoch> to skip messages they already saw
	// (useful for resilient reconnect logic on the client side).
	if h.chatReplayBuffer != nil {
		sinceMs := parseSinceQuery(c.Query("since"))
		replayed, err := h.chatReplayBuffer.GetSince(context.Background(), overlayID, sinceMs)
		if err != nil {
			h.logger.Warn("Failed to fetch chat replay buffer",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		} else if len(replayed) > 0 {
			h.logger.Info("Replaying buffered messages to reconnected client",
				zap.String("overlay_id", overlayID),
				zap.String("user_id", userID),
				zap.Int("message_count", len(replayed)),
				zap.Int64("since_ms", sinceMs),
			)
			for _, payload := range replayed {
				if !wsConn.SendBlocking(payload) {
					break // client gone or too slow; stop replaying
				}
			}
		}
	}

	h.logger.Info("WebSocket connection established",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", userID),
		zap.String("username", username),
	)

	// Set up cleanup callback when connection closes
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in WebSocket cleanup",
					zap.String("overlay_id", overlayID),
					zap.String("user_id", userID),
					zap.Any("panic", r),
				)
			}
			// Always clean up
			h.wsManager.RemoveConnection(wsConn)
			h.subscriber.Unsubscribe(context.Background(), overlayID)

			// Sources are NOT deactivated on disconnect here.
			// With multiple api-gateway replicas, HasPool is per-pod so a disconnect
			// on pod A with reconnect on pod B would falsely deactivate sources.
			// Sources are activated on connect and deactivated by the source-manager
			// cleanup job (24h stale threshold) or when the overlay is deleted.

			h.logger.Info("WebSocket connection closed",
				zap.String("overlay_id", overlayID),
				zap.String("user_id", userID),
			)
		}()

		// Wait for connection to close
		for !wsConn.IsClosed() {
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Return immediately - don't block the HTTP handler
	// The WebSocket connection will continue in the background
}

func loadAllowedOrigins() []string {
	value := strings.TrimSpace(os.Getenv("WEBSOCKET_ALLOWED_ORIGINS"))
	if value == "" {
		// Deny all when not configured — set WEBSOCKET_ALLOWED_ORIGINS explicitly or use "*" to allow all
		return nil
	}

	var result []string
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			result = append(result, origin)
		}
	}
	return result
}

// originAllowedForWS is the pure origin-check logic extracted from
// checkOrigin for testability (audit I1). A browser always sends an Origin
// header on a WebSocket handshake, so when the request carries the access_token
// cookie (the cookie-auth path, audit H3) an empty Origin is rejected as a
// CSRF defense-in-depth — an attacker page that suppresses Origin cannot
// open the owner socket even if it somehow has the victim's cookie. Empty
// Origin is still allowed for non-browser clients that authenticate via the
// subprotocol or ?token= query param (no cookie, no browser context).
// Non-empty Origin is validated against the allowlist via the shared M4
// matcher (exact + `*` + `/*` suffix).
func originAllowedForWS(allowed []string, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	hasAccessCookie := false
	if ck, err := r.Cookie(auth.CookieAccessToken); err == nil && ck.Value != "" {
		hasAccessCookie = true
	}
	if origin == "" {
		// Non-browser client (subprotocol/query token, no cookie) — allowed.
		// Browser with access cookie but suppressed Origin — reject (I1).
		return !hasAccessCookie
	}
	return sharedmiddleware.OriginAllowed(allowed, origin)
}

func (h *WebSocketHandler) checkOrigin(r *http.Request) bool {
	if !originAllowedForWS(h.allowedOriginList, r) {
		h.logger.Warn("Blocked WebSocket connection from disallowed or missing origin",
			zap.String("origin", r.Header.Get("Origin")),
		)
		return false
	}
	return true
}

// NotifyUser sends a notification to a specific user via WebSocket
// POST /internal/ws/notify
// Body: {"user_id": "uuid", "type": "share_accepted", "data": {...}}
func (h *WebSocketHandler) NotifyUser(c *gin.Context) {
	var req struct {
		UserID string                 `json:"user_id" binding:"required"`
		Type   string                 `json:"type" binding:"required"`
		Data   map[string]interface{} `json:"data"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find all connections for this user across all overlays
	connections := h.wsManager.GetConnectionsByUser(req.UserID)

	if len(connections) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not connected"})
		return
	}

	// Create notification message envelope
	message := map[string]interface{}{
		"type":      req.Type,
		"data":      req.Data,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Marshal to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal notification", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal message"})
		return
	}

	// Broadcast to all user's connections
	sentCount := 0
	for _, conn := range connections {
		if conn.Send(messageJSON) {
			sentCount++
		} else {
			h.logger.Error("Failed to send WebSocket message",
				zap.String("user_id", req.UserID))
		}
	}

	if sentCount == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send to any connection"})
		return
	}

	h.logger.Info("Notification sent to user",
		zap.String("user_id", req.UserID),
		zap.String("type", req.Type),
		zap.Int("connections", sentCount))

	c.JSON(http.StatusOK, gin.H{"status": "sent", "connections": sentCount})
}
