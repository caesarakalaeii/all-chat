package websocket

import (
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
func (p *Pool) Broadcast(message []byte) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	successCount := 0
	for conn := range p.connections {
		if conn.Send(message) {
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

// Size returns the number of connections in the pool
func (p *Pool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.connections)
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
