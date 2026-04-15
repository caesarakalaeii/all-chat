// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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

	// Get overlays from local WebSocket manager
	hc.manager.mu.RLock()
	localOverlays := make([]string, 0, len(hc.manager.pools))
	for overlayID := range hc.manager.pools {
		localOverlays = append(localOverlays, overlayID)
	}
	hc.manager.mu.RUnlock()

	// Check which overlays exist in Redis using pipelined EXISTS checks
	// This uses the same TTL-based keys that the manager uses (overlay:connected:{id})
	redisOverlays := make(map[string]bool)

	if len(localOverlays) > 0 {
		pipe := hc.redisClient.Pipeline()
		cmds := make(map[string]*redis.IntCmd)

		for _, overlayID := range localOverlays {
			key := "overlay:connected:" + overlayID
			cmds[overlayID] = pipe.Exists(ctx, key)
		}

		_, err := pipe.Exec(ctx)
		if err != nil && err != redis.Nil {
			hc.logger.Error("Failed to check overlay connection keys in Redis",
				zap.Error(err),
			)
			return
		}

		for overlayID, cmd := range cmds {
			exists, _ := cmd.Result()
			if exists > 0 {
				redisOverlays[overlayID] = true
			}
		}
	}

	// Find overlays in local but not in Redis (legitimate recovery scenario)
	missingInRedis := []string{}
	for _, overlayID := range localOverlays {
		if !redisOverlays[overlayID] {
			missingInRedis = append(missingInRedis, overlayID)
		}
	}

	// Fix inconsistencies (recovery only - TTL handles stale key cleanup automatically)
	if len(missingInRedis) > 0 {
		hc.logger.Warn("Found overlays with active connections but missing from Redis",
			zap.Int("count", len(missingInRedis)),
			zap.Strings("overlay_ids", missingInRedis),
		)

		for _, overlayID := range missingInRedis {
			// Set TTL key to match what manager uses
			key := "overlay:connected:" + overlayID
			if err := hc.redisClient.SetEx(ctx, key, "1", hc.manager.connectionTTL).Err(); err != nil {
				hc.logger.Error("Failed to set overlay connection key with TTL",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
				continue
			}

			// Publish connected event (legitimate recovery)
			hc.manager.publishConnectionEvent(ctx, overlayID, "connected")
			hc.metrics.RecordSubscriptionEvent("api-gateway", "health_check_recovered")

			hc.logger.Info("Recovered missing overlay in Redis",
				zap.String("overlay_id", overlayID),
			)
		}
	}

	// Log summary
	if len(missingInRedis) == 0 {
		hc.logger.Debug("WebSocket health check completed - no inconsistencies found",
			zap.Int("local_overlays", len(localOverlays)),
			zap.Int("redis_overlays", len(redisOverlays)),
		)
	} else {
		hc.logger.Info("WebSocket health check completed - inconsistencies fixed",
			zap.Int("local_overlays", len(localOverlays)),
			zap.Int("redis_overlays", len(redisOverlays)),
			zap.Int("recovered", len(missingInRedis)),
		)
	}
}
