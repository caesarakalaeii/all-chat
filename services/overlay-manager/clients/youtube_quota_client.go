package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// YouTubeQuotaClient is an HTTP client for tracking YouTube API quota usage
// via the youtube-listener service's quota API
type YouTubeQuotaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// QuotaStatus represents the global quota status from youtube-listener
type QuotaStatus struct {
	Used       int     `json:"used"`
	Remaining  int     `json:"remaining"`
	Limit      int     `json:"limit"`
	Percentage float64 `json:"percentage"`
}

// NewYouTubeQuotaClient creates a new quota client
func NewYouTubeQuotaClient(baseURL string, logger *zap.Logger) *YouTubeQuotaClient {
	return &YouTubeQuotaClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// CheckQuota validates if we have sufficient quota for a request
// Returns: allowed (bool), error
// Fails open (returns true) on network errors to prevent blocking users
func (c *YouTubeQuotaClient) CheckQuota(ctx context.Context, units int) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/quota/status", nil)
	if err != nil {
		c.logger.Warn("Failed to create quota check request (allowing request)", zap.Error(err))
		return true, nil  // Fail open
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("Failed to check quota - youtube-listener unavailable (allowing request)", zap.Error(err))
		return true, nil  // Fail open - don't block users if quota service is down
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Quota API returned non-200 status (allowing request)",
			zap.Int("status", resp.StatusCode),
		)
		return true, nil  // Fail open
	}

	var response struct {
		Global QuotaStatus `json:"global"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Warn("Failed to decode quota response (allowing request)", zap.Error(err))
		return true, nil  // Fail open
	}

	// Check if we have enough quota
	allowed := response.Global.Remaining >= units

	if !allowed {
		c.logger.Warn("Insufficient YouTube quota",
			zap.Int("required", units),
			zap.Int("remaining", response.Global.Remaining),
			zap.Float64("percentage_used", response.Global.Percentage),
		)
	}

	return allowed, nil
}

// RecordUsage records quota usage after a YouTube API call
// Always call this after making YouTube API calls, even if the call failed
// (YouTube charges quota for most error scenarios)
func (c *YouTubeQuotaClient) RecordUsage(ctx context.Context, units int) error {
	payload := map[string]int{
		"units": units,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("Failed to marshal quota usage payload", zap.Error(err))
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/quota/record", bytes.NewReader(body))
	if err != nil {
		c.logger.Error("Failed to create quota record request", zap.Error(err))
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to record quota usage - youtube-listener unavailable",
			zap.Int("units", units),
			zap.Error(err),
		)
		return fmt.Errorf("failed to record usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		c.logger.Error("Quota record API returned error status",
			zap.Int("status", resp.StatusCode),
			zap.Int("units", units),
		)
		return fmt.Errorf("quota API returned status %d", resp.StatusCode)
	}

	c.logger.Debug("Recorded quota usage",
		zap.Int("units", units),
	)

	return nil
}

// GetQuotaStatus fetches the current quota status (optional, for diagnostics)
func (c *YouTubeQuotaClient) GetQuotaStatus(ctx context.Context) (*QuotaStatus, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/quota/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quota API returned status %d", resp.StatusCode)
	}

	var response struct {
		Global QuotaStatus `json:"global"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response.Global, nil
}
