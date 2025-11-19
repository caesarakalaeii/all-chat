package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// SubscriptionManager handles EventSub subscriptions via Twitch Helix API
type SubscriptionManager struct {
	clientID     string
	accessToken  string
	httpClient   *http.Client
	logger       *zap.Logger
}

// EventSubSubscription represents a subscription request/response
type EventSubSubscription struct {
	ID        string                  `json:"id,omitempty"`
	Status    string                  `json:"status,omitempty"`
	Type      string                  `json:"type"`
	Version   string                  `json:"version"`
	Cost      int                     `json:"cost,omitempty"`
	Condition map[string]string       `json:"condition"`
	Transport EventSubTransport       `json:"transport"`
	CreatedAt time.Time               `json:"created_at,omitempty"`
}

// EventSubTransport defines how events are delivered
type EventSubTransport struct {
	Method    string `json:"method"`
	SessionID string `json:"session_id,omitempty"`
	Callback  string `json:"callback,omitempty"`
	Secret    string `json:"secret,omitempty"`
}

// SubscriptionResponse from Twitch Helix API
type SubscriptionResponse struct {
	Data         []EventSubSubscription `json:"data"`
	Total        int                    `json:"total"`
	TotalCost    int                    `json:"total_cost"`
	MaxTotalCost int                    `json:"max_total_cost"`
}

const (
	helixAPIURL = "https://api.twitch.tv/helix/eventsub/subscriptions"
)

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(clientID, accessToken string, logger *zap.Logger) *SubscriptionManager {
	return &SubscriptionManager{
		clientID:    clientID,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// SubscribeToEvents creates all necessary EventSub subscriptions for a broadcaster
func (sm *SubscriptionManager) SubscribeToEvents(ctx context.Context, broadcasterID, sessionID string) error {
	subscriptions := []EventSubSubscription{
		// Follow events
		{
			Type:    "channel.follow",
			Version: "2",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
				"moderator_user_id":   broadcasterID, // v2 requires this
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Subscribe events
		{
			Type:    "channel.subscribe",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Subscription message (resubs with messages)
		{
			Type:    "channel.subscription.message",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Gift subscriptions
		{
			Type:    "channel.subscription.gift",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Cheer events (bits)
		{
			Type:    "channel.cheer",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Raid events (incoming raids)
		{
			Type:    "channel.raid",
			Version: "1",
			Condition: map[string]string{
				"to_broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Stream online
		{
			Type:    "stream.online",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
		// Stream offline
		{
			Type:    "stream.offline",
			Version: "1",
			Condition: map[string]string{
				"broadcaster_user_id": broadcasterID,
			},
			Transport: EventSubTransport{
				Method:    "websocket",
				SessionID: sessionID,
			},
		},
	}

	// Create each subscription
	for _, sub := range subscriptions {
		if err := sm.createSubscription(ctx, &sub); err != nil {
			sm.logger.Error("Failed to create subscription",
				zap.String("type", sub.Type),
				zap.Error(err),
			)
			// Continue with other subscriptions even if one fails
			continue
		}

		sm.logger.Info("Created EventSub subscription",
			zap.String("type", sub.Type),
			zap.String("broadcaster_id", broadcasterID),
		)
	}

	return nil
}

// createSubscription creates a single EventSub subscription
func (sm *SubscriptionManager) createSubscription(ctx context.Context, sub *EventSubSubscription) error {
	body, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", helixAPIURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sm.accessToken)
	req.Header.Set("Client-Id", sm.clientID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []EventSubSubscription `json:"data"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(result.Data) == 0 {
		return fmt.Errorf("no subscription created")
	}

	sm.logger.Debug("Subscription created",
		zap.String("id", result.Data[0].ID),
		zap.String("type", result.Data[0].Type),
		zap.String("status", result.Data[0].Status),
	)

	return nil
}

// ListSubscriptions retrieves all active EventSub subscriptions
func (sm *SubscriptionManager) ListSubscriptions(ctx context.Context) (*SubscriptionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", helixAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sm.accessToken)
	req.Header.Set("Client-Id", sm.clientID)

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteSubscription removes an EventSub subscription
func (sm *SubscriptionManager) DeleteSubscription(ctx context.Context, subscriptionID string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", helixAPIURL+"?id="+subscriptionID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sm.accessToken)
	req.Header.Set("Client-Id", sm.clientID)

	resp, err := sm.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	sm.logger.Info("Deleted EventSub subscription", zap.String("id", subscriptionID))

	return nil
}

// DeleteAllSubscriptions removes all EventSub subscriptions
func (sm *SubscriptionManager) DeleteAllSubscriptions(ctx context.Context) error {
	subs, err := sm.ListSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	for _, sub := range subs.Data {
		if err := sm.DeleteSubscription(ctx, sub.ID); err != nil {
			sm.logger.Error("Failed to delete subscription",
				zap.String("id", sub.ID),
				zap.Error(err),
			)
		}
	}

	sm.logger.Info("Deleted all EventSub subscriptions", zap.Int("count", len(subs.Data)))

	return nil
}
