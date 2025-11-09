package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/internal/emote-service/core/domain"
)

const (
	SevenTVGlobalURL  = "https://7tv.io/v3/emote-sets/global"
	SevenTVChannelURL = "https://7tv.io/v3/users/twitch/%s"
	BTTVGlobalURL     = "https://api.betterttv.net/3/cached/emotes/global"
	BTTVChannelURL    = "https://api.betterttv.net/3/cached/users/twitch/%s"
	FFZGlobalURL      = "https://api.frankerfacez.com/v1/set/global"
	FFZChannelURL     = "https://api.frankerfacez.com/v1/room/%s"
)

type EmoteClient struct {
	httpClient *http.Client
}

func NewEmoteClient() *EmoteClient {
	return &EmoteClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// 7TV Emotes
func (c *EmoteClient) FetchSevenTVGlobal(ctx context.Context) ([]domain.Emote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", SevenTVGlobalURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("7TV API error: status %d", resp.StatusCode)
	}

	var response domain.SevenTVGlobalResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	emotes := make([]domain.Emote, 0, len(response.Emotes))
	for _, e := range response.Emotes {
		emotes = append(emotes, domain.Emote{
			Code:     e.Name,
			Provider: "7tv",
			URL:      fmt.Sprintf("https:%s/2x.webp", e.Data.Host.URL),
			Animated: e.Data.Animated,
			Channel:  "",
		})
	}

	return emotes, nil
}

func (c *EmoteClient) FetchSevenTVChannel(ctx context.Context, channel string) ([]domain.Emote, error) {
	url := fmt.Sprintf(SevenTVChannelURL, channel)
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
		return nil, fmt.Errorf("7TV API error: status %d", resp.StatusCode)
	}

	var response domain.SevenTVChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	emotes := make([]domain.Emote, 0, len(response.EmoteSet.Emotes))
	for _, e := range response.EmoteSet.Emotes {
		emotes = append(emotes, domain.Emote{
			Code:     e.Name,
			Provider: "7tv",
			URL:      fmt.Sprintf("https:%s/2x.webp", e.Data.Host.URL),
			Animated: e.Data.Animated,
			Channel:  channel,
		})
	}

	return emotes, nil
}

// BTTV Emotes
func (c *EmoteClient) FetchBTTVGlobal(ctx context.Context) ([]domain.Emote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", BTTVGlobalURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BTTV API error: status %d", resp.StatusCode)
	}

	var response domain.BTTVGlobalResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	emotes := make([]domain.Emote, 0, len(response))
	for _, e := range response {
		emotes = append(emotes, domain.Emote{
			Code:     e.Code,
			Provider: "bttv",
			URL:      fmt.Sprintf("https://cdn.betterttv.net/emote/%s/2x.%s", e.ID, e.ImageType),
			Animated: e.ImageType == "gif",
			Channel:  "",
		})
	}

	return emotes, nil
}

func (c *EmoteClient) FetchBTTVChannel(ctx context.Context, channel string) ([]domain.Emote, error) {
	url := fmt.Sprintf(BTTVChannelURL, channel)
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
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("BTTV API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var response domain.BTTVChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	allEmotes := append(response.ChannelEmotes, response.SharedEmotes...)
	emotes := make([]domain.Emote, 0, len(allEmotes))
	for _, e := range allEmotes {
		emotes = append(emotes, domain.Emote{
			Code:     e.Code,
			Provider: "bttv",
			URL:      fmt.Sprintf("https://cdn.betterttv.net/emote/%s/2x.%s", e.ID, e.ImageType),
			Animated: e.ImageType == "gif",
			Channel:  channel,
		})
	}

	return emotes, nil
}

// FFZ Emotes
func (c *EmoteClient) FetchFFZGlobal(ctx context.Context) ([]domain.Emote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", FFZGlobalURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FFZ API error: status %d", resp.StatusCode)
	}

	var response domain.FFZGlobalResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	emotes := []domain.Emote{}
	for _, set := range response.Sets {
		for _, e := range set.Emoticons {
			url := e.URLs.One
			if e.URLs.Two != "" {
				url = e.URLs.Two
			}
			emotes = append(emotes, domain.Emote{
				Code:     e.Name,
				Provider: "ffz",
				URL:      "https:" + url,
				Animated: false,
				Channel:  "",
			})
		}
	}

	return emotes, nil
}

func (c *EmoteClient) FetchFFZChannel(ctx context.Context, channel string) ([]domain.Emote, error) {
	url := fmt.Sprintf(FFZChannelURL, channel)
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
		return nil, fmt.Errorf("FFZ API error: status %d", resp.StatusCode)
	}

	var response domain.FFZChannelResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	emotes := []domain.Emote{}
	for _, set := range response.Sets {
		for _, e := range set.Emoticons {
			url := e.URLs.One
			if e.URLs.Two != "" {
				url = e.URLs.Two
			}
			emotes = append(emotes, domain.Emote{
				Code:     e.Name,
				Provider: "ffz",
				URL:      "https:" + url,
				Animated: false,
				Channel:  channel,
			})
		}
	}

	return emotes, nil
}
