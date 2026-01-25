package irc

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gempir/go-twitch-irc/v4"
	"go.uber.org/zap"
)

// ConnectionManager manages the Twitch IRC connection
type ConnectionManager struct {
	client      *twitch.Client
	parser      *Parser
	publisher   *publisher.StreamPublisher
	logger      *zap.Logger
	metrics     *metrics.ListenerMetrics
	connected   bool
	connectedAt time.Time
	mu          sync.RWMutex
	stopChan    chan struct{}
	wg          sync.WaitGroup
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
	m *metrics.ListenerMetrics,
) *ConnectionManager {
	client := twitch.NewClient(config.Username, config.OAuth)

	cm := &ConnectionManager{
		client:    client,
		parser:    parser,
		publisher: pub,
		logger:    logger,
		metrics:   m,
		stopChan:  make(chan struct{}),
	}

	// Set up message handler
	client.OnPrivateMessage(cm.handlePrivateMessage)

	// Set up event handlers
	client.OnUserNoticeMessage(cm.handleUserNotice)

	// Set up connection handlers
	client.OnConnect(cm.handleConnect)

	return cm
}

// Connect establishes connection to Twitch IRC
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.logger.Info("Connecting to Twitch IRC")
	cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "attempting")

	// Start client in goroutine
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		if err := cm.client.Connect(); err != nil {
			cm.logger.Error("IRC connection error", zap.Error(err))
			cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "failed")
			cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", false)
			cm.metrics.RecordError("twitch", "twitch-listener", "connection", "error")
		}
	}()

	return nil
}

// Disconnect gracefully disconnects from Twitch IRC
func (cm *ConnectionManager) Disconnect() error {
	cm.logger.Info("Disconnecting from Twitch IRC")

	close(cm.stopChan)

	// Record connection duration if we were connected
	cm.mu.RLock()
	if cm.connected && !cm.connectedAt.IsZero() {
		duration := time.Since(cm.connectedAt).Seconds()
		cm.metrics.ConnectionDuration.WithLabelValues("twitch", "twitch-listener", "normal").Observe(duration)
	}
	cm.mu.RUnlock()

	if err := cm.client.Disconnect(); err != nil {
		cm.logger.Warn("Error during disconnect", zap.Error(err))
		cm.metrics.RecordError("twitch", "twitch-listener", "connection", "warning")
	}

	cm.wg.Wait()

	// Mark as disconnected
	cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", false)

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
	cm.connectedAt = time.Now()
	cm.mu.Unlock()

	// Record successful connection
	cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "success")
	cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", true)

	cm.logger.Info("Connected to Twitch IRC")
}

// handlePrivateMessage processes incoming PRIVMSG from Twitch
func (cm *ConnectionManager) handlePrivateMessage(message twitch.PrivateMessage) {
	start := time.Now()

	// Record message received
	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "chat")

	// Parse IRC message into RawChatMessage
	rawMsg, err := cm.parser.ParsePrivateMessage(message)
	if err != nil {
		cm.logger.Warn("Failed to parse IRC message",
			zap.String("channel", message.Channel),
			zap.String("user", message.User.Name),
			zap.Error(err),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "warning")
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
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	// Record successful publish and latency
	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	cm.logger.Debug("Published message",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("username", rawMsg.Username),
		zap.String("text", rawMsg.Text),
	)
}

// handleUserNotice processes incoming USERNOTICE events from Twitch (subs, raids, bits, etc.)
func (cm *ConnectionManager) handleUserNotice(message twitch.UserNoticeMessage) {
	start := time.Now()

	// Record event received
	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "event")

	// Parse USERNOTICE into RawChatMessage with event fields
	rawMsg, err := cm.parser.ParseUserNotice(message)
	if err != nil {
		cm.logger.Warn("Failed to parse USERNOTICE",
			zap.String("channel", message.Channel),
			zap.String("msg-id", message.MsgID),
			zap.String("user", message.User.Name),
			zap.Error(err),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "warning")
		return
	}

	// Publish to Redis Streams (same flow as chat messages)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish event",
			zap.String("message_id", rawMsg.MessageID),
			zap.String("channel", rawMsg.ChannelID),
			zap.String("event_type", rawMsg.EventType),
			zap.Error(err),
		)
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	// Record successful publish and latency
	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	cm.logger.Info("Published Twitch event",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("event_type", rawMsg.EventType),
		zap.String("msg-id", message.MsgID),
		zap.String("username", rawMsg.Username),
	)
}

// GetClient returns the underlying Twitch client (for testing)
func (cm *ConnectionManager) GetClient() *twitch.Client {
	return cm.client
}
