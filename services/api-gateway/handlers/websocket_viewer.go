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
	"strings"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/replay"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ViewerWebSocketHandler handles WebSocket connections for viewers
// Viewers connect via /ws/chat/{streamer_username} without knowing the overlay ID
type ViewerWebSocketHandler struct {
	wsManager        *wsconn.Manager
	subscriber       *subscription.Subscriber
	repo             *subscription.Repository
	userKeyChain     *auth.KeyChain
	replayBuffer     replay.DeletionReplayBuffer
	chatReplayBuffer replay.ChatReplayBuffer
	logger           *zap.Logger
	upgrader         websocket.Upgrader
	allowedOrigins   map[string]struct{}
	allowedPrefixes  []string
	allowAllOrigins  bool
}

// NewViewerWebSocketHandler creates a new viewer WebSocket handler
func NewViewerWebSocketHandler(
	wsManager *wsconn.Manager,
	subscriber *subscription.Subscriber,
	repo *subscription.Repository,
	userKeyChain *auth.KeyChain,
	replayBuffer replay.DeletionReplayBuffer,
	chatReplayBuffer replay.ChatReplayBuffer,
	logger *zap.Logger,
) *ViewerWebSocketHandler {
	// Apply the same origin allowlist as the owner WS handler (M8).
	// Previously CheckOrigin returned true for all origins, allowing any
	// malicious page to open /ws/chat/<streamer> via a victim's browser.
	allowedOrigins, allowedPrefixes, allowAll := loadAllowedOrigins()

	h := &ViewerWebSocketHandler{
		wsManager:        wsManager,
		subscriber:       subscriber,
		repo:             repo,
		userKeyChain:     userKeyChain,
		replayBuffer:     replayBuffer,
		chatReplayBuffer: chatReplayBuffer,
		logger:           logger.Named("viewer-websocket"),
		allowedOrigins:   allowedOrigins,
		allowedPrefixes:  allowedPrefixes,
		allowAllOrigins:  allowAll,
	}

	if h.allowAllOrigins {
		h.logger.Info("Viewer WebSocket origin allowlist disabled; allowing all origins")
	} else {
		h.logger.Info("Configured viewer WebSocket origin allowlist",
			zap.Int("count", len(h.allowedOrigins)),
		)
	}

	// Configure WebSocket upgrader with the shared origin allowlist (M8).
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	return h
}

// checkOrigin enforces the configured origin allowlist for viewer connections (M8).
func (h *ViewerWebSocketHandler) checkOrigin(r *http.Request) bool {
	if h.allowAllOrigins {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients (e.g. curl) have no Origin header
		return true
	}

	if _, ok := h.allowedOrigins[origin]; ok {
		return true
	}

	for _, prefix := range h.allowedPrefixes {
		if strings.HasPrefix(origin, prefix) {
			return true
		}
	}

	h.logger.Warn("Blocked viewer WebSocket connection from disallowed origin",
		zap.String("origin", origin),
	)
	return false
}

// HandleViewerChatConnection handles WebSocket connection requests for viewers
// Endpoint: GET /ws/chat/:streamer_username
// Authentication: Optional (viewers can connect anonymously to view chat)
// This endpoint does NOT trigger YouTube polling (unlike /ws/overlay/{id})
func (h *ViewerWebSocketHandler) HandleViewerChatConnection(c *gin.Context) {
	streamerUsername := c.Param("streamer_username")
	if streamerUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "streamer_username is required"})
		return
	}

	// Optional JWT authentication from query parameter
	// Anonymous viewers can connect to view chat, authenticated viewers can send messages
	token := c.Query("token")
	var viewerID string
	var viewerUsername string
	isAuthenticated := false

	if token != "" {
		// Try to validate as viewer JWT
		viewerClaims, err := auth.ValidateViewerJWTWithKeyChain(token, h.userKeyChain)
		if err == nil && viewerClaims.IsViewer {
			viewerID = viewerClaims.SessionID
			viewerUsername = viewerClaims.Username
			isAuthenticated = true
			h.logger.Info("Authenticated viewer connecting",
				zap.String("viewer", viewerUsername),
				zap.String("streamer", streamerUsername))
		} else {
			h.logger.Warn("Invalid viewer token provided, connecting anonymously",
				zap.Error(err))
		}
	}

	// If not authenticated, use anonymous identifier
	if !isAuthenticated {
		viewerID = "anonymous"
		viewerUsername = "Anonymous Viewer"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Lookup streamer's public overlay by username
	overlayID, err := h.repo.GetPublicOverlayByUsername(ctx, streamerUsername)
	if err != nil {
		h.logger.Error("Failed to lookup public overlay",
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lookup streamer"})
		return
	}

	if overlayID == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "streamer not found or has no public overlay configured",
		})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket",
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		return
	}

	// Create WebSocket connection wrapper for viewer (marks as viewer connection)
	// Viewer connections will have overlay_id stripped from all messages
	wsConn := wsconn.NewViewerConnection(conn, overlayID, viewerID, h.replayBuffer, h.logger)

	// Allow viewers to authenticate via WebSocket message (preferred over URL token)
	wsConn.SetOnAuth(func(token string) (string, string, bool) {
		claims, err := auth.ValidateViewerJWTWithKeyChain(token, h.userKeyChain)
		if err != nil || !claims.IsViewer {
			return "", "", false
		}
		return claims.SessionID, claims.Username, true
	})

	// If already authenticated via URL param, mark connection as authenticated
	if isAuthenticated {
		// Backwards compatibility: URL token still works
	}

	// Subscribe to overlay's Redis Pub/Sub channel WITHOUT publishing connection event
	// This is critical: viewers should NOT trigger YouTube polling
	if err := h.subscriber.SubscribeViewerOnly(context.Background(), overlayID); err != nil {
		h.logger.Error("Failed to subscribe to overlay channel",
			zap.String("overlay_id", overlayID),
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to subscribe"})
		return
	}

	// Add connection to manager (but mark it as viewer connection)
	h.wsManager.AddConnection(context.Background(), wsConn)

	// Start the pumps BEFORE the connect burst so writePump drains the send
	// channel as we enqueue. Enqueuing the burst (connected frame + replay)
	// first would overflow the 256-slot channel on a large replay and self-close
	// the socket mid-replay, flapping busy overlays. Background context: the HTTP
	// request context is cancelled when this handler returns.
	wsCtx := context.Background()
	wsConn.Start(wsCtx)

	// Send connected message (without overlay_id for security). The burst uses
	// SendBlocking (backpressure) rather than Send (drop-and-close).
	connectedMsg := models.NewViewerConnected()
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.SendBlocking(connectedJSON)

	// Replay messages buffered since the viewer's last-seen timestamp.
	// For viewers we *require* an explicit ?since= — first-time viewers should
	// not receive a flood of 5 minutes of chat history they never saw before.
	// A reconnecting viewer client provides since=<last-msg-ms> to recover the
	// gap. The owner-handler replays everything by default; this is intentionally
	// stricter for public-facing viewer connections.
	if h.chatReplayBuffer != nil {
		if sinceMs := parseSinceQuery(c.Query("since")); sinceMs > 0 {
			replayed, err := h.chatReplayBuffer.GetSince(context.Background(), overlayID, sinceMs)
			if err != nil {
				h.logger.Warn("Failed to fetch chat replay buffer",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
			} else if len(replayed) > 0 {
				h.logger.Info("Replaying buffered messages to reconnected viewer",
					zap.String("overlay_id", overlayID),
					zap.String("viewer_id", viewerID),
					zap.Int("message_count", len(replayed)),
					zap.Int64("since_ms", sinceMs),
				)
				for _, payload := range replayed {
					if !wsConn.SendBlocking(payload) {
						break // viewer gone or too slow; stop replaying
					}
				}
			}
		}
	}

	h.logger.Info("Viewer WebSocket connection established",
		zap.String("overlay_id", overlayID),
		zap.String("streamer", streamerUsername),
		zap.String("viewer_id", viewerID),
		zap.String("viewer", viewerUsername),
		zap.Bool("authenticated", isAuthenticated),
	)

	// Set up cleanup callback when connection closes
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in viewer WebSocket cleanup",
					zap.String("overlay_id", overlayID),
					zap.String("viewer", viewerUsername),
					zap.Any("panic", r),
				)
			}
			// Always clean up
			h.wsManager.RemoveConnection(wsConn)
			h.subscriber.UnsubscribeViewerOnly(context.Background(), overlayID)
			h.logger.Info("Viewer WebSocket connection closed",
				zap.String("overlay_id", overlayID),
				zap.String("streamer", streamerUsername),
				zap.String("viewer", viewerUsername),
			)
		}()

		// Wait for connection to close
		for !wsConn.IsClosed() {
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Return immediately - WebSocket continues in background
}
