package websocket

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
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
}

// NewManager creates a new WebSocket manager
func NewManager(logger *zap.Logger, m *metrics.GatewayMetrics, redisClient *redis.Client) *Manager {
	// Get grace period from environment variable, default to 60 seconds
	gracePeriod := 60 * time.Second
	if envGrace := os.Getenv("WEBSOCKET_DISCONNECT_GRACE_PERIOD_SECONDS"); envGrace != "" {
		if seconds, err := strconv.Atoi(envGrace); err == nil && seconds > 0 {
			gracePeriod = time.Duration(seconds) * time.Second
		}
	}

	logger.Info("WebSocket manager initialized",
		zap.Duration("disconnect_grace_period", gracePeriod),
	)

	return &Manager{
		pools:                 make(map[string]*Pool),
		logger:                logger,
		metrics:               m,
		redisClient:           redisClient,
		gracePeriodTimers:     make(map[string]*time.Timer),
		disconnectGracePeriod: gracePeriod,
	}
}

// AddConnection adds a connection to the appropriate pool
func (m *Manager) AddConnection(ctx context.Context, conn *Connection) {
	overlayID := conn.OverlayID()

	// Cancel grace period timer if overlay was in grace period
	m.gracePeriodMu.Lock()
	if timer, exists := m.gracePeriodTimers[overlayID]; exists {
		timer.Stop()
		delete(m.gracePeriodTimers, overlayID)
		m.logger.Info("Cancelled disconnect grace period (overlay reconnected)",
			zap.String("overlay_id", overlayID),
		)
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

	// Update Redis SET to track connected overlays
	if eventType == "connected" {
		if err := m.redisClient.SAdd(ctx, "overlay:connected", overlayID).Err(); err != nil {
			m.logger.Error("Failed to add overlay to connected set",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	} else if eventType == "disconnected" {
		if err := m.redisClient.SRem(ctx, "overlay:connected", overlayID).Err(); err != nil {
			m.logger.Error("Failed to remove overlay from connected set",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	}

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
			// Still no connections, publish disconnected event
			ctx := context.Background()
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
