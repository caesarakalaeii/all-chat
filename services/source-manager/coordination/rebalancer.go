package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"go.uber.org/zap"
)

// AssignmentRegistryInterface defines the methods needed by Rebalancer
// This allows for dependency injection in tests
type AssignmentRegistryInterface interface {
	GetAssignmentsForPod(ctx context.Context, podID string) ([]*models.Assignment, error)
	StoreAssignment(ctx context.Context, sourceID, podID string) (int64, error)
}

// Rebalancer implements proportional channel redistribution strategy
// Moves many low-traffic channels (not just hot channels) from overloaded pods
// to underutilized pods, enforcing 20% per-pod migration limits
type Rebalancer struct {
	registry           AssignmentRegistryInterface
	assigner           *Assigner
	migrationPublisher *MigrationPublisher
	prometheusURL      string
	httpClient         *http.Client
	maxMigrationRatio  float64 // 0.20 (20% per CONTEXT.md)
	logger             *zap.Logger

	// channelLoadsFunc allows injection for testing
	channelLoadsFunc func(ctx context.Context, assignments []*models.Assignment) []ChannelLoad
}

// MigrationPlan represents a planned migration from source pod to target pod
type MigrationPlan struct {
	SourcePod      string
	TargetPod      string
	Channels       []string // Channel IDs to migrate
	TotalChannels  int      // Total channels on source pod
	MigrationCount int      // Number of channels being migrated
}

// ChannelLoad represents per-channel message rate information
type ChannelLoad struct {
	ChannelID   string
	MessageRate float64 // From Prometheus query
}

// NewRebalancer creates a new rebalancer instance
func NewRebalancer(
	registry AssignmentRegistryInterface,
	assigner *Assigner,
	migrationPublisher *MigrationPublisher,
	prometheusURL string,
	logger *zap.Logger,
) *Rebalancer {
	r := &Rebalancer{
		registry:           registry,
		assigner:           assigner,
		migrationPublisher: migrationPublisher,
		prometheusURL:      prometheusURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxMigrationRatio: 0.20, // 20% per CONTEXT.md
		logger:            logger,
	}

	// Set default implementation
	r.channelLoadsFunc = r.getChannelLoadsImpl

	return r
}

// PlanRebalancing analyzes pod loads and creates migration plans
// Uses proportional redistribution strategy: moves low-traffic channels first
// Enforces 20% per-pod migration limit to prevent thrashing
// Selects target pods via round-robin across underutilized pods
// If attemptCount >= 3, escalates to hybrid strategy (adds hot channel migrations)
func (r *Rebalancer) PlanRebalancing(ctx context.Context, loads []PodLoad, avgLoad float64, attemptCount int) ([]MigrationPlan, error) {
	r.logger.Info("Planning rebalancing",
		zap.Int("pod_count", len(loads)),
		zap.Float64("avg_load", avgLoad),
	)

	// Separate pods into overloaded and underutilized
	var overloadedPods []PodLoad
	var underutilizedPods []PodLoad

	for _, load := range loads {
		if load.LoadScore > avgLoad {
			overloadedPods = append(overloadedPods, load)
		} else if load.LoadScore < avgLoad {
			underutilizedPods = append(underutilizedPods, load)
		}
	}

	// Validate underutilized pods available
	if len(underutilizedPods) == 0 {
		r.logger.Warn("No underutilized pods available for rebalancing")
		return nil, fmt.Errorf("no underutilized pods available")
	}

	r.logger.Info("Identified overloaded and underutilized pods",
		zap.Int("overloaded_count", len(overloadedPods)),
		zap.Int("underutilized_count", len(underutilizedPods)),
	)

	// Sort overloaded pods by load score descending (highest load first)
	sort.Slice(overloadedPods, func(i, j int) bool {
		return overloadedPods[i].LoadScore > overloadedPods[j].LoadScore
	})

	// Initialize round-robin target index
	targetIdx := 0
	var plans []MigrationPlan

	// For each overloaded pod, create migration plan
	for _, overloadedPod := range overloadedPods {
		// Query all assigned channels
		assignments, err := r.registry.GetAssignmentsForPod(ctx, overloadedPod.PodID)
		if err != nil {
			r.logger.Error("Failed to get assignments for overloaded pod",
				zap.String("pod_id", overloadedPod.PodID),
				zap.Error(err),
			)
			continue
		}

		if len(assignments) == 0 {
			r.logger.Debug("Overloaded pod has no assignments",
				zap.String("pod_id", overloadedPod.PodID),
			)
			continue
		}

		// Get per-channel message rates
		channelLoads := r.channelLoadsFunc(ctx, assignments)

		// Proportional strategy: Sort channels by message rate ascending (lowest traffic first)
		// Rationale per CONTEXT.md: "Prefer moving many low-traffic channels over few high-traffic channels (reduces migration risk)"
		sort.Slice(channelLoads, func(i, j int) bool {
			return channelLoads[i].MessageRate < channelLoads[j].MessageRate
		})

		// Apply 20% limit with minimum 1 channel
		maxMigrations := int(float64(len(assignments)) * r.maxMigrationRatio)
		if maxMigrations == 0 {
			maxMigrations = 1 // Always allow at least 1 channel to migrate
		}

		// Select channels to migrate (bottom maxMigrations channels = lowest traffic)
		channelsToMigrate := make([]string, 0, maxMigrations)
		for i := 0; i < maxMigrations && i < len(channelLoads); i++ {
			channelsToMigrate = append(channelsToMigrate, channelLoads[i].ChannelID)
		}

		// Round-robin target selection
		targetPod := underutilizedPods[targetIdx%len(underutilizedPods)]
		targetIdx++

		// Build migration plan
		plan := MigrationPlan{
			SourcePod:      overloadedPod.PodID,
			TargetPod:      targetPod.PodID,
			Channels:       channelsToMigrate,
			TotalChannels:  len(assignments),
			MigrationCount: len(channelsToMigrate),
		}

		migrationRatio := float64(len(channelsToMigrate)) / float64(len(assignments)) * 100

		r.logger.Info("Planned migration",
			zap.String("from_pod", plan.SourcePod),
			zap.String("to_pod", plan.TargetPod),
			zap.Int("channels", len(channelsToMigrate)),
			zap.Int("total_channels", len(assignments)),
			zap.Float64("ratio_percent", migrationRatio),
		)

		plans = append(plans, plan)
	}

	// Hybrid strategy escalation: if attemptCount >= 3, add hot channel migrations
	// This handles persistent imbalance where proportional strategy alone is insufficient
	if attemptCount >= 3 {
		r.logger.Warn("Incomplete rebalancing detected, enabling hot channel migration",
			zap.Int("attempt_count", attemptCount),
		)

		hotPlans := r.hotChannelStrategy(ctx, overloadedPods, underutilizedPods, avgLoad, targetIdx)
		plans = append(plans, hotPlans...)

		r.logger.Info("Added hot channel migrations to plans",
			zap.Int("hot_channel_plans", len(hotPlans)),
		)
	}

	r.logger.Info("Rebalancing plan complete",
		zap.Int("migration_plans", len(plans)),
		zap.Int("total_plans", len(plans)),
	)

	return plans, nil
}

// getChannelLoadsImpl is the default implementation that queries Prometheus
// Returns ChannelLoad array with message rate for each channel
// Defaults to 0.0 for channels with no metrics or query failures (graceful degradation)
func (r *Rebalancer) getChannelLoadsImpl(ctx context.Context, assignments []*models.Assignment) []ChannelLoad {
	channelLoads := make([]ChannelLoad, 0, len(assignments))

	for _, assignment := range assignments {
		// Query Prometheus: rate(listener_messages_received_total{channel_id="X"}[5m])
		// 5-minute window for stability (longer than 30s monitoring window)
		rate, err := r.getChannelMessageRate(ctx, assignment.SourceID)
		if err != nil {
			// Default to 0.0 for missing data (treat as low-traffic)
			r.logger.Debug("Failed to get channel message rate, defaulting to 0",
				zap.String("channel_id", assignment.SourceID),
				zap.Error(err),
			)
			rate = 0.0
		}

		channelLoads = append(channelLoads, ChannelLoad{
			ChannelID:   assignment.SourceID,
			MessageRate: rate,
		})
	}

	return channelLoads
}

// getChannelMessageRate queries Prometheus for a single channel's message rate
func (r *Rebalancer) getChannelMessageRate(ctx context.Context, channelID string) (float64, error) {
	// Build PromQL query: rate over 5-minute window for stability
	query := fmt.Sprintf(`rate(listener_messages_received_total{channel_id="%s"}[5m])`, channelID)

	// HTTP GET to Prometheus /api/v1/query endpoint
	url := fmt.Sprintf("%s/api/v1/query?query=%s", r.prometheusURL, query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create Prometheus request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		// Network error - return 0 (graceful degradation)
		return 0, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus query failed: status %d", resp.StatusCode)
	}

	// Parse JSON response
	var result prometheusQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse Prometheus response: %w", err)
	}

	if result.Status != "success" {
		return 0, fmt.Errorf("prometheus query status: %s", result.Status)
	}

	// Extract value from result
	if len(result.Data.Result) == 0 {
		// No data returned (channel has no metrics yet) - graceful degradation
		return 0, nil
	}

	// Parse sample value (format: [timestamp, "value"])
	valueArray, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value format in Prometheus response")
	}

	var messageRate float64
	if _, err := fmt.Sscanf(valueArray, "%f", &messageRate); err != nil {
		return 0, fmt.Errorf("failed to parse message rate: %w", err)
	}

	return messageRate, nil
}

// getHotChannels identifies channels with message rate > 3x average
// Used for logging and observability (per REBAL-04 requirement)
// Not used for channel selection - proportional strategy takes precedence
func (r *Rebalancer) getHotChannels(channelLoads []ChannelLoad, avgRate float64) []string {
	hotChannels := make([]string, 0)

	hotThreshold := avgRate * 3.0 // >3x average per REBAL-04

	for _, load := range channelLoads {
		if load.MessageRate > hotThreshold {
			hotChannels = append(hotChannels, load.ChannelID)
		}
	}

	if len(hotChannels) > 0 {
		r.logger.Info("Identified hot channels",
			zap.Int("hot_channel_count", len(hotChannels)),
			zap.Float64("avg_rate", avgRate),
			zap.Float64("hot_threshold", hotThreshold),
		)
	}

	return hotChannels
}

// hotChannelStrategy selects hot channels (>3x average message rate) from overloaded pods
// Ignores 20% migration limit - this is the escalation strategy for persistent imbalance
// Used when proportional strategy fails to resolve imbalance after 3 attempts
func (r *Rebalancer) hotChannelStrategy(ctx context.Context, overloadedPods []PodLoad, underutilizedPods []PodLoad, avgLoad float64, startIdx int) []MigrationPlan {
	if len(underutilizedPods) == 0 {
		r.logger.Warn("No underutilized pods available for hot channel strategy")
		return nil
	}

	var hotPlans []MigrationPlan
	targetIdx := startIdx

	for _, overloadedPod := range overloadedPods {
		// Query all assigned channels
		assignments, err := r.registry.GetAssignmentsForPod(ctx, overloadedPod.PodID)
		if err != nil {
			r.logger.Error("Failed to get assignments for overloaded pod (hot strategy)",
				zap.String("pod_id", overloadedPod.PodID),
				zap.Error(err),
			)
			continue
		}

		if len(assignments) == 0 {
			continue
		}

		// Get per-channel message rates
		channelLoads := r.channelLoadsFunc(ctx, assignments)

		// Calculate average channel rate for this pod
		var totalRate float64
		for _, load := range channelLoads {
			totalRate += load.MessageRate
		}
		avgChannelRate := totalRate / float64(len(channelLoads))

		// Hot threshold: >3x average rate (per REBAL-04)
		hotThreshold := avgChannelRate * 3.0

		// Select hot channels (highest rate first)
		sort.Slice(channelLoads, func(i, j int) bool {
			return channelLoads[i].MessageRate > channelLoads[j].MessageRate
		})

		// Select top 1-2 hot channels per pod (ignoring 20% limit)
		maxHotChannels := 2
		hotChannels := make([]string, 0, maxHotChannels)

		for i := 0; i < len(channelLoads) && len(hotChannels) < maxHotChannels; i++ {
			if channelLoads[i].MessageRate > hotThreshold {
				hotChannels = append(hotChannels, channelLoads[i].ChannelID)
			}
		}

		if len(hotChannels) == 0 {
			r.logger.Debug("No hot channels found on overloaded pod",
				zap.String("pod_id", overloadedPod.PodID),
				zap.Float64("hot_threshold", hotThreshold),
			)
			continue
		}

		// Round-robin target selection
		targetPod := underutilizedPods[targetIdx%len(underutilizedPods)]
		targetIdx++

		// Build hot channel migration plan
		plan := MigrationPlan{
			SourcePod:      overloadedPod.PodID,
			TargetPod:      targetPod.PodID,
			Channels:       hotChannels,
			TotalChannels:  len(assignments),
			MigrationCount: len(hotChannels),
		}

		r.logger.Info("Planned hot channel migration",
			zap.String("from_pod", plan.SourcePod),
			zap.String("to_pod", plan.TargetPod),
			zap.Int("hot_channels", len(hotChannels)),
			zap.Int("total_channels", len(assignments)),
			zap.Float64("hot_threshold", hotThreshold),
		)

		hotPlans = append(hotPlans, plan)
	}

	return hotPlans
}
