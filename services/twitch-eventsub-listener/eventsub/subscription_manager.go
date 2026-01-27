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
	clientID      string
	clientSecret  string
	accessToken   string
	tokenExpiry   time.Time
	webhookSecret string // Webhook secret for signature verification
	callbackURL   string // Public webhook callback URL
	logger        *zap.Logger

	// Track active subscriptions
	mu            sync.RWMutex
	subscriptions map[string]string // broadcaster_id -> subscription_id
}

// NewSubscriptionManager creates a new subscription manager for webhooks
func NewSubscriptionManager(clientID, clientSecret, webhookSecret, callbackURL string, logger *zap.Logger) *SubscriptionManager {
	return &SubscriptionManager{
		clientID:      clientID,
		clientSecret:  clientSecret,
		webhookSecret: webhookSecret,
		callbackURL:   callbackURL,
		logger:        logger,
		subscriptions: make(map[string]string),
	}
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
func (sm *SubscriptionManager) SubscribeChannelPoints(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	return sm.Subscribe(ctx, "channel.channel_points_custom_reward_redemption.add", broadcasterID, userAccessToken, "1")
}

// SubscribeToSubscriptions creates a subscription for subscription events
func (sm *SubscriptionManager) SubscribeToSubscriptions(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Use user token for this subscription type
	return sm.SubscribeWithUserToken(ctx, "channel.subscribe", broadcasterID, userAccessToken, "1")
}

// SubscribeToGifts creates a subscription for gift subscription events
func (sm *SubscriptionManager) SubscribeToGifts(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Use user token for this subscription type
	return sm.SubscribeWithUserToken(ctx, "channel.subscription.gift", broadcasterID, userAccessToken, "1")
}

// SubscribeToResubscriptions creates a subscription for resub message events
func (sm *SubscriptionManager) SubscribeToResubscriptions(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Use user token for this subscription type
	return sm.SubscribeWithUserToken(ctx, "channel.subscription.message", broadcasterID, userAccessToken, "1")
}

// SubscribeToRaids creates a subscription for raid events (when this broadcaster is raided)
func (sm *SubscriptionManager) SubscribeToRaids(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Raids need special handling - use to_broadcaster_user_id in condition
	cacheKey := broadcasterID + ":channel.raid"
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	// Get app access token (raids work with app token)
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Raid subscription needs to_broadcaster_user_id (when this channel is raided)
	condition := map[string]string{
		"to_broadcaster_user_id": broadcasterID,
	}

	return sm.subscribeWithCondition(ctx, "channel.raid", broadcasterID, token, "1", condition, cacheKey)
}

// SubscribeToCheers creates a subscription for bits/cheer events
func (sm *SubscriptionManager) SubscribeToCheers(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Use user token for this subscription type
	return sm.SubscribeWithUserToken(ctx, "channel.cheer", broadcasterID, userAccessToken, "1")
}

// SubscribeToFollows creates a subscription for follow events
func (sm *SubscriptionManager) SubscribeToFollows(ctx context.Context, broadcasterID string, userAccessToken string) (string, error) {
	// Follows need special handling - requires moderator_user_id
	cacheKey := broadcasterID + ":channel.follow"
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	// Follow subscription needs both broadcaster_user_id and moderator_user_id
	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
		"moderator_user_id":   broadcasterID,
	}

	return sm.subscribeWithCondition(ctx, "channel.follow", broadcasterID, userAccessToken, "2", condition, cacheKey)
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

// Subscribe creates an EventSub subscription with webhook transport
func (sm *SubscriptionManager) Subscribe(ctx context.Context, subscriptionType string, broadcasterID string, userAccessToken string, version string) (string, error) {
	// Check if already subscribed
	cacheKey := broadcasterID + ":" + subscriptionType
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	// Get app access token (required for webhook subscriptions)
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Build condition based on subscription type
	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
	}

	// Some subscription types require moderator_user_id
	if subscriptionType == "channel.follow" {
		condition["moderator_user_id"] = broadcasterID
	}

	// Create subscription request
	reqBody := map[string]interface{}{
		"type":    subscriptionType,
		"version": version,
		"condition": condition,
		"transport": map[string]interface{}{
			"method":   "webhook",
			"callback": sm.callbackURL,
			"secret":   sm.webhookSecret,
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
	req.Header.Set("Authorization", "Bearer "+token)
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
	sm.subscriptions[cacheKey] = subscriptionID
	sm.mu.Unlock()

	sm.logger.Info("Created EventSub subscription",
		zap.String("broadcaster_id", broadcasterID),
		zap.String("subscription_id", subscriptionID),
		zap.String("type", subscriptionType),
	)

	return subscriptionID, nil
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

// SubscribeWithUserToken creates an EventSub subscription using a user OAuth token
func (sm *SubscriptionManager) SubscribeWithUserToken(ctx context.Context, subscriptionType string, broadcasterID string, userAccessToken string, version string) (string, error) {
	// Check if already subscribed
	cacheKey := broadcasterID + ":" + subscriptionType
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
	}

	return sm.subscribeWithCondition(ctx, subscriptionType, broadcasterID, userAccessToken, version, condition, cacheKey)
}

// subscribeWithCondition creates an EventSub subscription with custom condition and token
func (sm *SubscriptionManager) subscribeWithCondition(ctx context.Context, subscriptionType string, broadcasterID string, accessToken string, version string, condition map[string]string, cacheKey string) (string, error) {
	// Create subscription request
	reqBody := map[string]interface{}{
		"type":    subscriptionType,
		"version": version,
		"condition": condition,
		"transport": map[string]interface{}{
			"method":   "webhook",
			"callback": sm.callbackURL,
			"secret":   sm.webhookSecret,
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
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
	sm.subscriptions[cacheKey] = subscriptionID
	sm.mu.Unlock()

	sm.logger.Info("Created EventSub subscription",
		zap.String("broadcaster_id", broadcasterID),
		zap.String("subscription_id", subscriptionID),
		zap.String("type", subscriptionType),
	)

	return subscriptionID, nil
}
