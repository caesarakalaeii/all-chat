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
	"net/http"
	"net/url"
	"strconv"

	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/shared/youtubetoken"
)

// YouTubeTokens resolves and refreshes the overlay owner's YouTube credential
// for a channel; *youtubetoken.YouTubeSource satisfies it.
type YouTubeTokens interface {
	Resolve(ctx context.Context, userID, channelID string) (*youtubetoken.YouTubeCredential, error)
	Refresh(ctx context.Context, cred *youtubetoken.YouTubeCredential) error
}

// Quota is the shared reserve/confirm/rollback ledger (ADR-0006);
// *quota.Reserver satisfies it. Spending through it is what makes these polls
// visible to youtube-quota-monitor (ADR-0023), which reads the same table.
type Quota interface {
	Reserve(ctx context.Context, units int) (bool, error)
	Confirm(ctx context.Context, units int) error
	Rollback(ctx context.Context, units int) error
}

// channelsListCost is channels.list?part=statistics: 1 unit, the cheapest
// call that carries a subscriber count.
const channelsListCost = 1

// YouTubeCounter reads a channel's subscriber count. YouTube rounds public
// subscriber counts to three significant figures, so a large channel's bar
// moves in steps.
type YouTubeCounter struct {
	Tokens  YouTubeTokens
	Quota   Quota
	BaseURL string // default https://www.googleapis.com/youtube/v3
	HTTP    *http.Client
}

// Count returns the subscriber count. Every call reserves its unit first and
// confirms or rolls it back, mirroring the innertube subscriber poller.
func (c *YouTubeCounter) Count(ctx context.Context, g repository.PolledGoal) (int64, error) {
	cred, err := c.Tokens.Resolve(ctx, g.OwnerUserID, g.ChannelID)
	if errors.Is(err, youtubetoken.ErrNoCredential) {
		return 0, ErrReconnect
	}
	if err != nil {
		return 0, err
	}
	n, err := c.reservedFetch(ctx, cred, g.ChannelID)
	if !errors.Is(err, errUnauthorized) {
		return n, err
	}
	if err := c.Tokens.Refresh(ctx, cred); err != nil {
		return 0, fmt.Errorf("%w: refresh: %v", ErrReconnect, err)
	}
	n, err = c.reservedFetch(ctx, cred, g.ChannelID)
	if errors.Is(err, errUnauthorized) {
		return 0, ErrReconnect
	}
	return n, err
}

func (c *YouTubeCounter) reservedFetch(ctx context.Context, cred *youtubetoken.YouTubeCredential, channelID string) (int64, error) {
	ok, err := c.Quota.Reserve(ctx, channelsListCost)
	if err != nil || !ok {
		return 0, ErrQuota
	}
	n, err := c.fetch(ctx, cred, channelID)
	if err != nil && !errors.Is(err, ErrHidden) {
		_ = c.Quota.Rollback(ctx, channelsListCost)
		return 0, err
	}
	// The call reached YouTube (a hidden count included): the unit is spent.
	if cerr := c.Quota.Confirm(ctx, channelsListCost); cerr != nil {
		return n, fmt.Errorf("confirm youtube quota: %w", cerr)
	}
	return n, err
}

func (c *YouTubeCounter) fetch(ctx context.Context, cred *youtubetoken.YouTubeCredential, channelID string) (int64, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://www.googleapis.com/youtube/v3"
	}
	endpoint := base + "/channels?" + url.Values{"part": {"statistics"}, "id": {channelID}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	body, status, err := do(c.HTTP, req)
	if err != nil {
		return 0, fmt.Errorf("youtube channels.list: %w", err)
	}
	if status == http.StatusUnauthorized {
		return 0, errUnauthorized
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("youtube channels.list returned %d: %s", status, body)
	}
	var parsed struct {
		Items []struct {
			Statistics struct {
				SubscriberCount       string `json:"subscriberCount"`
				HiddenSubscriberCount bool   `json:"hiddenSubscriberCount"`
			} `json:"statistics"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode youtube channels.list: %w", err)
	}
	if len(parsed.Items) == 0 {
		return 0, fmt.Errorf("youtube channel %s not found", channelID)
	}
	stats := parsed.Items[0].Statistics
	if stats.HiddenSubscriberCount {
		return 0, ErrHidden
	}
	return strconv.ParseInt(stats.SubscriberCount, 10, 64)
}
