package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"go.uber.org/zap"
)

const (
	// InnerTubeEndpoint is the base URL for InnerTube live chat API
	InnerTubeEndpoint = "https://www.youtube.com/youtubei/v1/live_chat/get_live_chat"

	// DefaultAPIKey is the public InnerTube API key extracted from research
	// TODO: Phase 10 - Extract API key dynamically from stream HTML
	DefaultAPIKey = "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8"

	// DefaultClientVersion is the YouTube web client version
	// Update periodically to match current YouTube web client
	DefaultClientVersion = "2.20260312.01.00"

	// DefaultTimeout for HTTP requests
	DefaultTimeout = 10 * time.Second
)

// Client handles communication with the InnerTube API
type Client struct {
	httpClient *http.Client
	apiKey     string
	logger     *zap.Logger
	metrics    *metrics.InnerTubeMetrics
}

// ClientOptions configures the InnerTube client
type ClientOptions struct {
	APIKey     string
	Timeout    time.Duration
	Logger     *zap.Logger
	Metrics    *metrics.InnerTubeMetrics
}

// NewClient creates a new InnerTube API client
func NewClient(opts ClientOptions) *Client {
	if opts.APIKey == "" {
		opts.APIKey = DefaultAPIKey
	}
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
		apiKey:  opts.APIKey,
		logger:  opts.Logger,
		metrics: opts.Metrics,
	}
}

// GetLiveChatReplay fetches live chat messages using a continuation token
// For live streams, this uses the get_live_chat endpoint
func (c *Client) GetLiveChatReplay(ctx context.Context, continuation string) (*LiveChatResponse, error) {
	if continuation == "" {
		return nil, fmt.Errorf("continuation token is required")
	}

	// Construct InnerTube API URL with API key
	url := fmt.Sprintf("%s?key=%s", InnerTubeEndpoint, c.apiKey)

	// Build request payload matching InnerTube format
	payload := map[string]interface{}{
		"continuation": continuation,
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB",
				"clientVersion": DefaultClientVersion,
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request payload: %w", err)
	}

	c.logger.Debug("InnerTube API request",
		zap.String("url", url),
		zap.String("continuation", continuation[:min(len(continuation), 50)]+"..."),
	)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")

	// Track API request
	if c.metrics != nil {
		c.metrics.Requests.WithLabelValues(metrics.ServiceLabel).Inc()
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Classify and track network error
		if c.metrics != nil {
			errorType := classifyNetworkError(err)
			c.metrics.Errors.WithLabelValues(metrics.ServiceLabel, errorType).Inc()
		}
		return nil, fmt.Errorf("execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for error reporting
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Track parse error (failed to read body)
		if c.metrics != nil {
			c.metrics.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeParse).Inc()
		}
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		// Track specific error types based on status code
		if c.metrics != nil {
			if resp.StatusCode == http.StatusTooManyRequests {
				c.metrics.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeRateLimit).Inc()
			} else {
				c.metrics.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeHTTP).Inc()
			}
		}

		c.logger.Warn("InnerTube API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	// Parse JSON response
	var chatResp LiveChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		// Track parse error
		if c.metrics != nil {
			c.metrics.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeParse).Inc()
		}

		c.logger.Error("Failed to parse InnerTube response",
			zap.Error(err),
			zap.String("body_preview", string(body[:min(len(body), 200)])),
		)
		return nil, fmt.Errorf("decode JSON response: %w", err)
	}

	c.logger.Debug("InnerTube API response",
		zap.Int("action_count", len(chatResp.ContinuationContents.LiveChatContinuation.Actions)),
		zap.Int("continuation_count", len(chatResp.ContinuationContents.LiveChatContinuation.Continuations)),
	)

	return &chatResp, nil
}

// ExtractContinuation extracts the next continuation token from the response
// Returns empty string if no continuation is available (end of stream)
func (c *Client) ExtractContinuation(resp *LiveChatResponse) string {
	if resp == nil {
		return ""
	}

	continuations := resp.ContinuationContents.LiveChatContinuation.Continuations
	if len(continuations) == 0 {
		return ""
	}

	// Try each continuation type in priority order
	for _, cont := range continuations {
		if cont.TimedContinuationData != nil {
			return cont.TimedContinuationData.Continuation
		}
		if cont.InvalidationContinuationData != nil {
			return cont.InvalidationContinuationData.Continuation
		}
		if cont.LiveChatReplayContinuationData != nil {
			return cont.LiveChatReplayContinuationData.Continuation
		}
	}

	return ""
}

// GetPollInterval returns the recommended polling interval from the response
// Returns 0 if no interval is specified
func (c *Client) GetPollInterval(resp *LiveChatResponse) time.Duration {
	if resp == nil {
		return 0
	}

	continuations := resp.ContinuationContents.LiveChatContinuation.Continuations
	if len(continuations) == 0 {
		return 0
	}

	// Extract timeout from continuation data
	for _, cont := range continuations {
		if cont.TimedContinuationData != nil && cont.TimedContinuationData.TimeoutDurationMillis > 0 {
			return time.Duration(cont.TimedContinuationData.TimeoutDurationMillis) * time.Millisecond
		}
		if cont.InvalidationContinuationData != nil && cont.InvalidationContinuationData.TimeoutDurationMillis > 0 {
			return time.Duration(cont.InvalidationContinuationData.TimeoutDurationMillis) * time.Millisecond
		}
		if cont.LiveChatReplayContinuationData != nil && cont.LiveChatReplayContinuationData.TimeUntilLastMessageMsec > 0 {
			return time.Duration(cont.LiveChatReplayContinuationData.TimeUntilLastMessageMsec) * time.Millisecond
		}
	}

	return 0
}

// IsInitialized returns true if the client is ready to make API calls
// For PoC, this always returns true since the client is stateless
func (c *Client) IsInitialized() bool {
	return c != nil && c.httpClient != nil && c.apiKey != ""
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// classifyNetworkError determines if error is network-related
// Returns ErrorTypeNetwork for DNS, connection, timeout, and TLS errors
func classifyNetworkError(err error) string {
	if err == nil {
		return ""
	}

	// Check for common network errors
	errStr := err.Error()

	// DNS errors
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") {
		return metrics.ErrorTypeNetwork
	}

	// Connection errors
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") {
		return metrics.ErrorTypeNetwork
	}

	// Timeout errors
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return metrics.ErrorTypeNetwork
	}

	// TLS errors
	if strings.Contains(errStr, "tls") ||
		strings.Contains(errStr, "certificate") {
		return metrics.ErrorTypeNetwork
	}

	// Default to network for unknown errors before HTTP call
	return metrics.ErrorTypeNetwork
}
