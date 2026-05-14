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
	h := &ViewerWebSocketHandler{
		wsManager:        wsManager,
		subscriber:       subscriber,
		repo:             repo,
		userKeyChain:     userKeyChain,
		replayBuffer:     replayBuffer,
		chatReplayBuffer: chatReplayBuffer,
		logger:           logger.Named("viewer-websocket"),
	}

	// Configure WebSocket upgrader with permissive origin check for viewers
	// Viewers connect from browser extensions and web pages
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for viewer connections
			// This is safe because viewers can't trigger polling or access secrets
			return true
		},
	}

	return h
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

	// Send connected message (without overlay_id for security)
	connectedMsg := models.NewViewerConnected()
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.Send(connectedJSON)

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
					wsConn.Send(payload)
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

	// Create background context for WebSocket connection
	wsCtx := context.Background()

	// Start connection pumps (runs in goroutines)
	wsConn.Start(wsCtx)

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
