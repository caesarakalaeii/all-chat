package coordination

import (
	"context"
	"fmt"
	"testing"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestPlanRebalancing_Proportional validates that low-traffic channels are selected first
func TestPlanRebalancing_Proportional(t *testing.T) {
	logger := zap.NewNop()

	// Mock registry that returns channels with different traffic levels
	registry := &mockRegistry{
		assignments: map[string][]*models.Assignment{
			"pod-1": {
				{SourceID: "channel-high", PodID: "pod-1"},
				{SourceID: "channel-med", PodID: "pod-1"},
				{SourceID: "channel-low-1", PodID: "pod-1"},
				{SourceID: "channel-low-2", PodID: "pod-1"},
				{SourceID: "channel-low-3", PodID: "pod-1"},
			},
		},
	}

	// Mock rebalancer with channel rates
	rebalancer := &Rebalancer{
		registry:          registry,
		maxMigrationRatio: 0.20, // 20%
		logger:            logger,
	}

	// Override channelLoadsFunc to return mock data
	rebalancer.channelLoadsFunc = func(ctx context.Context, assignments []*models.Assignment) []ChannelLoad {
		// Return channels with different rates (high, medium, low)
		return []ChannelLoad{
			{ChannelID: "channel-high", MessageRate: 500.0},
			{ChannelID: "channel-med", MessageRate: 100.0},
			{ChannelID: "channel-low-1", MessageRate: 5.0},
			{ChannelID: "channel-low-2", MessageRate: 3.0},
			{ChannelID: "channel-low-3", MessageRate: 1.0},
		}
	}

	// Create pod loads (pod-1 overloaded, pod-2 underutilized)
	loads := []PodLoad{
		{PodID: "pod-1", LoadScore: 100.0}, // overloaded
		{PodID: "pod-2", LoadScore: 50.0},  // underutilized
	}
	avgLoad := 75.0

	// Plan rebalancing
	plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, 0)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, plans, 1)

	plan := plans[0]
	assert.Equal(t, "pod-1", plan.SourcePod)
	assert.Equal(t, "pod-2", plan.TargetPod)
	assert.Equal(t, 5, plan.TotalChannels)
	assert.Equal(t, 1, plan.MigrationCount) // 20% of 5 = 1

	// Verify that lowest-traffic channel was selected (proportional strategy)
	assert.Contains(t, plan.Channels, "channel-low-3") // Rate 1.0 (lowest)
}

// TestPlanRebalancing_20PercentLimit validates that migration count never exceeds 20% of pod channels
func TestPlanRebalancing_20PercentLimit(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name              string
		totalChannels     int
		expectedMigration int
	}{
		{
			name:              "10 channels -> max 2 migrated",
			totalChannels:     10,
			expectedMigration: 2, // 20% of 10
		},
		{
			name:              "5 channels -> 1 migrated",
			totalChannels:     5,
			expectedMigration: 1, // 20% of 5 = 1
		},
		{
			name:              "3 channels -> 1 migrated (minimum)",
			totalChannels:     3,
			expectedMigration: 1, // 20% of 3 = 0.6, rounds to 1 (minimum)
		},
		{
			name:              "1 channel -> 1 migrated (minimum)",
			totalChannels:     1,
			expectedMigration: 1, // Always allow at least 1
		},
		{
			name:              "100 channels -> max 20 migrated",
			totalChannels:     100,
			expectedMigration: 20, // 20% of 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock assignments
			assignments := make([]*models.Assignment, tt.totalChannels)
			for i := 0; i < tt.totalChannels; i++ {
				assignments[i] = &models.Assignment{
					SourceID: fmt.Sprintf("channel-%d", i),
					PodID:    "pod-1",
				}
			}

			registry := &mockRegistry{
				assignments: map[string][]*models.Assignment{
					"pod-1": assignments,
				},
			}

			rebalancer := &Rebalancer{
				registry:          registry,
				maxMigrationRatio: 0.20,
				logger:            logger,
			}

			// Override channelLoadsFunc to return all 0 rates (doesn't matter for this test)
			rebalancer.channelLoadsFunc = func(ctx context.Context, assignments []*models.Assignment) []ChannelLoad {
				loads := make([]ChannelLoad, len(assignments))
				for i, a := range assignments {
					loads[i] = ChannelLoad{ChannelID: a.SourceID, MessageRate: 0.0}
				}
				return loads
			}

			loads := []PodLoad{
				{PodID: "pod-1", LoadScore: 100.0}, // overloaded
				{PodID: "pod-2", LoadScore: 50.0},  // underutilized
			}
			avgLoad := 75.0

			plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, 0)

			assert.NoError(t, err)
			assert.Len(t, plans, 1)
			assert.Equal(t, tt.expectedMigration, plans[0].MigrationCount,
				"Expected %d channels to migrate from %d total", tt.expectedMigration, tt.totalChannels)
		})
	}
}

// TestPlanRebalancing_RoundRobin validates that multiple overloaded pods distribute to underutilized via round-robin
func TestPlanRebalancing_RoundRobin(t *testing.T) {
	logger := zap.NewNop()

	// Three overloaded pods, two underutilized pods
	assignments := make([]*models.Assignment, 10)
	for i := 0; i < 10; i++ {
		assignments[i] = &models.Assignment{
			SourceID: fmt.Sprintf("channel-%d", i),
			PodID:    "pod-1",
		}
	}

	registry := &mockRegistry{
		assignments: map[string][]*models.Assignment{
			"pod-1": assignments,
			"pod-2": assignments,
			"pod-3": assignments,
		},
	}

	rebalancer := &Rebalancer{
		registry:          registry,
		maxMigrationRatio: 0.20,
		logger:            logger,
	}

	rebalancer.channelLoadsFunc = func(ctx context.Context, assignments []*models.Assignment) []ChannelLoad {
		loads := make([]ChannelLoad, len(assignments))
		for i, a := range assignments {
			loads[i] = ChannelLoad{ChannelID: a.SourceID, MessageRate: 0.0}
		}
		return loads
	}

	loads := []PodLoad{
		{PodID: "pod-1", LoadScore: 100.0}, // overloaded
		{PodID: "pod-2", LoadScore: 90.0},  // overloaded
		{PodID: "pod-3", LoadScore: 85.0},  // overloaded
		{PodID: "pod-4", LoadScore: 50.0},  // underutilized
		{PodID: "pod-5", LoadScore: 45.0},  // underutilized
	}
	avgLoad := 74.0

	plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, 0)

	assert.NoError(t, err)
	assert.Len(t, plans, 3, "Should have 3 plans for 3 overloaded pods")

	// Verify round-robin distribution
	// Plan 0 (pod-1) -> pod-4
	// Plan 1 (pod-2) -> pod-5
	// Plan 2 (pod-3) -> pod-4 (round-robin back to first underutilized)
	assert.Equal(t, "pod-1", plans[0].SourcePod)
	assert.Equal(t, "pod-4", plans[0].TargetPod)

	assert.Equal(t, "pod-2", plans[1].SourcePod)
	assert.Equal(t, "pod-5", plans[1].TargetPod)

	assert.Equal(t, "pod-3", plans[2].SourcePod)
	assert.Equal(t, "pod-4", plans[2].TargetPod) // Round-robin back to pod-4
}

// TestPlanRebalancing_NoUnderutilized validates error when all pods are overloaded
func TestPlanRebalancing_NoUnderutilized(t *testing.T) {
	logger := zap.NewNop()

	rebalancer := &Rebalancer{
		registry:          &mockRegistry{},
		maxMigrationRatio: 0.20,
		logger:            logger,
	}

	// All pods above average (no underutilized)
	loads := []PodLoad{
		{PodID: "pod-1", LoadScore: 100.0},
		{PodID: "pod-2", LoadScore: 90.0},
		{PodID: "pod-3", LoadScore: 85.0},
	}
	avgLoad := 80.0 // All pods above this

	plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, 0)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no underutilized pods")
	assert.Nil(t, plans)
}

// TestGetHotChannels validates >3x average detection
func TestGetHotChannels(t *testing.T) {
	logger := zap.NewNop()

	rebalancer := &Rebalancer{
		logger: logger,
	}

	channelLoads := []ChannelLoad{
		{ChannelID: "channel-normal-1", MessageRate: 50.0},
		{ChannelID: "channel-normal-2", MessageRate: 80.0},
		{ChannelID: "channel-hot-1", MessageRate: 400.0},  // >3x (300)
		{ChannelID: "channel-hot-2", MessageRate: 500.0},  // >3x (300)
		{ChannelID: "channel-normal-3", MessageRate: 150.0},
	}

	avgRate := 100.0 // 3x = 300

	hotChannels := rebalancer.getHotChannels(channelLoads, avgRate)

	assert.Len(t, hotChannels, 2)
	assert.Contains(t, hotChannels, "channel-hot-1")
	assert.Contains(t, hotChannels, "channel-hot-2")
}

// TestIncompleteRebalancing_HybridStrategy validates that after 3 consecutive imbalance cycles,
// the rebalancer escalates to hybrid strategy (proportional + hot channel migrations)
func TestIncompleteRebalancing_HybridStrategy(t *testing.T) {
	logger := zap.NewNop()

	// Mock registry with channels on overloaded pod
	// Use 10 channels total: 2 hot + 8 low to make math work
	registry := &mockRegistry{
		assignments: map[string][]*models.Assignment{
			"pod-1": {
				{SourceID: "channel-hot-1", PodID: "pod-1"},
				{SourceID: "channel-hot-2", PodID: "pod-1"},
				{SourceID: "channel-low-1", PodID: "pod-1"},
				{SourceID: "channel-low-2", PodID: "pod-1"},
				{SourceID: "channel-low-3", PodID: "pod-1"},
				{SourceID: "channel-low-4", PodID: "pod-1"},
				{SourceID: "channel-low-5", PodID: "pod-1"},
				{SourceID: "channel-low-6", PodID: "pod-1"},
				{SourceID: "channel-low-7", PodID: "pod-1"},
				{SourceID: "channel-low-8", PodID: "pod-1"},
			},
		},
	}

	rebalancer := &Rebalancer{
		registry:          registry,
		maxMigrationRatio: 0.20, // 20%
		logger:            logger,
	}

	// Mock channel loads: 2 hot channels + 8 low channels
	// Average = (1000 + 900 + 10*8) / 10 = (1900 + 80) / 10 = 198
	// 3x average = 594
	// channel-hot-1 (1000) > 594 ✓ HOT
	// channel-hot-2 (900) > 594 ✓ HOT
	rebalancer.channelLoadsFunc = func(ctx context.Context, assignments []*models.Assignment) []ChannelLoad {
		return []ChannelLoad{
			{ChannelID: "channel-hot-1", MessageRate: 1000.0}, // Hot
			{ChannelID: "channel-hot-2", MessageRate: 900.0},  // Hot
			{ChannelID: "channel-low-1", MessageRate: 10.0},
			{ChannelID: "channel-low-2", MessageRate: 10.0},
			{ChannelID: "channel-low-3", MessageRate: 10.0},
			{ChannelID: "channel-low-4", MessageRate: 10.0},
			{ChannelID: "channel-low-5", MessageRate: 10.0},
			{ChannelID: "channel-low-6", MessageRate: 10.0},
			{ChannelID: "channel-low-7", MessageRate: 10.0},
			{ChannelID: "channel-low-8", MessageRate: 10.0},
		}
	}

	loads := []PodLoad{
		{PodID: "pod-1", LoadScore: 100.0}, // overloaded
		{PodID: "pod-2", LoadScore: 50.0},  // underutilized
	}
	avgLoad := 75.0

	// Simulate 3 consecutive rebalancing attempts (incomplete)
	// Attempt 0, 1, 2: proportional strategy only (20% of 10 = 2 low-traffic channels)
	for attemptCount := 0; attemptCount < 3; attemptCount++ {
		plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, attemptCount)

		assert.NoError(t, err)
		assert.Len(t, plans, 1, "Expected 1 plan for attempt %d", attemptCount)

		// Verify proportional strategy (low-traffic channels selected)
		plan := plans[0]
		assert.Equal(t, 2, plan.MigrationCount, "Expected 2 channel migrations (20%% of 10) for attempt %d", attemptCount)
		// Channels should be lowest traffic (all low channels are rate 10.0, so any 2 are valid)
	}

	// Attempt 3: Hybrid strategy kicks in (proportional + hot channels)
	plans, err := rebalancer.PlanRebalancing(context.Background(), loads, avgLoad, 3)

	assert.NoError(t, err)
	assert.Len(t, plans, 2, "Expected 2 plans (proportional + hot channel)")

	// First plan: proportional strategy (2 low-traffic channels, 20% of 10)
	proportionalPlan := plans[0]
	assert.Equal(t, "pod-1", proportionalPlan.SourcePod)
	assert.Equal(t, "pod-2", proportionalPlan.TargetPod)
	assert.Equal(t, 2, proportionalPlan.MigrationCount, "Proportional strategy: 20%% of 10 channels")
	assert.Equal(t, 10, proportionalPlan.TotalChannels)

	// Second plan: hot channel strategy (up to 2 hot channels, ignores 20% limit)
	hotPlan := plans[1]
	assert.Equal(t, "pod-1", hotPlan.SourcePod)
	assert.Equal(t, "pod-2", hotPlan.TargetPod)
	assert.LessOrEqual(t, hotPlan.MigrationCount, 2, "Hot strategy migrates max 2 channels")
	assert.Greater(t, hotPlan.MigrationCount, 0, "Hot strategy should migrate at least 1 hot channel")

	// Verify hot channels are selected (channel-hot-1 and/or channel-hot-2)
	// Average = (1000 + 900 + 80) / 10 = 198, 3x = 594
	// channel-hot-1 (1000) > 594 ✓, channel-hot-2 (900) > 594 ✓
	for _, channelID := range hotPlan.Channels {
		assert.Contains(t, []string{"channel-hot-1", "channel-hot-2"}, channelID, "Hot plan should contain hot channels")
	}

	// Verify we have both hot channels (since both qualify and max is 2)
	assert.Len(t, hotPlan.Channels, 2, "Both hot channels should be migrated")
}

// mockRegistry is a mock implementation of AssignmentRegistryInterface for testing
type mockRegistry struct {
	assignments map[string][]*models.Assignment
}

func (m *mockRegistry) GetAssignmentsForPod(ctx context.Context, podID string) ([]*models.Assignment, error) {
	if assignments, ok := m.assignments[podID]; ok {
		return assignments, nil
	}
	return []*models.Assignment{}, nil
}

func (m *mockRegistry) StoreAssignment(ctx context.Context, sourceID, podID string) (int64, error) {
	return 0, nil
}
