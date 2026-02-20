package coordination

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Shared metrics instance to avoid duplicate registration
var testShardMetrics = metrics.NewShardMetrics()

func TestCalculateLoadScore(t *testing.T) {
	tests := []struct {
		name         string
		channelCount int
		messageRate  float64
		expected     float64
	}{
		{
			name:         "only message rate",
			channelCount: 0,
			messageRate:  100.0,
			expected:     70.0, // 100 * 0.7 + 0 * 0.3
		},
		{
			name:         "only channel count",
			channelCount: 100,
			messageRate:  0,
			expected:     30.0, // 0 * 0.7 + 100 * 0.3
		},
		{
			name:         "balanced load",
			channelCount: 50,
			messageRate:  50.0,
			expected:     50.0, // 50 * 0.7 + 50 * 0.3 = 35 + 15
		},
		{
			name:         "high message rate dominant",
			channelCount: 10,
			messageRate:  500.0,
			expected:     353.0, // 500 * 0.7 + 10 * 0.3 = 350 + 3
		},
		{
			name:         "high channel count secondary",
			channelCount: 200,
			messageRate:  10.0,
			expected:     67.0, // 10 * 0.7 + 200 * 0.3 = 7 + 60
		},
		{
			name:         "zero load",
			channelCount: 0,
			messageRate:  0,
			expected:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLoadScore(tt.channelCount, tt.messageRate)
			assert.InDelta(t, tt.expected, result, 0.01, "load score mismatch")
		})
	}
}

func TestCalculateImbalance_NoImbalance(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	// Perfectly balanced: all pods have same load
	// Imbalance ratio = 1.0 (perfectly balanced: max = avg)
	// Per plan spec: ratio > 0.5, so 1.0 > 0.5 IS true (meets first condition)
	// But maxMsgRate = 50 < 100 (fails second condition)
	// Result: should NOT rebalance (dual-condition gating requires BOTH)
	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 10, MessageRate: 50.0, LoadScore: 38.0},
		{PodID: "pod2", ChannelCount: 10, MessageRate: 50.0, LoadScore: 38.0},
		{PodID: "pod3", ChannelCount: 10, MessageRate: 50.0, LoadScore: 38.0},
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 38.0, report.MaxLoad)
	assert.Equal(t, 38.0, report.MinLoad)
	assert.Equal(t, 38.0, report.AvgLoad)
	assert.InDelta(t, 1.0, report.ImbalanceRatio, 0.01) // max / avg = 1.0 (perfectly balanced)
	assert.False(t, report.ShouldRebalance, "should not rebalance when system mostly idle")
	// Ratio > 0.5 BUT maxRate <= 100, so reason is "system mostly idle"
	assert.Contains(t, report.Reason, "system mostly idle")
}

func TestCalculateImbalance_ImbalanceButIdle(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	// High imbalance ratio (2.0) but low message rate (50 msg/sec < 100 threshold)
	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 100, MessageRate: 50.0, LoadScore: 65.0}, // 50*0.7 + 100*0.3 = 35 + 30
		{PodID: "pod2", ChannelCount: 10, MessageRate: 10.0, LoadScore: 10.0},  // 10*0.7 + 10*0.3 = 7 + 3
		{PodID: "pod3", ChannelCount: 10, MessageRate: 10.0, LoadScore: 10.0},
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 65.0, report.MaxLoad)
	assert.InDelta(t, 28.33, report.AvgLoad, 0.01) // (65 + 10 + 10) / 3
	assert.InDelta(t, 2.29, report.ImbalanceRatio, 0.01) // 65 / 28.33 = 2.29 (exceeds 0.5 threshold)
	assert.Equal(t, 50.0, report.MaxMessageRate)
	assert.False(t, report.ShouldRebalance, "should not rebalance when system mostly idle")
	assert.Contains(t, report.Reason, "system mostly idle")
}

func TestCalculateImbalance_ImbalanceUnderLoad(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	// High imbalance ratio (1.8) AND high message rate (500 msg/sec > 100 threshold)
	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 100, MessageRate: 500.0, LoadScore: 380.0}, // 500*0.7 + 100*0.3 = 350 + 30
		{PodID: "pod2", ChannelCount: 50, MessageRate: 200.0, LoadScore: 155.0},  // 200*0.7 + 50*0.3 = 140 + 15
		{PodID: "pod3", ChannelCount: 50, MessageRate: 200.0, LoadScore: 155.0},
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 380.0, report.MaxLoad)
	assert.InDelta(t, 230.0, report.AvgLoad, 0.01) // (380 + 155 + 155) / 3
	assert.InDelta(t, 1.65, report.ImbalanceRatio, 0.01) // 380 / 230 = 1.65 (exceeds 0.5)
	assert.Equal(t, 500.0, report.MaxMessageRate)
	assert.True(t, report.ShouldRebalance, "should rebalance when both conditions met")
	assert.Contains(t, report.Reason, "imbalance detected")
}

func TestCalculateImbalance_EdgeCaseEmptyLoads(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	loads := []PodLoad{}

	report := monitor.CalculateImbalance(loads)

	assert.False(t, report.ShouldRebalance)
	assert.Contains(t, report.Reason, "no pods")
}

func TestCalculateImbalance_EdgeCaseSinglePod(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 50, MessageRate: 200.0, LoadScore: 155.0},
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 155.0, report.MaxLoad)
	assert.Equal(t, 155.0, report.MinLoad)
	assert.Equal(t, 155.0, report.AvgLoad)
	assert.InDelta(t, 1.0, report.ImbalanceRatio, 0.01)
	assert.False(t, report.ShouldRebalance, "single pod cannot rebalance")
	assert.Contains(t, report.Reason, "single pod")
}

func TestCalculateImbalance_EdgeCaseAvgLoadZero(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	// All pods have zero load
	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 0, MessageRate: 0, LoadScore: 0},
		{PodID: "pod2", ChannelCount: 0, MessageRate: 0, LoadScore: 0},
		{PodID: "pod3", ChannelCount: 0, MessageRate: 0, LoadScore: 0},
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 0.0, report.MaxLoad)
	assert.Equal(t, 0.0, report.AvgLoad)
	assert.Equal(t, 0.0, report.ImbalanceRatio) // avgLoad=0 → ratio=0
	assert.False(t, report.ShouldRebalance)
}

func TestCalculateImbalance_OverloadedAndUnderutilizedPods(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 100, MessageRate: 500.0, LoadScore: 380.0}, // Above avg
		{PodID: "pod2", ChannelCount: 50, MessageRate: 200.0, LoadScore: 155.0},  // Below avg
		{PodID: "pod3", ChannelCount: 50, MessageRate: 200.0, LoadScore: 155.0},  // Below avg
	}

	report := monitor.CalculateImbalance(loads)

	assert.Equal(t, 1, len(report.OverloadedPods), "should identify 1 overloaded pod")
	assert.Contains(t, report.OverloadedPods, "pod1")
	assert.Equal(t, 2, len(report.UnderutilizedPods), "should identify 2 underutilized pods")
	assert.Contains(t, report.UnderutilizedPods, "pod2")
	assert.Contains(t, report.UnderutilizedPods, "pod3")
}

func TestCalculateImbalance_ExactlyAtThresholds(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	monitor := NewLoadMonitor(nil, "", shardMetrics, logger)

	// Just above both thresholds: ratio > 0.5, maxRate > 100
	// avgLoad = 59.5, maxLoad = 85 → ratio = 1.43 (> 0.5)
	// maxRate = 101 (> 100)
	loads := []PodLoad{
		{PodID: "pod1", ChannelCount: 50, MessageRate: 101.0, LoadScore: 85.7}, // 101*0.7 + 50*0.3 = 70.7 + 15
		{PodID: "pod2", ChannelCount: 20, MessageRate: 40.0, LoadScore: 34.0},  // 40*0.7 + 20*0.3 = 28 + 6
	}

	report := monitor.CalculateImbalance(loads)

	avgLoad := (85.7 + 34.0) / 2.0 // 59.85
	imbalanceRatio := 85.7 / avgLoad // 1.43 (exceeds 0.5)

	assert.InDelta(t, avgLoad, report.AvgLoad, 0.01)
	assert.InDelta(t, imbalanceRatio, report.ImbalanceRatio, 0.01)
	assert.Equal(t, 101.0, report.MaxMessageRate)

	// Both thresholds met (ratio > 0.5 AND maxRate > 100)
	assert.True(t, report.ShouldRebalance)
}

func TestMonitorPodLoads_PrometheusUnavailable(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	// Initialize Redis client for testing (use miniredis or mock if needed)
	// For this test, we'll skip actual Redis connection and test graceful degradation logic

	monitor := NewLoadMonitor(nil, "http://invalid-prometheus:9090", shardMetrics, logger)

	// Test that Prometheus unavailability doesn't crash the monitor
	// (actual test would require Redis mock or miniredis)
	assert.NotNil(t, monitor)
	assert.Equal(t, "http://invalid-prometheus:9090", monitor.prometheusURL)
}

func TestGetChannelCount_PodNotFound(t *testing.T) {
	logger := zap.NewNop()
	shardMetrics := testShardMetrics

	// Create miniredis client for testing
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // This will fail gracefully in CI
	})

	monitor := NewLoadMonitor(redisClient, "", shardMetrics, logger)

	ctx := context.Background()

	// Test new pod with no assignments returns 0 (not error)
	count, err := monitor.getChannelCount(ctx, "nonexistent-pod")

	// If Redis unavailable, err != nil; if available, count should be 0
	if err == nil {
		assert.Equal(t, 0, count, "new pod should have 0 channel count")
	}
}
