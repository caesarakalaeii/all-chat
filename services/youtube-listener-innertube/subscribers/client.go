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

package subscribers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls the official YouTube Data API subscriptions.list with
// myRecentSubscribers=true: the authenticated channel's own subscribers,
// newest first, ~1000 most recent, 1 quota unit per call.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a subscribers API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://www.googleapis.com/youtube/v3/subscriptions",
	}
}

// listResponse is the subset of the subscriptionListResponse we consume.
type listResponse struct {
	Items []listItem `json:"items"`
}

// listItem maps the subscription resource fields the announcement needs.
type listItem struct {
	ID      string `json:"id"`
	Snippet struct {
		PublishedAt time.Time `json:"publishedAt"`
	} `json:"snippet"`
	SubscriberSnippet struct {
		Title      string `json:"title"`
		ChannelID  string `json:"channelId"`
		Thumbnails map[string]struct {
			URL string `json:"url"`
		} `json:"thumbnails"`
	} `json:"subscriberSnippet"`
}

// ListRecentSubscribers returns the channel's most recent public subscribers,
// newest first, one page. A 401 maps to ErrUnauthorized (the caller refreshes
// and retries once); other statuses return a descriptive error.
func (c *Client) ListRecentSubscribers(ctx context.Context, accessToken string) ([]Subscriber, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("subscribers: build request: %w", err)
	}
	q := req.URL.Query()
	q.Set("part", "snippet,subscriberSnippet")
	q.Set("myRecentSubscribers", "true")
	q.Set("maxResults", "50")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("subscribers: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Bounded read: a hostile or broken response must not exhaust memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("subscribers: read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, ErrUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("subscribers: forbidden (missing scope): %s", string(body))
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("subscribers: rate limited (429): %s", string(body))
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("subscribers: http %d: %s", resp.StatusCode, string(body))
	}

	var parsed listResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("subscribers: decode response: %w", err)
	}

	subs := make([]Subscriber, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		avatar := ""
		if t, ok := it.SubscriberSnippet.Thumbnails["default"]; ok {
			avatar = t.URL
		}
		subs = append(subs, Subscriber{
			SubscriptionID: it.ID,
			Title:          it.SubscriberSnippet.Title,
			ChannelID:      it.SubscriberSnippet.ChannelID,
			AvatarURL:      avatar,
			PublishedAt:    it.Snippet.PublishedAt,
		})
	}
	return subs, nil
}

// Compile-time guard: the client satisfies the poller's API interface.
var _ SubscriberAPI = (*Client)(nil)

var _ = errors.New
