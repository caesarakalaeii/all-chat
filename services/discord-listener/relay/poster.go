package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ErrRateLimited is returned when Discord responds with 429 and a Retry-After
// that exceeds maxRetryAfter. Callers should back off and drain queued messages
// instead of blocking the goroutine.
var ErrRateLimited = errors.New("discord webhook rate limited")

// maxRetryAfter is the longest the poster will sleep on a 429 before returning
// ErrRateLimited. Anything longer blocks the drain goroutine and causes the
// Redis Pub/Sub channel to overflow.
const maxRetryAfter = 5 * time.Second

// RelayPayload holds the fields sent to a Discord webhook.
type RelayPayload struct {
	Content   string
	Username  string // pre-formatted: "alice [Twitch]"
	AvatarURL string // may be empty
}

// DiscordPoster posts a message to a Discord webhook.
type DiscordPoster interface {
	Post(ctx context.Context, webhookURL string, msg RelayPayload) error
}

// webhookPoster sends messages to Discord webhook URLs.
type webhookPoster struct {
	client  *http.Client
	baseURL string // overridable for tests; empty in production (uses webhookURL directly)
	logger  *zap.Logger
}

// NewWebhookPoster creates a production DiscordPoster that sends via webhooks.
func NewWebhookPoster(client *http.Client, logger *zap.Logger) DiscordPoster {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &webhookPoster{
		client: client,
		logger: logger,
	}
}

// webhookBody is the JSON body sent to a Discord webhook endpoint.
type webhookBody struct {
	Content   string `json:"content"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// formatWebhookUsername returns a display string like "alice [Twitch]".
// The platform name is title-cased (first letter upper, rest lower).
func formatWebhookUsername(displayName, platform string) string {
	titled := strings.ToUpper(platform[:1]) + strings.ToLower(platform[1:])
	return fmt.Sprintf("%s [%s]", displayName, titled)
}

// Post sends a message to the specified Discord webhook URL.
// It handles 429 Retry-After with a single retry, and drops 403/404 silently.
func (p *webhookPoster) Post(ctx context.Context, webhookURL string, msg RelayPayload) error {
	return p.doPost(ctx, webhookURL, msg, false)
}

func (p *webhookPoster) doPost(ctx context.Context, webhookURL string, msg RelayPayload, isRetry bool) error {
	body := webhookBody{
		Content:   msg.Content,
		Username:  msg.Username,
		AvatarURL: msg.AvatarURL,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header — webhook token is embedded in the URL.

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("discord webhook POST failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent:
		return nil

	case resp.StatusCode == http.StatusTooManyRequests:
		retryAfterStr := resp.Header.Get("Retry-After")
		retryAfterSec, parseErr := strconv.ParseFloat(retryAfterStr, 64)
		if parseErr != nil || retryAfterSec < 0 {
			retryAfterSec = 1.0
		}
		retryAfterDur := time.Duration(retryAfterSec * float64(time.Second))

		// If the wait is too long, return immediately so the caller can drain
		// queued messages instead of blocking the goroutine for minutes.
		if retryAfterDur > maxRetryAfter || isRetry {
			if p.logger != nil {
				p.logger.Warn("Discord webhook 429: returning rate-limit error",
					zap.String("webhook_url", webhookURL),
					zap.Duration("retry_after", retryAfterDur),
					zap.Bool("is_retry", isRetry),
				)
			}
			return fmt.Errorf("%w (retry_after=%s)", ErrRateLimited, retryAfterDur)
		}

		// Short wait — sleep and retry once.
		if p.logger != nil {
			p.logger.Warn("Discord webhook 429: sleeping before retry",
				zap.String("webhook_url", webhookURL),
				zap.Duration("retry_after", retryAfterDur),
			)
		}
		timer := time.NewTimer(retryAfterDur)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		return p.doPost(ctx, webhookURL, msg, true)

	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound:
		if p.logger != nil {
			p.logger.Warn("Discord webhook POST dropped: permission denied or webhook not found",
				zap.Int("status", resp.StatusCode),
				zap.String("webhook_url", webhookURL),
			)
		}
		return nil

	default:
		return fmt.Errorf("discord webhook POST returned unexpected status %d for %s", resp.StatusCode, webhookURL)
	}
}
