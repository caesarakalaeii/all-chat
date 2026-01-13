package streams

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
)

// MessageHandler defines the interface for handling parsed messages
type MessageHandler interface {
	HandleMessages(ctx context.Context, messages []*models.RawChatMessage) error
}

// ConnectionChecker defines the interface for checking if an overlay is still connected
type ConnectionChecker interface {
	IsOverlayConnected(ctx context.Context, overlayID string) (bool, error)
}

// Poller polls a YouTube live stream for chat messages
type Poller struct {
	stream         *models.YouTubeStream
	apiClient      *api.Client
	parser         *api.Parser
	messageHandler MessageHandler
	logger         *zap.Logger
	ytMetrics      *metrics.YouTubeMetrics

	// Connection-aware polling (prevents quota waste when overlay disconnected)
	connectionChecker ConnectionChecker
	overlayID         string

	mu               sync.RWMutex
	stopChan         chan struct{}
	wg               sync.WaitGroup

	// Exponential backoff state (only for errors)
	consecutiveErrors int
	backoffDuration   time.Duration
	maxBackoff        time.Duration
}

// NewPoller creates a new stream poller
func NewPoller(
	stream *models.YouTubeStream,
	apiClient *api.Client,
	ytMetrics *metrics.YouTubeMetrics,
	logger *zap.Logger,
) *Poller {
	return &Poller{
		stream:            stream,
		apiClient:         apiClient,
		parser:            api.NewParser(),
		logger:            logger,
		ytMetrics:         ytMetrics,
		stopChan:          make(chan struct{}),
		maxBackoff:        5 * time.Minute, // Maximum backoff of 5 minutes (only for errors)
		backoffDuration:   0,               // Start with no backoff
		consecutiveErrors: 0,
	}
}

// SetMessageHandler sets the handler for parsed messages
func (p *Poller) SetMessageHandler(handler MessageHandler) {
	p.messageHandler = handler
}

// SetConnectionChecker sets the connection checker for overlay connection verification
func (p *Poller) SetConnectionChecker(checker ConnectionChecker, overlayID string) {
	p.connectionChecker = checker
	p.overlayID = overlayID
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

// shouldPoll checks if polling should proceed (connection check + quota check)
func (p *Poller) shouldPoll(ctx context.Context) (bool, error) {
	// CONNECTION CHECK: Verify overlay is still connected
	// This prevents wasting 5 units per poll when overlay is disconnected
	if p.connectionChecker != nil && p.overlayID != "" {
		connected, err := p.connectionChecker.IsOverlayConnected(ctx, p.overlayID)

		// METRICS: Track connection check result
		if p.ytMetrics != nil {
			if err != nil {
				p.ytMetrics.PollerConnectionChecks.WithLabelValues("error").Inc()
			} else if connected {
				p.ytMetrics.PollerConnectionChecks.WithLabelValues("connected").Inc()
			} else {
				p.ytMetrics.PollerConnectionChecks.WithLabelValues("disconnected").Inc()
			}
		}

		if err != nil {
			p.logger.Warn("Connection check failed, assuming disconnected",
				zap.String("overlay_id", p.overlayID),
				zap.String("stream_id", p.stream.StreamID),
				zap.Error(err),
			)
			return false, fmt.Errorf("connection check failed: %w", err)
		}

		if !connected {
			// METRICS: Track poller stopped by disconnect and quota saved
			if p.ytMetrics != nil {
				p.ytMetrics.PollerStoppedByDisconnect.WithLabelValues(p.stream.ChannelID).Inc()
				p.ytMetrics.PollerQuotaSaved.WithLabelValues(p.stream.ChannelID).Add(5) // Saved 5 units
			}

			p.logger.Info("Overlay disconnected, stopping poller to save quota",
				zap.String("overlay_id", p.overlayID),
				zap.String("stream_id", p.stream.StreamID),
				zap.String("channel_id", p.stream.ChannelID),
			)
			return false, fmt.Errorf("overlay disconnected")
		}
	}

	return true, nil
}

// pollLoop continuously streams for new messages with exponential backoff only on errors
func (p *Poller) pollLoop(ctx context.Context) {
	defer p.wg.Done()

	// Initial poll
	if shouldPoll, err := p.shouldPoll(ctx); !shouldPoll {
		p.logger.Info("Overlay not connected, skipping initial poll",
			zap.String("stream_id", p.stream.StreamID),
			zap.Error(err),
		)
		return
	}

	for {
		// CONNECTION-AWARE: Check if overlay still connected before streaming
		if shouldPoll, err := p.shouldPoll(ctx); !shouldPoll {
			p.logger.Info("Overlay disconnected during polling, stopping poller immediately",
				zap.String("stream_id", p.stream.StreamID),
				zap.Error(err),
			)
			return
		}

		// Apply exponential backoff only if we have consecutive errors
		if p.backoffDuration > 0 {
			p.logger.Warn("Applying error backoff before next stream request",
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

		pollCtx, cancel := context.WithCancel(ctx)
		monitorDone := make(chan struct{})
		go p.monitorConnection(pollCtx, cancel, monitorDone)

		err := p.poll(pollCtx)
		cancel()
		<-monitorDone

		if err != nil {
			p.handlePollError(err)

			// Check for stream ended - but don't stop immediately
			// Stream might come back (restream, temporary interruption)
			// Instead, apply exponential backoff and let manager handle cleanup
			if strings.Contains(err.Error(), "liveChatEnded") || strings.Contains(err.Error(), "live chat is no longer live") {
				p.logger.Warn("Stream appears ended, applying backoff (will auto-recover if stream returns)",
					zap.String("stream_id", p.stream.StreamID),
					zap.Duration("backoff", p.backoffDuration),
				)
				// Don't return - let backoff handle it
				// Manager will clean up if stream stays offline
			}

			// Check for quota exceeded - STOP ENTIRELY, don't retry
			// Quota resets daily at midnight PT, periodic sync will restart poller after reset
			if strings.Contains(err.Error(), "quotaExceeded") || strings.Contains(err.Error(), "insufficient quota") {
				p.logger.Error("Quota exceeded, stopping poller permanently (will resume after quota reset via periodic sync)",
					zap.String("stream_id", p.stream.StreamID),
				)
				return
			}
		} else {
			// Success - reset error backoff
			p.resetBackoff()
		}

		select {
		case <-p.stopChan:
			return
		default:
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
	response, err := p.apiClient.GetChatMessages(ctx, liveChatID, pageToken, &quota.AuditContext{
		ChannelID: p.stream.ChannelID,
		VideoID:   p.stream.StreamID,
		OverlayID: p.overlayID,
	})
	if err != nil {
		return fmt.Errorf("failed to get chat messages: %w", err)
	}
	if response.OfflineAt != "" {
		return fmt.Errorf("liveChatEnded")
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

func (p *Poller) monitorConnection(ctx context.Context, cancel context.CancelFunc, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			cancel()
			return
		case <-ticker.C:
			if p.connectionChecker == nil || p.overlayID == "" {
				continue
			}
			connected, err := p.connectionChecker.IsOverlayConnected(ctx, p.overlayID)
			if err != nil || !connected {
				cancel()
				return
			}
		}
	}
}

// GetStream returns the current stream state (thread-safe copy)
func (p *Poller) GetStream() *models.YouTubeStream {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy
	streamCopy := *p.stream
	return &streamCopy
}
