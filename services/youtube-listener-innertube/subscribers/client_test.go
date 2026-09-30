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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := &Client{httpClient: srv.Client(), baseURL: srv.URL}
	return c, srv.Close
}

// A 401 maps to ErrUnauthorized — the poller's refresh-and-retry trigger.
func TestClient_401MapsToErrUnauthorized(t *testing.T) {
	c, closeFn := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	defer closeFn()

	_, err := c.ListRecentSubscribers(context.Background(), "expired-token")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// A 403 is NOT ErrUnauthorized — it is a scope problem that no refresh fixes.
func TestClient_403IsNotUnauthorized(t *testing.T) {
	c, closeFn := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"insufficient permissions"}}`))
	})
	defer closeFn()

	_, err := c.ListRecentSubscribers(context.Background(), "token")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnauthorized)
}

// A 429 surfaces as a rate-limit error, not ErrUnauthorized.
func TestClient_429NotUnauthorized(t *testing.T) {
	c, closeFn := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	defer closeFn()

	_, err := c.ListRecentSubscribers(context.Background(), "token")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnauthorized)
}

// The happy path decodes subscriberSnippet fields into Subscriber.
func TestClient_DecodesSubscriberList(t *testing.T) {
	c, closeFn := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer live-token", r.Header.Get("Authorization"))
		assert.Equal(t, "snippet,subscriberSnippet", r.URL.Query().Get("part"))
		assert.Equal(t, "true", r.URL.Query().Get("myRecentSubscribers"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"kind": "youtube#subscriptionListResponse",
			"items": [{
				"id": "sub-1",
				"snippet": {"publishedAt": "2026-09-26T10:00:00Z"},
				"subscriberSnippet": {
					"title": "NewFan",
					"channelId": "UC_fan1",
					"thumbnails": {"default": {"url": "https://example.com/a.png"}}
				}
			}, {
				"id": "sub-2",
				"snippet": {"publishedAt": "2026-09-26T09:00:00Z"},
				"subscriberSnippet": {
					"title": "OldFan",
					"channelId": "UC_fan2",
					"thumbnails": {}
				}
			}]
		}`))
	})
	defer closeFn()

	subs, err := c.ListRecentSubscribers(context.Background(), "live-token")
	require.NoError(t, err)
	require.Len(t, subs, 2)

	assert.Equal(t, "sub-1", subs[0].SubscriptionID)
	assert.Equal(t, "NewFan", subs[0].Title)
	assert.Equal(t, "UC_fan1", subs[0].ChannelID)
	assert.Equal(t, "https://example.com/a.png", subs[0].AvatarURL)
	wantTime, _ := time.Parse(time.RFC3339, "2026-09-26T10:00:00Z")
	assert.Equal(t, wantTime, subs[0].PublishedAt)

	assert.Equal(t, "OldFan", subs[1].Title)
	assert.Empty(t, subs[1].AvatarURL, "a subscriber without a default thumbnail decodes to empty, never an error")
}
