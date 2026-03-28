package refresher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/token-refresh-service/repository"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	oauth2Lib "golang.org/x/oauth2"
)

// TokenRepo is the repository interface used by the Manager. Defining it here
// keeps the refresher package testable without a real database.
type TokenRepo interface {
	GetExpiringUserTokens(ctx context.Context, expiresWithin time.Duration) ([]*repository.ExpiringToken, error)
	GetExpiringViewerTokens(ctx context.Context, expiresWithin time.Duration) ([]*repository.ExpiringToken, error)
	GetExpiringYouTubeTokens(ctx context.Context, expiresWithin time.Duration) ([]*repository.ExpiringToken, error)

	UpdateUserTokens(ctx context.Context, userID string, token *oauth2Lib.Token) error
	UpdateViewerTokens(ctx context.Context, sessionID string, token *oauth2Lib.Token) error
	UpdateYouTubeTokens(ctx context.Context, userID, channelID string, token *oauth2Lib.Token) error

	GetUserOverlays(ctx context.Context, userID string) ([]string, error)

	MarkUserTokenPermanentlyFailed(ctx context.Context, userID string, suppressDuration time.Duration) error
	MarkViewerTokenPermanentlyFailed(ctx context.Context, sessionID string, suppressDuration time.Duration) error
	MarkYouTubeTokenPermanentlyFailed(ctx context.Context, userID, channelID string, suppressDuration time.Duration) error
}

// permanentFailSuppress is the duration pushed onto token_expires_at after a
// non-retryable refresh error. It must be long enough to stop the retry loop
// until the user re-authenticates. 30 days is the canonical value.
const permanentFailSuppress = 30 * 24 * time.Hour

// Manager handles periodic OAuth token refresh for all platforms
type Manager struct {
	repo      TokenRepo
	providers map[oauth.Platform]oauth.OAuthProvider
	redis     *redis.Client
	logger    *zap.Logger

	// Configuration
	refreshInterval time.Duration
	expiryBuffer    time.Duration
	batchSize       int
	retryAttempts   int

	// Prometheus metrics
	refreshTotal  *prometheus.CounterVec
	refreshErrors *prometheus.CounterVec

	// State
	mu           sync.RWMutex
	stats        Stats
	lastRun      time.Time
	isRunning    bool
	warningCache map[string]time.Time // Rate limiting for warnings
}

// Stats holds statistics about refresh activity
type Stats struct {
	LastRun           time.Time `json:"last_run"`
	TotalRefreshed    int64     `json:"total_refreshed"`
	TotalFailed       int64     `json:"total_failed"`
	TotalWarnings     int64     `json:"total_warnings"`
	LastBatchDuration string    `json:"last_batch_duration"`
	IsRunning         bool      `json:"is_running"`
}

// NewManager creates a new token refresh manager backed by a real *repository.TokenRepository.
func NewManager(
	repo *repository.TokenRepository,
	providers map[oauth.Platform]oauth.OAuthProvider,
	redis *redis.Client,
	logger *zap.Logger,
	refreshInterval time.Duration,
	expiryBuffer time.Duration,
	batchSize int,
	retryAttempts int,
) *Manager {
	return NewManagerWithRepo(repo, providers, redis, logger, refreshInterval, expiryBuffer, batchSize, retryAttempts, nil)
}

// NewManagerWithRepo creates a new token refresh manager from a TokenRepo interface.
// Pass a non-nil prometheus.Registerer to use an isolated registry (useful in tests
// to avoid duplicate-metric-registration panics). Pass nil to use the default global
// registry (production behaviour).
func NewManagerWithRepo(
	repo TokenRepo,
	providers map[oauth.Platform]oauth.OAuthProvider,
	redis *redis.Client,
	logger *zap.Logger,
	refreshInterval time.Duration,
	expiryBuffer time.Duration,
	batchSize int,
	retryAttempts int,
	reg prometheus.Registerer,
) *Manager {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}
	factory := promauto.With(reg)

	return &Manager{
		repo:            repo,
		providers:       providers,
		redis:           redis,
		logger:          logger,
		refreshInterval: refreshInterval,
		expiryBuffer:    expiryBuffer,
		batchSize:       batchSize,
		retryAttempts:   retryAttempts,
		warningCache:    make(map[string]time.Time),
		refreshTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "token_refresh_attempts_total",
			Help: "Total token refresh attempts",
		}, []string{"service", "platform", "result"}),
		refreshErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "token_refresh_errors_total",
			Help: "Token refresh errors by platform and error category",
		}, []string{"service", "platform", "error_type"}),
	}
}

// Start begins the refresh ticker loop
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting token refresh manager")

	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	// Run immediately on startup
	if err := m.ProcessBatch(ctx); err != nil {
		m.logger.Error("Initial batch processing failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Token refresh manager stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := m.ProcessBatch(ctx); err != nil {
				m.logger.Error("Batch processing failed", zap.Error(err))
			}
		}
	}
}

// ProcessBatch processes a batch of expiring tokens
func (m *Manager) ProcessBatch(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		m.logger.Warn("Batch processing already in progress, skipping")
		return nil
	}
	m.isRunning = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	m.logger.Info("Processing token refresh batch")

	// Query expiring tokens from all sources
	userTokens, err := m.repo.GetExpiringUserTokens(ctx, m.expiryBuffer)
	if err != nil {
		return fmt.Errorf("failed to get expiring user tokens: %w", err)
	}

	viewerTokens, err := m.repo.GetExpiringViewerTokens(ctx, m.expiryBuffer)
	if err != nil {
		return fmt.Errorf("failed to get expiring viewer tokens: %w", err)
	}

	youtubeTokens, err := m.repo.GetExpiringYouTubeTokens(ctx, m.expiryBuffer)
	if err != nil {
		return fmt.Errorf("failed to get expiring YouTube tokens: %w", err)
	}

	// Combine all tokens
	allTokens := append(userTokens, viewerTokens...)
	allTokens = append(allTokens, youtubeTokens...)

	m.logger.Info("Found expiring tokens",
		zap.Int("user_tokens", len(userTokens)),
		zap.Int("viewer_tokens", len(viewerTokens)),
		zap.Int("youtube_tokens", len(youtubeTokens)),
		zap.Int("total", len(allTokens)),
	)

	if len(allTokens) == 0 {
		duration := time.Since(startTime)
		m.updateStats(0, 0, duration)
		return nil
	}

	// Group tokens by platform for parallel processing
	tokensByPlatform := m.groupByPlatform(allTokens)

	// Process each platform in parallel
	var wg sync.WaitGroup
	var refreshed, failed int64

	for platform, tokens := range tokensByPlatform {
		wg.Add(1)
		go func(p oauth.Platform, t []*repository.ExpiringToken) {
			defer wg.Done()
			r, f := m.refreshPlatform(ctx, p, t)
			m.mu.Lock()
			refreshed += r
			failed += f
			m.mu.Unlock()
		}(platform, tokens)
	}

	wg.Wait()

	duration := time.Since(startTime)
	m.updateStats(refreshed, failed, duration)

	m.logger.Info("Batch processing completed",
		zap.Int64("refreshed", refreshed),
		zap.Int64("failed", failed),
		zap.Duration("duration", duration),
	)

	return nil
}

// groupByPlatform groups tokens by platform
func (m *Manager) groupByPlatform(tokens []*repository.ExpiringToken) map[oauth.Platform][] *repository.ExpiringToken {
	grouped := make(map[oauth.Platform][]*repository.ExpiringToken)

	for _, token := range tokens {
		platform := oauth.Platform(token.Platform)
		grouped[platform] = append(grouped[platform], token)
	}

	return grouped
}

// ExposedRefreshPlatform is a test helper that exposes refreshPlatform for
// unit tests in the refresher_test package. It must not be called from
// production code paths.
func (m *Manager) ExposedRefreshPlatform(ctx context.Context, platform oauth.Platform, tokens []*repository.ExpiringToken) (refreshed, failed int64) {
	return m.refreshPlatform(ctx, platform, tokens)
}

// refreshPlatform refreshes all tokens for a specific platform
func (m *Manager) refreshPlatform(ctx context.Context, platform oauth.Platform, tokens []*repository.ExpiringToken) (refreshed, failed int64) {
	provider, ok := m.providers[platform]
	if !ok {
		m.logger.Error("No provider for platform", zap.String("platform", string(platform)))
		return 0, int64(len(tokens))
	}

	for _, token := range tokens {
		// Refresh with retry
		newToken, err := m.refreshWithRetry(ctx, provider, token.RefreshToken)
		if err != nil {
			m.logger.Error("Failed to refresh token",
				zap.String("platform", string(platform)),
				zap.String("token_type", token.TokenType),
				zap.String("id", token.ID),
				zap.String("username", token.Username),
				zap.Error(err),
			)

			m.refreshTotal.WithLabelValues("token-refresh-service", string(platform), "error").Inc()
			errorCategory := categorizeRefreshError(err)
			m.refreshErrors.WithLabelValues("token-refresh-service", string(platform), errorCategory).Inc()

			// Suppress permanently-failed tokens so they are excluded from future batches.
			// The token_expires_at column is pushed 30 days into the future; the user
			// must re-authenticate through the normal login flow to restore access.
			if isNonRetryableErrorString(err.Error()) {
				m.markTokenPermanentlyFailed(ctx, token)
			}

			// Publish warning event
			m.publishWarning(ctx, token, string(platform), "refresh_failed")
			failed++
			continue
		}

		// Update database based on token type
		var updateErr error
		switch token.TokenType {
		case "user":
			updateErr = m.repo.UpdateUserTokens(ctx, token.ID, newToken)
		case "viewer":
			updateErr = m.repo.UpdateViewerTokens(ctx, token.SessionID, newToken)
		case "youtube_channel":
			updateErr = m.repo.UpdateYouTubeTokens(ctx, token.ID, token.ChannelID, newToken)
		default:
			updateErr = fmt.Errorf("unknown token type: %s", token.TokenType)
		}

		if updateErr != nil {
			m.logger.Error("Failed to update token in database",
				zap.String("platform", string(platform)),
				zap.String("token_type", token.TokenType),
				zap.String("id", token.ID),
				zap.Error(updateErr),
			)
			failed++
			continue
		}

		m.logger.Info("Successfully refreshed token",
			zap.String("platform", string(platform)),
			zap.String("token_type", token.TokenType),
			zap.String("username", token.Username),
			zap.Time("new_expiry", newToken.Expiry),
		)
		m.refreshTotal.WithLabelValues("token-refresh-service", string(platform), "success").Inc()
		refreshed++
	}

	return refreshed, failed
}

// refreshWithRetry attempts to refresh a token with exponential backoff
func (m *Manager) refreshWithRetry(ctx context.Context, provider oauth.OAuthProvider, refreshToken string) (*oauth2Lib.Token, error) {
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < m.retryAttempts; attempt++ {
		token, err := provider.RefreshToken(ctx, refreshToken)
		if err == nil {
			return token, nil
		}

		// Check if error is non-retryable
		if m.isNonRetryableError(err) {
			return nil, fmt.Errorf("non-retryable error: %w", err)
		}

		// Last attempt - don't sleep
		if attempt == m.retryAttempts-1 {
			return nil, fmt.Errorf("refresh failed after %d attempts: %w", m.retryAttempts, err)
		}

		// Exponential backoff: 1s, 2s, 4s
		delay := baseDelay * (1 << attempt)
		m.logger.Warn("Token refresh attempt failed, retrying",
			zap.Int("attempt", attempt+1),
			zap.Duration("retry_delay", delay),
			zap.Error(err),
		)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("refresh failed after %d attempts", m.retryAttempts)
}

// categorizeRefreshError classifies a refresh error into a label-safe category string
func categorizeRefreshError(err error) string {
	if err == nil {
		return "unknown"
	}
	errStr := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errStr, "invalid_grant") || strings.Contains(errStr, "token_revoked"):
		return "token_revoked"
	case strings.Contains(errStr, "unauthorized_client") || strings.Contains(errStr, "invalid_client"):
		return "invalid_client"
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout"):
		return "network_error"
	default:
		return "other"
	}
}

// isNonRetryableError checks if an error should not be retried
func (m *Manager) isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	return isNonRetryableErrorString(err.Error())
}

// isNonRetryableErrorString is the pure-string form used both by isNonRetryableError
// and by the permanent-failure marking path.
func isNonRetryableErrorString(errStr string) bool {
	lower := strings.ToLower(errStr)
	// OAuth errors that indicate revoked/invalid tokens — these cannot be resolved
	// by retrying; the user must re-authenticate.
	nonRetryable := []string{
		"invalid_grant",
		"unauthorized_client",
		"invalid_client",
		"token_revoked",
		"access_denied",
		"invalid refresh token",
	}
	for _, pattern := range nonRetryable {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// markTokenPermanentlyFailed pushes the token's expiry far into the future so
// that it is excluded from future refresh batches. The appropriate repository
// method is chosen based on token.TokenType.
func (m *Manager) markTokenPermanentlyFailed(ctx context.Context, token *repository.ExpiringToken) {
	var markErr error
	switch token.TokenType {
	case "user":
		markErr = m.repo.MarkUserTokenPermanentlyFailed(ctx, token.ID, permanentFailSuppress)
	case "viewer":
		markErr = m.repo.MarkViewerTokenPermanentlyFailed(ctx, token.SessionID, permanentFailSuppress)
	case "youtube_channel":
		markErr = m.repo.MarkYouTubeTokenPermanentlyFailed(ctx, token.ID, token.ChannelID, permanentFailSuppress)
	default:
		m.logger.Warn("Unknown token type for permanent-failure marking",
			zap.String("token_type", token.TokenType),
			zap.String("id", token.ID),
		)
		return
	}

	if markErr != nil {
		m.logger.Error("Failed to mark token as permanently failed",
			zap.String("token_type", token.TokenType),
			zap.String("id", token.ID),
			zap.String("session_id", token.SessionID),
			zap.Error(markErr),
		)
		return
	}

	m.logger.Warn("Token marked as permanently failed — user must re-authenticate",
		zap.String("token_type", token.TokenType),
		zap.String("platform", token.Platform),
		zap.String("username", token.Username),
		zap.String("id", token.ID),
		zap.Duration("suppress_duration", permanentFailSuppress),
	)
}

// publishWarning publishes a token expiration warning event to Redis Stream
func (m *Manager) publishWarning(ctx context.Context, token *repository.ExpiringToken, platform, reason string) {
	// Viewer sessions without a linked user account have no overlays to warn
	if token.ID == "" {
		m.logger.Debug("Skipping warning for token without user ID",
			zap.String("token_type", token.TokenType),
			zap.String("session_id", token.SessionID),
			zap.String("platform", platform),
		)
		return
	}

	// Rate limiting: Only send warning once per 30 minutes per user/platform
	cacheKey := fmt.Sprintf("%s:%s:%s", token.ID, platform, token.TokenType)
	m.mu.Lock()
	lastWarning, exists := m.warningCache[cacheKey]
	if exists && time.Since(lastWarning) < 30*time.Minute {
		m.mu.Unlock()
		return // Skip - warning sent recently
	}
	m.warningCache[cacheKey] = time.Now()
	m.mu.Unlock()

	// Get user's active overlays
	overlayIDs, err := m.repo.GetUserOverlays(ctx, token.ID)
	if err != nil {
		m.logger.Error("Failed to get user overlays for warning",
			zap.String("user_id", token.ID),
			zap.Error(err),
		)
		return
	}

	if len(overlayIDs) == 0 {
		m.logger.Debug("User has no active overlays, skipping warning",
			zap.String("user_id", token.ID),
		)
		return
	}

	// Publish warning to Redis Stream for each overlay
	for _, overlayID := range overlayIDs {
		rawMessage := map[string]interface{}{
			"message_id":   uuid.New().String(),
			"platform":     "system",
			"overlay_id":   overlayID,
			"channel_id":   "system",
			"channel_name": "All-Chat System",
			"user_id":      "system",
			"username":     "system",
			"text":         "",
			"timestamp":    time.Now().Format(time.RFC3339),
			"event_type":   "token_expiration_warning",
			"event_data": map[string]interface{}{
				"platform":        platform,
				"username":        token.DisplayName,
				"channel_id":      token.ChannelID,
				"expiration_time": token.ExpiresAt.Format(time.RFC3339),
				"failure_reason":  reason,
				"action_url":      "/settings/connections",
			},
		}

		// Marshal to JSON
		messageJSON, err := json.Marshal(rawMessage)
		if err != nil {
			m.logger.Error("Failed to marshal warning message",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			continue
		}

		// Publish to Redis Stream
		err = m.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: "chat:raw",
			Values: map[string]interface{}{
				"data": string(messageJSON),
			},
		}).Err()

		if err != nil {
			m.logger.Error("Failed to publish warning to Redis Stream",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		} else {
			m.logger.Info("Published token warning event",
				zap.String("overlay_id", overlayID),
				zap.String("platform", platform),
				zap.String("username", token.Username),
			)

			// Update stats
			m.mu.Lock()
			m.stats.TotalWarnings++
			m.mu.Unlock()
		}
	}
}

// updateStats updates internal statistics
func (m *Manager) updateStats(refreshed, failed int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.LastRun = time.Now()
	m.stats.TotalRefreshed += refreshed
	m.stats.TotalFailed += failed
	m.stats.LastBatchDuration = duration.String()
	m.lastRun = time.Now()
}

// GetStats returns current statistics
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Stats{
		LastRun:           m.stats.LastRun,
		TotalRefreshed:    m.stats.TotalRefreshed,
		TotalFailed:       m.stats.TotalFailed,
		TotalWarnings:     m.stats.TotalWarnings,
		LastBatchDuration: m.stats.LastBatchDuration,
		IsRunning:         m.isRunning,
	}
}
