package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Message represents a message to be broadcast
type Message struct {
	OverlayID string
	Data      []byte
}

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients by overlay ID
	clients map[string]map[*Client]bool

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to clients
	broadcast chan *Message

	// Redis client for pub/sub
	redisClient *redis.Client

	// Logger
	logger *zap.Logger

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub(redisClient *redis.Client, logger *zap.Logger) *Hub {
	return &Hub{
		clients:     make(map[string]map[*Client]bool),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		broadcast:   make(chan *Message, 256),
		redisClient: redisClient,
		logger:      logger,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	// Start Redis subscription in a separate goroutine
	go h.subscribeToRedis(ctx)

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			h.logger.Info("Hub shutting down")
			h.closeAllClients()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.overlayID] == nil {
		h.clients[client.overlayID] = make(map[*Client]bool)
	}
	h.clients[client.overlayID][client] = true

	h.logger.Info("Client registered",
		zap.String("overlay_id", client.overlayID),
		zap.Int("total_clients", h.countClients(client.overlayID)))
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.overlayID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)

			// Clean up empty overlay groups
			if len(clients) == 0 {
				delete(h.clients, client.overlayID)
			}

			h.logger.Info("Client unregistered",
				zap.String("overlay_id", client.overlayID),
				zap.Int("remaining_clients", len(clients)))
		}
	}
}

// broadcastMessage sends a message to all clients of a specific overlay
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	clients := h.clients[message.OverlayID]
	h.mu.RUnlock()

	if clients == nil {
		return
	}

	for client := range clients {
		select {
		case client.send <- message.Data:
		default:
			// Client's send channel is full, close the client
			h.unregister <- client
		}
	}
}

// subscribeToRedis subscribes to Redis pub/sub channels and forwards messages to the hub
func (h *Hub) subscribeToRedis(ctx context.Context) {
	// Subscribe to all overlay channels
	pubsub := h.redisClient.PSubscribe(ctx, "overlay:*")
	defer pubsub.Close()

	h.logger.Info("Subscribed to Redis pub/sub pattern: overlay:*")

	// Listen for messages
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return

		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Parse overlay ID from channel name (overlay:UUID)
			overlayID := msg.Channel[8:] // Remove "overlay:" prefix

			// Parse message data
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
				h.logger.Error("Failed to parse Redis message",
					zap.String("channel", msg.Channel),
					zap.Error(err))
				continue
			}

			// Broadcast to WebSocket clients
			h.broadcast <- &Message{
				OverlayID: overlayID,
				Data:      []byte(msg.Payload),
			}
		}
	}
}

// closeAllClients closes all active client connections
func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for overlayID, clients := range h.clients {
		for client := range clients {
			close(client.send)
		}
		delete(h.clients, overlayID)
	}
}

// countClients returns the number of clients for an overlay
func (h *Hub) countClients(overlayID string) int {
	if clients, ok := h.clients[overlayID]; ok {
		return len(clients)
	}
	return 0
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetStats returns statistics about connected clients
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalClients := 0
	overlayCount := len(h.clients)

	for _, clients := range h.clients {
		totalClients += len(clients)
	}

	return map[string]interface{}{
		"total_clients":   totalClients,
		"active_overlays": overlayCount,
	}
}
