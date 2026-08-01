// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package eventsub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	httpClient    *http.Client // Dedicated client with timeout (audit L25)

	// Endpoint overrides — default to the package consts EventSubAPIURL/TokenURL,
	// but overridable in tests so HTTP interactions can be pointed at httptest servers.
	apiURL   string
	tokenURL string

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
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		apiURL:        EventSubAPIURL,
		tokenURL:      TokenURL,
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
	tokenURL := fmt.Sprintf("%s?client_id=%s&client_secret=%s&grant_type=client_credentials",
		sm.tokenURL, sm.clientID, sm.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := sm.httpClient.Do(req)
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
func (sm *SubscriptionManager) SubscribeChannelPoints(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.channel_points_custom_reward_redemption.add", broadcasterID, "1")
}

// SubscribeToSubscriptions creates a subscription for subscription events
func (sm *SubscriptionManager) SubscribeToSubscriptions(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.subscribe", broadcasterID, "1")
}

// SubscribeToGifts creates a subscription for gift subscription events
func (sm *SubscriptionManager) SubscribeToGifts(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.subscription.gift", broadcasterID, "1")
}

// SubscribeToResubscriptions creates a subscription for resub message events
func (sm *SubscriptionManager) SubscribeToResubscriptions(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.subscription.message", broadcasterID, "1")
}

// SubscribeToRaids creates a subscription for raid events (when this broadcaster is raided)
func (sm *SubscriptionManager) SubscribeToRaids(ctx context.Context, broadcasterID string) (string, error) {
	// Raids need special handling - use to_broadcaster_user_id in condition
	cacheKey := broadcasterID + ":channel.raid"
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	// Get app access token
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
func (sm *SubscriptionManager) SubscribeToCheers(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.cheer", broadcasterID, "1")
}

// SubscribeToFollows creates a subscription for follow events
func (sm *SubscriptionManager) SubscribeToFollows(ctx context.Context, broadcasterID string) (string, error) {
	// Follows need special handling - requires moderator_user_id
	cacheKey := broadcasterID + ":channel.follow"
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	// Get app access token
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get app access token: %w", err)
	}

	// Follow subscription needs both broadcaster_user_id and moderator_user_id
	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
		"moderator_user_id":   broadcasterID,
	}

	return sm.subscribeWithCondition(ctx, "channel.follow", broadcasterID, token, "2", condition, cacheKey)
}

// SubscribeToStreamOffline creates a subscription for stream offline events.
// Requires only app access token (no user OAuth scope needed).
func (sm *SubscriptionManager) SubscribeToStreamOffline(ctx context.Context, broadcasterID string) (string, error) {
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	cacheKey := broadcasterID + ":stream.offline"
	condition := map[string]string{"broadcaster_user_id": broadcasterID}
	return sm.subscribeWithCondition(ctx, "stream.offline", broadcasterID, token, "1", condition, cacheKey)
}

// SubscribeToStreamOnline creates a subscription for stream online events.
// Requires only app access token (no user OAuth scope needed).
func (sm *SubscriptionManager) SubscribeToStreamOnline(ctx context.Context, broadcasterID string) (string, error) {
	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	cacheKey := broadcasterID + ":stream.online"
	condition := map[string]string{"broadcaster_user_id": broadcasterID}
	return sm.subscribeWithCondition(ctx, "stream.online", broadcasterID, token, "1", condition, cacheKey)
}

// Poll subscriptions (channel.poll.*, all v1, plain broadcaster_user_id condition).
// They require the broadcaster's channel:read:polls grant, validated by Twitch at
// creation time; a scope error means the owner hasn't opted into engagement mirroring
// and is handled non-fatally by the caller (issue #523, task H).
func (sm *SubscriptionManager) SubscribeToPollBegin(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.poll.begin", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToPollProgress(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.poll.progress", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToPollEnd(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.poll.end", broadcasterID, "1")
}

// Prediction subscriptions (channel.prediction.*, all v1). Require the broadcaster's
// channel:read:predictions grant; same non-fatal scope-error handling as polls.
func (sm *SubscriptionManager) SubscribeToPredictionBegin(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.prediction.begin", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToPredictionProgress(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.prediction.progress", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToPredictionLock(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.prediction.lock", broadcasterID, "1")
}

func (sm *SubscriptionManager) SubscribeToPredictionEnd(ctx context.Context, broadcasterID string) (string, error) {
	return sm.Subscribe(ctx, "channel.prediction.end", broadcasterID, "1")
}

// SubscribeToChatMessages creates a channel.chat.message subscription for reading chat.
//
// For own-channel reading the broadcaster authorizes their own channel, so the condition
// carries broadcaster_user_id == user_id == the streamer's Twitch ID. Webhook transport
// uses the app access token, but Twitch additionally requires the chatter (here, the
// broadcaster) to hold user:read:chat + user:bot, plus channel:bot on the channel — all
// satisfied because broadcaster == chatter. Authorization is validated by Twitch at
// subscription-creation time; a 4xx scope error means the streamer must re-auth with the
// chat scopes (handled non-fatally by the caller).
func (sm *SubscriptionManager) SubscribeToChatMessages(ctx context.Context, broadcasterID string) (string, error) {
	return sm.subscribeChatScoped(ctx, "channel.chat.message", broadcasterID)
}

// SubscribeToChatMessageDelete creates a channel.chat.message_delete subscription — fired when a
// moderator removes a single message. It honors the same own-channel authorization as
// channel.chat.message (user:read:chat + user:bot, broadcaster == chatter), so it is created and
// torn down together with the chat subscription. Without it, single-message deletions on
// EventSub-owned channels never reach the overlay, because IRC (which handled CLEARMSG) no longer
// sees these channels (ADR-0015).
func (sm *SubscriptionManager) SubscribeToChatMessageDelete(ctx context.Context, broadcasterID string) (string, error) {
	return sm.subscribeChatScoped(ctx, "channel.chat.message_delete", broadcasterID)
}

// SubscribeToChatClearUserMessages creates a channel.chat.clear_user_messages subscription — fired
// when all of a user's messages are removed (timeout or ban). Same authorization/lifecycle as
// channel.chat.message. Replaces, for EventSub-owned channels, the user-targeted CLEARCHAT that IRC
// no longer receives.
func (sm *SubscriptionManager) SubscribeToChatClearUserMessages(ctx context.Context, broadcasterID string) (string, error) {
	return sm.subscribeChatScoped(ctx, "channel.chat.clear_user_messages", broadcasterID)
}

// SubscribeToChatClear creates a channel.chat.clear subscription — fired when the entire chat is
// cleared. Same authorization/lifecycle as channel.chat.message. Replaces, for EventSub-owned
// channels, the full CLEARCHAT that IRC no longer receives.
func (sm *SubscriptionManager) SubscribeToChatClear(ctx context.Context, broadcasterID string) (string, error) {
	return sm.subscribeChatScoped(ctx, "channel.chat.clear", broadcasterID)
}

// SubscribeToChatNotifications creates a channel.chat.notification subscription — Twitch's feed of
// "events that appear in chat". It shares the chat subscription's authorization and lifecycle
// (user:read:chat + user:bot, broadcaster == chatter), so it is created and torn down alongside it.
//
// It is not optional decoration: watch streaks and announcements are delivered ONLY here, and both
// carry the chatter's own message text, so without this subscription those messages never arrive at
// all (ADR-0046). Notices that a dedicated subscription already delivers are dropped by the handler.
func (sm *SubscriptionManager) SubscribeToChatNotifications(ctx context.Context, broadcasterID string) (string, error) {
	return sm.subscribeChatScoped(ctx, "channel.chat.notification", broadcasterID)
}

// subscribeChatScoped creates a channel.chat.* EventSub subscription whose condition is the
// own-channel reading pair broadcaster_user_id == user_id == broadcasterID. Every channel.chat.*
// type (message, message_delete, clear_user_messages, clear) shares this exact condition, version,
// scope (user:read:chat) and webhook transport, so they are created and cached identically and gated
// together on chat scope + live-overlay demand. Cached under "broadcasterID:subType".
func (sm *SubscriptionManager) subscribeChatScoped(ctx context.Context, subType, broadcasterID string) (string, error) {
	cacheKey := broadcasterID + ":" + subType
	sm.mu.RLock()
	if subID, exists := sm.subscriptions[cacheKey]; exists {
		sm.mu.RUnlock()
		return subID, nil
	}
	sm.mu.RUnlock()

	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get app access token: %w", err)
	}

	// chatter (user_id) == broadcaster for own-channel reading
	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
		"user_id":             broadcasterID,
	}

	return sm.subscribeWithCondition(ctx, subType, broadcasterID, token, "1", condition, cacheKey)
}

// Unsubscribe deletes ALL EventSub subscriptions for a broadcaster.
//
// Every subscription is cached under "broadcasterID:type" (see subscribeWithCondition),
// so we delete every cache entry carrying that prefix — not a bare "broadcasterID" key,
// which never matches and previously made this a silent no-op that leaked subscriptions.
// That leak matters most for channel.chat.message: an orphaned chat subscription keeps
// delivering live chat traffic and consuming the subscription budget. HTTP 404 is treated
// as already-gone. Returns the first error encountered but attempts every deletion.
func (sm *SubscriptionManager) Unsubscribe(ctx context.Context, broadcasterID string) error {
	prefix := broadcasterID + ":"

	sm.mu.RLock()
	toDelete := make(map[string]string, 4) // cacheKey -> subscription_id
	for key, subID := range sm.subscriptions {
		if strings.HasPrefix(key, prefix) {
			toDelete[key] = subID
		}
	}
	sm.mu.RUnlock()

	if len(toDelete) == 0 {
		return nil // Already unsubscribed / never subscribed
	}

	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	var firstErr error
	for key, subscriptionID := range toDelete {
		deleteURL := fmt.Sprintf("%s?id=%s", sm.apiURL, subscriptionID)
		req, err := http.NewRequestWithContext(ctx, "DELETE", deleteURL, nil)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to create delete request for %s: %w", key, err)
			}
			continue
		}

		req.Header.Set("Client-Id", sm.clientID)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := sm.httpClient.Do(req)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to delete subscription %s: %w", key, err)
			}
			continue
		}

		// 204 = deleted; 404 = already gone on Twitch's side. Both let us drop the cache entry.
		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			sm.mu.Lock()
			delete(sm.subscriptions, key)
			sm.mu.Unlock()
			sm.logger.Info("Deleted EventSub subscription",
				zap.String("broadcaster_id", broadcasterID),
				zap.String("subscription_id", subscriptionID),
				zap.String("cache_key", key),
			)
		} else {
			body, _ := io.ReadAll(resp.Body)
			if firstErr == nil {
				firstErr = fmt.Errorf("delete %s failed with status %d: %s", key, resp.StatusCode, string(body))
			}
		}
		resp.Body.Close()
	}

	return firstErr
}

// UnsubscribeType deletes a single EventSub subscription type for a broadcaster (e.g.
// "channel.chat.message"), leaving the other types intact. Used to drop the chat
// subscription when an overlay's demand goes away while keeping the event subscriptions.
// HTTP 404 is treated as already-gone.
func (sm *SubscriptionManager) UnsubscribeType(ctx context.Context, broadcasterID, subType string) error {
	cacheKey := broadcasterID + ":" + subType
	sm.mu.RLock()
	subscriptionID, exists := sm.subscriptions[cacheKey]
	sm.mu.RUnlock()
	if !exists {
		return nil
	}

	token, err := sm.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s?id=%s", sm.apiURL, subscriptionID), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete %s failed with status %d: %s", cacheKey, resp.StatusCode, string(body))
	}

	sm.mu.Lock()
	delete(sm.subscriptions, cacheKey)
	sm.mu.Unlock()

	sm.logger.Info("Deleted EventSub subscription (single type)",
		zap.String("broadcaster_id", broadcasterID),
		zap.String("type", subType),
		zap.String("subscription_id", subscriptionID),
	)
	return nil
}

// Subscribe creates an EventSub subscription with webhook transport using app access token
func (sm *SubscriptionManager) Subscribe(ctx context.Context, subscriptionType string, broadcasterID string, version string) (string, error) {
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
		return "", fmt.Errorf("failed to get app access token: %w", err)
	}

	// Build condition based on subscription type
	condition := map[string]string{
		"broadcaster_user_id": broadcasterID,
	}

	// Some subscription types require moderator_user_id
	if subscriptionType == "channel.follow" {
		condition["moderator_user_id"] = broadcasterID
	}

	return sm.subscribeWithCondition(ctx, subscriptionType, broadcasterID, token, version, condition, cacheKey)
}

// subscribeWithCondition creates an EventSub subscription with custom condition fields
func (sm *SubscriptionManager) subscribeWithCondition(ctx context.Context, subscriptionType string, broadcasterID string, token string, version string, condition map[string]string, cacheKey string) (string, error) {

	// Create subscription request
	reqBody := map[string]interface{}{
		"type":      subscriptionType,
		"version":   version,
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

	req, err := http.NewRequestWithContext(ctx, "POST", sm.apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create subscription request: %w", err)
	}

	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create subscription: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// HTTP 409 = the subscription already exists on Twitch's side (e.g. a prior leadership
	// term created it, then the in-memory cache was cleared on leadership change). The POST
	// never returns the existing subscription id, so without reconciliation Unsubscribe /
	// UnsubscribeType — which look up sm.subscriptions — become no-ops and the live Twitch
	// subscription leaks. Fetch the existing id via GET and repopulate the cache so teardown
	// works. This matters especially for the always-on channel.poll.* / channel.prediction.*
	// types (#523/#524) whose 409s recur on every leadership change.
	if resp.StatusCode == http.StatusConflict {
		existingID, getErr := sm.getExistingSubscriptionID(ctx, subscriptionType, broadcasterID, token)
		if getErr != nil {
			return "", fmt.Errorf("subscription already exists (409) but failed to reconcile existing subscription for %s: %w", subscriptionType, getErr)
		}

		sm.mu.Lock()
		sm.subscriptions[cacheKey] = existingID
		sm.mu.Unlock()

		sm.logger.Info("Reconciled existing EventSub subscription on 409",
			zap.String("broadcaster_id", broadcasterID),
			zap.String("subscription_id", existingID),
			zap.String("type", subscriptionType),
		)

		return existingID, nil
	}

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

// getExistingSubscriptionID looks up the id of an EventSub subscription that already exists on
// Twitch's side by querying the subscriptions endpoint filtered by type + user_id. Used to
// reconcile the in-memory cache after a POST returns HTTP 409 ("subscription already exists").
// The caller must pass an already-obtained app access token (this method does not take sm.mu).
// Returns the id of the row whose Type matches subscriptionType, preferring a Status == "enabled"
// row when several are returned.
func (sm *SubscriptionManager) getExistingSubscriptionID(ctx context.Context, subscriptionType, broadcasterID, token string) (string, error) {
	getURL := sm.apiURL + "?type=" + url.QueryEscape(subscriptionType) + "&user_id=" + url.QueryEscape(broadcasterID)

	req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create get-existing subscription request: %w", err)
	}
	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get existing subscription: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get existing subscription failed with status %d: %s", resp.StatusCode, string(body))
	}

	var listResp struct {
		Data []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Type   string `json:"type"`
		} `json:"data"`
		Total int `json:"total"`
	}

	if err := json.Unmarshal(body, &listResp); err != nil {
		return "", fmt.Errorf("failed to decode existing subscription response: %w", err)
	}

	// Prefer an enabled row of the requested type; fall back to the first matching-type row.
	fallbackID := ""
	for _, row := range listResp.Data {
		if row.Type != subscriptionType {
			continue
		}
		if row.Status == "enabled" {
			return row.ID, nil
		}
		if fallbackID == "" {
			fallbackID = row.ID
		}
	}
	if fallbackID != "" {
		return fallbackID, nil
	}

	return "", fmt.Errorf("no existing subscription found for type %s and broadcaster %s", subscriptionType, broadcasterID)
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

	resp, err := sm.httpClient.Do(req)
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
