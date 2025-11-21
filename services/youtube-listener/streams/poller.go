package streams

import (
	"context"
	"fmt"
	"strings"
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

	mu               sync.RWMutex
	stopChan         chan struct{}
	wg               sync.WaitGroup

	// Exponential backoff state
	consecutiveErrors int
	backoffDuration   time.Duration
	maxBackoff        time.Duration

	// Polling limits
	pollStartTime     time.Time
	maxPollingTime    time.Duration
	totalPollCount    int
}

// NewPoller creates a new stream poller
func NewPoller(
	stream *models.YouTubeStream,
	apiClient *api.Client,
	logger *zap.Logger,
) *Poller {
	return &Poller{
		stream:            stream,
		apiClient:         apiClient,
		parser:            api.NewParser(),
		logger:            logger,
		stopChan:          make(chan struct{}),
		maxBackoff:        5 * time.Minute, // Maximum backoff of 5 minutes
		backoffDuration:   0,               // Start with no backoff
		consecutiveErrors: 0,
		pollStartTime:     time.Now(),
		maxPollingTime:    5 * time.Minute, // Maximum 5 minutes of polling per source
		totalPollCount:    0,
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

// pollLoop continuously polls for new messages with exponential backoff and time limits
func (p *Poller) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	interval := time.Duration(p.stream.PollingInterval) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial poll
	if err := p.poll(ctx); err != nil {
		p.handlePollError(err)
	} else {
		p.resetBackoff()
	}

	for {
		// Check if we've exceeded the maximum polling time per source
		if time.Since(p.pollStartTime) >= p.maxPollingTime {
			p.logger.Warn("Maximum polling time exceeded, pausing for backoff",
				zap.String("stream_id", p.stream.StreamID),
				zap.Duration("total_time", time.Since(p.pollStartTime)),
				zap.Int("total_polls", p.totalPollCount),
			)
			// Reset counters and wait for a longer backoff period
			p.pollStartTime = time.Now()
			p.totalPollCount = 0
			// Apply a long backoff (5 minutes) before resuming
			select {
			case <-time.After(5 * time.Minute):
			case <-p.stopChan:
				return
			}
			continue
		}

		select {
		case <-ticker.C:
			// Apply exponential backoff if we have errors
			if p.backoffDuration > 0 {
				p.logger.Debug("Applying exponential backoff",
					zap.String("stream_id", p.stream.StreamID),
					zap.Duration("backoff", p.backoffDuration),
					zap.Int("consecutive_errors", p.consecutiveErrors),
				)
				select {
				case <-time.After(p.backoffDuration):
				case <-p.stopChan:
					return
				}
			}

			if err := p.poll(ctx); err != nil {
				p.handlePollError(err)

				// Stop polling if the stream has ended
				if strings.Contains(err.Error(), "liveChatEnded") || strings.Contains(err.Error(), "live chat is no longer live") {
					p.logger.Info("Stream ended, stopping poller",
						zap.String("stream_id", p.stream.StreamID),
					)
					return
				}

				// Stop polling if quota exceeded
				if strings.Contains(err.Error(), "quotaExceeded") {
					p.logger.Warn("Quota exceeded, stopping poller temporarily",
						zap.String("stream_id", p.stream.StreamID),
					)
					// Wait 1 hour before retrying
					select {
					case <-time.After(1 * time.Hour):
					case <-p.stopChan:
						return
					}
				}
			} else {
				// Success - reset backoff
				p.resetBackoff()
				p.totalPollCount++
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

// handlePollError implements exponential backoff on consecutive errors
func (p *Poller) handlePollError(err error) {
	p.logger.Error("Poll failed",
		zap.String("stream_id", p.stream.StreamID),
		zap.Error(err),
		zap.Int("consecutive_errors", p.consecutiveErrors+1),
	)

	p.consecutiveErrors++

	// Calculate exponential backoff: 2^n seconds, capped at maxBackoff
	// 1st error: 2s, 2nd: 4s, 3rd: 8s, 4th: 16s, 5th: 32s, 6th: 64s, 7th: 128s, 8th: 256s (4.2min), 9th+: 5min
	backoffSeconds := 1 << uint(p.consecutiveErrors) // 2^n
	p.backoffDuration = time.Duration(backoffSeconds) * time.Second

	if p.backoffDuration > p.maxBackoff {
		p.backoffDuration = p.maxBackoff
	}

	p.logger.Info("Applying exponential backoff",
		zap.String("stream_id", p.stream.StreamID),
		zap.Duration("backoff_duration", p.backoffDuration),
		zap.Int("consecutive_errors", p.consecutiveErrors),
	)
}

// resetBackoff resets the exponential backoff state after a successful poll
func (p *Poller) resetBackoff() {
	if p.consecutiveErrors > 0 {
		p.logger.Info("Resetting backoff after successful poll",
			zap.String("stream_id", p.stream.StreamID),
			zap.Int("previous_errors", p.consecutiveErrors),
		)
		p.consecutiveErrors = 0
		p.backoffDuration = 0
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
