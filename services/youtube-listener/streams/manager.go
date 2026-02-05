package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/metrics"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/oauth"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	grpcoauth "google.golang.org/grpc/credentials/oauth"
)

// OverlayConnectionEvent represents an overlay connection event from API Gateway
type OverlayConnectionEvent struct {
	Type      string    `json:"type"` // "connected" or "disconnected"
	OverlayID string    `json:"overlay_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Manager manages active YouTube streams and coordinates polling
type Manager struct {

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	repository       *Repository

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	oauthManager     *oauth.Manager

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	messageHandler   MessageHandler

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	logger           *zap.Logger

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	leader           *sourcemanager.LeadershipCoordinator

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	quotaTracker     *quota.Tracker

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	quotaCoordinator *quota.Coordinator

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	quotaBudget      *QuotaBudget // Per-channel quota budgeting and adaptive throttling

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	ytMetrics        *metrics.YouTubeMetrics

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	mu            sync.RWMutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	activeStreams map[string]*models.YouTubeStream // streamID -> stream

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	pollers       map[string]*Poller               // streamID -> poller

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Overlay connection tracking

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	connMu                   sync.RWMutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	connectedOverlays        map[string]time.Time           // overlay_id -> connection_time

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	channelConnectedOverlays map[string]map[string]struct{} // channel_id -> overlay_ids

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	redisClient              *redis.Client

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Disconnection debouncing (prevents premature polling shutdown)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	disconnectDebounceTimers map[string]*time.Timer

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	disconnectDebounceMu     sync.Mutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	disconnectDebounceDelay  time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Livestream detection backoff (persistent via Redis)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	backoffStore          *BackoffStore

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	tokenStore            *TokenStore

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	streamStateStore      *StreamStateStore

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	baseDetectionInterval time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	maxDetectionInterval  time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Circuit breakers for offline channel detection (prevents quota waste)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	circuitBreakersMu sync.RWMutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	circuitBreakers   map[string]*CircuitBreaker // channelID -> circuit breaker

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	syncInterval time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	stopChan     chan struct{}

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	wg           sync.WaitGroup

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	dbConn       DBConnInterface // For PostgreSQL LISTEN

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Global sync leadership (prevents multiple replicas from doing expensive discovery)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Safe to share the same LeadershipCoordinator because stream IDs are globally unique

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// ("global-sync" will never conflict with actual video IDs which are alphanumeric)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	syncLeader         *sourcemanager.LeadershipCoordinator

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	syncLeaderStreamID string // Constant stream ID for global sync leadership

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Notification debouncing (prevents thundering herd on rapid notifications)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	notificationMu            sync.Mutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	notificationDebounceTimer *time.Timer

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	pendingNotificationCount  int

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	notificationDebounceDelay time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}


// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	// Connection sync debouncing (prevents expensive syncs on rapid overlay connections)

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	connectionSyncMu            sync.Mutex

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	connectionSyncDebounceTimer *time.Timer

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	pendingConnectionCount      int

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
	connectionSyncDebounceDelay time.Duration

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}
}

// ChannelDetectionState represents the detection state for a channel
type ChannelDetectionState struct {
	ChannelID            string                 `json:"channel_id"`
	BackoffState         *BackoffState          `json:"backoff_state,omitempty"`
	CircuitBreakerState  map[string]interface{} `json:"circuit_breaker_state,omitempty"`
	ConnectedOverlays    int                    `json:"connected_overlays"`
	HasActivePoller      bool                   `json:"has_active_poller"`
	Priority             string                 `json:"priority,omitempty"`
	DetectionsToday      int                    `json:"detections_today,omitempty"`
	QuotaCap             int                    `json:"quota_cap,omitempty"`
	RiskLevel            string                 `json:"risk_level"` // high/medium/low
	RecommendedAction    string                 `json:"recommended_action,omitempty"`
}

// DBConnInterface allows getting a raw pgxpool.Pool for LISTEN
type DBConnInterface interface {
	GetPool() interface{}
}

// NewManager creates a new stream manager
func NewManager(
	repository *Repository,
	oauthManager *oauth.Manager,
	messageHandler MessageHandler,
	dbConn DBConnInterface,
	leader *sourcemanager.LeadershipCoordinator,
	quotaTracker *quota.Tracker,
	perChannelTracker *quota.PerChannelTracker,
	redisClient *redis.Client,
	ytMetrics *metrics.YouTubeMetrics,
	logger *zap.Logger,
) *Manager {
	// Get disconnect debounce delay from environment variable, default to 90 seconds
	disconnectDebounce := 90 * time.Second
	if envDebounce := os.Getenv("OVERLAY_DISCONNECT_DEBOUNCE_SECONDS"); envDebounce != "" {
		if seconds, err := strconv.Atoi(envDebounce); err == nil && seconds > 0 {
			disconnectDebounce = time.Duration(seconds) * time.Second
		}
	}

	logger.Info("YouTube stream manager initialized",
		zap.Duration("disconnect_debounce_delay", disconnectDebounce),
	)

	// Create quota coordinator
	quotaCoordinator := quota.NewCoordinator(quotaTracker, perChannelTracker, logger)

	// Create backoff store for persistent backoff state
	backoffStore := NewBackoffStore(redisClient, logger)
	tokenStore := NewTokenStore(redisClient, logger)
	streamStateStore := NewStreamStateStore(redisClient, logger)

	// Create quota budget system for per-channel caps and adaptive throttling
	quotaBudget := NewQuotaBudget(quotaTracker, redisClient, ytMetrics, logger)

	return &Manager{
		repository:                  repository,
		oauthManager:                oauthManager,
		messageHandler:              messageHandler,
		dbConn:                      dbConn,
		logger:                      logger,
		leader:                      leader,
		quotaTracker:                quotaTracker,
		quotaCoordinator:            quotaCoordinator,
		quotaBudget:                 quotaBudget,
		redisClient:                 redisClient,
		ytMetrics:                   ytMetrics,
		activeStreams:               make(map[string]*models.YouTubeStream),
		pollers:                     make(map[string]*Poller),
		connectedOverlays:           make(map[string]time.Time),
		channelConnectedOverlays:    make(map[string]map[string]struct{}),
		disconnectDebounceTimers:    make(map[string]*time.Timer),
		disconnectDebounceDelay:     disconnectDebounce,
		backoffStore:                backoffStore,
		tokenStore:                  tokenStore,
		streamStateStore:            streamStateStore,
		circuitBreakers:             make(map[string]*CircuitBreaker), // Circuit breakers for offline channels
		baseDetectionInterval:       1 * time.Minute,                  // Start checking every 1m
		maxDetectionInterval:        10 * time.Minute,                 // Max 10 minutes between checks
		syncInterval:                30 * time.Second,
		stopChan:                    make(chan struct{}),
		syncLeader:                  leader,           // Use same coordinator for global sync leadership
		syncLeaderStreamID:          "global-sync",    // Constant stream ID for global sync leadership
		notificationDebounceDelay:   30 * time.Second, // Debounce notifications (YouTube API is expensive: 100 units per search)
		connectionSyncDebounceDelay: 5 * time.Second,  // Debounce overlay connections (saves 100+ units on rapid connections)
	}
}

// Start begins managing streams and PostgreSQL LISTEN
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting stream manager")

	// Start quota budget system
	if err := m.quotaBudget.Start(ctx); err != nil {
		m.logger.Error("Failed to start quota budget system", zap.Error(err))
		return fmt.Errorf("failed to start quota budget: %w", err)
	}

	// Load existing overlay connections from Redis
	if err := m.loadExistingConnections(ctx); err != nil {
		m.logger.Error("Failed to load existing overlay connections", zap.Error(err))
		// Don't fail startup, just log the error
	}

	// Start periodic cleanup goroutine
	m.wg.Add(1)
	go m.periodicConnectionCleanup(ctx)

	// Start periodic stream state refresh
	m.wg.Add(1)
	go m.periodicStreamStateRefresh(ctx)

	// Skip initial sync to avoid quota usage on pod restarts
	// The periodic sync and PostgreSQL LISTEN will handle updates
	m.logger.Info("Skipping initial sync to preserve quota, periodic sync will handle updates")

	// Start periodic sync (fallback)
	m.wg.Add(1)
	go m.periodicSync(ctx)

	// Start PostgreSQL LISTEN for instant notifications
	m.wg.Add(1)
	go m.listenForChanges(ctx)

	// Start Redis subscription for overlay connection events
	m.wg.Add(1)
	go m.listenForOverlayConnections(ctx)

	// Start periodic stuck state recovery
	m.wg.Add(1)
	go m.periodicStuckStateRecovery(ctx)

	// Start periodic metrics update
	m.wg.Add(1)
	go m.periodicMetricsUpdate(ctx)

	return nil
}

// Stop stops managing streams
func (m *Manager) Stop() {
	m.logger.Info("Stopping stream manager")

	// Signal stop
	close(m.stopChan)

	// Clear debounce timer
	m.notificationMu.Lock()
	if m.notificationDebounceTimer != nil {
		m.notificationDebounceTimer.Stop()
		m.notificationDebounceTimer = nil
	}
	m.notificationMu.Unlock()

	// Stop all pollers
	m.mu.Lock()
	for streamID, poller := range m.pollers {
		m.logger.Info("Stopping poller", zap.String("stream_id", streamID))
		poller.Stop()
		m.releaseLeadership(streamID)
	}
	m.pollers = make(map[string]*Poller)
	m.mu.Unlock()

	// Wait for goroutines
	m.wg.Wait()

	if m.leader != nil {
		m.leader.Stop()
	}

	m.logger.Info("Stream manager stopped")
}

// debounceConnectionSync debounces overlay connection events to batch expensive syncs
// This prevents wasting 100+ quota units when multiple overlays connect rapidly
func (m *Manager) debounceConnectionSync(ctx context.Context) {
	m.connectionSyncMu.Lock()
	defer m.connectionSyncMu.Unlock()

	m.pendingConnectionCount++

	// If timer already exists, just increment count and let it continue
	if m.connectionSyncDebounceTimer != nil {
		m.logger.Debug("Overlay connection batched with pending sync",
			zap.Int("pending_connections", m.pendingConnectionCount),
		)
		return
	}

	// Start new debounce timer
	m.connectionSyncDebounceTimer = time.AfterFunc(m.connectionSyncDebounceDelay, func() {
		m.connectionSyncMu.Lock()
		count := m.pendingConnectionCount
		m.pendingConnectionCount = 0
		m.connectionSyncDebounceTimer = nil
		m.connectionSyncMu.Unlock()

		m.logger.Info("Processing batched overlay connections (quota optimization)",
			zap.Int("connection_count", count),
			zap.Duration("debounce_delay", m.connectionSyncDebounceDelay),
			zap.Int("quota_saved_estimate", (count-1)*100), // Each avoided sync saves ~100 units
		)

		// Try to acquire global sync leadership
		// This prevents multiple replicas from racing after connections
		if m.syncLeader != nil {
			isLeader, err := m.syncLeader.EnsureLeadership(context.Background(), m.syncLeaderStreamID, nil)
			if err != nil {
				m.logger.Error("Failed to check global sync leadership after connections", zap.Error(err))
				return
			}
			if !isLeader {
				m.logger.Debug("Not global sync leader, skipping connection sync")
				return
			}
			m.logger.Debug("Global sync leader, performing connection sync")
		}

		// Perform single sync for all batched connections
		if err := m.syncStreams(context.Background()); err != nil {
			m.logger.Error("Failed to sync streams after batched connections", zap.Error(err))
		}
	})
}

// periodicSync periodically syncs streams from database
// Only the global sync leader performs this work to avoid quota waste
func (m *Manager) periodicSync(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Try to acquire global sync leadership
			// Note: No callback is needed for global sync leadership because:
			// 1. Losing leadership just means another replica will take over syncing
			// 2. There's no local state to clean up (unlike per-stream pollers)
			// 3. The next periodic sync will re-acquire leadership if available
			if m.syncLeader != nil {
				isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
				if err != nil {
					m.logger.Error("Failed to check global sync leadership", zap.Error(err))
					continue
				}
				if !isLeader {
					m.logger.Debug("Not global sync leader, skipping periodic sync")
					continue
				}
				m.logger.Debug("Global sync leader, performing periodic sync")
			}

			if err := m.syncStreams(ctx); err != nil {
				m.logger.Error("Failed to sync streams", zap.Error(err))
			}
		case <-m.stopChan:
			// Release global sync leadership on shutdown
			if m.syncLeader != nil {
				m.syncLeader.Release(m.syncLeaderStreamID)
				// Note: Ignoring error on shutdown - lock will expire naturally (10s TTL)
				// and failure to release is not critical during graceful shutdown
			}
			return
		}
	}
}

// listenForChanges listens for PostgreSQL NOTIFY events for instant source updates
func (m *Manager) listenForChanges(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Get connection from pool for LISTEN
			pool := m.dbConn.GetPool()
			if pool == nil {
				m.logger.Error("Failed to get database pool for LISTEN")
				time.Sleep(5 * time.Second)
				continue
			}

			// Acquire connection and LISTEN
			if err := m.listenAndWait(ctx, pool); err != nil {
				m.logger.Warn("PostgreSQL LISTEN error, will retry",
					zap.Error(err),
					zap.Duration("retry_in", 5*time.Second),
				)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// listenAndWait establishes LISTEN connection and waits for notifications
func (m *Manager) listenAndWait(ctx context.Context, poolInterface interface{}) error {
	pool, ok := poolInterface.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("invalid pool type")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Start listening for notifications
	_, err = conn.Exec(ctx, "LISTEN chat_source_changes")
	if err != nil {
		return fmt.Errorf("failed to LISTEN: %w", err)
	}

	m.logger.Info("PostgreSQL LISTEN active",
		zap.String("channel", "chat_source_changes"),
	)

	// Wait for notifications
	for {
		select {
		case <-m.stopChan:
			return nil
		case <-ctx.Done():
			return nil
		default:
			// Wait for notification with timeout
			_, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("notification wait failed: %w", err)
			}

			// Debounce rapid notifications to prevent log spam and reduce unnecessary syncs
			m.notificationMu.Lock()
			m.pendingNotificationCount++

			// If timer already running, it will handle this notification
			if m.notificationDebounceTimer != nil {
				m.notificationMu.Unlock()
				continue
			}

			// Start new debounce timer
			m.notificationDebounceTimer = time.AfterFunc(m.notificationDebounceDelay, func() {
				m.notificationMu.Lock()
				count := m.pendingNotificationCount
				m.pendingNotificationCount = 0
				m.notificationDebounceTimer = nil
				m.notificationMu.Unlock()

				m.logger.Info("Processing debounced notifications",
					zap.Int("notification_count", count),
					zap.Duration("debounce_ms", m.notificationDebounceDelay),
				)

				// Try to acquire global sync leadership before syncing
				// This prevents multiple replicas from racing to sync on the same notification
				if m.syncLeader != nil {
					isLeader, err := m.syncLeader.EnsureLeadership(ctx, m.syncLeaderStreamID, nil)
					if err != nil {
						m.logger.Error("Failed to check global sync leadership after notification", zap.Error(err))
						return
					}
					if !isLeader {
						m.logger.Debug("Not global sync leader, skipping sync after notification")
						return
					}
					m.logger.Debug("Global sync leader, performing sync after notification")
				}

				// Trigger sync after debounce
				if err := m.syncStreams(ctx); err != nil {
					m.logger.Error("Failed to sync after notification", zap.Error(err))
				}
			})
			m.notificationMu.Unlock()
		}
	}
}

// syncStreams fetches active sources and starts/stops pollers as needed
func (m *Manager) syncStreams(ctx context.Context) error {
	m.logger.Debug("Syncing streams from database")

	// Get active sources from database
	sources, err := m.repository.GetActiveSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active sources: %w", err)
	}

	// Filter sources to only those with connected overlays
	m.connMu.RLock()
	connectedSources := make([]*models.StreamSource, 0)
	for _, source := range sources {
		if _, connected := m.connectedOverlays[source.OverlayID]; connected {
			connectedSources = append(connectedSources, source)
		}
	}
	m.connMu.RUnlock()

	m.logger.Info("Filtered YouTube sources by overlay connections",
		zap.Int("total_sources", len(sources)),
		zap.Int("connected_sources", len(connectedSources)),
		zap.Int("connected_overlays", len(m.connectedOverlays)),
	)

	// Group connected sources by channel ID
	channelSources := make(map[string][]*models.StreamSource)
	channelConnectedOverlays := make(map[string]map[string]struct{})
	for _, source := range connectedSources {
		channelSources[source.ChannelID] = append(channelSources[source.ChannelID], source)
		if _, exists := channelConnectedOverlays[source.ChannelID]; !exists {
			channelConnectedOverlays[source.ChannelID] = make(map[string]struct{})
		}
		channelConnectedOverlays[source.ChannelID][source.OverlayID] = struct{}{}
	}

	m.connMu.Lock()
	m.channelConnectedOverlays = channelConnectedOverlays
	m.connMu.Unlock()

	m.logger.Info("Found active YouTube channels with connected overlays",
		zap.Int("channel_count", len(channelSources)),
		zap.Int("source_count", len(connectedSources)),
	)

	// For each channel, check for live streams (with exponential backoff)
	for channelID, channelSourceList := range channelSources {
		// CRITICAL OPTIMIZATION: Skip expensive discovery if poller already running
		// This prevents wasting 100 quota units on redundant searches
		m.mu.RLock()
		hasActivePoller := false
		for streamID := range m.pollers {
			stream := m.activeStreams[streamID]
			if stream != nil && stream.ChannelID == channelID {
				hasActivePoller = true
				m.logger.Debug("Skipping discovery for channel with active poller (saved 100 quota units)",
					zap.String("channel_id", channelID),
					zap.String("stream_id", streamID),
				)
				break
			}
		}
		m.mu.RUnlock()

		if hasActivePoller {
			if err := m.repository.TouchSourceActive(ctx, channelID); err != nil {
				m.logger.Warn("Failed to refresh source status for active poller",
					zap.String("channel_id", channelID),
					zap.Error(err),
				)
			}
			// Reset backoff since we have an active stream
			m.resetDetectionBackoff(channelID, "active_poller")
			continue
		}

		// Check if we should skip this channel due to backoff
		if m.shouldSkipDetection(channelID, channelSourceList) {
			continue
		}

		if err := m.syncChannel(ctx, channelID, channelSourceList); err != nil {
			m.logger.Error("Failed to sync channel",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			// Increase backoff on error
			m.increaseDetectionBackoff(channelID)
			continue
		}

		// Successfully checked - update backoff
		m.updateDetectionBackoff(channelID)
	}

	// Stop pollers for streams that are no longer active
	m.cleanupInactivePollers(ctx, channelSources)

	return nil
}

// syncChannel checks for live streams on a channel and starts pollers
// Uses a two-tier approach: lightweight status check (1 unit) for cached videos,
// full search (100 units) only when needed
func (m *Manager) syncChannel(ctx context.Context, channelID string, sources []*models.StreamSource) error {
	// Get user ID for OAuth
	userID, err := m.repository.GetUserIDForChannel(ctx, channelID)
	if err != nil {
		// Mark source as inactive - can't get OAuth
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after OAuth error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	// Ensure quota record exists before any quota operations
	// This prevents "no rows" errors in GetChannelQuota() calls
	if err := m.quotaCoordinator.GetPerChannelTracker().EnsureChannelExists(ctx, channelID, userID); err != nil {
		m.logger.Error("Failed to ensure quota record exists",
			zap.String("channel_id", channelID),
			zap.String("user_id", userID),
			zap.Error(err),
		)
		// Don't fail - continue and let downstream handle it
	}

	// Create YouTube service with OAuth
	service, httpClient, err := m.oauthManager.CreateYouTubeService(ctx, userID, channelID)
	if err != nil {
		// Mark source as inactive - OAuth failed
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after OAuth creation error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Create API client
	apiClient := api.NewClient(service, httpClient, m.quotaTracker, m.logger)

	// Create gRPC streaming client for true server-side streaming
	// Get token source for gRPC authentication
	oauth2TokenSource, err := m.oauthManager.CreateTokenSource(ctx, userID, channelID)
	if err != nil {
		m.logger.Warn("Failed to create token source for gRPC, will use HTTP fallback",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	} else {
		// Wrap oauth2.TokenSource in gRPC credentials
		grpcTokenSource := grpcoauth.TokenSource{TokenSource: oauth2TokenSource}
		grpcClient, err := api.NewGRPCStreamClient(ctx, grpcTokenSource, m.quotaTracker, m.logger)
		if err != nil {
			m.logger.Warn("Failed to create gRPC streaming client, will fallback to HTTP",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		} else {
			apiClient.SetGRPCClient(grpcClient)
			m.logger.Info("gRPC streaming enabled for channel",
				zap.String("channel_id", channelID),
			)
		}
	}

	overlayID := ""
	if len(sources) > 0 {
		overlayID = sources[0].OverlayID
	}

	// FAST PATH: Check Redis for existing stream state from other pods
	// This allows instant resumption after pod restarts without waiting for detection interval
	if m.streamStateStore != nil {
		streamState, stateErr := m.streamStateStore.LoadStreamState(ctx, channelID)
		if stateErr != nil {
			m.logger.Warn("Failed to load stream state from Redis",
				zap.String("channel_id", channelID),
				zap.Error(stateErr),
			)
		} else if streamState != nil && streamState.IsLive {
			m.logger.Info("Found active stream state in Redis, resuming immediately (no quota wasted!)",
				zap.String("channel_id", channelID),
				zap.String("stream_id", streamState.StreamID),
				zap.String("live_chat_id", streamState.LiveChatID),
				zap.Duration("state_age", time.Since(streamState.LastUpdated)),
			)

			// Reconstruct stream object from state
			stream := &models.YouTubeStream{
				StreamID:      streamState.StreamID,
				ChannelID:     streamState.ChannelID,
				ChannelName:   streamState.ChannelName,
				LiveChatID:    streamState.LiveChatID,
				IsLive:        true,
				OverlayID:     overlayID, // Use current overlay ID from sources
				NextPageToken: "",        // FIX: Always start WITHOUT pageToken for optimal gRPC streaming
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
			}

			// Start poller immediately
			if err := m.startPoller(ctx, stream, apiClient); err != nil {
				m.logger.Error("Failed to start poller from stream state",
					zap.String("stream_id", streamState.StreamID),
					zap.Error(err),
				)
				// Fall through to normal detection
			} else {
				m.logger.Info("Successfully resumed stream from Redis state",
					zap.String("channel_id", channelID),
					zap.String("stream_id", streamState.StreamID),
				)
				return nil
			}
		}
	}

	// Try lightweight status check first if we have a cached video ID
	cachedVideoID, err := m.repository.GetCachedVideoID(ctx, channelID)
	if err == nil && cachedVideoID != "" {
		m.logger.Debug("Attempting lightweight status check using cached video ID",
			zap.String("channel_id", channelID),
			zap.String("cached_video_id", cachedVideoID),
		)

		// Check quota for status check (only 1 unit!) - high priority polling
		decision := m.quotaCoordinator.CanMakeRequest(
			ctx,
			channelID,
			quota.RequestTypePolling,
			quota.PriorityHigh,
			1,
		)
		if !decision.Allowed {
			m.logger.Warn("Quota check denied for lightweight status check",
				zap.String("channel_id", channelID),
				zap.String("reason", string(decision.Reason)),
				zap.String("global_state", string(decision.GlobalState)),
			)
			return fmt.Errorf("quota check failed: %s", decision.Reason)
		}

		// Perform lightweight status check
		statusResult, statusErr := apiClient.CheckStreamStatus(ctx, cachedVideoID, &quota.AuditContext{
			ChannelID: channelID,
			VideoID:   cachedVideoID,
			OverlayID: overlayID,
		})
		if statusErr == nil {
			if statusResult.IsLive && statusResult.LiveChatID != "" {
				m.logger.Info("Cached video is live, using lightweight check (saved 100 quota units - no GetVideoDetails needed)",
					zap.String("channel_id", channelID),
					zap.String("video_id", cachedVideoID),
				)

				// Get full video details to start polling
				stream, detailsErr := apiClient.GetVideoDetails(ctx, cachedVideoID, &quota.AuditContext{
					ChannelID: channelID,
					VideoID:   cachedVideoID,
					OverlayID: overlayID,
				})
				if detailsErr == nil && stream.IsLive && stream.LiveChatID != "" {
					// Set the overlay ID from sources
					if len(sources) > 0 {
						stream.OverlayID = sources[0].OverlayID
					}
					stream.StreamID = cachedVideoID

					if err := m.startPoller(ctx, stream, apiClient); err != nil {
						m.logger.Error("Failed to start poller for cached video",
							zap.String("stream_id", cachedVideoID),
							zap.Error(err),
						)
					}
					return nil
				}
			}

			// Cached video is not live, clear the cache and fall through to full search
			m.logger.Debug("Cached video is not live, clearing cache",
				zap.String("channel_id", channelID),
				zap.String("video_id", cachedVideoID),
			)
			if clearErr := m.repository.ClearCachedVideoID(ctx, channelID); clearErr != nil {
				m.logger.Warn("Failed to clear cached video ID",
					zap.String("channel_id", channelID),
					zap.Error(clearErr),
				)
			}
		} else {
			m.logger.Debug("Status check failed for cached video, falling back to full search",
				zap.String("channel_id", channelID),
				zap.Error(statusErr),
			)
		}
	}

	// CIRCUIT BREAKER: Check if we should attempt expensive channel discovery
	// This prevents wasting quota (100 units per search) on channels that are offline
	circuitBreaker := m.getOrCreateCircuitBreaker(channelID)
	canAttempt, reason := circuitBreaker.CanAttemptDiscovery()

	if !canAttempt {
		m.logger.Debug("Circuit breaker blocking expensive discovery",
			zap.String("channel_id", channelID),
			zap.String("reason", reason),
		)
		return fmt.Errorf("circuit breaker open: %s", reason)
	}

	// Fallback to full search (expensive: 100 units)
	// This is discovery, so use normal priority (can be blocked in degraded/critical states)
	searchDecision := m.quotaCoordinator.CanMakeRequest(
		ctx,
		channelID,
		quota.RequestTypeSearch,
		quota.PriorityNormal,
		quota.QuotaCostSearch,
	)

	if !searchDecision.Allowed {
		m.logger.Warn("Quota check denied for full stream search",
			zap.String("channel_id", channelID),
			zap.String("reason", string(searchDecision.Reason)),
			zap.String("global_state", string(searchDecision.GlobalState)),
		)

		// Apply retry-after delay if provided
		if searchDecision.RetryAfter != nil {
			m.logger.Debug("Search blocked, will retry after delay",
				zap.String("channel_id", channelID),
				zap.Duration("retry_after", *searchDecision.RetryAfter),
			)
		}

		return fmt.Errorf("quota check failed: %s", searchDecision.Reason)
	}

	m.logger.Debug("Performing full live stream search",
		zap.String("channel_id", channelID),
	)

	// Get live streams for channel
	liveStreams, err := apiClient.GetLiveStreams(ctx, channelID, &quota.AuditContext{
		ChannelID: channelID,
		OverlayID: overlayID,
	})
	if err != nil {
		// Mark source as inactive - API call failed
		if setErr := m.repository.SetSourceActive(ctx, channelID, false); setErr != nil {
			m.logger.Error("Failed to mark source inactive after API error",
				zap.String("channel_id", channelID),
				zap.Error(setErr),
			)
		}
		return fmt.Errorf("failed to get live streams: %w", err)
	}

	if len(liveStreams) == 0 {
		// CIRCUIT BREAKER: Record failure (no stream found)
		circuitBreaker.RecordFailure()

		state, failures, _ := circuitBreaker.GetState()
		m.logger.Debug("No live streams found for channel (circuit breaker tracking)",
			zap.String("channel_id", channelID),
			zap.String("circuit_state", string(state)),
			zap.Int("consecutive_failures", failures),
		)
		// Don't deactivate sources when no stream is found
		// The channel might go live later, and we already have exponential backoff
		// Sources should only be deactivated on hard errors (OAuth, API failures)
		// or when explicitly removed by users
		return nil
	}

	// CIRCUIT BREAKER: Record success (stream found!)
	circuitBreaker.RecordSuccess()

	// Cache the first live stream's video ID for future lightweight checks
	if len(liveStreams) > 0 {
		videoID := liveStreams[0].StreamID
		videoTitle := liveStreams[0].Title
		if videoTitle == "" {
			videoTitle = liveStreams[0].ChannelName // Fallback if title is empty
		}
		if cacheErr := m.repository.UpdateCachedVideoID(ctx, channelID, videoID, videoTitle); cacheErr != nil {
			m.logger.Warn("Failed to cache video ID",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
				zap.Error(cacheErr),
			)
		} else {
			m.logger.Info("Cached video ID for future lightweight checks",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
			)
		}
	}

	// Start pollers for each live stream
	// Note: A channel can have multiple overlay sources, but we only poll once per stream
	// We'll use the first overlay's ID for tracking purposes
	for _, stream := range liveStreams {
		// Set the overlay ID from sources (use first one if multiple)
		if len(sources) > 0 {
			stream.OverlayID = sources[0].OverlayID
		}

		if err := m.startPoller(ctx, stream, apiClient); err != nil {
			m.logger.Error("Failed to start poller",
				zap.String("stream_id", stream.StreamID),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// startPoller starts a poller for a stream (if not already running)
func (m *Manager) startPoller(ctx context.Context, stream *models.YouTubeStream, apiClient *api.Client) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if poller already exists
	if _, exists := m.pollers[stream.StreamID]; exists {
		m.logger.Debug("Poller already running for stream",
			zap.String("stream_id", stream.StreamID),
		)
		// Update database status even when poller already exists
		// This ensures database reflects actual polling state
		if err := m.repository.SetSourceActive(ctx, stream.ChannelID, true); err != nil {
			m.logger.Error("Failed to update source status for existing poller",
				zap.String("channel_id", stream.ChannelID),
				zap.Error(err),
			)
		}
		return nil
	}

	if m.leader != nil {
		ok, err := m.leader.EnsureLeadership(ctx, stream.StreamID, func(streamID string) func() {
			// Capture context for leadership loss callback
			lossCtx := context.Background()
			return func() {
				m.handleLeadershipLoss(lossCtx, streamID)
			}
		}(stream.StreamID))
		if err != nil {
			return fmt.Errorf("failed to claim leadership: %w", err)
		}
		if !ok {
			m.logger.Debug("Leadership held by another instance, skipping poller",
				zap.String("stream_id", stream.StreamID),
			)
			return nil
		}
	}

	m.logger.Info("Starting poller for stream",
		zap.String("stream_id", stream.StreamID),
		zap.String("channel_id", stream.ChannelID),
		zap.String("channel_name", stream.ChannelName),
	)

	// Create and start poller
	poller := NewPoller(stream, apiClient, m.ytMetrics, m.logger, m.tokenStore)
	poller.SetMessageHandler(m.messageHandler)

	// Set connection checker for connection-aware polling
	// This prevents wasting quota (5 units per poll) when overlay disconnects
	poller.SetConnectionChecker(m, stream.ChannelID, stream.OverlayID)

	if err := poller.Start(ctx); err != nil {
		if m.leader != nil {
			m.leader.Release(stream.StreamID)
		}
		return fmt.Errorf("failed to start poller: %w", err)
	}

	m.activeStreams[stream.StreamID] = stream
	m.pollers[stream.StreamID] = poller

	// Save stream state to Redis for instant resumption by other pods
	if m.streamStateStore != nil {
		if err := m.streamStateStore.SaveStreamState(ctx, stream); err != nil {
			m.logger.Warn("Failed to save stream state to Redis",
				zap.String("channel_id", stream.ChannelID),
				zap.String("stream_id", stream.StreamID),
				zap.Error(err),
			)
			// Don't fail - this is just an optimization
		} else {
			m.logger.Debug("Saved stream state to Redis for fast resumption",
				zap.String("channel_id", stream.ChannelID),
				zap.String("stream_id", stream.StreamID),
			)
		}
	}

	// Update database status to active
	if err := m.repository.SetSourceActive(ctx, stream.ChannelID, true); err != nil {
		m.logger.Error("Failed to update source status after starting poller",
			zap.String("channel_id", stream.ChannelID),
			zap.Error(err),
		)
	}

	return nil
}

// cleanupInactivePollers stops pollers for channels that are no longer active
func (m *Manager) cleanupInactivePollers(ctx context.Context, activeChannels map[string][]*models.StreamSource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for streamID, poller := range m.pollers {
		stream := m.activeStreams[streamID]
		if stream == nil {
			continue
		}

		// Check if channel is still active
		if _, active := activeChannels[stream.ChannelID]; !active {
			m.logger.Info("Stopping poller for inactive channel",
				zap.String("stream_id", streamID),
				zap.String("channel_id", stream.ChannelID),
			)
			poller.Stop()
			delete(m.pollers, streamID)
			delete(m.activeStreams, streamID)
			m.releaseLeadership(streamID)

			// Clear stream state from Redis
			if m.streamStateStore != nil {
				if err := m.streamStateStore.ClearStreamState(ctx, stream.ChannelID); err != nil {
					m.logger.Warn("Failed to clear stream state from Redis",
						zap.String("channel_id", stream.ChannelID),
						zap.Error(err),
					)
				}
			}

			// Reset detection backoff to allow quick re-detection when channel goes live again
			m.resetDetectionBackoff(stream.ChannelID, "inactive_channel")

			// Don't deactivate database sources when cleanup runs
			// Sources should remain active in DB even if temporarily not polling
			// This allows quick resumption when overlays reconnect
			m.logger.Debug("Stopped poller for channel (sources remain active in DB)",
				zap.String("channel_id", stream.ChannelID),
			)
		}
	}
}

func (m *Manager) releaseLeadership(streamID string) {
	if m.leader == nil {
		return
	}
	m.leader.Release(streamID)
}

func (m *Manager) handleLeadershipLoss(ctx context.Context, streamID string) {
	m.mu.Lock()
	stream := m.activeStreams[streamID]
	poller, exists := m.pollers[streamID]
	if exists {
		poller.Stop()
		delete(m.pollers, streamID)
	}
	delete(m.activeStreams, streamID)

	// Update database status to inactive if we have stream info
	if stream != nil {
		if err := m.repository.SetSourceActive(ctx, stream.ChannelID, false); err != nil {
			m.logger.Error("Failed to update source status after leadership loss",
				zap.String("channel_id", stream.ChannelID),
				zap.Error(err),
			)
		}
	}
	m.mu.Unlock()

	if exists {
		m.logger.Warn("Stopped poller after losing leadership",
			zap.String("stream_id", streamID),
		)
	}
}

// GetActiveStreams returns a list of currently active streams
func (m *Manager) GetActiveStreams() []*models.YouTubeStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*models.YouTubeStream, 0, len(m.activeStreams))
	for _, stream := range m.activeStreams {
		streams = append(streams, stream)
	}

	return streams
}

// loadExistingConnections loads currently connected overlays from Redis on startup
func (m *Manager) loadExistingConnections(ctx context.Context) error {
	// Scan for all overlay:connected:* keys
	var cursor uint64
	var overlayIDs []string

	for {
		keys, nextCursor, err := m.redisClient.Scan(ctx, cursor, "overlay:connected:*", 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan connected overlay keys: %w", err)
		}

		// Extract overlay IDs from keys (format: overlay:connected:OVERLAY_ID)
		for _, key := range keys {
			if len(key) > len("overlay:connected:") {
				overlayID := key[len("overlay:connected:"):]
				overlayIDs = append(overlayIDs, overlayID)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	if len(overlayIDs) == 0 {
		m.logger.Info("No existing overlay connections found")
		return nil
	}

	// Add to connectedOverlays map
	m.connMu.Lock()
	now := time.Now()
	for _, overlayID := range overlayIDs {
		m.connectedOverlays[overlayID] = now
	}
	m.connMu.Unlock()

	m.logger.Info("Loaded existing overlay connections",
		zap.Int("count", len(overlayIDs)),
		zap.Strings("overlay_ids", overlayIDs),
	)

	return nil
}

// periodicConnectionCleanup periodically verifies overlay connections are still valid
// Removes stale connections from memory if their Redis keys have expired
func (m *Manager) periodicConnectionCleanup(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(2 * time.Minute) // Check every 2 minutes
	defer ticker.Stop()

	m.logger.Info("Started periodic connection cleanup",
		zap.Duration("interval", 2*time.Minute),
	)

	for {
		select {
		case <-ticker.C:
			m.cleanupStaleConnections(ctx)
		case <-m.stopChan:
			m.logger.Info("Stopping periodic connection cleanup")
			return
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping periodic connection cleanup")
			return
		}
	}
}

// cleanupStaleConnections removes connections from memory if their Redis keys have expired
func (m *Manager) cleanupStaleConnections(ctx context.Context) {
	m.connMu.RLock()
	overlayIDs := make([]string, 0, len(m.connectedOverlays))
	for overlayID := range m.connectedOverlays {
		overlayIDs = append(overlayIDs, overlayID)
	}
	m.connMu.RUnlock()

	if len(overlayIDs) == 0 {
		return
	}

	// Check which connections still exist in Redis
	staleOverlays := make([]string, 0)
	for _, overlayID := range overlayIDs {
		key := "overlay:connected:" + overlayID
		exists, err := m.redisClient.Exists(ctx, key).Result()
		if err != nil {
			m.logger.Error("Failed to check connection key existence",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			continue
		}

		if exists == 0 {
			// Key expired or was deleted - connection is stale
			staleOverlays = append(staleOverlays, overlayID)
		}
	}

	if len(staleOverlays) > 0 {
		// Remove stale connections from memory
		m.connMu.Lock()
		for _, overlayID := range staleOverlays {
			delete(m.connectedOverlays, overlayID)
		}
		m.connMu.Unlock()

		m.logger.Warn("Cleaned up stale overlay connections (Redis TTL expired)",
			zap.Int("count", len(staleOverlays)),
			zap.Strings("overlay_ids", staleOverlays),
		)
	} else {
		m.logger.Debug("Connection cleanup check completed, all connections valid",
			zap.Int("checked", len(overlayIDs)),
		)
	}
}

// periodicStreamStateRefresh periodically refreshes stream state TTL in Redis
// This keeps stream state available for pod restarts
func (m *Manager) periodicStreamStateRefresh(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // Refresh every 5 minutes
	defer ticker.Stop()

	m.logger.Info("Started periodic stream state refresh",
		zap.Duration("interval", 5*time.Minute),
	)

	for {
		select {
		case <-ticker.C:
			m.refreshActiveStreamStates(ctx)
		case <-m.stopChan:
			m.logger.Info("Stopping periodic stream state refresh")
			return
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping periodic stream state refresh")
			return
		}
	}
}

// refreshActiveStreamStates updates stream state in Redis for all active streams
func (m *Manager) refreshActiveStreamStates(ctx context.Context) {
	if m.streamStateStore == nil {
		return
	}

	m.mu.RLock()
	streams := make([]*models.YouTubeStream, 0, len(m.activeStreams))
	for _, stream := range m.activeStreams {
		streams = append(streams, stream)
	}
	m.mu.RUnlock()

	if len(streams) == 0 {
		return
	}

	for _, stream := range streams {
		if err := m.streamStateStore.SaveStreamState(ctx, stream); err != nil {
			m.logger.Warn("Failed to refresh stream state in Redis",
				zap.String("channel_id", stream.ChannelID),
				zap.String("stream_id", stream.StreamID),
				zap.Error(err),
			)
		}
	}

	m.logger.Debug("Refreshed stream states in Redis",
		zap.Int("count", len(streams)),
	)
}

// listenForOverlayConnections subscribes to Redis overlay connection events
func (m *Manager) listenForOverlayConnections(ctx context.Context) {
	defer m.wg.Done()

	m.logger.Info("Starting overlay connection listener")

	pubsub := m.redisClient.Subscribe(ctx, "overlay:connections")
	defer pubsub.Close()

	ch := pubsub.Channel()

	for {
		select {
		case msg := <-ch:
			var event OverlayConnectionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				m.logger.Error("Failed to unmarshal overlay connection event",
					zap.Error(err),
					zap.String("payload", msg.Payload),
				)
				continue
			}

			m.logger.Info("Received overlay connection event",
				zap.String("type", event.Type),
				zap.String("overlay_id", event.OverlayID),
			)

			switch event.Type {
			case "connected":
				m.handleOverlayConnected(ctx, event.OverlayID)
			case "disconnected":
				m.handleOverlayDisconnected(ctx, event.OverlayID)
			default:
				m.logger.Warn("Unknown overlay connection event type",
					zap.String("type", event.Type),
				)
			}

		case <-m.stopChan:
			m.logger.Info("Stopping overlay connection listener")
			return
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping overlay connection listener")
			return
		}
	}
}

// handleOverlayConnected handles an overlay connection event
func (m *Manager) handleOverlayConnected(ctx context.Context, overlayID string) {
	// Cancel debounce timer if overlay was in debounce period
	m.disconnectDebounceMu.Lock()
	if timer, exists := m.disconnectDebounceTimers[overlayID]; exists {
		timer.Stop()
		delete(m.disconnectDebounceTimers, overlayID)
		m.logger.Info("Cancelled disconnect debounce (overlay reconnected)",
			zap.String("overlay_id", overlayID),
		)
	}
	m.disconnectDebounceMu.Unlock()

	m.connMu.Lock()
	m.connectedOverlays[overlayID] = time.Now()
	m.connMu.Unlock()

	m.logger.Info("Overlay connected, queueing sync with debounce",
		zap.String("overlay_id", overlayID),
		zap.Duration("debounce_delay", m.connectionSyncDebounceDelay),
	)

	// OPTIMIZATION: Debounce rapid overlay connections to batch syncs
	// Saves 100+ quota units when multiple overlays connect quickly
	m.debounceConnectionSync(ctx)
}

// handleOverlayDisconnected handles an overlay disconnection event
func (m *Manager) handleOverlayDisconnected(ctx context.Context, overlayID string) {
	// Remove overlay from connected map immediately
	m.connMu.Lock()
	delete(m.connectedOverlays, overlayID)
	hasOtherConnections := len(m.connectedOverlays) > 0
	m.connMu.Unlock()

	// OPTIMIZATION: If NO other overlays are connected at all, stop pollers immediately
	// This saves 75-90 quota units per disconnect when you're the only user
	if !hasOtherConnections {
		m.logger.Info("Last overlay disconnected, stopping pollers immediately (quota optimization)",
			zap.String("overlay_id", overlayID),
		)

		// Immediately sync to stop all pollers (no sources have connected overlays)
		if err := m.syncStreams(ctx); err != nil {
			m.logger.Error("Failed to sync streams after last overlay disconnect",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
		return
	}

	// OTHER OVERLAYS EXIST - Use debounce period to handle potential reconnection
	// This prevents stopping pollers if user is just refreshing the page
	m.logger.Info("Overlay disconnection event received, starting debounce period (other overlays connected)",
		zap.String("overlay_id", overlayID),
		zap.Duration("debounce_delay", m.disconnectDebounceDelay),
		zap.Int("other_connected_overlays", len(m.connectedOverlays)),
	)

	m.disconnectDebounceMu.Lock()
	defer m.disconnectDebounceMu.Unlock()

	// Cancel existing timer if present
	if timer, exists := m.disconnectDebounceTimers[overlayID]; exists {
		timer.Stop()
	}

	// Create debounce timer
	timer := time.AfterFunc(m.disconnectDebounceDelay, func() {
		m.disconnectDebounceMu.Lock()
		delete(m.disconnectDebounceTimers, overlayID)
		m.disconnectDebounceMu.Unlock()

		// Check if overlay reconnected during debounce
		m.connMu.RLock()
		_, stillConnected := m.connectedOverlays[overlayID]
		m.connMu.RUnlock()

		if stillConnected {
			m.logger.Info("Overlay reconnected during debounce period, keeping pollers active",
				zap.String("overlay_id", overlayID),
			)
			return
		}

		m.logger.Info("Debounce period expired, syncing to check if pollers still needed",
			zap.String("overlay_id", overlayID),
		)

		// Sync will automatically stop pollers that have no connected overlays
		if err := m.syncStreams(context.Background()); err != nil {
			m.logger.Error("Failed to sync streams after debounce",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
	})

	m.disconnectDebounceTimers[overlayID] = timer
}

// IsChannelConnected checks if any overlay is connected for a channel.
// Implements ConnectionChecker interface for connection-aware polling.
func (m *Manager) IsChannelConnected(ctx context.Context, channelID string) (bool, error) {
	m.connMu.RLock()
	defer m.connMu.RUnlock()

	overlays := m.channelConnectedOverlays[channelID]
	return len(overlays) > 0, nil
}

// shouldSkipDetection checks if we should skip livestream detection for a channel due to backoff
// UPDATED: Now implements quota-aware tiered backoff strategy with per-channel caps
func (m *Manager) shouldSkipDetection(channelID string, sources []*models.StreamSource) bool {
	ctx := context.Background()

	// PRIORITY 0: Check Redis stream state (instant resumption bypass)
	// If we have cached stream state, skip all backoff/delay checks
	if m.streamStateStore != nil {
		streamState, err := m.streamStateStore.LoadStreamState(ctx, channelID)
		if err != nil {
			m.logger.Warn("Failed to load stream state for skip check",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		} else if streamState != nil && streamState.IsLive {
			m.logger.Debug("Stream state exists, allowing immediate detection (bypass all delays)",
				zap.String("channel_id", channelID),
				zap.String("stream_id", streamState.StreamID),
			)
			return false // Don't skip - we have active stream state
		}
	}

	// PRIORITY 0.5: Check quota budget - emergency mode blocks low priority channels
	if m.quotaBudget.IsEmergencyMode() {
		priority := m.quotaBudget.GetChannelPriority(channelID)
		if priority == PriorityLow {
			m.quotaBudget.RecordDetectionSkipped("emergency_mode_low_priority")
			m.logger.Debug("Skipping detection due to emergency mode (low priority channel)",
				zap.String("channel_id", channelID),
				zap.String("priority", fmt.Sprintf("%v", priority)),
				zap.Float64("quota_remaining_percent", m.quotaBudget.GetRemainingQuotaPercent()),
			)
			return true
		}
	}

	// PRIORITY 1: Check negative cache (cheapest check, most aggressive)
	isNegativeCached, err := m.backoffStore.IsNegativeCached(ctx, channelID)
	if err != nil {
		m.logger.Warn("Failed to check negative cache, continuing",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	} else if isNegativeCached {
		m.quotaBudget.RecordDetectionSkipped("negative_cache")
		m.logger.Debug("Skipping detection due to negative cache",
			zap.String("channel_id", channelID),
		)
		return true // Channel recently checked and offline
	}

	// PRIORITY 2: Check persistent backoff state from Redis
	backoffState, err := m.backoffStore.LoadBackoffState(ctx, channelID)
	if err != nil {
		m.logger.Warn("Failed to load backoff state, allowing check",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return false // On error, allow check (fail open)
	}

	if backoffState == nil {
		firstConnected := m.earliestConnectedTime(sources)
		if !firstConnected.IsZero() && time.Since(firstConnected) < 5*time.Minute {
			m.quotaBudget.RecordDetectionSkipped("initial_delay")
			m.logger.Debug("Skipping detection for newly connected overlay (initial delay)",
				zap.String("channel_id", channelID),
				zap.Duration("time_since_connected", time.Since(firstConnected)),
				zap.Duration("initial_delay", 5*time.Minute),
			)
			return true
		}

		return false // No backoff state exists, allow check
	}

	// Calculate time since last check
	timeSinceLastCheck := time.Since(backoffState.LastCheckTime)
	shouldSkip := timeSinceLastCheck < backoffState.CurrentInterval

	if shouldSkip {
		m.quotaBudget.RecordDetectionSkipped("backoff_interval")
		m.logger.Debug("Skipping detection due to backoff interval",
			zap.String("channel_id", channelID),
			zap.Duration("current_interval", backoffState.CurrentInterval),
			zap.Duration("time_since_last_check", timeSinceLastCheck),
			zap.Int("failure_count", backoffState.FailureCount),
			zap.Int("consecutive_offline", backoffState.ConsecutiveOffline),
		)
	}

	return shouldSkip
}

func (m *Manager) earliestConnectedTime(sources []*models.StreamSource) time.Time {
	m.connMu.RLock()
	defer m.connMu.RUnlock()

	var earliest time.Time
	for _, source := range sources {
		connectedAt, ok := m.connectedOverlays[source.OverlayID]
		if !ok {
			continue
		}
		if earliest.IsZero() || connectedAt.Before(earliest) {
			earliest = connectedAt
		}
	}

	return earliest
}

// updateDetectionBackoff updates backoff after successful livestream detection
// UPDATED: Implements quota-aware tiered backoff strategy based on channel priority and quota
func (m *Manager) updateDetectionBackoff(channelID string) {
	ctx := context.Background()

	// Load or create backoff state
	backoffState, err := m.backoffStore.LoadBackoffState(ctx, channelID)
	if err != nil {
		m.logger.Error("Failed to load backoff state", zap.String("channel_id", channelID), zap.Error(err))
		return
	}
	if backoffState == nil {
		backoffState = &BackoffState{
			LastCheckTime:   time.Now(),
			CurrentInterval: m.baseDetectionInterval,
			FailureCount:    0,
		}
	}

	// Update last check time
	backoffState.LastCheckTime = time.Now()

	// Check if we found a stream (have active poller)
	m.mu.RLock()
	hasActivePoller := false
	for streamID := range m.pollers {
		if stream, ok := m.activeStreams[streamID]; ok && stream.ChannelID == channelID {
			hasActivePoller = true
			break
		}
	}
	m.mu.RUnlock()

	if hasActivePoller {
		// Stream found - set to max backoff (no need to check while polling)
		backoffState.CurrentInterval = m.maxDetectionInterval
		backoffState.FailureCount = 0
		backoffState.ConsecutiveOffline = 0
		backoffState.LastSeenLive = time.Now()

		// Update quota budget with last live time
		m.quotaBudget.UpdateChannelLastLive(ctx, channelID, time.Now())

		// Clear negative cache
		if err := m.backoffStore.ClearBackoff(ctx, channelID); err != nil {
			m.logger.Warn("Failed to clear negative cache", zap.String("channel_id", channelID), zap.Error(err))
		}

		m.logger.Info("Stream detected, set max backoff",
			zap.String("channel_id", channelID),
			zap.Duration("backoff", backoffState.CurrentInterval),
		)
	} else {
		// No stream found - use tiered backoff strategy based on priority and quota
		backoffState.FailureCount++
		backoffState.ConsecutiveOffline++

		// Get channel priority and quota availability
		priority := m.quotaBudget.GetChannelPriority(channelID)
		quotaRemainingPercent := m.quotaBudget.GetRemainingQuotaPercent()

		// Calculate new backoff interval based on tier
		var baseInterval, maxInterval time.Duration

		switch priority {
		case PriorityHigh: // Recently live < 24h
			if quotaRemainingPercent > 50 {
				// Quota available: 30s base, 2m max
				baseInterval = 30 * time.Second
				maxInterval = 2 * time.Minute
			} else if quotaRemainingPercent > 20 {
				// Quota low: 1m base, 5m max
				baseInterval = 1 * time.Minute
				maxInterval = 5 * time.Minute
			} else {
				// Quota critical: 2m base, 10m max (status checks only)
				baseInterval = 2 * time.Minute
				maxInterval = 10 * time.Minute
			}

		case PriorityStandard: // 24h to 7 days
			if quotaRemainingPercent > 50 {
				// Quota available: 1m base, 5m max
				baseInterval = 1 * time.Minute
				maxInterval = 5 * time.Minute
			} else if quotaRemainingPercent > 20 {
				// Quota low: 2m base, 10m max
				baseInterval = 2 * time.Minute
				maxInterval = 10 * time.Minute
			} else {
				// Quota critical: 5m base, 15m max (status checks only)
				baseInterval = 5 * time.Minute
				maxInterval = 15 * time.Minute
			}

		case PriorityLow: // > 7 days
			if quotaRemainingPercent < 30 {
				// Quota critical: Pause detection entirely
				baseInterval = 1 * time.Hour
				maxInterval = 1 * time.Hour
			} else {
				// Always: 5m base, 20m max
				baseInterval = 5 * time.Minute
				maxInterval = 20 * time.Minute
			}
		}

		// Calculate exponential backoff within tier limits
		currentInterval := backoffState.CurrentInterval
		if currentInterval == 0 || currentInterval < baseInterval {
			currentInterval = baseInterval
		}

		var newInterval time.Duration
		if backoffState.FailureCount == 1 {
			newInterval = baseInterval
		} else {
			// Double the backoff (exponential), up to tier max
			newInterval = currentInterval * 2
			if newInterval > maxInterval {
				newInterval = maxInterval
			}
		}
		backoffState.CurrentInterval = newInterval

		// Set negative cache (tiered TTL based on consecutive offline)
		if err := m.backoffStore.SetNegativeCache(ctx, channelID, backoffState.ConsecutiveOffline); err != nil {
			m.logger.Warn("Failed to set negative cache", zap.String("channel_id", channelID), zap.Error(err))
		}

		m.logger.Info("No stream found, updated backoff with tiered strategy",
			zap.String("channel_id", channelID),
			zap.String("priority", fmt.Sprintf("%v", priority)),
			zap.Float64("quota_remaining_percent", quotaRemainingPercent),
			zap.Duration("previous_backoff", currentInterval),
			zap.Duration("new_backoff", newInterval),
			zap.Duration("tier_base", baseInterval),
			zap.Duration("tier_max", maxInterval),
			zap.Int("consecutive_offline", backoffState.ConsecutiveOffline),
		)
	}

	// Save updated state to Redis
	if err := m.backoffStore.SaveBackoffState(ctx, channelID, backoffState); err != nil {
		m.logger.Error("Failed to save backoff state", zap.String("channel_id", channelID), zap.Error(err))
	}
}

// increaseDetectionBackoff increases backoff after detection error
func (m *Manager) increaseDetectionBackoff(channelID string) {
	ctx := context.Background()

	// Load existing state
	backoffState, err := m.backoffStore.LoadBackoffState(ctx, channelID)
	if err != nil {
		m.logger.Error("Failed to load backoff state", zap.String("channel_id", channelID), zap.Error(err))
		return
	}
	if backoffState == nil {
		backoffState = &BackoffState{
			LastCheckTime:   time.Now(),
			CurrentInterval: m.baseDetectionInterval,
		}
	}

	// Update state
	backoffState.LastCheckTime = time.Now()
	backoffState.FailureCount++
	backoffState.ConsecutiveOffline++

	// Double the backoff on error
	currentInterval := backoffState.CurrentInterval
	if currentInterval == 0 {
		currentInterval = m.baseDetectionInterval
	}
	var newInterval time.Duration
	if backoffState.FailureCount == 1 {
		newInterval = m.baseDetectionInterval
	} else {
		newInterval = currentInterval * 2
		if newInterval > m.maxDetectionInterval {
			newInterval = m.maxDetectionInterval
		}
	}
	backoffState.CurrentInterval = newInterval

	// Save to Redis
	if err := m.backoffStore.SaveBackoffState(ctx, channelID, backoffState); err != nil {
		m.logger.Error("Failed to save backoff state", zap.String("channel_id", channelID), zap.Error(err))
	}

	m.logger.Warn("Increased detection backoff due to error",
		zap.String("channel_id", channelID),
		zap.Duration("new_backoff", newInterval),
		zap.Int("consecutive_offline", backoffState.ConsecutiveOffline),
	)
}

// resetDetectionBackoff resets backoff to base interval when a poller stops
// This allows quick re-detection if the channel goes live again shortly after
func (m *Manager) resetDetectionBackoff(channelID, reason string) {
	ctx := context.Background()

	// Clear all backoff state (backoff + negative cache)
	if err := m.backoffStore.ClearBackoff(ctx, channelID); err != nil {
		m.logger.Error("Failed to clear backoff state", zap.String("channel_id", channelID), zap.Error(err))
		return
	}

	fields := []zap.Field{
		zap.String("channel_id", channelID),
		zap.Duration("backoff", m.baseDetectionInterval),
		zap.String("reason", reason),
	}
	if reason == "active_poller" {
		m.logger.Debug("Reset detection backoff", fields...)
		return
	}
	m.logger.Info("Reset detection backoff", fields...)
}

// getOrCreateCircuitBreaker returns the circuit breaker for a channel, creating if needed
func (m *Manager) getOrCreateCircuitBreaker(channelID string) *CircuitBreaker {
	// Fast path: read lock
	m.circuitBreakersMu.RLock()
	cb, exists := m.circuitBreakers[channelID]
	m.circuitBreakersMu.RUnlock()

	if exists {
		return cb
	}

	// Slow path: write lock
	m.circuitBreakersMu.Lock()
	defer m.circuitBreakersMu.Unlock()

	// Double-check after acquiring write lock
	cb, exists = m.circuitBreakers[channelID]
	if exists {
		return cb
	}

	// Create new circuit breaker
	cb = NewCircuitBreaker(channelID, m.logger, m.ytMetrics)
	m.circuitBreakers[channelID] = cb

	m.logger.Info("Created circuit breaker for channel",
		zap.String("channel_id", channelID),
	)

	return cb
}

// GetAllCircuitBreakers returns statistics for all circuit breakers
// Implements CircuitBreakerGetter interface for quota handler
func (m *Manager) GetAllCircuitBreakers() map[string]map[string]interface{} {
	m.circuitBreakersMu.RLock()
	defer m.circuitBreakersMu.RUnlock()

	result := make(map[string]map[string]interface{}, len(m.circuitBreakers))

	for channelID, cb := range m.circuitBreakers {
		result[channelID] = cb.GetStats()
	}

	return result
}

// ResetCircuitBreaker manually resets a circuit breaker for a specific channel
// Implements CircuitBreakerResetter interface for admin endpoints
func (m *Manager) ResetCircuitBreaker(channelID string) error {
	m.circuitBreakersMu.RLock()
	cb, exists := m.circuitBreakers[channelID]
	m.circuitBreakersMu.RUnlock()

	if !exists {
		return fmt.Errorf("no circuit breaker found for channel: %s", channelID)
	}

	cb.Reset()

	m.logger.Warn("Circuit breaker manually reset by admin",
		zap.String("channel_id", channelID),
	)

	return nil
}

// ResetAllCircuitBreakers manually resets all circuit breakers
// Implements CircuitBreakerResetter interface for admin endpoints
func (m *Manager) ResetAllCircuitBreakers() {
	m.circuitBreakersMu.RLock()
	breakers := make([]*CircuitBreaker, 0, len(m.circuitBreakers))
	for _, cb := range m.circuitBreakers {
		breakers = append(breakers, cb)
	}
	m.circuitBreakersMu.RUnlock()

	for _, cb := range breakers {
		cb.Reset()
	}

	m.logger.Warn("All circuit breakers manually reset by admin",
		zap.Int("count", len(breakers)),
	)
}

// ============================================================================
// Detection Control Interface Implementation (for handlers.DetectionManager)
// ============================================================================

// GetChannelDetectionState returns the detection state for a specific channel
func (m *Manager) GetChannelDetectionState(channelID string) (*ChannelDetectionState, error) {
	ctx := context.Background()
	
	// Load backoff state
	backoffState, err := m.backoffStore.LoadBackoffState(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to load backoff state: %w", err)
	}

	// Get circuit breaker state
	var cbState map[string]interface{}
	m.circuitBreakersMu.RLock()
	if cb, exists := m.circuitBreakers[channelID]; exists {
		cbState = cb.GetStats()
	}
	m.circuitBreakersMu.RUnlock()

	// Count connected overlays for this channel
	m.connMu.RLock()
	overlayIDs, hasOverlays := m.channelConnectedOverlays[channelID]
	connectedOverlays := 0
	if hasOverlays {
		connectedOverlays = len(overlayIDs)
	}
	m.connMu.RUnlock()

	// Check if has active poller
	m.mu.RLock()
	hasActivePoller := false
	for streamID, stream := range m.activeStreams {
		if stream.ChannelID == channelID {
			if _, polling := m.pollers[streamID]; polling {
				hasActivePoller = true
				break
			}
		}
	}
	m.mu.RUnlock()

	// Get quota budget info
	var priority string
	var detectionsToday int
	var quotaCap int
	if m.quotaBudget != nil {
		p := m.quotaBudget.GetChannelPriority(channelID)
		switch p {
		case PriorityHigh:
			priority = "high"
		case PriorityStandard:
			priority = "standard"
		case PriorityLow:
			priority = "low"
		}
		
		if state := m.quotaBudget.GetState(channelID); state != nil {
			detectionsToday = state.DetectionsToday
		}
		quotaCap = m.quotaBudget.GetChannelQuotaCap(channelID)
	}

	// Determine risk level
	riskLevel := "low"
	recommendedAction := ""
	
	if backoffState != nil && connectedOverlays > 0 {
		backoffMinutes := backoffState.CurrentInterval.Minutes()
		if backoffMinutes > 5 {
			riskLevel = "high"
			recommendedAction = "Consider manual force-check or reset-backoff"
		} else if backoffMinutes > 2 {
			riskLevel = "medium"
			recommendedAction = "Monitor for automatic recovery"
		}
	}

	if cbState != nil {
		if state, ok := cbState["state"].(string); ok && state == "OPEN" {
			if connectedOverlays > 0 {
				riskLevel = "high"
				recommendedAction = "Consider reset-circuit-breaker"
			}
		}
	}

	state := &ChannelDetectionState{
		ChannelID:           channelID,
		BackoffState:        backoffState,
		CircuitBreakerState: cbState,
		ConnectedOverlays:   connectedOverlays,
		HasActivePoller:     hasActivePoller,
		Priority:            priority,
		DetectionsToday:     detectionsToday,
		QuotaCap:            quotaCap,
		RiskLevel:           riskLevel,
		RecommendedAction:   recommendedAction,
	}

	return state, nil
}

// GetAllChannelStates returns detection state for all tracked channels
func (m *Manager) GetAllChannelStates() ([]*ChannelDetectionState, error) {
	ctx := context.Background()
	
	// Collect unique channel IDs from multiple sources
	channelIDsMap := make(map[string]struct{})

	// From connected overlays
	m.connMu.RLock()
	for channelID := range m.channelConnectedOverlays {
		channelIDsMap[channelID] = struct{}{}
	}
	m.connMu.RUnlock()

	// From active streams
	m.mu.RLock()
	for _, stream := range m.activeStreams {
		channelIDsMap[stream.ChannelID] = struct{}{}
	}
	m.mu.RUnlock()

	// From circuit breakers
	m.circuitBreakersMu.RLock()
	for channelID := range m.circuitBreakers {
		channelIDsMap[channelID] = struct{}{}
	}
	m.circuitBreakersMu.RUnlock()

	// From quota budget
	if m.quotaBudget != nil {
		for _, channelID := range m.quotaBudget.GetAllChannels() {
			channelIDsMap[channelID] = struct{}{}
		}
	}

	// Get state for each channel
	states := make([]*ChannelDetectionState, 0, len(channelIDsMap))
	for channelID := range channelIDsMap {
		state, err := m.GetChannelDetectionState(channelID)
		if err != nil {
			m.logger.Warn("Failed to get channel state",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			continue
		}
		states = append(states, state)
	}

	return states, nil
}

// ResetChannelBackoff resets backoff for a specific channel
func (m *Manager) ResetChannelBackoff(ctx context.Context, channelID string) error {
	// Clear backoff state from Redis
	if err := m.backoffStore.ClearBackoff(ctx, channelID); err != nil {
		return fmt.Errorf("failed to clear backoff: %w", err)
	}

	m.logger.Info("Channel backoff reset (admin action)",
		zap.String("channel_id", channelID),
	)

	return nil
}

// ForceChannelDetection forces immediate detection for a channel
func (m *Manager) ForceChannelDetection(ctx context.Context, channelID string) error {
	// Reset backoff first
	if err := m.ResetChannelBackoff(ctx, channelID); err != nil {
		return err
	}

	// Reset circuit breaker if exists
	m.circuitBreakersMu.RLock()
	if cb, exists := m.circuitBreakers[channelID]; exists {
		cb.Reset()
	}
	m.circuitBreakersMu.RUnlock()

	// Trigger immediate sync (will check this channel on next cycle)
	m.triggerSync()

	m.logger.Info("Forced channel detection (admin action)",
		zap.String("channel_id", channelID),
	)

	return nil
}

// ResetAllBackoff resets backoff for all tracked channels
func (m *Manager) ResetAllBackoff(ctx context.Context) error {
	// Get all channel IDs
	states, err := m.GetAllChannelStates()
	if err != nil {
		return fmt.Errorf("failed to get channel states: %w", err)
	}

	resetCount := 0
	for _, state := range states {
		if err := m.ResetChannelBackoff(ctx, state.ChannelID); err != nil {
			m.logger.Warn("Failed to reset backoff for channel",
				zap.String("channel_id", state.ChannelID),
				zap.Error(err),
			)
			continue
		}
		resetCount++
	}

	// Reset all circuit breakers
	m.ResetAllCircuitBreakers()

	// Trigger immediate sync
	m.triggerSync()

	m.logger.Warn("Reset all channel backoff (admin action)",
		zap.Int("channels_reset", resetCount),
	)

	return nil
}

// GetQuotaBudgetSummary returns quota budget summary
func (m *Manager) GetQuotaBudgetSummary() map[string]interface{} {
	if m.quotaBudget == nil {
		return map[string]interface{}{"error": "quota budget not initialized"}
	}
	return m.quotaBudget.GetBudgetSummary()
}

// triggerSync triggers an immediate sync cycle (debounced)
func (m *Manager) triggerSync() {
	// Use notification debounce mechanism to trigger sync
	m.notificationMu.Lock()
	defer m.notificationMu.Unlock()

	m.pendingNotificationCount++

	if m.notificationDebounceTimer != nil {
		m.notificationDebounceTimer.Stop()
	}

	// Short delay to allow batching
	m.notificationDebounceTimer = time.AfterFunc(2*time.Second, func() {
		m.handleDebouncedSync()
	})
}

// GetQuotaBudget returns the quota budget instance
func (m *Manager) GetQuotaBudget() *QuotaBudget {
	return m.quotaBudget
}

// periodicStuckStateRecovery runs every 5 minutes to detect and recover stuck channels
func (m *Manager) periodicStuckStateRecovery(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	m.logger.Info("Started periodic stuck state recovery check (every 5 minutes)")

	for {
		select {
		case <-ticker.C:
			m.detectAndRecoverStuckChannels(ctx)
		case <-m.stopChan:
			m.logger.Info("Stopped periodic stuck state recovery check")
			return
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping stuck state recovery check")
			return
		}
	}
}

// detectAndRecoverStuckChannels detects channels stuck in backoff or circuit breaker and recovers them
func (m *Manager) detectAndRecoverStuckChannels(ctx context.Context) {
	m.logger.Debug("Running stuck state recovery check")

	recoveredCount := 0
	checkedCount := 0

	// Get all channel states
	states, err := m.GetAllChannelStates()
	if err != nil {
		m.logger.Error("Failed to get channel states for stuck recovery", zap.Error(err))
		return
	}

	for _, state := range states {
		checkedCount++

		// Skip if no connected overlays (no risk)
		if state.ConnectedOverlays == 0 {
			continue
		}

		// Skip if already has active poller (not stuck)
		if state.HasActivePoller {
			continue
		}

		shouldRecover := false
		reason := ""

		// Check 1: Circuit breaker OPEN for >30 minutes with connected overlays
		if state.CircuitBreakerState != nil {
			if cbState, ok := state.CircuitBreakerState["state"].(string); ok && cbState == "OPEN" {
				if lastChange, ok := state.CircuitBreakerState["last_state_change"].(string); ok {
					lastChangeTime, err := time.Parse(time.RFC3339, lastChange)
					if err == nil && time.Since(lastChangeTime) > 30*time.Minute {
						shouldRecover = true
						reason = "circuit_breaker_open_30min"
						m.logger.Warn("Detected stuck circuit breaker",
							zap.String("channel_id", state.ChannelID),
							zap.Duration("open_duration", time.Since(lastChangeTime)),
							zap.Int("connected_overlays", state.ConnectedOverlays),
						)
					}
				}
			}
		}

		// Check 2: Backoff >10 minutes for recently active channel (<2h since last live)
		if !shouldRecover && state.BackoffState != nil {
			backoffMinutes := state.BackoffState.CurrentInterval.Minutes()
			if backoffMinutes > 10 {
				// Check if recently active
				recentlyActive := false
				if !state.BackoffState.LastSeenLive.IsZero() {
					timeSinceLive := time.Since(state.BackoffState.LastSeenLive)
					if timeSinceLive < 2*time.Hour {
						recentlyActive = true
					}
				}

				if recentlyActive {
					shouldRecover = true
					reason = "high_backoff_recently_active"
					m.logger.Warn("Detected stuck backoff for recently active channel",
						zap.String("channel_id", state.ChannelID),
						zap.Duration("backoff", state.BackoffState.CurrentInterval),
						zap.Duration("time_since_live", time.Since(state.BackoffState.LastSeenLive)),
						zap.Int("connected_overlays", state.ConnectedOverlays),
					)
				}
			}
		}

		if shouldRecover {
			// Auto-recover: reset circuit breaker and backoff
			if err := m.ResetChannelBackoff(ctx, state.ChannelID); err != nil {
				m.logger.Error("Failed to auto-reset backoff during stuck recovery",
					zap.String("channel_id", state.ChannelID),
					zap.Error(err),
				)
				continue
			}

			// Reset circuit breaker if exists
			m.circuitBreakersMu.RLock()
			if cb, exists := m.circuitBreakers[state.ChannelID]; exists {
				cb.Reset()
			}
			m.circuitBreakersMu.RUnlock()

			recoveredCount++

			m.logger.Info("Auto-recovered stuck channel",
				zap.String("channel_id", state.ChannelID),
				zap.String("reason", reason),
				zap.Int("connected_overlays", state.ConnectedOverlays),
				zap.String("action", "auto_recovery"),
			)

			// Record metric
			if m.ytMetrics != nil {
				m.ytMetrics.AutoRecoveryTotal.WithLabelValues(state.ChannelID, reason).Inc()
			}
		}
	}

	if recoveredCount > 0 {
		m.logger.Info("Stuck state recovery cycle complete",
			zap.Int("checked", checkedCount),
			zap.Int("recovered", recoveredCount),
		)

		// Trigger immediate sync after recovery
		m.triggerSync()
	} else {
		m.logger.Debug("Stuck state recovery cycle complete - no channels needed recovery",
			zap.Int("checked", checkedCount),
		)
	}
}

// periodicMetricsUpdate updates backoff and detection metrics every minute
func (m *Manager) periodicMetricsUpdate(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	m.logger.Debug("Started periodic metrics update (every 1 minute)")

	for {
		select {
		case <-ticker.C:
			m.updateBackoffMetrics(ctx)
		case <-m.stopChan:
			m.logger.Debug("Stopped periodic metrics update")
			return
		case <-ctx.Done():
			return
		}
	}
}

// updateBackoffMetrics updates all backoff-related Prometheus metrics
func (m *Manager) updateBackoffMetrics(ctx context.Context) {
	if m.ytMetrics == nil {
		return
	}

	// Get all channel states
	states, err := m.GetAllChannelStates()
	if err != nil {
		m.logger.Warn("Failed to get channel states for metrics", zap.Error(err))
		return
	}

	stuckCount := 0
	atRiskCounts := map[string]int{"high": 0, "medium": 0, "low": 0}

	for _, state := range states {
		// Update backoff interval metric
		if state.BackoffState != nil {
			intervalSeconds := state.BackoffState.CurrentInterval.Seconds()
			m.ytMetrics.BackoffCurrentInterval.WithLabelValues(state.ChannelID).Set(intervalSeconds)

			// Count stuck channels (>5 min backoff)
			if intervalSeconds > 300 {
				stuckCount++
			}
		}

		// Count at-risk channels
		if state.RiskLevel != "" {
			atRiskCounts[state.RiskLevel]++
		}

		// Update quota budget metrics
		if m.quotaBudget != nil && state.Priority != "" {
			cap := state.QuotaCap
			used := state.DetectionsToday
			remaining := cap - used
			if remaining < 0 {
				remaining = 0
			}
			m.ytMetrics.QuotaBudgetRemaining.WithLabelValues(
				state.ChannelID,
				state.Priority,
			).Set(float64(remaining))
		}
	}

	// Update aggregate metrics
	m.ytMetrics.BackoffChannelsStuck.WithLabelValues().Set(float64(stuckCount))
	m.ytMetrics.ChannelsAtRisk.WithLabelValues("high").Set(float64(atRiskCounts["high"]))
	m.ytMetrics.ChannelsAtRisk.WithLabelValues("medium").Set(float64(atRiskCounts["medium"]))
	m.ytMetrics.ChannelsAtRisk.WithLabelValues("low").Set(float64(atRiskCounts["low"]))
}
