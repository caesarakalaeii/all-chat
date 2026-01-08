package websocket

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// HealthChecker performs periodic state reconciliation between WebSocket manager and Redis
type HealthChecker struct {
	manager        *Manager
	redisClient    *redis.Client
	logger         *zap.Logger
	metrics        *metrics.GatewayMetrics
	checkInterval  time.Duration
	stopChan       chan struct{}
	stoppedChan    chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(
	manager *Manager,
	redisClient *redis.Client,
	logger *zap.Logger,
	m *metrics.GatewayMetrics,
) *HealthChecker {
	// Get check interval from environment variable, default to 5 minutes
	checkInterval := 5 * time.Minute
	if envInterval := os.Getenv("WEBSOCKET_HEALTH_CHECK_INTERVAL_SECONDS"); envInterval != "" {
		if seconds, err := strconv.Atoi(envInterval); err == nil && seconds > 0 {
			checkInterval = time.Duration(seconds) * time.Second
		}
	}

	logger.Info("WebSocket health checker initialized",
		zap.Duration("check_interval", checkInterval),
	)

	return &HealthChecker{
		manager:       manager,
		redisClient:   redisClient,
		logger:        logger,
		metrics:       m,
		checkInterval: checkInterval,
		stopChan:      make(chan struct{}),
		stoppedChan:   make(chan struct{}),
	}
}

// Start begins the periodic health check loop
func (hc *HealthChecker) Start() {
	go hc.runHealthCheckLoop()
}

// Stop gracefully stops the health checker
func (hc *HealthChecker) Stop() {
	close(hc.stopChan)
	<-hc.stoppedChan
	hc.logger.Info("WebSocket health checker stopped")
}

// runHealthCheckLoop runs the periodic health check
func (hc *HealthChecker) runHealthCheckLoop() {
	defer close(hc.stoppedChan)

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	// Run first check immediately
	hc.performHealthCheck(context.Background())

	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck(context.Background())
		case <-hc.stopChan:
			return
		}
	}
}

// performHealthCheck reconciles WebSocket state with Redis
func (hc *HealthChecker) performHealthCheck(ctx context.Context) {
	hc.logger.Debug("Starting WebSocket health check")

	// Get overlays from Redis
	redisOverlays, err := hc.redisClient.SMembers(ctx, "overlay:connected").Result()
	if err != nil {
		hc.logger.Error("Failed to get connected overlays from Redis",
			zap.Error(err),
		)
		return
	}

	// Convert Redis overlays to set for faster lookup
	redisSet := make(map[string]bool)
	for _, overlayID := range redisOverlays {
		redisSet[overlayID] = true
	}

	// Get overlays from local WebSocket manager
	hc.manager.mu.RLock()
	localOverlays := make([]string, 0, len(hc.manager.pools))
	for overlayID := range hc.manager.pools {
		localOverlays = append(localOverlays, overlayID)
	}
	hc.manager.mu.RUnlock()

	// Convert local overlays to set
	localSet := make(map[string]bool)
	for _, overlayID := range localOverlays {
		localSet[overlayID] = true
	}

	// Find inconsistencies
	missingInRedis := []string{}
	staleInRedis := []string{}

	// Check for overlays in local but not in Redis
	for overlayID := range localSet {
		if !redisSet[overlayID] {
			missingInRedis = append(missingInRedis, overlayID)
		}
	}

	// Check for overlays in Redis but not in local
	for overlayID := range redisSet {
		if !localSet[overlayID] {
			staleInRedis = append(staleInRedis, overlayID)
		}
	}

	// Fix inconsistencies
	if len(missingInRedis) > 0 {
		hc.logger.Warn("Found overlays with active connections but missing from Redis",
			zap.Int("count", len(missingInRedis)),
			zap.Strings("overlay_ids", missingInRedis),
		)

		for _, overlayID := range missingInRedis {
			// Add to Redis
			if err := hc.redisClient.SAdd(ctx, "overlay:connected", overlayID).Err(); err != nil {
				hc.logger.Error("Failed to add overlay to Redis connected set",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
				continue
			}

			// Publish connected event
			hc.manager.publishConnectionEvent(ctx, overlayID, "connected")
			hc.metrics.RecordSubscriptionEvent("api-gateway", "health_check_recovered")

			hc.logger.Info("Recovered missing overlay in Redis",
				zap.String("overlay_id", overlayID),
			)
		}
	}

	if len(staleInRedis) > 0 {
		hc.logger.Warn("Found stale overlays in Redis with no active connections",
			zap.Int("count", len(staleInRedis)),
			zap.Strings("overlay_ids", staleInRedis),
		)

		for _, overlayID := range staleInRedis {
			// Check if overlay is in grace period (don't remove if it is)
			hc.manager.gracePeriodMu.Lock()
			_, inGracePeriod := hc.manager.gracePeriodTimers[overlayID]
			hc.manager.gracePeriodMu.Unlock()

			if inGracePeriod {
				hc.logger.Debug("Overlay is in grace period, skipping removal",
					zap.String("overlay_id", overlayID),
				)
				continue
			}

			// Remove from Redis
			if err := hc.redisClient.SRem(ctx, "overlay:connected", overlayID).Err(); err != nil {
				hc.logger.Error("Failed to remove stale overlay from Redis",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
				continue
			}

			// Publish disconnected event
			hc.manager.publishConnectionEvent(ctx, overlayID, "disconnected")
			hc.metrics.RecordSubscriptionEvent("api-gateway", "health_check_cleaned")

			hc.logger.Info("Removed stale overlay from Redis",
				zap.String("overlay_id", overlayID),
			)
		}
	}

	// Log summary
	if len(missingInRedis) == 0 && len(staleInRedis) == 0 {
		hc.logger.Debug("WebSocket health check completed - no inconsistencies found",
			zap.Int("local_overlays", len(localOverlays)),
			zap.Int("redis_overlays", len(redisOverlays)),
		)
	} else {
		hc.logger.Info("WebSocket health check completed - inconsistencies fixed",
			zap.Int("local_overlays", len(localOverlays)),
			zap.Int("redis_overlays", len(redisOverlays)),
			zap.Int("recovered", len(missingInRedis)),
			zap.Int("cleaned", len(staleInRedis)),
		)
	}
}
