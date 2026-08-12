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

// YouTube moderation goes through the Data API's liveChatBans endpoint with the acting human's own
// force-ssl token: permanent ban, and timeout as a temporary ban (YouTube models a timeout that
// way, so it is the same call). Both need the live broadcast's liveChatId, which is not on the
// message — it is read from the youtube-listener's Redis stream-state cache (no expensive
// search.list, so the only quota cost is the ban itself; if the stream isn't live/cached the action
// cannot proceed).
//
// Two actions are deliberately absent, both for lack of an id rather than lack of an endpoint:
//
//   - **Single-message delete.** `liveChatMessages.delete` keys on a Data API message id
//     ("LCC."-prefixed). Production ingests chat through the InnerTube listener, whose renderer ids
//     are a different encoding, so the id All-Chat holds for a YouTube message is not the id this
//     endpoint accepts. Shipping it would light a delete button that 404s. ADR-0048 lists the
//     equivalence as an unmeasured unknown; it stays unbuilt until someone measures it.
//   - **Unban.** `liveChatBans.delete` keys on the ban resource id returned by insert, which
//     nothing persists, and there is no list endpoint to recover it from.
var (
	// ErrYouTubeUnauthorized indicates the access token is invalid/expired (HTTP 401).
	ErrYouTubeUnauthorized = errors.New("youtube: access token unauthorized")
	// ErrYouTubeForbidden indicates the token lacks force-ssl or the user is not a
	// moderator/owner of the channel (HTTP 403) — surfaced as a re-consent prompt.
	ErrYouTubeForbidden = errors.New("youtube: forbidden (missing scope or not a moderator)")
	// ErrYouTubeNotLive indicates no cached live chat for the channel (not live, or the
	// youtube-listener has not cached the stream state) — the ban cannot proceed.
	ErrYouTubeNotLive = errors.New("youtube: no active live chat for channel (stream not live or not cached)")
	// ErrYouTubeQuotaExceeded indicates Google refused the call for quota/rate reasons, not
	// permission ones. It arrives as an HTTP 403 like a genuine permission failure, and telling
	// them apart matters: reported as a permission problem it sends a streamer round a re-consent
	// that cannot help, or tells a delegated moderator they are not a moderator of a channel they
	// do moderate.
	ErrYouTubeQuotaExceeded = errors.New("youtube: quota or rate limit exceeded")
	// ErrYouTubeBanNotAllowed indicates YouTube refused to ban this particular user — the chat
	// owner or another moderator (`liveChatBanInsertionNotAllowed`). Also a 403, and also not about
	// the caller's authority: nobody can perform it, and no re-consent changes that.
	ErrYouTubeBanNotAllowed = errors.New("youtube: this user cannot be banned (chat owner or moderator)")
)

// quotaReasons are Google's 403 reasons that mean "not now" rather than "not allowed".
var quotaReasons = map[string]bool{
	"quotaExceeded":         true,
	"rateLimitExceeded":     true,
	"dailyLimitExceeded":    true,
	"userRateLimitExceeded": true,
}

// classifyForbidden separates the three things a YouTube 403 can mean, using the `reason` Google
// puts in the error body. An unrecognised reason falls back to a permission failure, which is the
// conservative reading: it prompts a re-consent rather than silently swallowing a real refusal.
func classifyForbidden(body []byte) error {
	var parsed struct {
		Error struct {
			Errors []struct {
				Reason string `json:"reason"`
			} `json:"errors"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ErrYouTubeForbidden
	}
	for _, e := range parsed.Error.Errors {
		switch {
		case quotaReasons[e.Reason]:
			return ErrYouTubeQuotaExceeded
		case e.Reason == "liveChatBanInsertionNotAllowed":
			return ErrYouTubeBanNotAllowed
		}
	}
	return ErrYouTubeForbidden
}

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
	return y.insertBan(ctx, token, liveChatID, bannedChannelID, 0)
}

// TimeoutUser removes a user's messages for durationSeconds.
//
// The same endpoint as BanUser with type=temporary — YouTube models a timeout as a temporary ban,
// which is why there is no separate call. Without this, YouTube moderation was permanent-ban-only
// (ADR-0048): the only tool available was the most severe one, which is a moderation-safety problem
// in its own right and doubly so for a delegated volunteer.
func (y *YouTubeClient) TimeoutUser(ctx context.Context, token, liveChatID, bannedChannelID string, durationSeconds int) error {
	if durationSeconds <= 0 {
		// YouTube would fall back to its documented 300s default, turning a caller's bug into a
		// surprise five-minute timeout that looks deliberate.
		return fmt.Errorf("youtube: timeout duration must be positive, got %d", durationSeconds)
	}
	return y.insertBan(ctx, token, liveChatID, bannedChannelID, durationSeconds)
}

// insertBan posts to liveChatBans. A positive durationSeconds makes it a temporary ban (timeout);
// zero makes it permanent, and then banDurationSeconds is omitted entirely rather than sent as 0 —
// YouTube ignores it for permanent bans, and sending both would state two intentions at once.
func (y *YouTubeClient) insertBan(ctx context.Context, token, liveChatID, bannedChannelID string, durationSeconds int) error {
	snippet := map[string]any{
		"liveChatId":        liveChatID,
		"type":              "permanent",
		"bannedUserDetails": map[string]any{"channelId": bannedChannelID},
	}
	if durationSeconds > 0 {
		snippet["type"] = "temporary"
		snippet["banDurationSeconds"] = durationSeconds
	}
	body, err := json.Marshal(map[string]any{"snippet": snippet})
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
		// Three different meanings share this status; the body's `reason` tells them apart.
		return classifyForbidden(readLimited(resp.Body))
	default:
		return fmt.Errorf("youtube: liveChatBans returned %s: %s",
			strconv.Itoa(resp.StatusCode), string(readLimited(resp.Body)))
	}
}

// readLimited reads a bounded prefix of an error body: enough for Google's `reason`, and never
// enough for a hostile or runaway response to matter.
func readLimited(r io.Reader) []byte {
	b, _ := io.ReadAll(io.LimitReader(r, 1024))
	return b
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
