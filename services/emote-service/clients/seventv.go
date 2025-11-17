package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
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
	baseURL      string
	httpClient   *http.Client
	logger       *zap.Logger
	twitchClient TwitchUserLookup
}

type sevenTVUserResponse struct {
	EmoteSet sevenTVEmoteSet `json:"emote_set"`
}

type sevenTVEmoteSet struct {
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Emotes []sevenTVActiveEmote `json:"emotes"`
}

type sevenTVActiveEmote struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Data *sevenTVEmoteData `json:"data"`
}

type sevenTVEmoteData struct {
	Name  string      `json:"name"`
	Host  sevenTVHost `json:"host"`
	Flags int         `json:"flags"`
}

type sevenTVHost struct {
	URL   string            `json:"url"`
	Files []sevenTVHostFile `json:"files"`
}

type sevenTVHostFile struct {
	Name       string `json:"name"`
	StaticName string `json:"static_name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// NewSevenTVClient creates a new 7TV API client
func NewSevenTVClient(logger *zap.Logger, twitchClient TwitchUserLookup) *SevenTVClient {
	return &SevenTVClient{
		baseURL: seventvAPIURL,
		httpClient: &http.Client{
			Timeout: seventvAPITimeout,
		},
		logger:       logger,
		twitchClient: twitchClient,
	}
}

// FetchEmotes fetches emotes from 7TV for a given channel
func (c *SevenTVClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	var (
		urlPath    string
		outputChan = channel
	)

	if strings.EqualFold(channel, "global") {
		urlPath = fmt.Sprintf("%s/v3/emote-sets/global", c.baseURL)
	} else {
		resolved := channel
		if !isNumeric(channel) {
			if c.twitchClient == nil {
				return nil, fmt.Errorf("twitch client is not configured")
			}
			twitchID, err := c.twitchClient.GetUserID(ctx, channel)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve twitch user: %w", err)
			}
			resolved = twitchID
		}
		urlPath = fmt.Sprintf("%s/v3/users/twitch/%s", c.baseURL, resolved)
	}

	c.logger.Debug("Fetching 7TV emotes",
		zap.String("channel", channel),
		zap.String("url", urlPath))

	req, err := http.NewRequestWithContext(ctx, "GET", urlPath, nil)
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

	var emoteSet sevenTVEmoteSet
	if strings.EqualFold(channel, "global") {
		if err := json.NewDecoder(resp.Body).Decode(&emoteSet); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	} else {
		var apiResp sevenTVUserResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		emoteSet = apiResp.EmoteSet
	}

	emotes := make([]models.Emote, 0, len(emoteSet.Emotes))
	for _, e := range emoteSet.Emotes {
		if e.Data == nil {
			continue
		}
		url := buildSevenTVURL(e.Data.Host)
		if url == "" {
			continue
		}
		emotes = append(emotes, models.Emote{
			Code:     e.Name,
			URL:      url,
			Provider: "7tv",
			Channel:  outputChan,
		})
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

func buildSevenTVURL(host sevenTVHost) string {
	if host.URL == "" || len(host.Files) == 0 {
		return ""
	}

	base := host.URL
	if !strings.HasPrefix(base, "http") {
		base = "https:" + base
	}

	best := host.Files[0]
	for _, file := range host.Files[1:] {
		if file.Width > best.Width {
			best = file
		}
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(best.Name, "/"))
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}
