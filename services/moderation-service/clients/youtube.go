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

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// YouTube moderation is ban-only against the Data API's liveChatBans endpoint, as the
// broadcaster (force-ssl scope). A ban needs the live broadcast's liveChatId, which is
// not on the message — it is read from the youtube-listener's Redis stream-state cache
// (no expensive search.list, so the only quota cost is the 50-unit ban itself; if the
// stream isn't live/cached the ban cannot proceed).
var (
	// ErrYouTubeUnauthorized indicates the access token is invalid/expired (HTTP 401).
	ErrYouTubeUnauthorized = errors.New("youtube: access token unauthorized")
	// ErrYouTubeForbidden indicates the token lacks force-ssl or the user is not a
	// moderator/owner of the channel (HTTP 403) — surfaced as a re-consent prompt.
	ErrYouTubeForbidden = errors.New("youtube: forbidden (missing scope or not a moderator)")
	// ErrYouTubeNotLive indicates no cached live chat for the channel (not live, or the
	// youtube-listener has not cached the stream state) — the ban cannot proceed.
	ErrYouTubeNotLive = errors.New("youtube: no active live chat for channel (stream not live or not cached)")
)

const defaultYouTubeBaseURL = "https://www.googleapis.com/youtube/v3"

// YouTubeClient calls the YouTube Data API liveChatBans endpoint as the broadcaster.
type YouTubeClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewYouTubeClient builds a YouTube moderation client.
func NewYouTubeClient() *YouTubeClient {
	return &YouTubeClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    defaultYouTubeBaseURL,
	}
}

// BanUser permanently bans a user from the live chat.
// POST /liveChat/bans?part=snippet — scope youtube.force-ssl.
func (y *YouTubeClient) BanUser(ctx context.Context, token, liveChatID, bannedChannelID string) error {
	body, err := json.Marshal(map[string]any{
		"snippet": map[string]any{
			"liveChatId":        liveChatID,
			"type":              "permanent",
			"bannedUserDetails": map[string]any{"channelId": bannedChannelID},
		},
	})
	if err != nil {
		return fmt.Errorf("youtube: marshal ban body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, y.baseURL+"/liveChat/bans?part=snippet", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("youtube: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := y.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("youtube: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrYouTubeUnauthorized
	case http.StatusForbidden:
		return ErrYouTubeForbidden
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("youtube: liveChatBans returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
}

// YouTubeLiveChatResolver resolves a channel's active liveChatId from the
// youtube-listener's Redis stream-state cache (`youtube:stream:state:{channelID}`),
// the same key chat_send.go reads. It never falls back to the quota-costly search.list.
type YouTubeLiveChatResolver struct {
	redis *redis.Client
}

// NewYouTubeLiveChatResolver wires a resolver over the shared Redis.
func NewYouTubeLiveChatResolver(client *redis.Client) *YouTubeLiveChatResolver {
	return &YouTubeLiveChatResolver{redis: client}
}

// Resolve returns the channel's active liveChatId, or ErrYouTubeNotLive when the
// channel is not currently live / not cached.
func (r *YouTubeLiveChatResolver) Resolve(ctx context.Context, channelID string) (string, error) {
	data, err := r.redis.Get(ctx, "youtube:stream:state:"+channelID).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrYouTubeNotLive
	}
	if err != nil {
		return "", fmt.Errorf("youtube: read stream state: %w", err)
	}
	var state struct {
		LiveChatID string `json:"live_chat_id"`
		IsLive     bool   `json:"is_live"`
	}
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return "", fmt.Errorf("youtube: decode stream state: %w", err)
	}
	if !state.IsLive || state.LiveChatID == "" {
		return "", ErrYouTubeNotLive
	}
	return state.LiveChatID, nil
}
