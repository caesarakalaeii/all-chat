package poller

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
)

// ClientInterface defines the methods needed from innertube.Client for testing
type ClientInterface interface {
	GetLiveChatReplay(ctx context.Context, continuation string) (*innertube.LiveChatResponse, error)
	ExtractContinuation(resp *innertube.LiveChatResponse) string
	GetPollInterval(resp *innertube.LiveChatResponse) int
}

// Poller manages the continuation-based polling loop for InnerTube live chat
//
// Architecture:
//   - Fixed 2-second polling interval (not adaptive)
//   - Exponential backoff for transient errors (2s → 60s max)
//   - Fatal errors stop polling immediately
//   - Graceful shutdown via context cancellation
//
// State transitions:
//   - Active → Failed (fatal error)
//   - Active → Offline (stream ended)
//   - Active → Active (transient error, backoff, resume)
type Poller struct {
	client       ClientInterface
	continuation string
	channelID    string
	interval     time.Duration
	backoff      *Backoff
	state        *State
	logger       *zap.Logger
	logLevel     string // "debug" or "info"

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PollerOptions configures the Poller
type PollerOptions struct {
	// Interval is the fixed polling interval (default: 2s)
	Interval time.Duration

	// LogLevel controls verbosity: "debug" or "info" (default: "info")
	LogLevel string
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

	// Default interval: 2 seconds (user decision: fixed, not adaptive)
	interval := opts.Interval
	if interval == 0 {
		interval = 2 * time.Second
	}

	// Default log level: info
	logLevel := opts.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	return &Poller{
		client:       client,
		continuation: initialContinuation,
		channelID:    channelID,
		interval:     interval,
		backoff:      NewBackoff(logger),
		state:        NewState(),
		logger:       logger,
		logLevel:     logLevel,
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
// Waits for the current poll to complete, then returns.
// Blocks until polling goroutine exits (max ~2s for current poll).
func (p *Poller) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()

	p.logger.Info("Poller stopped gracefully",
		zap.String("final_state", string(p.state.GetState())))
}

// GetState returns the current polling state
func (p *Poller) GetState() StreamState {
	return p.state.GetState()
}

// pollingLoop is the main polling loop (runs in background goroutine)
func (p *Poller) pollingLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.logger.Debug("Polling loop started")

	for {
		select {
		case <-p.ctx.Done():
			// Graceful shutdown requested
			p.logger.Info("Polling loop shutting down",
				zap.String("reason", "context_cancelled"))
			return

		case <-ticker.C:
			// Execute poll iteration
			p.poll()
		}
	}
}

// poll executes a single poll iteration
func (p *Poller) poll() {
	if p.logLevel == "debug" {
		// Debug mode: log every poll attempt
		continuationPreview := p.continuation
		if len(continuationPreview) > 20 {
			continuationPreview = continuationPreview[:20] + "..."
		}
		p.logger.Debug("Polling InnerTube",
			zap.String("continuation", continuationPreview))
	}

	// Call InnerTube API
	resp, err := p.client.GetLiveChatReplay(p.ctx, p.continuation)
	if err != nil {
		p.handleError(err)
		return
	}

	// Nil response check
	if resp == nil {
		p.logger.Warn("Received nil response from InnerTube API")
		return
	}

	// Extract continuation token for next poll
	nextContinuation := p.client.ExtractContinuation(resp)
	if nextContinuation == "" {
		// No continuation token = stream ended
		p.logger.Info("Stream ended (no continuation token)")
		p.state.SetState(StateOffline)
		p.state.SetError(nil)
		return
	}

	// Parse messages using package-level ParseMessages function
	// Check if continuation contents exist before parsing
	var actions []innertube.ChatAction
	if resp.ContinuationContents.LiveChatContinuation.Actions != nil {
		actions = resp.ContinuationContents.LiveChatContinuation.Actions
	}
	messages, err := innertube.ParseMessages(actions, p.channelID)
	if err != nil {
		p.logger.Warn("Failed to parse messages",
			zap.Error(err),
			zap.Int("action_count", len(actions)))
		// Continue polling despite parse errors (non-fatal)
	}

	// Log parsed messages in debug mode
	if p.logLevel == "debug" && len(messages) > 0 {
		p.logger.Debug("Parsed messages",
			zap.Int("count", len(messages)))
		for _, msg := range messages {
			p.logger.Debug("Message",
				zap.String("user", msg.Username),
				zap.String("text", msg.Text))
		}
	}

	// Update state after successful poll
	p.continuation = nextContinuation
	p.state.SetState(StateActive)
	p.state.SetError(nil)
	p.state.UpdatePollTime()
	p.backoff.Reset()

	// TODO Phase 03: Publish messages to Redis Streams here
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
