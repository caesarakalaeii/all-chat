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

package websocket

import (
	"encoding/json"
	"sync"

	"go.uber.org/zap"
)

// Pool manages a collection of WebSocket connections for a single overlay
type Pool struct {
	overlayID   string
	connections map[*Connection]bool
	mu          sync.RWMutex
	logger      *zap.Logger
}

// NewPool creates a new connection pool
func NewPool(overlayID string, logger *zap.Logger) *Pool {
	return &Pool{
		overlayID:   overlayID,
		connections: make(map[*Connection]bool),
		logger:      logger,
	}
}

// Add adds a connection to the pool
func (p *Pool) Add(conn *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.connections[conn] = true

	p.logger.Debug("Added connection to pool",
		zap.String("overlay_id", p.overlayID),
		zap.Int("pool_size", len(p.connections)),
	)
}

// Remove removes a connection from the pool
func (p *Pool) Remove(conn *Connection) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.connections, conn)

	p.logger.Debug("Removed connection from pool",
		zap.String("overlay_id", p.overlayID),
		zap.Int("pool_size", len(p.connections)),
	)
}

// Broadcast sends a message to all connections in the pool
// Returns the number of successful sends
// For viewer connections, overlay_id is stripped from the message
func (p *Pool) Broadcast(message []byte) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	successCount := 0
	for conn := range p.connections {
		// Strip overlay_id from message if this is a viewer connection
		messageToSend := message
		if conn.IsViewer() {
			messageToSend = stripOverlayID(message)
		}

		if conn.Send(messageToSend) {
			successCount++
		}
	}

	p.logger.Debug("Broadcast to pool",
		zap.String("overlay_id", p.overlayID),
		zap.Int("pool_size", len(p.connections)),
		zap.Int("success_count", successCount),
	)

	return successCount
}

// BroadcastFiltered sends to all connections but skips engagement-only connections when
// the frame is NOT a poll/prediction update — so a participate tab never receives chat.
// Returns the number of successful sends. For viewer connections, overlay_id is stripped.
func (p *Pool) BroadcastFiltered(message []byte, engagementFrame bool) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	successCount := 0
	for conn := range p.connections {
		// Engagement-only sockets only ever receive poll/prediction updates.
		if conn.IsEngagementOnly() && !engagementFrame {
			continue
		}

		// Strip overlay_id from message if this is a viewer connection
		messageToSend := message
		if conn.IsViewer() {
			messageToSend = stripOverlayID(message)
		}

		if conn.Send(messageToSend) {
			successCount++
		}
	}

	p.logger.Debug("Filtered broadcast to pool",
		zap.String("overlay_id", p.overlayID),
		zap.Int("pool_size", len(p.connections)),
		zap.Int("success_count", successCount),
		zap.Bool("engagement_frame", engagementFrame),
	)

	return successCount
}

// stripOverlayID removes overlay_id field from WebSocket messages
// This prevents leaking sensitive overlay IDs to viewers
func stripOverlayID(message []byte) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(message, &data); err != nil {
		// If we can't parse it, return original (better than failing)
		return message
	}

	// Remove overlay_id from data.overlay_id (for ChatMessageData)
	if msgData, ok := data["data"].(map[string]interface{}); ok {
		delete(msgData, "overlay_id")
	}

	// Re-marshal without overlay_id
	cleaned, err := json.Marshal(data)
	if err != nil {
		// If marshaling fails, return original
		return message
	}

	return cleaned
}

// Size returns the number of connections in the pool
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.connections)
}

// GetConnectionsByUser returns all connections for a specific user in this pool
func (p *Pool) GetConnectionsByUser(userID string) []*Connection {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var connections []*Connection
	for conn := range p.connections {
		if conn.UserID() == userID {
			connections = append(connections, conn)
		}
	}

	return connections
}

// CloseAll closes all connections in the pool
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for conn := range p.connections {
		conn.Close()
	}

	p.connections = make(map[*Connection]bool)

	p.logger.Info("Closed all connections in pool",
		zap.String("overlay_id", p.overlayID),
	)
}
