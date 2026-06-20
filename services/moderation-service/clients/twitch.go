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

// Package clients holds the per-platform moderation API clients. They are pure HTTP
// callers: given a broadcaster access token and target identifiers, they invoke the
// platform's moderation endpoints. Token resolution/decryption and authorization
// happen in the handler/token layers, not here.
package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Sentinel errors let the handler map platform failures to the right HTTP status and
// audit outcome (e.g. surface a re-consent prompt on ErrForbidden).
var (
	// ErrUnauthorized indicates the access token is invalid/expired (HTTP 401) — the
	// caller should refresh the token and retry once.
	ErrUnauthorized = errors.New("twitch: access token unauthorized")
	// ErrForbidden indicates the token lacks the moderation scope, or the moderator is
	// not permitted on the channel (HTTP 403).
	ErrForbidden = errors.New("twitch: forbidden (missing scope or not a moderator)")
)

const defaultHelixBaseURL = "https://api.twitch.tv/helix"

// TwitchClient calls Twitch Helix moderation endpoints. For own-channel moderation the
// broadcaster is also the moderator (moderator_id == broadcaster_id).
type TwitchClient struct {
	httpClient *http.Client
	clientID   string
	baseURL    string
}

// NewTwitchClient builds a client for the given Twitch application Client-Id.
func NewTwitchClient(clientID string) *TwitchClient {
	return &TwitchClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		clientID:   clientID,
		baseURL:    defaultHelixBaseURL,
	}
}

// DeleteMessage removes a single chat message.
// DELETE /helix/moderation/chat — scope moderator:manage:chat_messages.
func (t *TwitchClient) DeleteMessage(ctx context.Context, token, broadcasterID, nativeMessageID string) error {
	q := url.Values{
		"broadcaster_id": {broadcasterID},
		"moderator_id":   {broadcasterID},
		"message_id":     {nativeMessageID},
	}
	return t.do(ctx, http.MethodDelete, "/moderation/chat?"+q.Encode(), token, nil)
}

// TimeoutUser removes a user's messages for durationSeconds.
// POST /helix/moderation/bans with a duration — scope moderator:manage:banned_users.
func (t *TwitchClient) TimeoutUser(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds int, reason string) error {
	return t.ban(ctx, token, broadcasterID, targetUserID, &durationSeconds, reason)
}

// BanUser permanently bans a user.
// POST /helix/moderation/bans without a duration — scope moderator:manage:banned_users.
func (t *TwitchClient) BanUser(ctx context.Context, token, broadcasterID, targetUserID, reason string) error {
	return t.ban(ctx, token, broadcasterID, targetUserID, nil, reason)
}

// UnbanUser lifts a ban or timeout.
// DELETE /helix/moderation/bans — scope moderator:manage:banned_users.
func (t *TwitchClient) UnbanUser(ctx context.Context, token, broadcasterID, targetUserID string) error {
	q := url.Values{
		"broadcaster_id": {broadcasterID},
		"moderator_id":   {broadcasterID},
		"user_id":        {targetUserID},
	}
	return t.do(ctx, http.MethodDelete, "/moderation/bans?"+q.Encode(), token, nil)
}

// ban posts to /moderation/bans; a non-nil duration makes it a timeout.
func (t *TwitchClient) ban(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds *int, reason string) error {
	q := url.Values{
		"broadcaster_id": {broadcasterID},
		"moderator_id":   {broadcasterID},
	}
	data := map[string]any{"user_id": targetUserID}
	if durationSeconds != nil {
		data["duration"] = *durationSeconds
	}
	if reason != "" {
		data["reason"] = reason
	}
	body, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return fmt.Errorf("twitch: marshal ban body: %w", err)
	}
	return t.do(ctx, http.MethodPost, "/moderation/bans?"+q.Encode(), token, body)
}

// do issues a Helix request and maps the response status to an error. 2xx is success;
// 401/403 map to the sentinels; other codes return a descriptive error with the body.
func (t *TwitchClient) do(ctx context.Context, method, path, token string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("twitch: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", t.clientID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twitch: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("twitch: helix returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
}
