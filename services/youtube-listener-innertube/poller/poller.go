package poller

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
)

// ClientInterface defines the methods needed from innertube.Client for testing
type ClientInterface interface {
	GetLiveChatReplay(ctx context.Context, continuation string, visitorData string) (*innertube.LiveChatResponse, error)
	ExtractContinuation(resp *innertube.LiveChatResponse) string
	GetPollInterval(resp *innertube.LiveChatResponse) time.Duration
}

// ContinuationRefresher can fetch a fresh continuation token for a video.
// Used to recover when a continuation goes stale (persistent zero-action responses).
type ContinuationRefresher interface {
	GetInitialContinuation(ctx context.Context, videoID, channelID string) (string, string, error)
}

// MessageCallback is called with parsed messages after each successful poll
type MessageCallback func(messages []*innertube.RawChatMessage)

// Poller manages the continuation-based polling loop for InnerTube live chat
//
// Architecture:
//   - Fixed 2-second polling interval (not adaptive)
//   - Exponential backoff for transient errors (2s → 60s max)
//   - Fatal errors stop polling immediately
//   - Offline detection via empty continuation array
//   - Graceful shutdown via context cancellation (5-second timeout)
//
// State transitions:
//   - Active → Failed (fatal error)
//   - Active → Offline (stream ended, detected via DetectOffline)
//   - Active → Active (transient error, backoff, resume)
type Poller struct {
	client          ClientInterface
	continuation    string
	visitorData     string
	channelID       string
	videoID         string // Current video ID being polled
	interval        time.Duration
	backoff         *Backoff
	state           *State
	repository      *Repository        // Redis repository for lifecycle operations
	publisher       LifecyclePublisher // Optional: publishes stream end events
	logger          *zap.Logger
	logLevel        string // "debug" or "info"
	messageCallback MessageCallback
	metrics         *metrics.InnerTubeMetrics
	refresher       ContinuationRefresher // Optional: re-fetches continuation when stale

	// Stale-continuation detection: refresh after this many consecutive zero-action polls
	zeroActionCount      int
	zeroActionThreshold  int

	// Empty-continuation detection: attempt refresh before declaring offline
	emptyContCount     int
	emptyContThreshold int

	// Graceful shutdown
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	doneChan chan struct{} // closed when pollingLoop exits
}

// PollerOptions configures the Poller
type PollerOptions struct {
	// Interval is the minimum polling interval (default: 2s).
	// The actual interval will be max(Interval, YouTube's recommended timeoutDurationMillis).
	Interval time.Duration

	// LogLevel controls verbosity: "debug" or "info" (default: "info")
	LogLevel string

	// VideoID is the current video ID being polled (optional)
	VideoID string

	// VisitorData is the YouTube visitor data token required by get_live_chat endpoint
	VisitorData string

	// Repository for lifecycle operations (optional, for offline detection)
	Repository *Repository

	// Publisher for lifecycle events (optional, for stream end notifications)
	Publisher LifecyclePublisher

	// Metrics for Prometheus instrumentation (optional)
	Metrics *metrics.InnerTubeMetrics

	// Refresher fetches a fresh continuation token when the current one goes stale.
	// When ZeroActionThreshold consecutive polls return 0 actions, the continuation
	// is re-fetched from the YouTube /next API so the poller re-anchors to live position.
	Refresher ContinuationRefresher

	// ZeroActionThreshold is the number of consecutive zero-action polls before
	// attempting a continuation refresh (default: 150, ~5 minutes at 2s interval).
	ZeroActionThreshold int

	// EmptyContThreshold is the number of consecutive empty-continuation responses
	// before declaring the stream offline. On each attempt the poller first tries to
	// obtain a fresh continuation via Refresher; only once Refresher also fails (or
	// is not configured) do we count towards the threshold. Defaults to 3.
	EmptyContThreshold int
}

// NewPoller creates a new polling loop manager
//
// Parameters:
//   - client: InnerTube HTTP client for API requests
//   - initialContinuation: Initial continuation token from stream HTML
//   - channelID: YouTube channel ID for message attribution
//   - logger: Structured logger
//   - opts: Optional configuration (nil for defaults)
//
// Defaults:
//   - Interval: 2 seconds
//   - LogLevel: "info"
func NewPoller(
	client ClientInterface,
	initialContinuation string,
	channelID string,
	logger *zap.Logger,
	opts *PollerOptions,
) *Poller {
	if opts == nil {
		opts = &PollerOptions{}
	}

	// Default interval: 2 seconds minimum
	interval := opts.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}

	// Default log level: info
	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	// Default zero-action threshold: 150 polls (~5 minutes at 2s)
	zeroActionThreshold := opts.ZeroActionThreshold
	if zeroActionThreshold == 0 {
		zeroActionThreshold = 150
	}

	// Default empty-continuation threshold: 3 attempts (~6s at 2s interval)
	emptyContThreshold := opts.EmptyContThreshold
	if emptyContThreshold == 0 {
		emptyContThreshold = 3
	}

	return &Poller{
		client:              client,
		continuation:        initialContinuation,
		visitorData:         opts.VisitorData,
		channelID:           channelID,
		videoID:             opts.VideoID,
		interval:            interval,
		backoff:             NewBackoff(logger),
		state:               NewState(),
		repository:          opts.Repository,
		publisher:           opts.Publisher,
		logger:              logger,
		logLevel:            logLevel,
		metrics:             opts.Metrics,
		refresher:           opts.Refresher,
		zeroActionThreshold: zeroActionThreshold,
		emptyContThreshold:  emptyContThreshold,
		doneChan:            make(chan struct{}),
	}
}

// Start begins the polling loop in a background goroutine
//
// The loop:
//   1. Waits for interval ticker
//   2. Calls InnerTube API with current continuation token
//   3. Parses messages and updates continuation
//   4. Handles errors (transient → backoff, fatal → stop)
//   5. Repeats until context is cancelled
//
// Returns immediately. Call Stop() to gracefully shut down.
func (p *Poller) Start(ctx context.Context) error {
	// Create cancellable context for this poller
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start polling loop in background
	p.wg.Add(1)
	go p.pollingLoop()

	p.logger.Info("Poller started",
		zap.String("channel_id", p.channelID),
		zap.Duration("interval", p.interval),
		zap.String("log_level", p.logLevel))

	return nil
}

// Stop gracefully shuts down the polling loop
//
// Waits for the current poll to complete with a 5-second timeout.
// If polling doesn't exit within timeout, returns immediately (force exit).
// This ensures the 25-second Kubernetes termination deadline is respected.
func (p *Poller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}

	// Wait for polling goroutine with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Graceful shutdown completed
		p.logger.Info("Poller stopped gracefully",
			zap.String("final_state", string(p.state.GetState())))
	case <-time.After(5 * time.Second):
		// Force exit after timeout
		p.logger.Warn("Poller force exit after timeout",
			zap.Duration("timeout", 5*time.Second),
			zap.String("final_state", string(p.state.GetState())))
	}
}

// GetState returns the current polling state
func (p *Poller) GetState() StreamState {
	return p.state.GetState()
}

// SetMessageCallback sets the callback function to be called with parsed messages
// This callback is invoked after each successful poll with any messages received
func (p *Poller) SetMessageCallback(callback MessageCallback) {
	p.messageCallback = callback
}

// IsDone returns a channel that is closed when the polling loop has exited.
// The manager uses this to detect pollers that self-terminated (e.g. stream offline).
func (p *Poller) IsDone() <-chan struct{} {
	return p.doneChan
}

// pollingLoop is the main polling loop (runs in background goroutine)
func (p *Poller) pollingLoop() {
	defer close(p.doneChan)
	defer p.wg.Done()

	p.logger.Debug("Polling loop started")

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Info("Polling loop shutting down",
				zap.String("reason", "context_cancelled"))
			return
		default:
		}

		p.poll()
	}
}

// poll executes a single poll iteration, then sleeps for the YouTube-recommended interval.
func (p *Poller) poll() {
	// Call InnerTube API
	resp, err := p.client.GetLiveChatReplay(p.ctx, p.continuation, p.visitorData)
	if err != nil {
		p.handleError(err)
		p.sleep(p.interval)
		return
	}

	// Nil response check
	if resp == nil {
		p.logger.Warn("Received nil response from InnerTube API")
		p.sleep(p.interval)
		return
	}

	// Offline detection: Check for empty continuation array (stream ended).
	// Before declaring the stream offline we attempt to fetch a fresh continuation
	// token – YouTube occasionally sends an empty continuation array transiently
	// even while the stream is live. Only after emptyContThreshold consecutive
	// failed recovery attempts do we treat the stream as genuinely offline.
	if DetectOffline(resp) {
		if p.refresher != nil && p.videoID != "" {
			p.emptyContCount++
			p.logger.Info("Empty continuation received, attempting token refresh",
				zap.String("channel_id", p.channelID),
				zap.String("video_id", p.videoID),
				zap.Int("attempt", p.emptyContCount),
				zap.Int("threshold", p.emptyContThreshold),
			)
			if fresh, freshVisitorData, err := p.refresher.GetInitialContinuation(p.ctx, p.videoID, p.channelID); err == nil {
				// Stream is still live – resume with the fresh token.
				p.continuation = fresh
				if freshVisitorData != "" {
					p.visitorData = freshVisitorData
				}
				p.emptyContCount = 0
				p.logger.Info("Stream still live, resuming with fresh continuation",
					zap.String("channel_id", p.channelID),
					zap.String("video_id", p.videoID),
				)
				p.sleep(p.interval)
				return
			} else {
				p.logger.Warn("Continuation refresh failed after empty response",
					zap.String("channel_id", p.channelID),
					zap.String("video_id", p.videoID),
					zap.Int("attempt", p.emptyContCount),
					zap.Error(err),
				)
			}

			if p.emptyContCount < p.emptyContThreshold {
				// Not yet at threshold – keep retrying on the next poll cycle.
				p.sleep(p.interval)
				return
			}

			p.logger.Info("Stream confirmed offline after repeated empty continuations",
				zap.String("channel_id", p.channelID),
				zap.String("video_id", p.videoID),
				zap.Int("empty_cont_count", p.emptyContCount),
			)
		} else {
			p.logger.Info("Stream went offline (empty continuation)",
				zap.String("channel_id", p.channelID),
				zap.String("video_id", p.videoID),
			)
		}

		p.state.SetState(StateOffline)
		p.state.SetError(nil)

		if p.repository != nil && p.videoID != "" {
			if err := HandleStreamOffline(p.ctx, p.channelID, p.videoID, p.repository, p.publisher, p.logger); err != nil {
				p.logger.Debug("HandleStreamOffline completed",
					zap.String("channel_id", p.channelID))
			}
		}

		if p.cancel != nil {
			p.cancel()
		}
		return
	}

	// Got a valid continuation – reset the empty-continuation counter.
	p.emptyContCount = 0

	// Extract continuation token for next poll
	nextContinuation := p.client.ExtractContinuation(resp)
	if nextContinuation == "" {
		p.logger.Info("Stream ended (no continuation token)")
		p.state.SetState(StateOffline)
		p.state.SetError(nil)

		if p.repository != nil && p.videoID != "" {
			_ = HandleStreamOffline(p.ctx, p.channelID, p.videoID, p.repository, p.publisher, p.logger)
		}

		if p.cancel != nil {
			p.cancel()
		}
		return
	}

	// Determine sleep interval: respect YouTube's recommended timeout, floor at p.interval
	sleepDuration := p.interval
	if ytInterval := p.client.GetPollInterval(resp); ytInterval > sleepDuration {
		sleepDuration = ytInterval
	}

	// Parse messages
	var actions []innertube.ChatAction
	if resp.ContinuationContents.LiveChatContinuation.Actions != nil {
		actions = resp.ContinuationContents.LiveChatContinuation.Actions
	}
	messages, err := innertube.ParseMessages(actions, p.channelID)
	if err != nil {
		p.logger.Warn("Failed to parse messages",
			zap.Error(err),
			zap.Int("action_count", len(actions)))
	}

	if p.logLevel == "debug" && len(messages) > 0 {
		p.logger.Debug("Parsed messages", zap.Int("count", len(messages)))
		for _, msg := range messages {
			p.logger.Debug("Message",
				zap.String("user", msg.Username),
				zap.String("text", msg.Text))
		}
	}

	// Update state
	p.continuation = nextContinuation
	p.state.SetState(StateActive)
	p.state.SetError(nil)
	p.state.UpdatePollTime()
	p.backoff.Reset()

	// Track consecutive zero-action polls and refresh continuation if stale
	if len(actions) == 0 {
		p.zeroActionCount++
		if p.refresher != nil && p.videoID != "" && p.zeroActionCount >= p.zeroActionThreshold {
			p.zeroActionCount = 0
			p.logger.Info("Continuation appears stale, refreshing from YouTube /next API",
				zap.String("channel_id", p.channelID),
				zap.String("video_id", p.videoID),
				zap.Int("zero_action_polls", p.zeroActionThreshold),
			)
			if fresh, freshVisitorData, err := p.refresher.GetInitialContinuation(p.ctx, p.videoID, p.channelID); err != nil {
				// If the error indicates the stream ended (no liveChatRenderer),
				// treat it as offline rather than continuing to poll forever.
				errStr := err.Error()
				if strings.Contains(errStr, "stream may have ended") || strings.Contains(errStr, "isReplay") {
					p.logger.Info("Stream ended during stale-continuation refresh",
						zap.String("channel_id", p.channelID),
						zap.String("video_id", p.videoID),
						zap.Error(err),
					)
					p.state.SetState(StateOffline)
					p.state.SetError(nil)
					if p.repository != nil && p.videoID != "" {
						if err := HandleStreamOffline(p.ctx, p.channelID, p.videoID, p.repository, p.publisher, p.logger); err != nil {
							p.logger.Debug("HandleStreamOffline completed",
								zap.String("channel_id", p.channelID))
						}
					}
					if p.cancel != nil {
						p.cancel()
					}
					return
				}
				p.logger.Warn("Failed to refresh continuation, continuing with current token",
					zap.String("video_id", p.videoID),
					zap.Error(err),
				)
			} else {
				p.continuation = fresh
				if freshVisitorData != "" {
					p.visitorData = freshVisitorData
				}
				p.logger.Info("Continuation refreshed successfully",
					zap.String("channel_id", p.channelID),
					zap.String("video_id", p.videoID),
				)
			}
		}
	} else {
		p.zeroActionCount = 0
	}

	// Call message callback
	if p.messageCallback != nil && len(messages) > 0 {
		p.messageCallback(messages)
	}

	// Only sleep when the poll returned no messages. When messages arrived,
	// re-poll immediately so the next batch is fetched without an artificial delay.
	// This reduces average viewer latency from ~(interval/2) to near zero on
	// active streams, while still backing off on quiet/idle channels.
	if len(messages) == 0 {
		p.sleep(sleepDuration)
	}
}

// sleep blocks for d or until context is cancelled.
func (p *Poller) sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-p.ctx.Done():
	}
}

// handleError processes errors from InnerTube API calls
func (p *Poller) handleError(err error) {
	if innertube.IsFatalError(err) {
		// Fatal errors: stop polling immediately
		p.logger.Error("Fatal error, stopping poller",
			zap.Error(err),
			zap.String("channel_id", p.channelID))

		p.state.SetState(StateFailed)
		p.state.SetError(err)

		// Stop polling (context cancel stops the loop)
		if p.cancel != nil {
			p.cancel()
		}
		return
	}

	if innertube.IsTransientError(err) {
		// Transient errors: apply backoff and continue
		p.logger.Warn("Transient error encountered",
			zap.Error(err),
			zap.String("channel_id", p.channelID))

		p.state.SetError(err)

		// Track reconnection attempt due to error
		if p.metrics != nil {
			p.metrics.Reconnections.WithLabelValues(
				metrics.ServiceLabel,
				p.channelID,
				metrics.ReconnectionReasonError,
			).Inc()
		}

		// Wait with exponential backoff
		backoffErr := p.backoff.Wait(p.ctx, err)
		if backoffErr != nil {
			// Context cancelled during backoff
			p.logger.Info("Backoff interrupted by shutdown",
				zap.Error(backoffErr))
			return
		}

		// After backoff, continue polling (state remains Active)
		return
	}

	// Unknown error type (should not happen, but log it)
	p.logger.Warn("Unknown error type encountered",
		zap.Error(err),
		zap.String("channel_id", p.channelID))

	p.state.SetError(err)

	// Treat as transient (apply backoff)
	_ = p.backoff.Wait(p.ctx, err)
}
