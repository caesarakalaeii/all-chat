package streams

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
	"google.golang.org/api/youtube/v3"
)

// MessageHandler defines the interface for handling parsed messages
type MessageHandler interface {
	HandleMessages(ctx context.Context, messages []*models.RawChatMessage) error
}

// ConnectionChecker defines the interface for checking if a channel still has connected overlays
type ConnectionChecker interface {
	IsChannelConnected(ctx context.Context, channelID string) (bool, error)
}

// Poller polls a YouTube live stream for chat messages
type Poller struct {
	stream         *models.YouTubeStream
	apiClient      *api.Client
	parser         *api.Parser
	messageHandler MessageHandler
	logger         *zap.Logger
	ytMetrics      *metrics.YouTubeMetrics
	tokenStore     *TokenStore

	// Connection-aware polling (prevents quota waste when overlay disconnected)
	connectionChecker ConnectionChecker
	overlayID         string
	channelID         string

	mu                sync.RWMutex
	stopChan          chan struct{}
	wg                sync.WaitGroup
	lastStreamRequest time.Time

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
	tokenStore *TokenStore,
) *Poller {
	return &Poller{
		stream:            stream,
		apiClient:         apiClient,
		parser:            api.NewParser(),
		logger:            logger,
		ytMetrics:         ytMetrics,
		tokenStore:        tokenStore,
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
func (p *Poller) SetConnectionChecker(checker ConnectionChecker, channelID, overlayID string) {
	p.connectionChecker = checker
	p.overlayID = overlayID
	p.channelID = channelID
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
	if p.connectionChecker != nil && p.channelID != "" {
		connected, err := p.connectionChecker.IsChannelConnected(ctx, p.channelID)

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
				zap.String("channel_id", p.channelID),
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
				zap.String("channel_id", p.channelID),
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
		disconnectCh := make(chan struct{}, 1)
		go p.monitorConnection(pollCtx, cancel, disconnectCh, monitorDone)

		if p.ytMetrics != nil {
			p.ytMetrics.StreamConnectionsStarted.WithLabelValues(p.stream.ChannelID, p.stream.StreamID).Inc()
			p.ytMetrics.StreamConnectionsActive.WithLabelValues(p.stream.ChannelID, p.stream.StreamID).Inc()
		}

		// FIX: Track stream connection duration to detect rapid disconnections
		streamStartTime := time.Now()

		err := p.poll(pollCtx)
		cancel()
		<-monitorDone

		// FIX: Calculate stream connection duration
		connectionDuration := time.Since(streamStartTime)

		if p.ytMetrics != nil {
			p.ytMetrics.StreamConnectionsActive.WithLabelValues(p.stream.ChannelID, p.stream.StreamID).Dec()
			reason := streamEndReason(err, disconnectCh, p.stopChan)
			p.ytMetrics.StreamConnectionsEnded.WithLabelValues(p.stream.ChannelID, p.stream.StreamID, reason).Inc()
			if err != nil {
				p.ytMetrics.StreamErrors.WithLabelValues(p.stream.ChannelID, p.stream.StreamID, classifyStreamError(err)).Inc()
			}
		}

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

			// FIX: Apply minimum reconnection delay if stream ended very quickly
			// This indicates a connection issue rather than normal stream behavior
			const minConnectionDuration = 10 * time.Second
			const minReconnectDelay = 2 * time.Second

			if connectionDuration < minConnectionDuration {
				p.logger.Warn("Stream connection ended quickly, applying minimum reconnection delay",
					zap.String("stream_id", p.stream.StreamID),
					zap.Duration("connection_duration", connectionDuration),
					zap.Duration("min_reconnect_delay", minReconnectDelay),
				)

				select {
				case <-time.After(minReconnectDelay):
					// Delay completed
				case <-p.stopChan:
					return
				}
			}
		} else {
			// Success - reset error backoff
			p.resetBackoff()
		}

		p.sleepToRespectInterval()

		if err != nil && p.ytMetrics != nil {
			p.ytMetrics.StreamReconnects.WithLabelValues(p.stream.ChannelID, p.stream.StreamID).Inc()
		}

		select {
		case <-p.stopChan:
			return
		default:
		}
	}
}

func (p *Poller) sleepToRespectInterval() {
	p.mu.RLock()
	interval := time.Duration(p.stream.PollingInterval) * time.Millisecond
	lastRequest := p.lastStreamRequest
	p.mu.RUnlock()

	if interval <= 0 || lastRequest.IsZero() {
		return
	}

	elapsed := time.Since(lastRequest)
	if elapsed >= interval {
		return
	}

	wait := interval - elapsed
	if p.ytMetrics != nil {
		p.ytMetrics.StreamIntervalSleeps.WithLabelValues(p.stream.ChannelID, p.stream.StreamID).Inc()
	}
	select {
	case <-time.After(wait):
	case <-p.stopChan:
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

	if strings.Contains(err.Error(), "rateLimitExceeded") || strings.Contains(err.Error(), "resourceExhausted") {
		if p.backoffDuration < time.Minute {
			p.backoffDuration = time.Minute
		}
	}

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

	// FIX: Do NOT load cached pageToken on every connection!
	// The pageToken should only be used WITHIN a streaming session to track progress.
	// Loading a cached token causes YouTube to send "catch-up" messages and close
	// the stream after ~10 seconds, resulting in excessive quota consumption.
	//
	// IMPORTANT: pageToken is automatically updated from each response (line 404)
	// and persisted to Redis (line 410). On pod restart, streams are detected
	// fresh without cached tokens, which is the correct behavior.
	//
	// Removed: Loading cached pageToken from TokenStore
	// This was causing the 10-second disconnect issue described in
	// YOUTUBE_GRPC_STREAMING_CHECKPOINT.md

	p.mu.Lock()
	p.lastStreamRequest = time.Now()
	p.mu.Unlock()

	// Fetch messages from API
	err := p.apiClient.StreamChatMessages(ctx, liveChatID, pageToken, &quota.AuditContext{
		ChannelID: p.stream.ChannelID,
		VideoID:   p.stream.StreamID,
		OverlayID: p.overlayID,
	}, func(response *youtube.LiveChatMessageListResponse) error {
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

		// Extract polling interval for sleep (must be done before mutex lock)
		pollingInterval := p.parser.ExtractPollingInterval(response)

		// Update stream state
		p.mu.Lock()
		p.stream.NextPageToken = p.parser.ExtractNextPageToken(response)
		p.stream.PollingInterval = pollingInterval
		p.stream.LastPolledAt = time.Now()
		p.stream.UpdatedAt = time.Now()
		p.mu.Unlock()

		// FIX: Removed pageToken persistence to Redis.
		// We no longer cache pageTokens across connections since this was causing
		// the 10-second disconnect issue. The token is tracked in-memory during
		// the streaming session, which is the correct approach for gRPC streaming.

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

		// NOTE: For gRPC streaming, DO NOT sleep between responses!
		// The pollingIntervalMillis is only for REST API polling mode.
		// Sleeping inside the handler blocks the gRPC receive loop, causing YouTube's
		// server to close the connection due to inactivity (~15s timeout).
		// The gRPC stream should continuously call Recv() to maintain the connection.

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to stream chat messages: %w", err)
	}

	return nil
}

func (p *Poller) monitorConnection(ctx context.Context, cancel context.CancelFunc, disconnected chan<- struct{}, done chan<- struct{}) {
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
			if p.connectionChecker == nil || p.channelID == "" {
				continue
			}
			connected, err := p.connectionChecker.IsChannelConnected(ctx, p.channelID)
			if err != nil || !connected {
				select {
				case disconnected <- struct{}{}:
				default:
				}
				cancel()
				return
			}
		}
	}
}

func streamEndReason(err error, disconnected <-chan struct{}, stop <-chan struct{}) string {
	if err == nil {
		return "completed"
	}

	select {
	case <-disconnected:
		return "overlay_disconnected"
	default:
	}

	select {
	case <-stop:
		return "stopped"
	default:
	}

	if strings.Contains(err.Error(), "liveChatEnded") || strings.Contains(err.Error(), "live chat is no longer live") {
		return "live_chat_ended"
	}
	if strings.Contains(err.Error(), "quotaExceeded") || strings.Contains(err.Error(), "insufficient quota") {
		return "quota_exceeded"
	}
	if strings.Contains(err.Error(), "rateLimitExceeded") {
		return "rate_limit"
	}
	if strings.Contains(err.Error(), "liveChatDisabled") {
		return "live_chat_disabled"
	}
	if strings.Contains(err.Error(), "liveChatNotFound") || strings.Contains(err.Error(), "notFound") {
		return "not_found"
	}
	if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
		return "canceled"
	}

	return "error"
}

func classifyStreamError(err error) string {
	if err == nil {
		return "none"
	}
	if strings.Contains(err.Error(), "liveChatEnded") || strings.Contains(err.Error(), "live chat is no longer live") {
		return "live_chat_ended"
	}
	if strings.Contains(err.Error(), "quotaExceeded") || strings.Contains(err.Error(), "insufficient quota") {
		return "quota_exceeded"
	}
	if strings.Contains(err.Error(), "rateLimitExceeded") {
		return "rate_limit"
	}
	if strings.Contains(err.Error(), "liveChatDisabled") {
		return "live_chat_disabled"
	}
	if strings.Contains(err.Error(), "liveChatNotFound") || strings.Contains(err.Error(), "notFound") {
		return "not_found"
	}
	if errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled") {
		return "canceled"
	}

	return "error"
}

// GetStream returns the current stream state (thread-safe copy)
func (p *Poller) GetStream() *models.YouTubeStream {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy
	streamCopy := *p.stream
	return &streamCopy
}
