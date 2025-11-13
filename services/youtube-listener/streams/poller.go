package streams

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"go.uber.org/zap"
)

// MessageHandler defines the interface for handling parsed messages
type MessageHandler interface {
	HandleMessages(ctx context.Context, messages []*models.RawChatMessage) error
}

// Poller polls a YouTube live stream for chat messages
type Poller struct {
	stream         *models.YouTubeStream
	apiClient      *api.Client
	parser         *api.Parser
	messageHandler MessageHandler
	logger         *zap.Logger

	mu       sync.RWMutex
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewPoller creates a new stream poller
func NewPoller(
	stream *models.YouTubeStream,
	apiClient *api.Client,
	logger *zap.Logger,
) *Poller {
	return &Poller{
		stream:    stream,
		apiClient: apiClient,
		parser:    api.NewParser(),
		logger:    logger,
		stopChan:  make(chan struct{}),
	}
}

// SetMessageHandler sets the handler for parsed messages
func (p *Poller) SetMessageHandler(handler MessageHandler) {
	p.messageHandler = handler
}

// Start begins polling the stream
func (p *Poller) Start(ctx context.Context) error {
	p.logger.Info("Starting poller",
		zap.String("stream_id", p.stream.StreamID),
		zap.String("live_chat_id", p.stream.LiveChatID),
	)

	p.wg.Add(1)
	go p.pollLoop(ctx)

	return nil
}

// Stop stops polling the stream
func (p *Poller) Stop() {
	p.logger.Info("Stopping poller",
		zap.String("stream_id", p.stream.StreamID),
	)

	close(p.stopChan)
	p.wg.Wait()
}

// pollLoop continuously polls for new messages
func (p *Poller) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	interval := time.Duration(p.stream.PollingInterval) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial poll
	if err := p.poll(ctx); err != nil {
		p.logger.Error("Initial poll failed",
			zap.String("stream_id", p.stream.StreamID),
			zap.Error(err),
		)
	}

	for {
		select {
		case <-ticker.C:
			if err := p.poll(ctx); err != nil {
				p.logger.Error("Poll failed",
					zap.String("stream_id", p.stream.StreamID),
					zap.Error(err),
				)
			}

			// Update ticker interval if it changed
			p.mu.RLock()
			newInterval := time.Duration(p.stream.PollingInterval) * time.Millisecond
			p.mu.RUnlock()

			if newInterval != interval {
				interval = newInterval
				ticker.Reset(interval)
				p.logger.Debug("Updated polling interval",
					zap.String("stream_id", p.stream.StreamID),
					zap.Duration("interval", interval),
				)
			}

		case <-p.stopChan:
			return
		}
	}
}

// poll fetches and processes new messages
func (p *Poller) poll(ctx context.Context) error {
	p.mu.RLock()
	liveChatID := p.stream.LiveChatID
	pageToken := p.stream.NextPageToken
	p.mu.RUnlock()

	// Fetch messages from API
	response, err := p.apiClient.GetChatMessages(ctx, liveChatID, pageToken)
	if err != nil {
		return fmt.Errorf("failed to get chat messages: %w", err)
	}

	// Parse messages
	messages, err := p.parser.ParseBatch(response.Items, p.stream.ChannelID, p.stream.StreamID)
	if err != nil {
		p.logger.Error("Failed to parse messages",
			zap.String("stream_id", p.stream.StreamID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to parse messages: %w", err)
	}

	// Update stream state
	p.mu.Lock()
	p.stream.NextPageToken = p.parser.ExtractNextPageToken(response)
	p.stream.PollingInterval = p.parser.ExtractPollingInterval(response)
	p.stream.LastPolledAt = time.Now()
	p.stream.UpdatedAt = time.Now()
	p.mu.Unlock()

	// Handle messages if we have any
	if len(messages) > 0 {
		p.logger.Debug("Received messages",
			zap.String("stream_id", p.stream.StreamID),
			zap.Int("count", len(messages)),
		)

		if p.messageHandler != nil {
			if err := p.messageHandler.HandleMessages(ctx, messages); err != nil {
				p.logger.Error("Failed to handle messages",
					zap.String("stream_id", p.stream.StreamID),
					zap.Error(err),
				)
				return fmt.Errorf("failed to handle messages: %w", err)
			}
		}
	}

	return nil
}

// GetStream returns the current stream state (thread-safe copy)
func (p *Poller) GetStream() *models.YouTubeStream {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy
	streamCopy := *p.stream
	return &streamCopy
}
