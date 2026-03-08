package innertube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"go.uber.org/zap"
)

// Discovery handles YouTube live stream discovery from channel pages
type Discovery struct {
	httpClient *http.Client
	logger     *zap.Logger
}

// NewDiscovery creates a new Discovery instance
func NewDiscovery(httpClient *http.Client, logger *zap.Logger) *Discovery {
	return &Discovery{
		httpClient: httpClient,
		logger:     logger,
	}
}

// DiscoverLiveStream discovers the live video ID for a given channel ID
// Returns the video ID if a live stream is found, or an error if:
// - No live stream exists
// - The stream is a premiere (not live)
// - Network or parsing errors occur
func (d *Discovery) DiscoverLiveStream(ctx context.Context, channelID string) (string, error) {
	d.logger.Info("discovering live stream",
		zap.String("channel_id", channelID),
	)

	// Construct channel live URL
	url := fmt.Sprintf("https://www.youtube.com/channel/%s/live", channelID)

	// Fetch channel page
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set comprehensive browser headers to avoid bot detection
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Error("failed to fetch channel page",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return "", fmt.Errorf("fetch channel page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("unexpected status code",
			zap.String("channel_id", channelID),
			zap.Int("status_code", resp.StatusCode),
		)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read HTML body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	bodyStr := string(body)

	// Extract canonical video ID using regex (more reliable than HTML parsing)
	// Look for: <link rel="canonical" href="https://www.youtube.com/watch?v=VIDEO_ID">
	// Use flexible regex to handle attribute ordering and whitespace
	canonicalRegex := regexp.MustCompile(`<link[^>]+rel="canonical"[^>]+href="https://www\.youtube\.com/watch\?v=([a-zA-Z0-9_-]+)"`)
	matches := canonicalRegex.FindStringSubmatch(bodyStr)

	// Try reverse attribute order if first didn't match
	if len(matches) < 2 {
		canonicalRegex = regexp.MustCompile(`<link[^>]+href="https://www\.youtube\.com/watch\?v=([a-zA-Z0-9_-]+)"[^>]+rel="canonical"`)
		matches = canonicalRegex.FindStringSubmatch(bodyStr)
	}

	if len(matches) < 2 {
		d.logger.Info("no live stream found",
			zap.String("channel_id", channelID),
			zap.Int("body_length", len(bodyStr)),
		)
		return "", fmt.Errorf("no live stream found for channel %s", channelID)
	}

	videoID := matches[1]

	d.logger.Info("discovered live stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	return videoID, nil
}

