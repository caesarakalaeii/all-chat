package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"go.uber.org/zap"
)

const (
	seventvAPIURL     = "https://7tv.io"
	seventvAPITimeout = 5 * time.Second
)

// SevenTVClient implements EmoteClient for 7TV API
type SevenTVClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// SevenTVResponse represents the 7TV API response
type SevenTVResponse struct {
	User struct {
		Username    string `json:"username"`
		Connections []struct {
			Platform string `json:"platform"`
			ID       string `json:"id"`
		} `json:"connections"`
	} `json:"user"`
	EmoteSet struct {
		Emotes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Data struct {
				Host struct {
					URL   string `json:"url"`
					Files []struct {
						Name string `json:"name"`
					} `json:"files"`
				} `json:"host"`
			} `json:"data"`
		} `json:"emotes"`
	} `json:"emote_set"`
}

// NewSevenTVClient creates a new 7TV API client
func NewSevenTVClient(logger *zap.Logger) *SevenTVClient {
	return &SevenTVClient{
		baseURL: seventvAPIURL,
		httpClient: &http.Client{
			Timeout: seventvAPITimeout,
		},
		logger: logger,
	}
}

// FetchEmotes fetches emotes from 7TV for a given channel
func (c *SevenTVClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	// 7TV API uses Twitch user ID, but we can also query by username
	url := fmt.Sprintf("%s/v3/users/twitch/%s", c.baseURL, channel)

	c.logger.Debug("Fetching 7TV emotes",
		zap.String("channel", channel),
		zap.String("url", url))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch emotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp SevenTVResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert 7TV emotes to our internal format
	emotes := make([]models.Emote, 0, len(apiResp.EmoteSet.Emotes))
	for _, e := range apiResp.EmoteSet.Emotes {
		if len(e.Data.Host.Files) == 0 {
			continue
		}

		// Build emote URL
		url := e.Data.Host.URL
		if !strings.HasPrefix(url, "http") {
			url = "https:" + url
		}
		url = url + "/" + e.Data.Host.Files[0].Name

		emote := models.Emote{
			Code:     e.Name,
			URL:      url,
			Provider: "7tv",
			Channel:  channel,
		}

		emotes = append(emotes, emote)
	}

	c.logger.Debug("Fetched 7TV emotes",
		zap.String("channel", channel),
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Provider returns the provider name
func (c *SevenTVClient) Provider() string {
	return "7tv"
}
