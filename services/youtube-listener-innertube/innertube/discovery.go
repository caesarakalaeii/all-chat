package innertube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/net/html"
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

	// Parse HTML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}

	// Extract canonical video ID
	videoID := extractCanonicalVideoID(doc)
	if videoID == "" {
		d.logger.Info("no live stream found",
			zap.String("channel_id", channelID),
		)
		return "", fmt.Errorf("no live stream found for channel %s", channelID)
	}

	// Check if it's actually live (not a premiere)
	isLive := checkIsLiveMeta(doc)
	if !isLive {
		d.logger.Info("stream is a premiere, not live",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
		)
		return "", fmt.Errorf("stream %s is a premiere, not live", videoID)
	}

	d.logger.Info("discovered live stream",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	return videoID, nil
}

// extractCanonicalVideoID extracts the video ID from the canonical link tag
// Looks for: <link rel="canonical" href="https://www.youtube.com/watch?v=VIDEO_ID">
func extractCanonicalVideoID(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "link" {
		var rel, href string
		for _, attr := range n.Attr {
			switch attr.Key {
			case "rel":
				rel = attr.Val
			case "href":
				href = attr.Val
			}
		}

		// Check if this is the canonical link
		if rel == "canonical" && strings.Contains(href, "youtube.com/watch?v=") {
			// Extract video ID from URL
			parts := strings.Split(href, "?v=")
			if len(parts) == 2 {
				// Handle additional query parameters
				videoID := strings.Split(parts[1], "&")[0]
				return videoID
			}
		}
	}

	// Recursively search child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if videoID := extractCanonicalVideoID(c); videoID != "" {
			return videoID
		}
	}

	return ""
}

// checkIsLiveMeta checks if the og:video:type meta tag indicates a live stream
// Looks for: <meta property="og:video:type" content="live">
// Returns false for premieres (content="premiere") or missing tag
func checkIsLiveMeta(n *html.Node) bool {
	if n.Type == html.ElementNode && n.Data == "meta" {
		var property, content string
		for _, attr := range n.Attr {
			switch attr.Key {
			case "property":
				property = attr.Val
			case "content":
				content = attr.Val
			}
		}

		// Check if this is the og:video:type meta tag
		if property == "og:video:type" {
			return content == "live"
		}
	}

	// Recursively search child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if checkIsLiveMeta(c) {
			return true
		}
	}

	return false
}
