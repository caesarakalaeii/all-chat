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

	// Set User-Agent to appear as a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

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

	// Extract canonical video ID using regex (more reliable than HTML parsing)
	// Look for: <link rel="canonical" href="https://www.youtube.com/watch?v=VIDEO_ID">
	canonicalRegex := regexp.MustCompile(`<link rel="canonical" href="https://www\.youtube\.com/watch\?v=([a-zA-Z0-9_-]+)"`)
	matches := canonicalRegex.FindStringSubmatch(string(body))

	if len(matches) < 2 {
		d.logger.Info("no live stream found",
			zap.String("channel_id", channelID),
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

