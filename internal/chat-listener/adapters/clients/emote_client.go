package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/internal/chat-listener/core/domain"
)

// HTTPEmoteClient implements the EmoteClient interface using HTTP calls to the emote service
type HTTPEmoteClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPEmoteClient creates a new HTTP emote client
func NewHTTPEmoteClient(baseURL string) *HTTPEmoteClient {
	return &HTTPEmoteClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// EmoteResponse represents the response from the emote service
type EmoteResponse struct {
	Emotes []struct {
		Code     string `json:"code"`
		URL      string `json:"url"`
		Provider string `json:"provider"`
		Animated bool   `json:"animated"`
	} `json:"emotes"`
}

// GetChannelEmotes retrieves all emotes for a channel from all enabled providers
func (c *HTTPEmoteClient) GetChannelEmotes(ctx context.Context, channel string, enable7TV, enableBTTV, enableFFZ bool) ([]domain.Emote, error) {
	emotes := make([]domain.Emote, 0)

	// Fetch from each enabled provider
	if enable7TV {
		e, err := c.fetchEmotes(ctx, channel, "7tv")
		if err == nil {
			emotes = append(emotes, e...)
		}
	}

	if enableBTTV {
		e, err := c.fetchEmotes(ctx, channel, "bttv")
		if err == nil {
			emotes = append(emotes, e...)
		}
	}

	if enableFFZ {
		e, err := c.fetchEmotes(ctx, channel, "ffz")
		if err == nil {
			emotes = append(emotes, e...)
		}
	}

	return emotes, nil
}

// fetchEmotes fetches emotes from a specific provider
func (c *HTTPEmoteClient) fetchEmotes(ctx context.Context, channel, provider string) ([]domain.Emote, error) {
	url := fmt.Sprintf("%s/api/v1/emotes/%s/%s", c.baseURL, provider, channel)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("emote service returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var emoteResp EmoteResponse
	if err := json.Unmarshal(body, &emoteResp); err != nil {
		return nil, err
	}

	emotes := make([]domain.Emote, len(emoteResp.Emotes))
	for i, e := range emoteResp.Emotes {
		emotes[i] = domain.Emote{
			Code:     e.Code,
			URL:      e.URL,
			Provider: e.Provider,
			Animated: e.Animated,
		}
	}

	return emotes, nil
}
