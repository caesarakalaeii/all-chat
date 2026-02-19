package coordination

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/redis/go-redis/v9"
)

// Redis key constants
const (
	assignmentKeyPrefix = "shard:assignment:" // Hash: shard:assignment:{source_id} -> {pod_id, timestamp, version}
	loadKey             = "shard:load"        // Sorted Set: {pod_id: channel_count}
	versionKey          = "shard:version"     // Integer: global version counter
)

// AssignmentRegistry manages channel-to-pod assignments in Redis with O(1) lookups
type AssignmentRegistry struct {
	client  *redis.Client
	version int64      // Local version counter (synchronized with Redis)
	mu      sync.Mutex // Protects version counter
}

// NewAssignmentRegistry creates a new assignment registry
func NewAssignmentRegistry(client *redis.Client) *AssignmentRegistry {
	return &AssignmentRegistry{
		client:  client,
		version: 0,
	}
}

// StoreAssignment stores a channel-to-pod assignment in Redis
//
// Operations performed atomically via Pipeline:
// 1. Increment global version counter
// 2. Store assignment: HSET shard:assignment:{source_id} {pod_id, timestamp, version}
// 3. Increment pod load: ZINCRBY shard:load 1 {pod_id}
// 4. Update global version: SET shard:version {version}
//
// Returns the new version number for fencing.
func (r *AssignmentRegistry) StoreAssignment(ctx context.Context, sourceID, podID string) (int64, error) {
	// Increment version (thread-safe)
	r.mu.Lock()
	r.version++
	version := r.version
	r.mu.Unlock()

	pipe := r.client.Pipeline()

	// Store assignment with metadata
	assignmentKey := assignmentKeyPrefix + sourceID
	pipe.HSet(ctx, assignmentKey, map[string]interface{}{
		"pod_id":    podID,
		"timestamp": time.Now().Unix(),
		"version":   version,
	})

	// Increment pod load
	pipe.ZIncrBy(ctx, loadKey, 1, podID)

	// Update global version
	pipe.Set(ctx, versionKey, version, 0)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to store assignment: %w", err)
	}

	return version, nil
}

// GetAssignment retrieves the assignment for a given source
//
// O(1) lookup via HGETALL shard:assignment:{source_id}
func (r *AssignmentRegistry) GetAssignment(ctx context.Context, sourceID string) (*models.Assignment, error) {
	assignmentKey := assignmentKeyPrefix + sourceID
	result, err := r.client.HGetAll(ctx, assignmentKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("assignment not found for source %s", sourceID)
	}

	// Parse timestamp
	timestampUnix, _ := strconv.ParseInt(result["timestamp"], 10, 64)
	timestamp := time.Unix(timestampUnix, 0)

	// Parse version
	version, _ := strconv.ParseInt(result["version"], 10, 64)

	return &models.Assignment{
		SourceID:  sourceID,
		PodID:     result["pod_id"],
		Timestamp: timestamp,
		Version:   version,
	}, nil
}

// GetLeastLoadedPod returns the pod with minimum load
//
// O(log N) query via ZRANGEBYSCORE with limit=1
func (r *AssignmentRegistry) GetLeastLoadedPod(ctx context.Context) (string, int64, error) {
	// Get pod with minimum score (lowest load)
	pods, err := r.client.ZRangeByScoreWithScores(ctx, loadKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   "+inf",
		Count: 1,
	}).Result()

	if err != nil {
		return "", 0, fmt.Errorf("failed to get least loaded pod: %w", err)
	}

	if len(pods) == 0 {
		return "", 0, fmt.Errorf("no pods available")
	}

	podID := pods[0].Member.(string)
	load := int64(pods[0].Score)

	return podID, load, nil
}

// GetPodLoad returns the current load (channel count) for a pod
//
// O(1) lookup via ZSCORE
func (r *AssignmentRegistry) GetPodLoad(ctx context.Context, podID string) (int64, error) {
	score, err := r.client.ZScore(ctx, loadKey, podID).Result()
	if err == redis.Nil {
		return 0, nil // Pod has no assignments yet
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get pod load: %w", err)
	}

	return int64(score), nil
}

// GetGlobalVersion returns the current global version counter from Redis
func (r *AssignmentRegistry) GetGlobalVersion(ctx context.Context) (int64, error) {
	result, err := r.client.Get(ctx, versionKey).Result()
	if err == redis.Nil {
		return 0, nil // No version set yet
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get global version: %w", err)
	}

	version, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version: %w", err)
	}

	return version, nil
}

// RemoveAssignment removes a channel assignment and decrements pod load
//
// Operations performed atomically via Pipeline:
// 1. Delete assignment: DEL shard:assignment:{source_id}
// 2. Decrement pod load: ZINCRBY shard:load -1 {pod_id}
func (r *AssignmentRegistry) RemoveAssignment(ctx context.Context, sourceID, podID string) error {
	pipe := r.client.Pipeline()

	// Delete assignment
	assignmentKey := assignmentKeyPrefix + sourceID
	pipe.Del(ctx, assignmentKey)

	// Decrement pod load
	pipe.ZIncrBy(ctx, loadKey, -1, podID)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove assignment: %w", err)
	}

	return nil
}

// GetAssignmentsForPod returns all assignments for a given pod
//
// O(N) scan operation - use sparingly, primarily for debugging/monitoring
func (r *AssignmentRegistry) GetAssignmentsForPod(ctx context.Context, podID string) ([]*models.Assignment, error) {
	var assignments []*models.Assignment

	// Scan all assignment keys
	iter := r.client.Scan(ctx, 0, assignmentKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		sourceID := key[len(assignmentKeyPrefix):] // Extract source_id from key

		assignment, err := r.GetAssignment(ctx, sourceID)
		if err != nil {
			continue // Skip if assignment no longer exists
		}

		if assignment.PodID == podID {
			assignments = append(assignments, assignment)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan assignments: %w", err)
	}

	return assignments, nil
}

// IncrementVersion increments the local version counter
//
// Thread-safe. Used by coordinator when computing assignments.
func (r *AssignmentRegistry) IncrementVersion() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.version++
	return r.version
}

// GetAllAssignments returns all assignments as a map of source_id -> pod_id
//
// O(N) scan operation - used for orphaned assignment cleanup
func (r *AssignmentRegistry) GetAllAssignments(ctx context.Context) (map[string]string, error) {
	assignments := make(map[string]string)

	// Scan all assignment keys
	iter := r.client.Scan(ctx, 0, assignmentKeyPrefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		sourceID := key[len(assignmentKeyPrefix):] // Extract source_id from key

		assignment, err := r.GetAssignment(ctx, sourceID)
		if err != nil {
			continue // Skip if assignment no longer exists
		}

		assignments[sourceID] = assignment.PodID
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan all assignments: %w", err)
	}

	return assignments, nil
}

// DeleteAssignment deletes an assignment without knowing the pod_id
//
// Used for orphaned assignment cleanup. Queries assignment first to get pod_id,
// then removes the assignment and decrements pod load.
func (r *AssignmentRegistry) DeleteAssignment(ctx context.Context, sourceID string) error {
	// First, get the assignment to find the pod_id
	assignment, err := r.GetAssignment(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get assignment for deletion: %w", err)
	}

	// Now remove the assignment using existing method
	return r.RemoveAssignment(ctx, sourceID, assignment.PodID)
}
