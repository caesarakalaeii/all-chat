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

// platformEmoji maps platform names to their display emoji.
var platformEmoji = map[string]string{
	"twitch":  "🟣",
	"youtube": "🔴",
	"kick":    "💚",
	"tiktok":  "🎵",
}

// formatRelayContent returns the formatted relay message string.
func formatRelayContent(platform, username, text string) string {
	emoji, ok := platformEmoji[platform]
	if !ok {
		emoji = "💬"
	}
	return fmt.Sprintf("%s %s: %s", emoji, username, text)
}

// DiscordPoster posts a message to a Discord channel.
type DiscordPoster interface {
	Post(ctx context.Context, channelID, content string) error
}

// httpPoster sends messages to the Discord REST API.
type httpPoster struct {
	token   string
	client  *http.Client
	baseURL string // overridable for tests; production uses discordAPIBase
	logger  *zap.Logger
}

const discordAPIBase = "https://discord.com/api/v10"

// NewHTTPPoster creates a production DiscordPoster.
func NewHTTPPoster(token string, client *http.Client, logger *zap.Logger) DiscordPoster {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &httpPoster{
		token:   token,
		client:  client,
		baseURL: discordAPIBase,
		logger:  logger,
	}
}

type discordMessageBody struct {
	Content string `json:"content"`
}

// Post sends content to the specified Discord channel.
// It handles 429 Retry-After with a single retry, and drops 403/404 silently.
func (p *httpPoster) Post(ctx context.Context, channelID, content string) error {
	return p.doPost(ctx, channelID, content, false)
}

func (p *httpPoster) doPost(ctx context.Context, channelID, content string, isRetry bool) error {
	body := discordMessageBody{Content: content}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal discord message body: %w", err)
	}

	url := fmt.Sprintf("%s/channels/%s/messages", p.baseURL, channelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord POST failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return nil

	case resp.StatusCode == http.StatusTooManyRequests:
		if isRetry {
			// Already retried once — give up.
			return fmt.Errorf("discord rate limit persists after retry (channel %s)", channelID)
		}
		retryAfterStr := resp.Header.Get("Retry-After")
		retryAfterSec, parseErr := strconv.ParseFloat(retryAfterStr, 64)
		if parseErr != nil || retryAfterSec < 0 {
			retryAfterSec = 1.0
		}
		if p.logger != nil {
			p.logger.Warn("Discord 429: sleeping before retry",
				zap.String("channel_id", channelID),
				zap.Float64("retry_after_seconds", retryAfterSec),
			)
		}
		timer := time.NewTimer(time.Duration(retryAfterSec * float64(time.Second)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		return p.doPost(ctx, channelID, content, true)

	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound:
		if p.logger != nil {
			p.logger.Warn("Discord POST dropped: permission denied or channel not found",
				zap.Int("status", resp.StatusCode),
				zap.String("channel_id", channelID),
			)
		}
		return nil

	default:
		return fmt.Errorf("discord POST returned unexpected status %d for channel %s", resp.StatusCode, channelID)
	}
}
