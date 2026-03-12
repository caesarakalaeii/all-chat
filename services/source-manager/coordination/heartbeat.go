package coordination

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// SourceRepository defines the interface for querying active sources from DB
type SourceRepository interface {
	GetAllActiveSources(ctx context.Context) ([]*models.ActiveSource, error)
}

const (
	// HeartbeatKey is the Redis Sorted Set storing pod heartbeats
	// Format: ZADD shard:heartbeats timestamp podID
	HeartbeatKey = "shard:heartbeats"

	// HeartbeatTimeout is the duration after which a pod is considered failed
	// User constraint from CONTEXT.md: 15 seconds
	HeartbeatTimeout = 15 * time.Second

	// StaleHeartbeatCleanup is the duration after which old heartbeats are removed
	// Pods definitely dead after 5 minutes of no heartbeat
	StaleHeartbeatCleanup = 5 * time.Minute
)

// HeartbeatMonitor manages pod heartbeat publishing and failure detection
type HeartbeatMonitor struct {
	client  *redis.Client
	logger  *zap.Logger
	metrics *metrics.ShardMetrics
}

// NewHeartbeatMonitor creates a new heartbeat monitor instance
func NewHeartbeatMonitor(client *redis.Client, logger *zap.Logger, metrics *metrics.ShardMetrics) *HeartbeatMonitor {
	return &HeartbeatMonitor{
		client:  client,
		logger:  logger,
		metrics: metrics,
	}
}

// PublishHeartbeat publishes a heartbeat for the given pod ID
// Uses ZADD to store pod's current timestamp in sorted set
func (h *HeartbeatMonitor) PublishHeartbeat(ctx context.Context, podID string) error {
	now := time.Now().Unix()

	err := h.client.ZAdd(ctx, HeartbeatKey, redis.Z{
		Score:  float64(now),
		Member: podID,
	}).Err()

	if err != nil {
		h.logger.Error("Failed to publish heartbeat",
			zap.String("pod_id", podID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish heartbeat: %w", err)
	}

	h.logger.Debug("Published heartbeat",
		zap.String("pod_id", podID),
		zap.Int64("timestamp", now),
	)

	return nil
}

// GetFailedPods returns list of pod IDs with heartbeat older than HeartbeatTimeout
// Uses ZRANGEBYSCORE to query pods with timestamp < (now - 15s)
func (h *HeartbeatMonitor) GetFailedPods(ctx context.Context) ([]string, error) {
	cutoff := time.Now().Add(-HeartbeatTimeout).Unix()

	failedPods, err := h.client.ZRangeByScore(ctx, HeartbeatKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", cutoff),
	}).Result()

	if err != nil {
		h.logger.Error("Failed to query failed pods", zap.Error(err))
		return nil, fmt.Errorf("failed to query failed pods: %w", err)
	}

	if len(failedPods) > 0 {
		h.logger.Info("Detected failed pods",
			zap.Int("count", len(failedPods)),
			zap.Strings("pod_ids", failedPods),
		)
	}

	return failedPods, nil
}

// GetHealthyPods returns list of pod IDs with heartbeat within HeartbeatTimeout
// Uses ZRANGEBYSCORE to query pods with timestamp > (now - 15s)
// Note: Using exclusive boundary with "(" to match inverse of GetFailedPods
func (h *HeartbeatMonitor) GetHealthyPods(ctx context.Context) ([]string, error) {
	cutoff := time.Now().Add(-HeartbeatTimeout).Unix()

	healthyPods, err := h.client.ZRangeByScore(ctx, HeartbeatKey, &redis.ZRangeBy{
		Min: fmt.Sprintf("(%d", cutoff), // Exclusive boundary: score > cutoff
		Max: "+inf",
	}).Result()

	if err != nil {
		h.logger.Error("Failed to query healthy pods", zap.Error(err))
		return nil, fmt.Errorf("failed to query healthy pods: %w", err)
	}

	h.logger.Debug("Queried healthy pods", zap.Int("count", len(healthyPods)))

	return healthyPods, nil
}

// CleanupStaleHeartbeats removes heartbeats older than HeartbeatTimeout (15 seconds)
// Uses ZREMRANGEBYSCORE to delete old entries
// Changed from 5min to 15s to eliminate ghost pods faster
func (h *HeartbeatMonitor) CleanupStaleHeartbeats(ctx context.Context) error {
	// Use HeartbeatTimeout directly instead of StaleHeartbeatCleanup
	// Pods detected as failed at 15s should be removed immediately, not kept for 5 minutes
	cutoff := time.Now().Add(-HeartbeatTimeout).Unix()

	removed, err := h.client.ZRemRangeByScore(ctx, HeartbeatKey, "-inf", fmt.Sprintf("%d", cutoff)).Result()
	if err != nil {
		h.logger.Error("Failed to cleanup stale heartbeats", zap.Error(err))
		return fmt.Errorf("failed to cleanup stale heartbeats: %w", err)
	}

	if removed > 0 {
		h.metrics.StaleHeartbeatsRemoved.Add(float64(removed))
		h.logger.Info("Cleaned up stale heartbeats",
			zap.Int64("removed", removed),
			zap.Duration("timeout", HeartbeatTimeout),
		)
	}

	return nil
}

// RemovePodHeartbeat explicitly removes a pod's heartbeat entry
// Called when Kubernetes detects pod termination for immediate cleanup
func (h *HeartbeatMonitor) RemovePodHeartbeat(ctx context.Context, podID string) error {
	err := h.client.ZRem(ctx, HeartbeatKey, podID).Err()
	if err != nil {
		return fmt.Errorf("failed to remove pod heartbeat: %w", err)
	}

	h.logger.Info("Removed pod heartbeat entry",
		zap.String("pod_id", podID),
	)

	return nil
}

// GetAllHeartbeatPods returns all pod IDs currently in heartbeat registry
// Used for ghost pod detection (pods with heartbeats but not in K8s)
func (h *HeartbeatMonitor) GetAllHeartbeatPods(ctx context.Context) ([]string, error) {
	allPods, err := h.client.ZRange(ctx, HeartbeatKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get all heartbeat pods: %w", err)
	}
	return allPods, nil
}

// RemoveOrphanedAssignments removes assignments for sources that no longer exist in DB
// Defense-in-depth approach per CONTEXT.md user constraint
func (h *HeartbeatMonitor) RemoveOrphanedAssignments(ctx context.Context, registry *AssignmentRegistry, sourceRepo SourceRepository) error {
	// Query all assignments from Redis
	allAssignments, err := registry.GetAllAssignments(ctx)
	if err != nil {
		h.logger.Error("Failed to query all assignments", zap.Error(err))
		return fmt.Errorf("failed to query all assignments: %w", err)
	}

	if len(allAssignments) == 0 {
		// No assignments to check
		return nil
	}

	// Query all active sources from DB
	activeSources, err := sourceRepo.GetAllActiveSources(ctx)
	if err != nil {
		h.logger.Error("Failed to query active sources", zap.Error(err))
		return fmt.Errorf("failed to query active sources: %w", err)
	}

	// Build set of active source IDs for fast lookup
	activeSourceIDs := make(map[string]bool)
	for _, source := range activeSources {
		activeSourceIDs[source.ID] = true
	}

	// Check each assignment and remove orphans
	orphanedCount := 0
	for sourceID := range allAssignments {
		// Extract real source ID from composite keys (format: "{uuid}:{platform}")
		realSourceID := sourceID
		if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
			realSourceID = sourceID[:colonIdx]
		}

		if !activeSourceIDs[realSourceID] {
			// Source no longer exists in DB - delete assignment
			if err := registry.DeleteAssignment(ctx, sourceID); err != nil {
				h.logger.Error("Failed to delete orphaned assignment",
					zap.String("source_id", sourceID),
					zap.Error(err),
				)
				continue
			}
			orphanedCount++
		}
	}

	if orphanedCount > 0 {
		h.logger.Info("Removed orphaned assignments", zap.Int("count", orphanedCount))
	}

	return nil
}
