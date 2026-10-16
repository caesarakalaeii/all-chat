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

package goals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
)

// TwitchTokens resolves and refreshes the overlay owner's own Twitch
// credential for a channel. *tokens.TwitchSource (moderation-service, the
// ADR-0016 users-row-or-linked-token resolution) satisfies it.
type TwitchTokens interface {
	Resolve(ctx context.Context, userID, channelID string) (*tokens.TwitchCredential, error)
	Refresh(ctx context.Context, cred *tokens.TwitchCredential) error
}

// TwitchCounter reads a channel's follower total from Helix
// GET /channels/followers. Helix returns `total` for any user token; the
// moderator:read:followers scope (requested at every Twitch login) only adds
// the follower list, which this counter does not read.
type TwitchCounter struct {
	Tokens   TwitchTokens
	ClientID string
	BaseURL  string // default https://api.twitch.tv/helix
	HTTP     *http.Client
}

var errUnauthorized = errors.New("goals: unauthorized")

// Count returns the follower total. A missing or revoked credential, or one
// still refused after a refresh, is ErrReconnect.
func (c *TwitchCounter) Count(ctx context.Context, g repository.PolledGoal) (int64, error) {
	cred, err := c.Tokens.Resolve(ctx, g.OwnerUserID, g.ChannelID)
	if errors.Is(err, tokens.ErrNoCredential) {
		return 0, ErrReconnect
	}
	if err != nil {
		return 0, err
	}
	n, err := c.fetch(ctx, cred)
	if !errors.Is(err, errUnauthorized) {
		return n, err
	}
	if err := c.Tokens.Refresh(ctx, cred); err != nil {
		return 0, fmt.Errorf("%w: refresh: %v", ErrReconnect, err)
	}
	n, err = c.fetch(ctx, cred)
	if errors.Is(err, errUnauthorized) {
		return 0, ErrReconnect
	}
	return n, err
}

func (c *TwitchCounter) fetch(ctx context.Context, cred *tokens.TwitchCredential) (int64, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://api.twitch.tv/helix"
	}
	endpoint := base + "/channels/followers?" + url.Values{"broadcaster_id": {cred.BroadcasterID}, "first": {"1"}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("Client-Id", c.ClientID)
	body, status, err := do(c.HTTP, req)
	if err != nil {
		return 0, fmt.Errorf("helix followers: %w", err)
	}
	if status == http.StatusUnauthorized {
		return 0, errUnauthorized
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("helix followers returned %d: %s", status, body)
	}
	var parsed struct {
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode helix followers: %w", err)
	}
	return parsed.Total, nil
}

// do runs req and returns a bounded body and the status code.
func do(client *http.Client, req *http.Request) ([]byte, int, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return body, resp.StatusCode, err
}
