package coordination

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caesar/all-chat/shared/auth"
	"go.uber.org/zap"
)

// CoordinatorClient is an HTTP client for coordinator integration
type CoordinatorClient struct {
	baseURL       string
	serviceSecret string
	serviceJWT    string
	serviceName   string
	httpClient    *http.Client
	logger        *zap.Logger
}

// Assignment represents a channel assignment from the coordinator
// Matches source-manager models.Assignment structure
type Assignment struct {
	SourceID  string    `json:"source_id"`
	PodID     string    `json:"pod_id"`
	Timestamp time.Time `json:"timestamp"`
	Version   int       `json:"version"`
}

// HeartbeatRequest is the payload for heartbeat publishing
type HeartbeatRequest struct {
	PodID string `json:"pod_id"`
}

// NewCoordinatorClient creates a new coordinator client
func NewCoordinatorClient(baseURL, serviceSecret string, logger *zap.Logger) *CoordinatorClient {
	// Determine service name from hostname (pod name)
	hostname := os.Getenv("HOSTNAME")
	serviceName := "listener" // default
	if strings.HasPrefix(hostname, "twitch-listener") {
		serviceName = "twitch-listener"
	} else if strings.HasPrefix(hostname, "kick-listener") {
		serviceName = "kick-listener"
	} else if strings.HasPrefix(hostname, "tiktok-listener") {
		serviceName = "tiktok-listener"
	}

	// Generate initial JWT (24 hour expiry)
	jwt, err := auth.GenerateServiceJWT(serviceName, serviceSecret, 24*time.Hour)
	if err != nil {
		logger.Fatal("Failed to generate service JWT", zap.Error(err))
	}

	return &CoordinatorClient{
		baseURL:       baseURL,
		serviceSecret: serviceSecret,
		serviceJWT:    jwt,
		serviceName:   serviceName,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// QueryAssignments queries the coordinator for channel assignments for a specific pod
// Implements TWITCH-01, KICK-01, TIKTOK-01
// Blocks indefinitely with exponential backoff until coordinator responds (per CONTEXT.md user decision)
func (c *CoordinatorClient) QueryAssignments(ctx context.Context, podID string) ([]*Assignment, error) {
	url := fmt.Sprintf("%s/assignments?pod_id=%s", c.baseURL, podID)

	// Exponential backoff configuration: 1s, 2s, 4s, 8s, 16s, 30s (max)
	backoff := time.Second
	maxBackoff := 30 * time.Second

	c.logger.Info("Querying coordinator for assignments",
		zap.String("pod_id", podID),
		zap.String("url", url),
	)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Add JWT authorization header for SERVICE_JWT_AUTH middleware
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.serviceJWT))

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error - coordinator might be starting, retry with backoff
			c.logger.Warn("Failed to connect to coordinator, retrying with backoff",
				zap.String("pod_id", podID),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			time.Sleep(backoff)

			// Increase backoff exponentially
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			c.logger.Error("Failed to read response body",
				zap.String("pod_id", podID),
				zap.Int("status_code", resp.StatusCode),
				zap.Error(err),
			)
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		// Check for successful response
		if resp.StatusCode == http.StatusOK {
			var response AssignmentResponse
			if err := json.Unmarshal(body, &response); err != nil {
				c.logger.Error("Failed to parse assignments response",
					zap.String("pod_id", podID),
					zap.String("body", string(body)),
					zap.Error(err),
				)
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}

			c.logger.Info("Successfully retrieved assignments from coordinator",
				zap.String("pod_id", podID),
				zap.Int("assignment_count", response.Count),
			)

			return response.Assignments, nil
		}

		// Handle error responses
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			// Client error - configuration issue, don't retry
			c.logger.Error("Coordinator returned client error",
				zap.String("pod_id", podID),
				zap.Int("status_code", resp.StatusCode),
				zap.String("body", string(body)),
			)
			return nil, fmt.Errorf("coordinator returned %d: %s", resp.StatusCode, string(body))
		}

		if resp.StatusCode >= 500 {
			// Server error - coordinator might be starting, retry with backoff
			c.logger.Warn("Coordinator returned server error, retrying with backoff",
				zap.String("pod_id", podID),
				zap.Int("status_code", resp.StatusCode),
				zap.Duration("backoff", backoff),
			)
			time.Sleep(backoff)

			// Increase backoff exponentially
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Unexpected status code
		c.logger.Error("Coordinator returned unexpected status code",
			zap.String("pod_id", podID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}
}

// PublishHeartbeat publishes a heartbeat to the coordinator
// Returns nil on success (200 status), error otherwise
func (c *CoordinatorClient) PublishHeartbeat(ctx context.Context, podID string) error {
	url := fmt.Sprintf("%s/heartbeat", c.baseURL)

	// Create request body
	reqBody := HeartbeatRequest{
		PodID: podID,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	// Add headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.serviceJWT))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to publish heartbeat",
			zap.String("pod_id", podID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish heartbeat: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error messages
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Heartbeat request failed",
			zap.String("pod_id", podID),
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return fmt.Errorf("heartbeat failed with status %d: %s", resp.StatusCode, string(body))
	}

	c.logger.Debug("Successfully published heartbeat",
		zap.String("pod_id", podID),
	)

	return nil
}
