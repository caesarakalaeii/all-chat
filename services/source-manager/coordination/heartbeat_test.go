package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockSourceRepository implements SourceRepository for testing
type mockSourceRepository struct {
	sources []*models.ActiveSource
}

func (m *mockSourceRepository) GetAllActiveSources(ctx context.Context) ([]*models.ActiveSource, error) {
	return m.sources, nil
}

func setupTestHeartbeatMonitor(t *testing.T) (*HeartbeatMonitor, *miniredis.Miniredis, *redis.Client) {
	// Create miniredis instance
	mr := miniredis.RunT(t)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// Create logger
	logger := zap.NewNop()

	// Create monitor
	monitor := NewHeartbeatMonitor(client, logger)

	return monitor, mr, client
}

func TestPublishHeartbeat(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	podID := "test-pod-1"

	// Publish heartbeat
	err := monitor.PublishHeartbeat(ctx, podID)
	require.NoError(t, err)

	// Query Redis directly to verify heartbeat stored
	score, err := client.ZScore(ctx, HeartbeatKey, podID).Result()
	require.NoError(t, err)

	// Verify timestamp is within last second
	now := time.Now().Unix()
	assert.InDelta(t, float64(now), score, 1.0, "Timestamp should be within 1 second of now")
}

func TestFailureDetection(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Publish heartbeats for 3 pods
	pod1, pod2, pod3 := "pod-1", "pod-2", "pod-3"
	now := time.Now().Unix()

	// Set heartbeat timestamps directly via Redis (simulate old heartbeats)
	err := client.ZAdd(ctx, HeartbeatKey,
		redis.Z{Score: float64(now - 20), Member: pod1}, // 20s old - FAILED
		redis.Z{Score: float64(now - 16), Member: pod2}, // 16s old - FAILED
		redis.Z{Score: float64(now - 5), Member: pod3},  // 5s old - HEALTHY
	).Err()
	require.NoError(t, err)

	// Call GetFailedPods - should return pod1 and pod2 (>15s old)
	failedPods, err := monitor.GetFailedPods(ctx)
	require.NoError(t, err)

	// Verify failed pods detected
	assert.ElementsMatch(t, []string{pod1, pod2}, failedPods, "Should detect pods with heartbeat >15s old")

	// Verify pod3 NOT in failed list
	assert.NotContains(t, failedPods, pod3, "Pod3 should NOT be in failed list")
}

func TestHealthyPods(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Publish heartbeats for 3 pods (all healthy)
	pod1, pod2, pod3 := "pod-1", "pod-2", "pod-3"
	now := time.Now().Unix()

	// Set recent heartbeats
	err := client.ZAdd(ctx, HeartbeatKey,
		redis.Z{Score: float64(now - 5), Member: pod1},
		redis.Z{Score: float64(now - 10), Member: pod2},
		redis.Z{Score: float64(now - 14), Member: pod3}, // Just within threshold
	).Err()
	require.NoError(t, err)

	// Call GetHealthyPods - all should be healthy
	healthyPods, err := monitor.GetHealthyPods(ctx)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{pod1, pod2, pod3}, healthyPods, "All pods should be healthy")

	// Now simulate timeout - set all heartbeats to 20s old
	err = client.ZAdd(ctx, HeartbeatKey,
		redis.Z{Score: float64(now - 20), Member: pod1},
		redis.Z{Score: float64(now - 20), Member: pod2},
		redis.Z{Score: float64(now - 20), Member: pod3},
	).Err()
	require.NoError(t, err)

	// Call GetHealthyPods again - should be empty
	healthyPods, err = monitor.GetHealthyPods(ctx)
	require.NoError(t, err)

	assert.Empty(t, healthyPods, "No pods should be healthy after timeout")
}

func TestCleanupStaleHeartbeats(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Create heartbeats with different ages
	now := time.Now().Unix()
	pod1 := "pod-1"
	pod2 := "pod-2"
	pod3 := "pod-3"

	// pod1: 6 minutes old (should be removed)
	// pod2: 4 minutes old (should be kept)
	// pod3: 1 minute old (should be kept)
	err := client.ZAdd(ctx, HeartbeatKey,
		redis.Z{Score: float64(now - 360), Member: pod1}, // 6 min
		redis.Z{Score: float64(now - 240), Member: pod2}, // 4 min
		redis.Z{Score: float64(now - 60), Member: pod3},  // 1 min
	).Err()
	require.NoError(t, err)

	// Verify all 3 exist
	count, err := client.ZCard(ctx, HeartbeatKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Cleanup stale heartbeats
	err = monitor.CleanupStaleHeartbeats(ctx)
	require.NoError(t, err)

	// Verify only pod1 removed (6 minutes old)
	remaining, err := client.ZRange(ctx, HeartbeatKey, 0, -1).Result()
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{pod2, pod3}, remaining, "Only pods newer than 5 minutes should remain")
	assert.NotContains(t, remaining, pod1, "Pod1 (6 min old) should be removed")
}

func TestRemoveOrphanedAssignments(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Create assignment registry
	registry := NewAssignmentRegistry(client)

	// Store 3 assignments
	sourceID1 := "source-1"
	sourceID2 := "source-2"
	sourceID3 := "source-3"
	podID := "pod-1"

	_, err := registry.StoreAssignment(ctx, sourceID1, podID)
	require.NoError(t, err)
	_, err = registry.StoreAssignment(ctx, sourceID2, podID)
	require.NoError(t, err)
	_, err = registry.StoreAssignment(ctx, sourceID3, podID)
	require.NoError(t, err)

	// Verify all 3 assignments exist
	assignments, err := registry.GetAllAssignments(ctx)
	require.NoError(t, err)
	assert.Len(t, assignments, 3)

	// Create mock source repository with only 2 active sources (source-1 and source-2)
	// source-3 is orphaned (deleted from DB)
	mockRepo := &mockSourceRepository{
		sources: []*models.ActiveSource{
			{ID: sourceID1, Platform: "twitch", ChannelID: "channel-1"},
			{ID: sourceID2, Platform: "twitch", ChannelID: "channel-2"},
			// source-3 is NOT in active sources (orphaned)
		},
	}

	// Remove orphaned assignments
	err = monitor.RemoveOrphanedAssignments(ctx, registry, mockRepo)
	require.NoError(t, err)

	// Verify source-3 assignment removed
	assignments, err = registry.GetAllAssignments(ctx)
	require.NoError(t, err)
	assert.Len(t, assignments, 2, "Only 2 assignments should remain")

	// Verify source-1 and source-2 still exist
	_, err = registry.GetAssignment(ctx, sourceID1)
	assert.NoError(t, err, "source-1 should still exist")

	_, err = registry.GetAssignment(ctx, sourceID2)
	assert.NoError(t, err, "source-2 should still exist")

	// Verify source-3 removed
	_, err = registry.GetAssignment(ctx, sourceID3)
	assert.Error(t, err, "source-3 should be removed")
}

func TestHeartbeatTimeout15Seconds(t *testing.T) {
	// Verify the timeout constant matches user constraint
	assert.Equal(t, 15*time.Second, HeartbeatTimeout, "Heartbeat timeout must be 15 seconds per user constraint")
}

func TestFailureDetectionBoundary(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()
	now := time.Now().Unix()

	// Test boundary cases
	pod1 := "pod-exactly-15s"
	pod2 := "pod-14s"
	pod3 := "pod-16s"

	// pod1: exactly 15 seconds old (should be FAILED - boundary is exclusive)
	// pod2: 14 seconds old (should be HEALTHY)
	// pod3: 16 seconds old (should be FAILED)
	err := client.ZAdd(ctx, HeartbeatKey,
		redis.Z{Score: float64(now - 15), Member: pod1},
		redis.Z{Score: float64(now - 14), Member: pod2},
		redis.Z{Score: float64(now - 16), Member: pod3},
	).Err()
	require.NoError(t, err)

	// Get failed pods
	failedPods, err := monitor.GetFailedPods(ctx)
	require.NoError(t, err)

	// Verify pod1 and pod3 marked as failed
	assert.Contains(t, failedPods, pod1, "Pod with exactly 15s heartbeat should be failed")
	assert.Contains(t, failedPods, pod3, "Pod with 16s heartbeat should be failed")

	// Verify pod2 NOT marked as failed
	assert.NotContains(t, failedPods, pod2, "Pod with 14s heartbeat should NOT be failed")

	// Get healthy pods
	healthyPods, err := monitor.GetHealthyPods(ctx)
	require.NoError(t, err)

	// Verify only pod2 is healthy
	assert.ElementsMatch(t, []string{pod2}, healthyPods, "Only pod2 should be healthy")
}

func TestRemoveOrphanedAssignments_EmptyAssignments(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Create assignment registry (empty)
	registry := NewAssignmentRegistry(client)

	// Create mock source repository
	mockRepo := &mockSourceRepository{
		sources: []*models.ActiveSource{
			{ID: "source-1", Platform: "twitch", ChannelID: "channel-1"},
		},
	}

	// Remove orphaned assignments (should not error with empty assignments)
	err := monitor.RemoveOrphanedAssignments(ctx, registry, mockRepo)
	require.NoError(t, err)
}

func TestPublishHeartbeat_RedisError(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer client.Close()

	ctx := context.Background()

	// Close Redis to simulate error
	mr.Close()

	// Attempt to publish heartbeat
	err := monitor.PublishHeartbeat(ctx, "pod-1")
	assert.Error(t, err, "Should return error when Redis unavailable")
}

func TestGetFailedPods_EmptyHeartbeats(t *testing.T) {
	monitor, mr, client := setupTestHeartbeatMonitor(t)
	defer mr.Close()
	defer client.Close()

	ctx := context.Background()

	// Get failed pods with no heartbeats in Redis
	failedPods, err := monitor.GetFailedPods(ctx)
	require.NoError(t, err)

	assert.Empty(t, failedPods, "Should return empty list when no heartbeats exist")
}
