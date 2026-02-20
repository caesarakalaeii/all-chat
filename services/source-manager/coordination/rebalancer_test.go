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
