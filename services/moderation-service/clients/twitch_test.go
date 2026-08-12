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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedRequest struct {
	method string
	path   string
	// escapedPath is the still-encoded request target. Asserting on it is the only way to
	// see whether an id interpolated into a path was escaped: r.URL.Path has already been
	// decoded, so a smuggled %2F is indistinguishable from a real path separator there.
	escapedPath string
	query       map[string]string
	auth        string
	client      string
	body        map[string]any
}

// newTestClient returns a TwitchClient pointed at a server that records the request
// and replies with the given status.
func newTestClient(t *testing.T, status int, captured *capturedRequest) (*TwitchClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.query = map[string]string{}
		for k := range r.URL.Query() {
			captured.query[k] = r.URL.Query().Get(k)
		}
		captured.auth = r.Header.Get("Authorization")
		captured.client = r.Header.Get("Client-Id")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &captured.body)
		}
		w.WriteHeader(status)
	}))
	c := &TwitchClient{httpClient: srv.Client(), clientID: "test-client-id", baseURL: srv.URL}
	return c, srv.Close
}

func TestDeleteMessage(t *testing.T) {
	var got capturedRequest
	c, done := newTestClient(t, http.StatusNoContent, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "tok", "12345", "12345", "msg-1")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/moderation/chat", got.path)
	assert.Equal(t, "12345", got.query["broadcaster_id"])
	assert.Equal(t, "12345", got.query["moderator_id"], "own-channel moderation: moderator == broadcaster")
	assert.Equal(t, "msg-1", got.query["message_id"])
	assert.Equal(t, "Bearer tok", got.auth)
	assert.Equal(t, "test-client-id", got.client)
}

func TestBanUser_PermanentOmitsDuration(t *testing.T) {
	var got capturedRequest
	c, done := newTestClient(t, http.StatusOK, &got)
	defer done()

	err := c.BanUser(context.Background(), "tok", "12345", "12345", "999", "spam")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/moderation/bans", got.path)
	data, ok := got.body["data"].(map[string]any)
	require.True(t, ok, "ban body must wrap fields under data")
	assert.Equal(t, "999", data["user_id"])
	assert.Equal(t, "spam", data["reason"])
	_, hasDuration := data["duration"]
	assert.False(t, hasDuration, "a permanent ban must omit duration")
}

func TestTimeoutUser_IncludesDuration(t *testing.T) {
	var got capturedRequest
	c, done := newTestClient(t, http.StatusOK, &got)
	defer done()

	err := c.TimeoutUser(context.Background(), "tok", "12345", "12345", "999", 600, "cool down")
	require.NoError(t, err)
	data := got.body["data"].(map[string]any)
	assert.Equal(t, float64(600), data["duration"], "timeout must carry the duration (seconds)")
	assert.Equal(t, "999", data["user_id"])
}

func TestUnbanUser(t *testing.T) {
	var got capturedRequest
	c, done := newTestClient(t, http.StatusNoContent, &got)
	defer done()

	err := c.UnbanUser(context.Background(), "tok", "12345", "12345", "999")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/moderation/bans", got.path)
	assert.Equal(t, "999", got.query["user_id"])
}

func TestStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrForbidden},
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestClient(t, tc.status, &got)
		err := c.BanUser(context.Background(), "tok", "1", "1", "2", "")
		assert.ErrorIs(t, err, tc.want, "status %d", tc.status)
		done()
	}

	// Unexpected statuses surface a descriptive error (not a sentinel).
	var got capturedRequest
	c, done := newTestClient(t, http.StatusInternalServerError, &got)
	defer done()
	err := c.BanUser(context.Background(), "tok", "1", "1", "2", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrUnauthorized)
	assert.NotErrorIs(t, err, ErrForbidden)
}

// A delegated moderator's write is the whole point of the moderator_id/broadcaster_id split
// (ADR-0048): Helix re-checks that moderator_id is the token's own user and that they moderate
// broadcaster_id, which is the authority All-Chat cannot and must not replicate. Collapsing these
// back into one parameter would either act as the streamer or fail every delegated call.
func TestDelegatedWriteSendsDistinctModeratorID(t *testing.T) {
	t.Run("delete", func(t *testing.T) {
		var got capturedRequest
		c, done := newTestClient(t, http.StatusNoContent, &got)
		defer done()

		require.NoError(t, c.DeleteMessage(context.Background(), "mod-tok", "12345", "777", "msg-1"))
		assert.Equal(t, "12345", got.query["broadcaster_id"], "the channel being moderated")
		assert.Equal(t, "777", got.query["moderator_id"], "the moderator's own id, not the streamer's")
		assert.Equal(t, "Bearer mod-tok", got.auth, "the moderator's own token performs the call")
	})

	t.Run("ban", func(t *testing.T) {
		var got capturedRequest
		c, done := newTestClient(t, http.StatusOK, &got)
		defer done()

		require.NoError(t, c.BanUser(context.Background(), "mod-tok", "12345", "777", "999", ""))
		assert.Equal(t, "12345", got.query["broadcaster_id"])
		assert.Equal(t, "777", got.query["moderator_id"])
	})

	t.Run("unban", func(t *testing.T) {
		var got capturedRequest
		c, done := newTestClient(t, http.StatusNoContent, &got)
		defer done()

		require.NoError(t, c.UnbanUser(context.Background(), "mod-tok", "12345", "777", "999"))
		assert.Equal(t, "12345", got.query["broadcaster_id"])
		assert.Equal(t, "777", got.query["moderator_id"])
	})

	t.Run("timeout", func(t *testing.T) {
		var got capturedRequest
		c, done := newTestClient(t, http.StatusOK, &got)
		defer done()

		require.NoError(t, c.TimeoutUser(context.Background(), "mod-tok", "12345", "777", "999", 60, ""))
		assert.Equal(t, "12345", got.query["broadcaster_id"])
		assert.Equal(t, "777", got.query["moderator_id"])
	})
}
