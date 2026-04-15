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
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/sessions"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// OverlayConnectionEvent represents an overlay connection/disconnection event
type OverlayConnectionEvent struct {
	Type      string    `json:"type"`       // "connected" or "disconnected"
	OverlayID string    `json:"overlay_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Manager manages all WebSocket connections grouped by overlay
type Manager struct {
	pools       map[string]*Pool // overlay_id -> connection pool
	mu          sync.RWMutex
	logger      *zap.Logger
	metrics     *metrics.GatewayMetrics
	redisClient *redis.Client

	// Grace period tracking for disconnection events
	gracePeriodTimers map[string]*time.Timer
	gracePeriodMu     sync.Mutex
	disconnectGracePeriod time.Duration

	// Heartbeat tracking for connection TTL
	heartbeatInterval time.Duration
	connectionTTL     time.Duration
	stopHeartbeat     chan struct{}
	heartbeatWg       sync.WaitGroup

	// Session management for credit roll
	sessionManager *sessions.SessionManager
}

// NewManager creates a new WebSocket manager
func NewManager(logger *zap.Logger, m *metrics.GatewayMetrics, redisClient *redis.Client, db *pgxpool.Pool) *Manager {
	// Get grace period from environment variable, default to 60 seconds
	gracePeriod := 60 * time.Second
	if envGrace := os.Getenv("WEBSOCKET_DISCONNECT_GRACE_PERIOD_SECONDS"); envGrace != "" {
		if seconds, err := strconv.Atoi(envGrace); err == nil && seconds > 0 {
			gracePeriod = time.Duration(seconds) * time.Second
		}
	}

	// Connection TTL: 10 minutes (auto-expire if heartbeat stops)
	// Increased from 5 minutes to tolerate temporary Redis failures
	connectionTTL := 10 * time.Minute
	if envTTL := os.Getenv("WEBSOCKET_CONNECTION_TTL_MINUTES"); envTTL != "" {
		if minutes, err := strconv.Atoi(envTTL); err == nil && minutes > 0 {
			connectionTTL = time.Duration(minutes) * time.Minute
		}
	}

	// Heartbeat interval: 2 minutes (refresh TTL before expiration)
	heartbeatInterval := 2 * time.Minute
	if envInterval := os.Getenv("WEBSOCKET_HEARTBEAT_INTERVAL_MINUTES"); envInterval != "" {
		if minutes, err := strconv.Atoi(envInterval); err == nil && minutes > 0 {
			heartbeatInterval = time.Duration(minutes) * time.Minute
		}
	}

	logger.Info("WebSocket manager initialized",
		zap.Duration("disconnect_grace_period", gracePeriod),
		zap.Duration("connection_ttl", connectionTTL),
		zap.Duration("heartbeat_interval", heartbeatInterval),
	)

	mgr := &Manager{
		pools:                 make(map[string]*Pool),
		logger:                logger,
		metrics:               m,
		redisClient:           redisClient,
		gracePeriodTimers:     make(map[string]*time.Timer),
		disconnectGracePeriod: gracePeriod,
		heartbeatInterval:     heartbeatInterval,
		connectionTTL:         connectionTTL,
		stopHeartbeat:         make(chan struct{}),
		sessionManager:        sessions.NewSessionManager(redisClient, db, logger, gracePeriod),
	}

	// Start heartbeat goroutine to refresh connection TTLs
	mgr.startHeartbeat()

	return mgr
}

// AddConnection adds a connection to the appropriate pool
func (m *Manager) AddConnection(ctx context.Context, conn *Connection) {
	overlayID := conn.OverlayID()

	// Ensure session exists (creates new or reactivates existing)
	if err := m.sessionManager.EnsureSession(ctx, overlayID); err != nil {
		m.logger.Error("Failed to ensure session",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		// Continue - session management is non-critical for overlay display
	}

	// Cancel grace period timer if overlay was in grace period
	m.gracePeriodMu.Lock()
	if timer, exists := m.gracePeriodTimers[overlayID]; exists {
		timer.Stop()
		delete(m.gracePeriodTimers, overlayID)
		m.logger.Info("Cancelled disconnect grace period (overlay reconnected)",
			zap.String("overlay_id", overlayID),
		)
		// Update session state back to ACTIVE
		if err := m.sessionManager.CancelGracePeriod(ctx, overlayID); err != nil {
			m.logger.Error("Failed to cancel grace period",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	}
	m.gracePeriodMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create pool for this overlay
	pool, exists := m.pools[overlayID]
	isFirstConnection := !exists

	if !exists {
		pool = NewPool(overlayID, m.logger)
		m.pools[overlayID] = pool
		m.logger.Info("Created connection pool",
			zap.String("overlay_id", overlayID),
		)
		m.metrics.RecordSubscriptionEvent("api-gateway", "pool_created")
	}

	pool.Add(conn)

	// Record metrics
	m.metrics.RecordWebSocketConnectionAttempt("api-gateway", "success")
	m.metrics.RecordWebSocketConnection("api-gateway", "overlay", 1)
	m.metrics.RecordOverlaySubscription("api-gateway", overlayID, 1)
	m.metrics.RecordSubscriptionEvent("api-gateway", "subscribed")

	m.logger.Info("WebSocket connection added",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", conn.UserID()),
		zap.Int("pool_size", pool.Size()),
	)

	// Publish overlay connection event if this is the first connection
	if isFirstConnection {
		m.publishConnectionEvent(ctx, overlayID, "connected")
	}
}

// publishConnectionEvent publishes an overlay connection event to Redis
func (m *Manager) publishConnectionEvent(ctx context.Context, overlayID, eventType string) {
	event := OverlayConnectionEvent{
		Type:      eventType,
		OverlayID: overlayID,
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		m.logger.Error("Failed to marshal overlay connection event",
			zap.String("overlay_id", overlayID),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return
	}

	// Update Redis with TTL-based tracking
	key := "overlay:connected:" + overlayID
	if eventType == "connected" {
		// Set connection key with TTL (will auto-expire if heartbeat stops)
		if err := m.redisClient.SetEx(ctx, key, "1", m.connectionTTL).Err(); err != nil {
			m.logger.Error("Failed to set overlay connection with TTL",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	} else if eventType == "disconnected" {
		// Immediately delete connection key
		if err := m.redisClient.Del(ctx, key).Err(); err != nil {
			m.logger.Error("Failed to delete overlay connection key",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	}

	// Publish event to subscribers
	if err := m.redisClient.Publish(ctx, "overlay:connections", payload).Err(); err != nil {
		m.logger.Error("Failed to publish overlay connection event",
			zap.String("overlay_id", overlayID),
			zap.String("event_type", eventType),
			zap.Error(err),
		)
		return
	}

	m.logger.Info("Published overlay connection event",
		zap.String("overlay_id", overlayID),
		zap.String("event_type", eventType),
	)
}

// RemoveConnection removes a connection from its pool
func (m *Manager) RemoveConnection(conn *Connection) {
	m.mu.Lock()
	overlayID := conn.OverlayID()

	pool, exists := m.pools[overlayID]
	if !exists {
		m.mu.Unlock()
		return
	}

	pool.Remove(conn)
	poolSize := pool.Size()

	// Record metrics
	m.metrics.RecordWebSocketConnection("api-gateway", "overlay", -1)
	m.metrics.RecordOverlaySubscription("api-gateway", overlayID, -1)
	m.metrics.RecordSubscriptionEvent("api-gateway", "unsubscribed")

	m.logger.Info("WebSocket connection removed",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", conn.UserID()),
		zap.Int("pool_size", poolSize),
	)

	// Handle empty pool with grace period
	if poolSize == 0 {
		delete(m.pools, overlayID)
		m.metrics.RecordSubscriptionEvent("api-gateway", "pool_destroyed")
		m.logger.Info("Connection pool empty, starting grace period",
			zap.String("overlay_id", overlayID),
			zap.Duration("grace_period", m.disconnectGracePeriod),
		)
		m.mu.Unlock()

		// Start grace period timer
		m.startDisconnectGracePeriod(overlayID)
	} else {
		m.mu.Unlock()
	}
}

// startDisconnectGracePeriod starts a grace period timer for an overlay disconnection
func (m *Manager) startDisconnectGracePeriod(overlayID string) {
	m.gracePeriodMu.Lock()
	defer m.gracePeriodMu.Unlock()

	// Update session state to ENDING
	ctx := context.Background()
	if err := m.sessionManager.StartGracePeriod(ctx, overlayID); err != nil {
		m.logger.Error("Failed to start grace period",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
	}

	// Cancel existing timer if present
	if timer, exists := m.gracePeriodTimers[overlayID]; exists {
		timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(m.disconnectGracePeriod, func() {
		m.gracePeriodMu.Lock()
		delete(m.gracePeriodTimers, overlayID)
		m.gracePeriodMu.Unlock()

		// Check if still disconnected after grace period
		m.mu.RLock()
		_, hasPool := m.pools[overlayID]
		m.mu.RUnlock()

		if !hasPool {
			// Still no connections, end session and publish disconnected event
			ctx := context.Background()

			// End session
			if err := m.sessionManager.EndSession(ctx, overlayID); err != nil {
				m.logger.Error("Failed to end session",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
			}

			m.publishConnectionEvent(ctx, overlayID, "disconnected")
			m.logger.Info("Grace period expired, overlay still disconnected",
				zap.String("overlay_id", overlayID),
			)
		} else {
			m.logger.Info("Grace period expired, but overlay reconnected",
				zap.String("overlay_id", overlayID),
			)
		}
	})

	m.gracePeriodTimers[overlayID] = timer
}

// BroadcastToOverlay sends a message to all connections in an overlay pool
func (m *Manager) BroadcastToOverlay(overlayID string, message []byte) int {
	m.mu.RLock()
	pool, exists := m.pools[overlayID]
	m.mu.RUnlock()

	if !exists {
		return 0
	}

	return pool.Broadcast(message)
}

// BroadcastToAll sends a message to all connected clients (all overlays)
func (m *Manager) BroadcastToAll(message []byte) int {
	m.mu.RLock()
	pools := make([]*Pool, 0, len(m.pools))
	for _, pool := range m.pools {
		pools = append(pools, pool)
	}
	m.mu.RUnlock()

	totalSent := 0
	for _, pool := range pools {
		totalSent += pool.Broadcast(message)
	}

	return totalSent
}

// GetConnectedOverlayIDs returns the IDs of all overlays with active WebSocket connections.
func (m *Manager) GetConnectedOverlayIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.pools))
	for id := range m.pools {
		ids = append(ids, id)
	}
	return ids
}

// GetPoolSize returns the number of connections for an overlay
func (m *Manager) GetPoolSize(overlayID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[overlayID]
	if !exists {
		return 0
	}

	return pool.Size()
}

// GetTotalConnections returns the total number of active connections
func (m *Manager) GetTotalConnections() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, pool := range m.pools {
		total += pool.Size()
	}

	return total
}

// GetPoolCount returns the number of active pools
func (m *Manager) GetPoolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.pools)
}

// HasPool checks if a pool exists for the given overlay
func (m *Manager) HasPool(overlayID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.pools[overlayID]
	return exists
}

// GetConnectionsByUser returns all connections for a specific user across all overlays
func (m *Manager) GetConnectionsByUser(userID string) []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connections []*Connection
	for _, pool := range m.pools {
		poolConns := pool.GetConnectionsByUser(userID)
		connections = append(connections, poolConns...)
	}

	return connections
}

// startHeartbeat starts a background goroutine to refresh connection TTLs
func (m *Manager) startHeartbeat() {
	m.heartbeatWg.Add(1)
	go func() {
		defer m.heartbeatWg.Done()

		ticker := time.NewTicker(m.heartbeatInterval)
		defer ticker.Stop()

		m.logger.Info("Connection heartbeat started",
			zap.Duration("interval", m.heartbeatInterval),
		)

		for {
			select {
			case <-ticker.C:
				m.refreshConnectionTTLs()
			case <-m.stopHeartbeat:
				m.logger.Info("Connection heartbeat stopped")
				return
			}
		}
	}()
}

// refreshConnectionTTLs refreshes TTL for all active overlay connections
func (m *Manager) refreshConnectionTTLs() {
	ctx := context.Background()

	m.mu.RLock()
	overlayIDs := make([]string, 0, len(m.pools))
	for overlayID := range m.pools {
		overlayIDs = append(overlayIDs, overlayID)
	}
	m.mu.RUnlock()

	if len(overlayIDs) == 0 {
		return
	}

	// Refresh TTL for each connected overlay
	refreshed := 0
	for _, overlayID := range overlayIDs {
		key := "overlay:connected:" + overlayID
		err := m.redisClient.SetEx(ctx, key, "1", m.connectionTTL).Err()

		if err != nil {
			// Retry once after 100ms for transient failures
			time.Sleep(100 * time.Millisecond)
			retryErr := m.redisClient.SetEx(ctx, key, "1", m.connectionTTL).Err()

			if retryErr != nil {
				m.logger.Error("Failed to refresh connection TTL after retry",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
					zap.NamedError("retry_error", retryErr),
				)
				m.metrics.RecordSubscriptionEvent("api-gateway", "heartbeat_failed")
			} else {
				m.logger.Warn("Heartbeat retry succeeded after initial failure",
					zap.String("overlay_id", overlayID),
				)
				refreshed++
			}
		} else {
			refreshed++
		}
	}

	m.logger.Debug("Refreshed connection TTLs",
		zap.Int("count", refreshed),
		zap.Int("total_overlays", len(overlayIDs)),
	)
}

// Shutdown gracefully stops the WebSocket manager
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down WebSocket manager")

	// Stop heartbeat
	close(m.stopHeartbeat)
	m.heartbeatWg.Wait()

	// Close all connections and clear Redis state
	m.mu.Lock()
	overlayIDs := make([]string, 0, len(m.pools))
	for overlayID := range m.pools {
		overlayIDs = append(overlayIDs, overlayID)
	}
	m.mu.Unlock()

	// Remove all connection keys from Redis
	for _, overlayID := range overlayIDs {
		m.publishConnectionEvent(ctx, overlayID, "disconnected")
	}

	m.logger.Info("WebSocket manager shutdown complete")
	return nil
}
