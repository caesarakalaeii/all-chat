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

package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	webhookName    = "AllChat Relay"
	discordBaseURL = "https://discord.com/api/v10"
)

// discordWebhook is the response structure from the Discord webhook API.
type discordWebhook struct {
	ID            string  `json:"id"`
	Token         string  `json:"token"`
	Name          string  `json:"name"`
	ApplicationID *string `json:"application_id"`
}

// WebhookProvisioner auto-creates Discord webhooks for relay-enabled sources
// that have a channel ID but no webhook URL.
type WebhookProvisioner struct {
	botToken   string
	httpClient *http.Client
	repo       RepositoryInterface
	logger     *zap.Logger
}

// NewWebhookProvisioner creates a WebhookProvisioner.
func NewWebhookProvisioner(botToken string, httpClient *http.Client, repo RepositoryInterface, logger *zap.Logger) *WebhookProvisioner {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookProvisioner{
		botToken:   botToken,
		httpClient: httpClient,
		repo:       repo,
		logger:     logger,
	}
}

// ProvisionPending finds all sources that need a webhook and provisions them.
// Errors are logged per-source but do not stop processing of other sources.
func (p *WebhookProvisioner) ProvisionPending(ctx context.Context) error {
	pending, err := p.repo.GetPendingRelayConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending relay configs: %w", err)
	}

	if len(pending) == 0 {
		return nil
	}

	if p.logger != nil {
		p.logger.Info("Provisioning webhooks for pending relay sources",
			zap.Int("count", len(pending)),
		)
	}

	for _, cfg := range pending {
		if err := p.ensureWebhook(ctx, cfg); err != nil {
			if p.logger != nil {
				p.logger.Error("Failed to provision webhook for relay source",
					zap.String("source_id", cfg.SourceID),
					zap.String("overlay_id", cfg.OverlayID),
					zap.String("channel_id", cfg.ChannelID),
					zap.Error(err),
				)
			}
			// Continue to next source -- do not block others.
		}
	}

	return nil
}

// ensureWebhook checks for an existing "AllChat Relay" webhook in the channel
// and creates one if not found, then stores the URL in the database.
func (p *WebhookProvisioner) ensureWebhook(ctx context.Context, cfg pendingRelayConfig) error {
	// 1. List existing webhooks in the channel.
	webhooks, err := p.listChannelWebhooks(ctx, cfg.ChannelID)
	if err != nil {
		return err
	}

	// 2. Check if an AllChat Relay webhook already exists.
	var webhookURL string
	for _, wh := range webhooks {
		if wh.Name == webhookName && wh.Token != "" {
			webhookURL = fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", wh.ID, wh.Token)
			if p.logger != nil {
				p.logger.Info("Reusing existing AllChat Relay webhook",
					zap.String("overlay_id", cfg.OverlayID),
					zap.String("channel_id", cfg.ChannelID),
					zap.String("webhook_id", wh.ID),
				)
			}
			break
		}
	}

	// 3. Create a new webhook if none found.
	if webhookURL == "" {
		wh, err := p.createWebhook(ctx, cfg.ChannelID)
		if err != nil {
			return err
		}
		webhookURL = fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", wh.ID, wh.Token)
		if p.logger != nil {
			p.logger.Info("Created new AllChat Relay webhook",
				zap.String("overlay_id", cfg.OverlayID),
				zap.String("channel_id", cfg.ChannelID),
				zap.String("webhook_id", wh.ID),
			)
		}
	}

	// 4. Store the webhook URL in the database.
	if err := p.repo.StoreWebhookURL(ctx, cfg.SourceID, webhookURL); err != nil {
		return fmt.Errorf("failed to store webhook URL: %w", err)
	}

	return nil
}

// listChannelWebhooks fetches all webhooks for a channel via Discord REST API.
func (p *WebhookProvisioner) listChannelWebhooks(ctx context.Context, channelID string) ([]discordWebhook, error) {
	url := fmt.Sprintf("%s/channels/%s/webhooks", discordBaseURL, channelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list webhooks request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+p.botToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list webhooks HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		var webhooks []discordWebhook
		if err := json.Unmarshal(body, &webhooks); err != nil {
			return nil, fmt.Errorf("failed to unmarshal webhooks response: %w", err)
		}
		return webhooks, nil

	case http.StatusForbidden:
		return nil, fmt.Errorf("bot lacks MANAGE_WEBHOOKS permission in channel %s", channelID)

	case http.StatusNotFound:
		return nil, fmt.Errorf("channel %s not found", channelID)

	case http.StatusTooManyRequests:
		retryAfter := p.parseRetryAfter(resp)
		if retryAfter > 0 {
			timer := time.NewTimer(retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			return p.listChannelWebhooks(ctx, channelID)
		}
		return nil, fmt.Errorf("rate limited listing webhooks for channel %s", channelID)

	default:
		return nil, fmt.Errorf("list webhooks returned status %d for channel %s: %s", resp.StatusCode, channelID, string(body))
	}
}

// createWebhook creates a new webhook in the specified channel.
func (p *WebhookProvisioner) createWebhook(ctx context.Context, channelID string) (*discordWebhook, error) {
	url := fmt.Sprintf("%s/channels/%s/webhooks", discordBaseURL, channelID)
	reqBody, _ := json.Marshal(map[string]string{"name": webhookName})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook creation request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+p.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("create webhook HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		var wh discordWebhook
		if err := json.Unmarshal(body, &wh); err != nil {
			return nil, fmt.Errorf("failed to unmarshal created webhook: %w", err)
		}
		return &wh, nil

	case http.StatusForbidden:
		return nil, fmt.Errorf("bot lacks MANAGE_WEBHOOKS permission in channel %s", channelID)

	case http.StatusNotFound:
		return nil, fmt.Errorf("channel %s not found", channelID)

	case http.StatusTooManyRequests:
		retryAfter := p.parseRetryAfter(resp)
		if retryAfter > 0 {
			timer := time.NewTimer(retryAfter)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
			return p.createWebhook(ctx, channelID)
		}
		return nil, fmt.Errorf("rate limited creating webhook for channel %s", channelID)

	default:
		return nil, fmt.Errorf("create webhook returned status %d for channel %s: %s", resp.StatusCode, channelID, string(body))
	}
}

// parseRetryAfter extracts the Retry-After duration from a 429 response.
func (p *WebhookProvisioner) parseRetryAfter(resp *http.Response) time.Duration {
	retryAfterStr := resp.Header.Get("Retry-After")
	retryAfterSec, err := strconv.ParseFloat(retryAfterStr, 64)
	if err != nil || retryAfterSec <= 0 {
		return time.Second
	}
	return time.Duration(retryAfterSec * float64(time.Second))
}
