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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestYouTubeClient(t *testing.T, status int, captured *capturedRequest) (*YouTubeClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = map[string]string{}
		for k := range r.URL.Query() {
			captured.query[k] = r.URL.Query().Get(k)
		}
		captured.auth = r.Header.Get("Authorization")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &captured.body)
		}
		w.WriteHeader(status)
	}))
	c := &YouTubeClient{httpClient: srv.Client(), baseURL: srv.URL}
	return c, srv.Close
}

func TestYouTubeBanUser(t *testing.T) {
	var got capturedRequest
	c, done := newTestYouTubeClient(t, http.StatusOK, &got)
	defer done()

	err := c.BanUser(context.Background(), "tok", "livechat-1", "UCbanned")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/liveChat/bans", got.path)
	assert.Equal(t, "snippet", got.query["part"])
	assert.Equal(t, "Bearer tok", got.auth)

	snippet, ok := got.body["snippet"].(map[string]any)
	require.True(t, ok, "ban body must wrap fields under snippet")
	assert.Equal(t, "livechat-1", snippet["liveChatId"])
	assert.Equal(t, "permanent", snippet["type"])
	details, ok := snippet["bannedUserDetails"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "UCbanned", details["channelId"], "the banned user's YouTube channel id")
}

// A timeout is the SAME endpoint with type=temporary plus a duration. Getting the type wrong makes
// a timeout a permanent ban, which is why it is asserted rather than assumed.
func TestYouTubeTimeoutUser(t *testing.T) {
	var got capturedRequest
	c, done := newTestYouTubeClient(t, http.StatusOK, &got)
	defer done()

	err := c.TimeoutUser(context.Background(), "tok", "livechat-1", "UCbanned", 600)
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/liveChat/bans", got.path)

	snippet, ok := got.body["snippet"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "temporary", snippet["type"], "a timeout must never be sent as a permanent ban")
	assert.Equal(t, float64(600), snippet["banDurationSeconds"])
	details, ok := snippet["bannedUserDetails"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "UCbanned", details["channelId"])
}

// A permanent ban must not carry a duration: YouTube ignores banDurationSeconds unless the type is
// temporary, and sending both states two intentions at once.
func TestYouTubeBanUser_OmitsDuration(t *testing.T) {
	var got capturedRequest
	c, done := newTestYouTubeClient(t, http.StatusOK, &got)
	defer done()

	require.NoError(t, c.BanUser(context.Background(), "tok", "lc", "UC"))
	snippet := got.body["snippet"].(map[string]any)
	_, hasDuration := snippet["banDurationSeconds"]
	assert.False(t, hasDuration)
}

// A non-positive duration would silently become YouTube's 300s default, so it is refused before the
// request: a caller asking for "0 seconds" has a bug, and a surprise 5-minute timeout hides it.
func TestYouTubeTimeoutUser_RejectsNonPositiveDuration(t *testing.T) {
	for _, seconds := range []int{0, -1} {
		var got capturedRequest
		c, done := newTestYouTubeClient(t, http.StatusOK, &got)
		err := c.TimeoutUser(context.Background(), "tok", "lc", "UC", seconds)
		require.Error(t, err, "duration %d", seconds)
		assert.Empty(t, got.method, "must fail before any HTTP request")
		done()
	}
}

func TestYouTubeTimeoutUser_StatusMapping(t *testing.T) {
	var got capturedRequest
	c, done := newTestYouTubeClient(t, http.StatusForbidden, &got)
	defer done()

	err := c.TimeoutUser(context.Background(), "tok", "lc", "UC", 60)
	assert.ErrorIs(t, err, ErrYouTubeForbidden)
}

// A YouTube 403 means three different things, and only one of them is about the caller's
// authority. Conflating them tells a streamer to re-consent when the project ran out of quota, or
// tells a delegated moderator they do not moderate a channel they do.
func TestYouTubeForbiddenIsClassifiedByReason(t *testing.T) {
	cases := []struct {
		name string
		body string
		want error
	}{
		{
			"quota exhausted",
			`{"error":{"code":403,"errors":[{"reason":"quotaExceeded","message":"quota"}]}}`,
			ErrYouTubeQuotaExceeded,
		},
		{
			"rate limited",
			`{"error":{"code":403,"errors":[{"reason":"rateLimitExceeded"}]}}`,
			ErrYouTubeQuotaExceeded,
		},
		{
			"target cannot be banned",
			`{"error":{"code":403,"errors":[{"reason":"liveChatBanInsertionNotAllowed"}]}}`,
			ErrYouTubeBanNotAllowed,
		},
		{
			"genuine permission failure",
			`{"error":{"code":403,"errors":[{"reason":"insufficientPermissions"}]}}`,
			ErrYouTubeForbidden,
		},
		{"unparseable body falls back to a permission failure", `not json`, ErrYouTubeForbidden},
		{"empty body falls back to a permission failure", ``, ErrYouTubeForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := &YouTubeClient{httpClient: srv.Client(), baseURL: srv.URL}

			err := c.BanUser(context.Background(), "tok", "lc", "UC")
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestYouTubeBanUser_StatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrYouTubeUnauthorized},
		{http.StatusForbidden, ErrYouTubeForbidden},
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestYouTubeClient(t, tc.status, &got)
		err := c.BanUser(context.Background(), "tok", "lc", "UC")
		assert.ErrorIs(t, err, tc.want, "status %d", tc.status)
		done()
	}

	var got capturedRequest
	c, done := newTestYouTubeClient(t, http.StatusInternalServerError, &got)
	defer done()
	err := c.BanUser(context.Background(), "tok", "lc", "UC")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrYouTubeUnauthorized)
	assert.NotErrorIs(t, err, ErrYouTubeForbidden)
}

func newMiniredisResolver(t *testing.T) (*YouTubeLiveChatResolver, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewYouTubeLiveChatResolver(client), mr
}

func TestYouTubeLiveChatResolver_LiveChannel(t *testing.T) {
	r, mr := newMiniredisResolver(t)
	require.NoError(t, mr.Set("youtube:stream:state:UCabc", `{"live_chat_id":"lc-99","is_live":true}`))

	got, err := r.Resolve(context.Background(), "UCabc")
	require.NoError(t, err)
	assert.Equal(t, "lc-99", got)
}

func TestYouTubeLiveChatResolver_NotCachedOrOffline(t *testing.T) {
	r, mr := newMiniredisResolver(t)

	// No cached state.
	_, err := r.Resolve(context.Background(), "UCmissing")
	assert.ErrorIs(t, err, ErrYouTubeNotLive)

	// Cached but offline.
	require.NoError(t, mr.Set("youtube:stream:state:UCoff", `{"live_chat_id":"lc","is_live":false}`))
	_, err = r.Resolve(context.Background(), "UCoff")
	assert.ErrorIs(t, err, ErrYouTubeNotLive)
}
