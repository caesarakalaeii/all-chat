package quota

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// RequestPriority defines the priority level of an API request
type RequestPriority int

const (
	// PriorityHigh - Active stream polling (always allowed unless depleted)
	PriorityHigh RequestPriority = iota

	// PriorityNormal - Regular discovery for inactive channels
	PriorityNormal

	// PriorityLow - Low-priority background discovery
	PriorityLow
)

// RequestType defines the type of API operation
type RequestType string

const (
	RequestTypePolling   RequestType = "polling"
	RequestTypeDiscovery RequestType = "discovery"
	RequestTypeSearch    RequestType = "search"
)

// DecisionReason explains why a request was allowed or denied
type DecisionReason string

const (
	ReasonAllowed                DecisionReason = "allowed"
	ReasonGlobalQuotaDepleted    DecisionReason = "global_quota_depleted"
	ReasonGlobalQuotaExhausted   DecisionReason = "global_quota_exhausted"
	ReasonGlobalQuotaCritical    DecisionReason = "global_quota_critical"
	ReasonGlobalQuotaDegraded    DecisionReason = "global_quota_degraded"
	ReasonChannelQuotaExceeded   DecisionReason = "channel_quota_exceeded"
	ReasonLowPriorityBlocked     DecisionReason = "low_priority_blocked"
	ReasonDiscoveryDisabled      DecisionReason = "discovery_disabled"
)

// RequestDecision represents the result of a quota check
type RequestDecision struct {
	Allowed       bool
	Reason        DecisionReason
	GlobalState   QuotaState
	ChannelUsage  int
	ChannelLimit  int
	RetryAfter    *time.Duration // Optional: how long to wait before retry
}

// Coordinator unifies quota decisions from global and per-channel trackers
type Coordinator struct {
	globalTracker   *Tracker
	perChannelTracker *PerChannelTracker
	logger          *zap.Logger
}

// NewCoordinator creates a new quota coordinator
func NewCoordinator(
	globalTracker *Tracker,
	perChannelTracker *PerChannelTracker,
	logger *zap.Logger,
) *Coordinator {
	return &Coordinator{
		globalTracker:   globalTracker,
		perChannelTracker: perChannelTracker,
		logger:          logger,
	}
}

// CanMakeRequest checks if a request should be allowed based on:
// - Global quota state
// - Per-channel quota allocation
// - Request priority
// - Request type
func (c *Coordinator) CanMakeRequest(
	ctx context.Context,
	channelID string,
	requestType RequestType,
	priority RequestPriority,
	cost int,
) RequestDecision {
	// Get global state
	globalState := c.globalTracker.GetState()

	// Check global hard blocks first
	if globalState == QuotaStateDepleted {
		c.logger.Debug("Request denied - global quota depleted",
			zap.String("channel_id", channelID),
			zap.String("request_type", string(requestType)),
		)
		return RequestDecision{
			Allowed:     false,
			Reason:      ReasonGlobalQuotaDepleted,
			GlobalState: globalState,
		}
	}

	// Apply state-based restrictions
	switch globalState {
	case QuotaStateExhausted:
		// In EXHAUSTED state, only allow high-priority polling
		if priority != PriorityHigh {
			c.logger.Debug("Request denied - quota exhausted, only high-priority allowed",
				zap.String("channel_id", channelID),
				zap.String("request_type", string(requestType)),
				zap.Int("priority", int(priority)),
			)
			retryAfter := 5 * time.Minute
			return RequestDecision{
				Allowed:     false,
				Reason:      ReasonGlobalQuotaExhausted,
				GlobalState: globalState,
				RetryAfter:  &retryAfter,
			}
		}

	case QuotaStateCritical:
		// In CRITICAL state, block all discovery
		if requestType == RequestTypeDiscovery || requestType == RequestTypeSearch {
			c.logger.Debug("Request denied - quota critical, discovery disabled",
				zap.String("channel_id", channelID),
				zap.String("request_type", string(requestType)),
			)
			retryAfter := 10 * time.Minute
			return RequestDecision{
				Allowed:     false,
				Reason:      ReasonDiscoveryDisabled,
				GlobalState: globalState,
				RetryAfter:  &retryAfter,
			}
		}

		// Block low-priority polling
		if priority == PriorityLow {
			c.logger.Debug("Request denied - quota critical, low-priority blocked",
				zap.String("channel_id", channelID),
			)
			retryAfter := 5 * time.Minute
			return RequestDecision{
				Allowed:     false,
				Reason:      ReasonLowPriorityBlocked,
				GlobalState: globalState,
				RetryAfter:  &retryAfter,
			}
		}

	case QuotaStateDegraded:
		// In DEGRADED state, block low-priority discovery
		if (requestType == RequestTypeDiscovery || requestType == RequestTypeSearch) && priority == PriorityLow {
			c.logger.Debug("Request denied - quota degraded, low-priority discovery blocked",
				zap.String("channel_id", channelID),
				zap.String("request_type", string(requestType)),
			)
			retryAfter := 2 * time.Minute
			return RequestDecision{
				Allowed:     false,
				Reason:      ReasonGlobalQuotaDegraded,
				GlobalState: globalState,
				RetryAfter:  &retryAfter,
			}
		}
	}

	// Check per-channel quota
	if channelID != "" {
		channelQuota, err := c.perChannelTracker.GetChannelQuota(ctx, channelID)
		if err != nil {
			c.logger.Error("Failed to get channel quota - quota record missing after initialization",
				zap.String("channel_id", channelID),
				zap.Error(err))

			// Block the request to make missing quota records visible
			// This should never happen after proactive initialization in syncChannel()
			return RequestDecision{
				Allowed:     false,
				Reason:      DecisionReason("channel_quota_lookup_failed"),
				GlobalState: globalState,
			}
		}

		channelUsage := channelQuota.DailyQuotaUsed
		channelLimit := channelQuota.DailyQuotaLimit

		if channelUsage+cost > channelLimit {
			c.logger.Debug("Request denied - channel quota exceeded",
				zap.String("channel_id", channelID),
				zap.Int("usage", channelUsage),
				zap.Int("limit", channelLimit),
				zap.Int("cost", cost),
			)
			retryAfter := 1 * time.Hour
			return RequestDecision{
				Allowed:      false,
				Reason:       ReasonChannelQuotaExceeded,
				GlobalState:  globalState,
				ChannelUsage: channelUsage,
				ChannelLimit: channelLimit,
				RetryAfter:   &retryAfter,
			}
		}
	}

	// Check global remaining quota
	if !c.globalTracker.CanMakeRequest(cost) {
		c.logger.Warn("Request denied - insufficient global quota",
			zap.String("channel_id", channelID),
			zap.Int("cost", cost),
			zap.Int("remaining", c.globalTracker.GetRemainingQuota()),
		)
		retryAfter := 30 * time.Minute
		return RequestDecision{
			Allowed:     false,
			Reason:      ReasonGlobalQuotaDepleted,
			GlobalState: globalState,
			RetryAfter:  &retryAfter,
		}
	}

	// Request is allowed
	c.logger.Debug("Request allowed",
		zap.String("channel_id", channelID),
		zap.String("request_type", string(requestType)),
		zap.Int("priority", int(priority)),
		zap.Int("cost", cost),
		zap.String("global_state", string(globalState)),
	)

	return RequestDecision{
		Allowed:     true,
		Reason:      ReasonAllowed,
		GlobalState: globalState,
	}
}

// RecordSuccess records a successful API request
func (c *Coordinator) RecordSuccess(ctx context.Context, channelID string, cost int) error {
	// Record in global tracker
	if err := c.globalTracker.RecordUsage(ctx, cost); err != nil {
		return fmt.Errorf("failed to record global usage: %w", err)
	}

	// Record in per-channel tracker
	if channelID != "" {
		if err := c.perChannelTracker.RecordUsage(ctx, channelID, cost); err != nil {
			c.logger.Warn("Failed to record per-channel usage", zap.String("channel_id", channelID), zap.Error(err))
		}
	}

	return nil
}

// RecordFailure records a failed API request (e.g., API errors, timeouts)
// For now, we don't charge quota for failures, but we log them for monitoring
func (c *Coordinator) RecordFailure(ctx context.Context, channelID string, reason string) {
	c.logger.Warn("API request failed",
		zap.String("channel_id", channelID),
		zap.String("reason", reason),
	)
	// TODO: Implement circuit breaker pattern if needed
}

// GetGlobalState returns the current global quota state
func (c *Coordinator) GetGlobalState() QuotaState {
	return c.globalTracker.GetState()
}

// GetGlobalStateInfo returns detailed global quota state information
func (c *Coordinator) GetGlobalStateInfo() (QuotaState, float64, time.Time) {
	return c.globalTracker.GetStateInfo()
}

// GetChannelUsage returns the current usage for a specific channel
func (c *Coordinator) GetChannelUsage(ctx context.Context, channelID string) (int, int, error) {
	quota, err := c.perChannelTracker.GetChannelQuota(ctx, channelID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get channel quota: %w", err)
	}
	return quota.DailyQuotaUsed, quota.DailyQuotaLimit, nil
}

// ShouldSlowDownPolling returns whether polling should be slowed down
func (c *Coordinator) ShouldSlowDownPolling() bool {
	return c.globalTracker.ShouldSlowDownPolling()
}

// GetPollingDelayMultiplier returns a multiplier for polling delays based on quota state
func (c *Coordinator) GetPollingDelayMultiplier() float64 {
	state := c.globalTracker.GetState()
	switch state {
	case QuotaStateDepleted:
		return 0.0 // Stop polling entirely
	case QuotaStateExhausted:
		return 2.0 // Double the delay
	case QuotaStateCritical:
		return 1.5 // 50% longer delays
	case QuotaStateDegraded:
		return 1.2 // 20% longer delays
	case QuotaStateHealthy:
		return 1.0 // Normal delays
	default:
		return 1.0
	}
}

// GetPerChannelTracker returns the per-channel tracker for direct quota operations
// Used by Manager to initialize channel quota records before quota checks
func (c *Coordinator) GetPerChannelTracker() *PerChannelTracker {
	return c.perChannelTracker
}
