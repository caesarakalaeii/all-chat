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

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

// MockBadge mirrors the message processor badge schema.
type MockBadge struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	IconURL string `json:"icon_url"`
}

// MockEventInfo contains event metadata for subscriptions, donations, raids, etc.
type MockEventInfo struct {
	Type          string                 `json:"type"`
	Tier          string                 `json:"tier"`
	Duration      int                    `json:"duration,omitempty"`
	IsUpdate      bool                   `json:"is_update,omitempty"`
	AggregationID string                 `json:"aggregation_id,omitempty"`
	Value         *MockEventValue        `json:"value,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// MockEventValue represents monetary or numeric value associated with an event
type MockEventValue struct {
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	DisplayText string  `json:"display_text"`
}

// MockMessagePayload is forwarded to the message processor's mock endpoint.
type MockMessagePayload struct {
	OverlayID   string                 `json:"overlay_id"`
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	AvatarURL   string                 `json:"avatar_url,omitempty"`
	Color       string                 `json:"color,omitempty"`
	Badges      []MockBadge            `json:"badges,omitempty"`
	Event       *MockEventInfo         `json:"event,omitempty"`
	Text        string                 `json:"text"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type MessageProcessorClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
}

func NewMessageProcessorClient(baseURL, apiKey string, tracingEnabled bool, logger *zap.Logger) *MessageProcessorClient {
	// Create custom transport with connection pooling for internal service calls
	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Conditionally wrap with OpenTelemetry instrumentation
	var finalTransport http.RoundTripper = baseTransport
	if tracingEnabled {
		finalTransport = otelhttp.NewTransport(baseTransport)
	}

	return &MessageProcessorClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Transport: finalTransport,
			Timeout:   5 * time.Second,
		},
		logger: logger.With(zap.String("component", "message-processor-client")),
	}
}

func (c *MessageProcessorClient) SendMockMessage(ctx context.Context, payload *MockMessagePayload) error {
	if c.baseURL == "" {
		return fmt.Errorf("message processor url not configured")
	}
	if c.apiKey == "" {
		return fmt.Errorf("message processor api key not configured")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/mock-messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call message processor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		c.logger.Warn("Message processor rejected mock message",
			zap.Int("status", resp.StatusCode),
			zap.ByteString("body", respBody),
		)
		return fmt.Errorf("message processor returned status %d", resp.StatusCode)
	}

	return nil
}
