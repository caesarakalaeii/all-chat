package websocket

import (
	"context"
	"sync"

	"github.com/caesar/all-chat/shared/metrics"
	"go.uber.org/zap"
)

// Manager manages all WebSocket connections grouped by overlay
type Manager struct {
	pools   map[string]*Pool // overlay_id -> connection pool
	mu      sync.RWMutex
	logger  *zap.Logger
	metrics *metrics.GatewayMetrics
}

// NewManager creates a new WebSocket manager
func NewManager(logger *zap.Logger, m *metrics.GatewayMetrics) *Manager {
	return &Manager{
		pools:   make(map[string]*Pool),
		logger:  logger,
		metrics: m,
	}
}

// AddConnection adds a connection to the appropriate pool
func (m *Manager) AddConnection(ctx context.Context, conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	overlayID := conn.OverlayID()

	// Get or create pool for this overlay
	pool, exists := m.pools[overlayID]
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
}

// RemoveConnection removes a connection from its pool
func (m *Manager) RemoveConnection(conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	overlayID := conn.OverlayID()

	pool, exists := m.pools[overlayID]
	if !exists {
		return
	}

	pool.Remove(conn)

	// Record metrics
	m.metrics.RecordWebSocketConnection("api-gateway", "overlay", -1)
	m.metrics.RecordOverlaySubscription("api-gateway", overlayID, -1)
	m.metrics.RecordSubscriptionEvent("api-gateway", "unsubscribed")

	m.logger.Info("WebSocket connection removed",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", conn.UserID()),
		zap.Int("pool_size", pool.Size()),
	)

	// Remove pool if empty
	if pool.Size() == 0 {
		delete(m.pools, overlayID)
		m.metrics.RecordSubscriptionEvent("api-gateway", "pool_destroyed")
		m.logger.Info("Removed empty connection pool",
			zap.String("overlay_id", overlayID),
		)
	}
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
