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
)

// Kick moderation uses the broadcaster's own OAuth token (OAuth 2.1) against the
// public Kick API. Kick exposes ban / timeout / unban but no single-message delete,
// so this client has no DeleteMessage. The bans endpoint takes numeric ids and a
// timeout duration in MINUTES (our pipeline carries seconds), so the client converts.
//
// The Kick public API is young (GA May 2025): bodies are logged defensively by the
// caller on unexpected statuses, and the broadcaster/user id JSON type (integer per
// the published OpenAPI) is a documented staging-validation point.
var (
	// ErrKickUnauthorized indicates the access token is invalid/expired (HTTP 401) —
	// the dispatcher refreshes and retries once.
	ErrKickUnauthorized = errors.New("kick: access token unauthorized")
	// ErrKickForbidden indicates the token lacks the moderation scope or the user is
	// not a moderator of the channel (HTTP 403) — surfaced as a re-consent prompt.
	ErrKickForbidden = errors.New("kick: forbidden (missing scope or not a moderator)")
)

const defaultKickBaseURL = "https://api.kick.com/public/v1"

// KickClient calls Kick's public moderation endpoints as the broadcaster.
type KickClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewKickClient builds a Kick moderation client.
func NewKickClient() *KickClient {
	return &KickClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    defaultKickBaseURL,
	}
}

// TimeoutUser bans a user for durationSeconds (converted to whole minutes, the unit
// the Kick API expects; rounded up so a sub-minute timeout is at least one minute).
// POST /moderation/bans with a duration.
func (k *KickClient) TimeoutUser(ctx context.Context, token, broadcasterID, targetUserID string, durationSeconds int, reason string) error {
	minutes := (durationSeconds + 59) / 60
	if minutes < 1 {
		minutes = 1
	}
	return k.ban(ctx, token, broadcasterID, targetUserID, &minutes, reason)
}

// BanUser permanently bans a user. POST /moderation/bans without a duration.
func (k *KickClient) BanUser(ctx context.Context, token, broadcasterID, targetUserID, reason string) error {
	return k.ban(ctx, token, broadcasterID, targetUserID, nil, reason)
}

// UnbanUser lifts a ban or timeout. DELETE /moderation/bans.
func (k *KickClient) UnbanUser(ctx context.Context, token, broadcasterID, targetUserID string) error {
	bID, uID, err := kickIDs(broadcasterID, targetUserID)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"broadcaster_user_id": bID, "user_id": uID})
	if err != nil {
		return fmt.Errorf("kick: marshal unban body: %w", err)
	}
	return k.do(ctx, http.MethodDelete, "/moderation/bans", token, body)
}

// ban posts to /moderation/bans; a non-nil durationMinutes makes it a timeout.
func (k *KickClient) ban(ctx context.Context, token, broadcasterID, targetUserID string, durationMinutes *int, reason string) error {
	bID, uID, err := kickIDs(broadcasterID, targetUserID)
	if err != nil {
		return err
	}
	data := map[string]any{"broadcaster_user_id": bID, "user_id": uID}
	if durationMinutes != nil {
		data["duration"] = *durationMinutes
	}
	if reason != "" {
		data["reason"] = reason
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("kick: marshal ban body: %w", err)
	}
	return k.do(ctx, http.MethodPost, "/moderation/bans", token, body)
}

// kickIDs parses the broadcaster and target ids to integers, the type the Kick bans
// endpoint expects. A non-numeric id (should never happen for Kick, whose ids are
// numeric) is a clear error rather than a silent malformed request.
func kickIDs(broadcasterID, targetUserID string) (int, int, error) {
	bID, err := strconv.Atoi(broadcasterID)
	if err != nil {
		return 0, 0, fmt.Errorf("kick: non-numeric broadcaster id %q: %w", broadcasterID, err)
	}
	uID, err := strconv.Atoi(targetUserID)
	if err != nil {
		return 0, 0, fmt.Errorf("kick: non-numeric target user id %q: %w", targetUserID, err)
	}
	return bID, uID, nil
}

// do issues a Kick request and maps the status to an error. 2xx is success; 401/403
// map to the sentinels; other codes return a descriptive error with the body.
func (k *KickClient) do(ctx context.Context, method, path, token string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, k.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("kick: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("kick: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrKickUnauthorized
	case http.StatusForbidden:
		return ErrKickForbidden
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("kick: api returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
}
