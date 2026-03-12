package streams

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/services/youtube-listener/status"
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
	statusPublisher *status.Publisher

	// Connection-aware polling (prevents quota waste when overlay disconnected)
	connectionChecker ConnectionChecker
	overlayID         string
	channelID         string

	mu                sync.RWMutex
	stopChan          chan struct{}
	doneChan          chan struct{} // closed when pollLoop exits (allows caller to detect dead pollers)
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
	statusPublisher *status.Publisher,
) *Poller {
	return &Poller{
		stream:            stream,
		apiClient:         apiClient,
		parser:            api.NewParser(),
		logger:            logger,
		ytMetrics:         ytMetrics,
		tokenStore:        tokenStore,
		statusPublisher:   statusPublisher,
		stopChan:          make(chan struct{}),
		doneChan:          make(chan struct{}),
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

	// Publish offline status when poller stops gracefully
	if p.statusPublisher != nil && p.channelID != "" {
		_ = p.statusPublisher.PublishStatus(context.Background(), status.StatusMessage{
			Platform:  "youtube",
			ChannelID: p.channelID,
			Status:    "offline",
		})
	}

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

// IsDone returns a channel that is closed when the poller's poll loop has exited.
// This allows the Manager to detect zombie pollers (exited but not removed from map).
func (p *Poller) IsDone() <-chan struct{} {
	return p.doneChan
}

// pollLoop continuously streams for new messages with exponential backoff only on errors
func (p *Poller) pollLoop(ctx context.Context) {
	defer close(p.doneChan)
	defer p.wg.Done()

	// CRITICAL FIX: Initialize local page token that Manager cannot overwrite
	// This prevents race condition where Manager's periodic sync overwrites p.stream
	// with stale database token, causing YouTube to reject reconnections
	p.mu.RLock()
	activePageToken := p.stream.NextPageToken
	p.mu.RUnlock()

	p.logger.Info("Poller starting with local token state",
		zap.String("stream_id", p.stream.StreamID),
		zap.Bool("has_initial_token", activePageToken != ""),
		zap.String("token_prefix", truncateString(activePageToken, 20)),
	)

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

		// CRITICAL FIX: Pass local token IN and capture returned token OUT
		// This prevents Manager's periodic sync from corrupting our token state
		nextToken, err := p.poll(pollCtx, activePageToken)
		cancel()
		<-monitorDone

		// CRITICAL FIX: Update local token immediately (immune to Manager sync)
		if nextToken != "" {
			p.logger.Info("Preserving token for next connection",
				zap.String("stream_id", p.stream.StreamID),
				zap.String("token_prefix", truncateString(nextToken, 20)),
				zap.Bool("token_changed", nextToken != activePageToken),
			)
			activePageToken = nextToken
		} else if err == nil {
			// Poll succeeded but returned empty token - keep old token to retry
			p.logger.Warn("Poll returned empty token, keeping previous token for retry",
				zap.String("stream_id", p.stream.StreamID),
				zap.String("token_prefix", truncateString(activePageToken, 20)),
			)
		}
		// If poll failed AND returned empty token, we still keep activePageToken
		// to attempt recovery on next iteration

		// CRITICAL DEBUG: Verify local token state is preserved
		p.logger.Debug("After poll completed, local token state",
			zap.String("stream_id", p.stream.StreamID),
			zap.Bool("has_local_token", activePageToken != ""),
			zap.Int("local_token_length", len(activePageToken)),
			zap.String("local_token_prefix", truncateString(activePageToken, 20)),
		)

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
	// Check if this is a terminal error (stream ended gracefully)
	isTerminalError := strings.Contains(err.Error(), "liveChatEnded") ||
		strings.Contains(err.Error(), "liveChatNotFound") ||
		strings.Contains(err.Error(), "videoNotFound")

	if isTerminalError {
		p.logger.Info("Stream ended or not found, stopping poller",
			zap.String("stream_id", p.stream.StreamID),
			zap.String("channel_id", p.channelID),
			zap.Error(err),
		)

		// Publish offline status (stream ended gracefully)
		if p.statusPublisher != nil && p.channelID != "" {
			_ = p.statusPublisher.PublishStatus(context.Background(), status.StatusMessage{
				Platform:     "youtube",
				ChannelID:    p.channelID,
				Status:       "offline",
				ErrorMessage: "Stream ended",
			})
		}

		// Stop the poller - no point in retrying an ended stream
		go p.Stop()
		return
	}

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

	// Publish reconnecting status if statusPublisher is available
	if p.statusPublisher != nil && p.channelID != "" {
		nextRetry := time.Now().Add(p.backoffDuration)
		_ = p.statusPublisher.PublishStatus(context.Background(), status.StatusMessage{
			Platform:     "youtube",
			ChannelID:    p.channelID,
			Status:       "reconnecting",
			NextRetryAt:  &nextRetry,
			ErrorMessage: err.Error(),
		})
	}
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

		// Publish connected status if statusPublisher is available
		if p.statusPublisher != nil && p.channelID != "" {
			_ = p.statusPublisher.PublishStatus(context.Background(), status.StatusMessage{
				Platform:  "youtube",
				ChannelID: p.channelID,
				Status:    "connected",
			})
		}
	}
}

// poll fetches and processes new messages
// CRITICAL: Accepts pageToken as parameter and returns the latest token received
// This prevents race conditions with Manager's periodic sync overwriting p.stream
func (p *Poller) poll(ctx context.Context, pageToken string) (string, error) {
	// Read liveChatID from shared state (this doesn't change during stream lifecycle)
	p.mu.RLock()
	liveChatID := p.stream.LiveChatID
	p.mu.RUnlock()

	// Track the latest token we receive (starts with input token)
	latestToken := pageToken

	// CRITICAL DEBUG: Log the actual token value we're using
	p.logger.Debug("Starting poll with local token parameter",
		zap.String("stream_id", p.stream.StreamID),
		zap.Bool("has_token", pageToken != ""),
		zap.Int("token_length", len(pageToken)),
		zap.String("token_prefix", truncateString(pageToken, 20)),
	)

	// CRITICAL UNDERSTANDING: pageToken is for RESUMING across reconnections
	//
	// How YouTube StreamList Works:
	// 1. First connection (no token): YouTube sends ~80 messages (history buffer)
	// 2. After 10s: YouTube closes stream with EOF (signals "caught up, reconnect")
	// 3. Reconnect WITH token: YouTube resumes from last position (only NEW messages)
	// 4. Without token: YouTube re-sends same 80 messages → sees as scraper → 10s timeout
	//
	// The 10-second disconnect was caused by CLEARING the token (see line 212 comment)
	// which caused us to re-fetch the same history every 10 seconds.
	//
	// Token is updated on line 410 from each response's nextPageToken field.

	p.mu.Lock()
	p.lastStreamRequest = time.Now()
	p.mu.Unlock()

	// Fetch messages from API
	err := p.apiClient.StreamChatMessages(ctx, liveChatID, pageToken, &quota.AuditContext{
		ChannelID: p.stream.ChannelID,
		VideoID:   p.stream.StreamID,
		OverlayID: p.overlayID,
	}, func(response *youtube.LiveChatMessageListResponse) error {
		handlerEntryTime := time.Now()

		p.logger.Debug("Handler invoked with response",
			zap.String("stream_id", p.stream.StreamID),
			zap.Int("items_count", len(response.Items)),
			zap.Bool("has_offline_at", response.OfflineAt != ""),
		)

		if response.OfflineAt != "" {
			return fmt.Errorf("liveChatEnded")
		}

		// Parse messages
		parseStart := time.Now()
		messages, err := p.parser.ParseBatch(response.Items, p.stream.ChannelID, p.stream.StreamID)
		parseDuration := time.Since(parseStart)

		p.logger.Debug("Message parsing completed",
			zap.String("stream_id", p.stream.StreamID),
			zap.Duration("parse_duration", parseDuration),
			zap.Int("parsed_count", len(messages)),
		)

		if err != nil {
			p.logger.Error("Failed to parse messages",
				zap.String("stream_id", p.stream.StreamID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to parse messages: %w", err)
		}

		// Extract polling interval for sleep (must be done before mutex lock)
		extractStart := time.Now()
		pollingInterval := p.parser.ExtractPollingInterval(response)

		// Extract token from response
		newToken := p.parser.ExtractNextPageToken(response)
		extractDuration := time.Since(extractStart)

		// CRITICAL FIX: Update local latestToken (return value for pollLoop)
		if newToken != "" {
			latestToken = newToken
		}

		// CRITICAL DEBUG: Log token extraction
		p.logger.Debug("Updating local and shared token from response",
			zap.String("stream_id", p.stream.StreamID),
			zap.String("response_token", truncateString(newToken, 20)),
			zap.Int("token_length", len(newToken)),
			zap.Bool("token_changed", newToken != p.stream.NextPageToken),
			zap.String("latest_local_token", truncateString(latestToken, 20)),
			zap.Duration("extract_duration", extractDuration),
		)

		// Update stream state (for UI/logging/status, but NOT used for next poll)
		mutexStart := time.Now()
		p.mu.Lock()
		p.stream.NextPageToken = newToken
		p.stream.PollingInterval = pollingInterval
		p.stream.LastPolledAt = time.Now()
		p.stream.UpdatedAt = time.Now()
		p.mu.Unlock()
		mutexDuration := time.Since(mutexStart)

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
				// CRITICAL: Process messages asynchronously to prevent blocking gRPC receive loop
				// If HandleMessages() blocks (Redis publish ~50-130ms for 75 messages), the gRPC
				// loop can't send WINDOW_UPDATE frames to YouTube. After ~10 seconds of stalling,
				// YouTube's flow control watchdog kills the connection.
				//
				// By spawning a goroutine, the gRPC loop continues immediately, keeps sending
				// WINDOW_UPDATE frames, and maintains the connection for hours.
				messagesCopy := make([]*models.RawChatMessage, len(messages))
				copy(messagesCopy, messages)
				go func() {
					publishStart := time.Now()
					if err := p.messageHandler.HandleMessages(context.Background(), messagesCopy); err != nil {
						p.logger.Error("Failed to handle messages",
							zap.String("stream_id", p.stream.StreamID),
							zap.Error(err),
						)
						// Don't return error - we don't want to kill the stream if publishing fails
					}
					publishDuration := time.Since(publishStart)
					p.logger.Debug("Message publishing completed (async)",
						zap.String("stream_id", p.stream.StreamID),
						zap.Duration("publish_duration", publishDuration),
						zap.Int("messages_count", len(messagesCopy)),
					)
				}()
			}
		}

		// Calculate total handler time (synchronous portion only)
		totalHandlerTime := time.Since(handlerEntryTime)
		p.logger.Debug("Handler returning (sync work completed)",
			zap.String("stream_id", p.stream.StreamID),
			zap.Duration("total_handler_time", totalHandlerTime),
			zap.Duration("parse_time", parseDuration),
			zap.Duration("extract_time", extractDuration),
			zap.Duration("mutex_time", mutexDuration),
			zap.Int("messages_count", len(messages)),
		)

		// CRITICAL WARNING: If total handler time >500ms, we're at risk of blocking
		if totalHandlerTime > 500*time.Millisecond {
			p.logger.Warn("Handler sync work is SLOW - may block gRPC receive loop",
				zap.String("stream_id", p.stream.StreamID),
				zap.Duration("total_handler_time", totalHandlerTime),
				zap.Int("items_processed", len(response.Items)),
			)
		}

		// NOTE: For gRPC streaming, DO NOT sleep between responses!
		// The pollingIntervalMillis is only for REST API polling mode.
		// Sleeping inside the handler blocks the gRPC receive loop, causing YouTube's
		// server to close the connection due to inactivity (~15s timeout).
		// The gRPC stream should continuously call Recv() to maintain the connection.

		return nil
	})
	if err != nil {
		return latestToken, fmt.Errorf("failed to stream chat messages: %w", err)
	}

	return latestToken, nil
}

func (p *Poller) monitorConnection(ctx context.Context, cancel context.CancelFunc, disconnected chan<- struct{}, done chan<- struct{}) {
	defer close(done)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Track consecutive disconnects for debounce
	consecutiveDisconnects := 0
	disconnectThreshold := 2
	if envThreshold := os.Getenv("YOUTUBE_DISCONNECT_THRESHOLD_COUNT"); envThreshold != "" {
		if count, err := strconv.Atoi(envThreshold); err == nil && count > 0 {
			disconnectThreshold = count
		}
	}

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
				consecutiveDisconnects++

				if consecutiveDisconnects >= disconnectThreshold {
					// Stop after N consecutive failures
					p.logger.Info("Disconnect confirmed after grace period",
						zap.String("channel_id", p.channelID),
						zap.Int("consecutive_failures", consecutiveDisconnects),
					)
					select {
					case disconnected <- struct{}{}:
					default:
					}
					cancel()
					return
				}

				p.logger.Warn("Disconnect detected, waiting for confirmation",
					zap.String("channel_id", p.channelID),
					zap.Int("consecutive_failures", consecutiveDisconnects),
					zap.Int("threshold", disconnectThreshold),
				)
			} else {
				// Reset counter on successful connection check
				if consecutiveDisconnects > 0 {
					p.logger.Info("Connection restored after temporary disconnect",
						zap.String("channel_id", p.channelID),
						zap.Int("previous_failures", consecutiveDisconnects),
					)
				}
				consecutiveDisconnects = 0
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

// truncateString returns first n characters of string (for logging)
func truncateString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
