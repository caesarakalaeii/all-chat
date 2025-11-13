package irc

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/gempir/go-twitch-irc/v4"
	"go.uber.org/zap"
)

// ConnectionManager manages the Twitch IRC connection
type ConnectionManager struct {
	client    *twitch.Client
	parser    *Parser
	publisher *publisher.StreamPublisher
	logger    *zap.Logger
	connected bool
	mu        sync.RWMutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// Config holds the configuration for IRC connection
type Config struct {
	Username string // Twitch bot username
	OAuth    string // OAuth token (oauth:abc123...)
}

// NewConnectionManager creates a new IRC connection manager
func NewConnectionManager(
	config Config,
	parser *Parser,
	pub *publisher.StreamPublisher,
	logger *zap.Logger,
) *ConnectionManager {
	client := twitch.NewClient(config.Username, config.OAuth)

	cm := &ConnectionManager{
		client:    client,
		parser:    parser,
		publisher: pub,
		logger:    logger,
		stopChan:  make(chan struct{}),
	}

	// Set up message handler
	client.OnPrivateMessage(cm.handlePrivateMessage)

	// Set up connection handlers
	client.OnConnect(cm.handleConnect)

	return cm
}

// Connect establishes connection to Twitch IRC
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.logger.Info("Connecting to Twitch IRC")

	// Start client in goroutine
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		if err := cm.client.Connect(); err != nil {
			cm.logger.Error("IRC connection error", zap.Error(err))
		}
	}()

	return nil
}

// Disconnect gracefully disconnects from Twitch IRC
func (cm *ConnectionManager) Disconnect() error {
	cm.logger.Info("Disconnecting from Twitch IRC")

	close(cm.stopChan)

	if err := cm.client.Disconnect(); err != nil {
		cm.logger.Warn("Error during disconnect", zap.Error(err))
	}

	cm.wg.Wait()

	cm.logger.Info("Disconnected from Twitch IRC")
	return nil
}

// Join joins a Twitch channel
func (cm *ConnectionManager) Join(channel string) {
	cm.client.Join(channel)
	cm.logger.Debug("IRC JOIN", zap.String("channel", channel))
}

// Depart leaves a Twitch channel
func (cm *ConnectionManager) Depart(channel string) {
	cm.client.Depart(channel)
	cm.logger.Debug("IRC PART", zap.String("channel", channel))
}

// IsConnected returns whether the IRC client is connected
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connected
}

// handleConnect is called when connection is established
func (cm *ConnectionManager) handleConnect() {
	cm.mu.Lock()
	cm.connected = true
	cm.mu.Unlock()

	cm.logger.Info("Connected to Twitch IRC")
}

// handlePrivateMessage processes incoming PRIVMSG from Twitch
func (cm *ConnectionManager) handlePrivateMessage(message twitch.PrivateMessage) {
	// Parse IRC message into RawChatMessage
	rawMsg, err := cm.parser.ParsePrivateMessage(message)
	if err != nil {
		cm.logger.Warn("Failed to parse IRC message",
			zap.String("channel", message.Channel),
			zap.String("user", message.User.Name),
			zap.Error(err),
		)
		return
	}

	// Publish to Redis Streams
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish message",
			zap.String("message_id", rawMsg.MessageID),
			zap.String("channel", rawMsg.ChannelID),
			zap.Error(err),
		)
		return
	}

	cm.logger.Debug("Published message",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("username", rawMsg.Username),
		zap.String("text", rawMsg.Text),
	)
}

// GetClient returns the underlying Twitch client (for testing)
func (cm *ConnectionManager) GetClient() *twitch.Client {
	return cm.client
}
