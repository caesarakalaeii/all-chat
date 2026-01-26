package eventsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// EventSubAPIURL is the Twitch EventSub API endpoint
	EventSubAPIURL = "https://api.twitch.tv/helix/eventsub/subscriptions"

	// TokenURL is the OAuth token endpoint for app access tokens
	TokenURL = "https://id.twitch.tv/oauth2/token"
)

// SubscriptionManager manages EventSub subscriptions via the Twitch API
type SubscriptionManager struct {
	clientID     string
	clientSecret string
	accessToken  string
	tokenExpiry  time.Time
	sessionID    string
	logger       *zap.Logger

	// Track active subscriptions
	mu            sync.RWMutex
	subscriptions map[string]string // broadcaster_id -> subscription_id
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(clientID, clientSecret string, logger *zap.Logger) *SubscriptionManager {
	return &SubscriptionManager{
		clientID:      clientID,
		clientSecret:  clientSecret,
		logger:        logger,
		subscriptions: make(map[string]string),
	}
}

// SetSessionID sets the WebSocket session ID (from Welcome message)
func (sm *SubscriptionManager) SetSessionID(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionID = sessionID
}

// getAccessToken obtains or refreshes the app access token
func (sm *SubscriptionManager) getAccessToken(ctx context.Context) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Return cached token if valid
	if sm.accessToken != "" && time.Now().Before(sm.tokenExpiry) {
		return sm.accessToken, nil
	}

	// Request new token using client credentials flow
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&grant_type=client_credentials",
		TokenURL, sm.clientID, sm.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // seconds
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	sm.accessToken = tokenResp.AccessToken
	sm.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	sm.logger.Info("Obtained app access token",
		zap.Duration("expires_in", time.Until(sm.tokenExpiry)),
	)

	return sm.accessToken, nil
}

// SubscribeChannelPoints creates a subscription for channel point redemptions
// Uses the broadcaster's user OAuth token (required by Twitch EventSub for channel points)
func (sm *SubscriptionManager) SubscribeChannelPoints(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	sm.mu.RLock()
	sessionID := sm.sessionID
	sm.mu.RUnlock()

	if sessionID == "" {
		return "", fmt.Errorf("no active session")
	}

	// Check if already subscribed
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[broadcasterID]; exists {
		sm.mu.RUnlock()
		sm.logger.Debug("Already subscribed to channel points",
			zap.String("broadcaster_id", broadcasterID),
			zap.String("subscription_id", subID),
		)
		return subID, nil
	}
	sm.mu.RUnlock()

	// Create subscription request
	reqBody := map[string]interface{}{
		"type":    "channel.channel_points_custom_reward_redemption.add",
		"version": "1",
		"condition": map[string]string{
			"broadcaster_user_id": broadcasterID,
		},
		"transport": map[string]string{
			"method":     "websocket",
			"session_id": sessionID,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", EventSubAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create subscription request: %w", err)
	}

	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Authorization", "Bearer "+userAccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create subscription: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("subscription failed with status %d: %s", resp.StatusCode, string(body))
	}

	var subResp struct {
		Data []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"data"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(body, &subResp); err != nil {
		return "", fmt.Errorf("failed to decode subscription response: %w", err)
	}

	if len(subResp.Data) == 0 {
		return "", fmt.Errorf("subscription response missing data")
	}

	subscriptionID := subResp.Data[0].ID

	// Store subscription
	sm.mu.Lock()
	sm.subscriptions[broadcasterID] = subscriptionID
	sm.mu.Unlock()

	sm.logger.Info("Created EventSub subscription",
		zap.String("broadcaster_id", broadcasterID),
		zap.String("subscription_id", subscriptionID),
		zap.String("type", "channel.channel_points_custom_reward_redemption.add"),
	)

	return subscriptionID, nil
}

// Unsubscribe deletes a subscription
func (sm *SubscriptionManager) Unsubscribe(ctx context.Context, broadcasterID string) error {
	sm.mu.RLock()
	subscriptionID, exists := sm.subscriptions[broadcasterID]
	sm.mu.RUnlock()

	if !exists {
		return nil // Already unsubscribed
	}

	// Get access token
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	// Delete subscription
	url := fmt.Sprintf("%s?id=%s", EventSubAPIURL, subscriptionID)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Remove from tracking
	sm.mu.Lock()
	delete(sm.subscriptions, broadcasterID)
	sm.mu.Unlock()

	sm.logger.Info("Deleted EventSub subscription",
		zap.String("broadcaster_id", broadcasterID),
		zap.String("subscription_id", subscriptionID),
	)

	return nil
}

// GetActiveSubscriptions returns the list of active broadcaster IDs
func (sm *SubscriptionManager) GetActiveSubscriptions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	broadcasters := make([]string, 0, len(sm.subscriptions))
	for broadcasterID := range sm.subscriptions {
		broadcasters = append(broadcasters, broadcasterID)
	}
	return broadcasters
}

// GetUserIDByLogin resolves a Twitch username (login) to a user ID
func (sm *SubscriptionManager) GetUserIDByLogin(ctx context.Context, login string) (string, error) {
	// Get access token
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Call Twitch Helix API to get user by login
	url := fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", login)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", sm.clientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			ID    string `json:"id"`
			Login string `json:"login"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("user not found: %s", login)
	}

	return result.Data[0].ID, nil
}
