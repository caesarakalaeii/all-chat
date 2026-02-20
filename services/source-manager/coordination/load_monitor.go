package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LoadMonitor monitors pod load and calculates composite load scores
type LoadMonitor struct {
	redisClient    *redis.Client
	prometheusURL  string
	httpClient     *http.Client
	metrics        *metrics.ShardMetrics
	logger         *zap.Logger
}

// PodLoad represents the load information for a single pod
type PodLoad struct {
	PodID        string
	ChannelCount int
	MessageRate  float64 // messages/sec from Prometheus
	LoadScore    float64 // composite weighted score
}

// prometheusQueryResult represents the Prometheus API response structure
type prometheusQueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// NewLoadMonitor creates a new LoadMonitor instance
func NewLoadMonitor(
	redisClient *redis.Client,
	prometheusURL string,
	shardMetrics *metrics.ShardMetrics,
	logger *zap.Logger,
) *LoadMonitor {
	return &LoadMonitor{
		redisClient:   redisClient,
		prometheusURL: prometheusURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		metrics: shardMetrics,
		logger:  logger,
	}
}

// CalculateLoadScore computes composite load score from channel count and message rate
// Uses weighted combination: 70% message rate, 30% channel count
// Rationale: Message processing dominates CPU (per RESEARCH.md), connection overhead is secondary
func CalculateLoadScore(channelCount int, messageRate float64) float64 {
	const (
		messageRateWeight  = 0.7 // Message processing dominates CPU
		channelCountWeight = 0.3 // Connection overhead (memory, goroutines)
	)

	return (messageRate * messageRateWeight) + (float64(channelCount) * channelCountWeight)
}

// MonitorPodLoads queries current load for all healthy pods
// Returns PodLoad array with channel count, message rate, and composite load score
// Gracefully degrades to channel count only if Prometheus unavailable
func (m *LoadMonitor) MonitorPodLoads(ctx context.Context, podIDs []string) ([]PodLoad, error) {
	loads := make([]PodLoad, 0, len(podIDs))

	m.logger.Debug("Monitoring pod loads", zap.Int("pod_count", len(podIDs)))

	for _, podID := range podIDs {
		// Query channel count from Redis assignment registry
		channelCount, err := m.getChannelCount(ctx, podID)
		if err != nil {
			m.logger.Error("Failed to get channel count",
				zap.String("pod_id", podID),
				zap.Error(err),
			)
			// Continue processing remaining pods
			continue
		}

		// Query message rate from Prometheus (last 30s average)
		// Graceful degradation: default to 0 if Prometheus unavailable
		messageRate, err := m.getMessageRate(ctx, podID)
		if err != nil {
			m.logger.Warn("Failed to get message rate, using 0 (graceful degradation)",
				zap.String("pod_id", podID),
				zap.Error(err),
			)
			messageRate = 0
		}

		// Calculate composite load score
		loadScore := CalculateLoadScore(channelCount, messageRate)

		m.logger.Debug("Pod load calculated",
			zap.String("pod_id", podID),
			zap.Int("channel_count", channelCount),
			zap.Float64("message_rate", messageRate),
			zap.Float64("load_score", loadScore),
		)

		// Update per-pod load score metric
		m.metrics.PodLoadScore.WithLabelValues(podID).Set(loadScore)

		loads = append(loads, PodLoad{
			PodID:        podID,
			ChannelCount: channelCount,
			MessageRate:  messageRate,
			LoadScore:    loadScore,
		})
	}

	m.logger.Info("Pod load monitoring complete",
		zap.Int("pods_monitored", len(loads)),
	)

	return loads, nil
}

// getChannelCount queries Redis Sorted Set for pod's channel count
// Uses Phase 5 infrastructure: shard:load sorted set (O(1) ZCOUNT)
func (m *LoadMonitor) getChannelCount(ctx context.Context, podID string) (int, error) {
	// Query Redis Sorted Set shard:load via ZSCORE
	// Phase 5 infrastructure: score represents channel count for pod
	score, err := m.redisClient.ZScore(ctx, "shard:load", podID).Result()
	if err != nil {
		// If pod not found, it's a new pod with no assignments yet
		if err == redis.Nil {
			m.logger.Debug("Pod not found in load registry (new pod with no assignments)",
				zap.String("pod_id", podID),
			)
			return 0, nil
		}
		return 0, fmt.Errorf("failed to query channel count: %w", err)
	}

	return int(score), nil
}

// getMessageRate queries Prometheus for pod's message rate (last 30s average)
// PromQL: rate(listener_messages_received_total{pod=~"podID.*"}[30s])
// Returns total message rate across all channels for this pod
func (m *LoadMonitor) getMessageRate(ctx context.Context, podID string) (float64, error) {
	// Build PromQL query: rate over 30s window (per requirements)
	query := fmt.Sprintf(`sum(rate(listener_messages_received_total{pod=~"%s.*"}[30s]))`, podID)

	// HTTP GET to Prometheus /api/v1/query endpoint
	url := fmt.Sprintf("%s/api/v1/query?query=%s", m.prometheusURL, query)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create Prometheus request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		// Network error - return 0 (graceful degradation)
		m.logger.Warn("Prometheus unavailable (network error), defaulting to 0",
			zap.String("pod_id", podID),
			zap.Error(err),
		)
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
		// No data returned (pod has no metrics yet) - graceful degradation
		m.logger.Debug("No message rate data for pod (no metrics yet)",
			zap.String("pod_id", podID),
		)
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
