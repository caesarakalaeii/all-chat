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

func newTestKickClient(t *testing.T, status int, captured *capturedRequest) (*KickClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.escapedPath = r.URL.EscapedPath()
		captured.auth = r.Header.Get("Authorization")
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &captured.body)
		}
		w.WriteHeader(status)
	}))
	c := &KickClient{httpClient: srv.Client(), baseURL: srv.URL}
	return c, srv.Close
}

func TestKickBanUser_PermanentOmitsDuration(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusOK, &got)
	defer done()

	err := c.BanUser(context.Background(), "tok", "12345", "999", "spam")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, got.method)
	assert.Equal(t, "/moderation/bans", got.path)
	assert.Equal(t, "Bearer tok", got.auth)
	assert.Equal(t, float64(12345), got.body["broadcaster_user_id"], "ids are sent as JSON numbers")
	assert.Equal(t, float64(999), got.body["user_id"])
	assert.Equal(t, "spam", got.body["reason"])
	_, hasDuration := got.body["duration"]
	assert.False(t, hasDuration, "a permanent ban must omit duration")
}

func TestKickTimeoutUser_ConvertsSecondsToMinutes(t *testing.T) {
	cases := []struct {
		seconds     int
		wantMinutes float64
	}{
		{60, 1},
		{600, 10},
		{3600, 60},
		{30, 1}, // rounds up: a sub-minute timeout is at least one minute
		{90, 2}, // 1.5 min rounds up to 2
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestKickClient(t, http.StatusOK, &got)
		err := c.TimeoutUser(context.Background(), "tok", "12345", "999", tc.seconds, "")
		require.NoError(t, err)
		assert.Equal(t, tc.wantMinutes, got.body["duration"], "%ds should be %v minutes", tc.seconds, tc.wantMinutes)
		done()
	}
}

func TestKickUnbanUser(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusNoContent, &got)
	defer done()

	err := c.UnbanUser(context.Background(), "tok", "12345", "999")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/moderation/bans", got.path)
	assert.Equal(t, float64(12345), got.body["broadcaster_user_id"])
	assert.Equal(t, float64(999), got.body["user_id"])
}

func TestKickDeleteMessage(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusOK, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "tok", "8f2c1d64-0b3a-4f19-9a2e-1b7c3d5e6f70")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/chat/8f2c1d64-0b3a-4f19-9a2e-1b7c3d5e6f70", got.path,
		"the message id is a path segment, not a body field")
	assert.Equal(t, "Bearer tok", got.auth)
	assert.Empty(t, got.body, "the delete endpoint takes no body")
}

// The message id reaches us from the chat pipeline, so it is attacker-influenced input
// interpolated into a URL path. Escaping keeps it inside its own segment: without it, an id
// of "../moderation/bans" would resolve to a completely different endpoint.
func TestKickDeleteMessage_EscapesTheMessageID(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusOK, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "tok", "../moderation/bans")
	require.NoError(t, err)
	assert.Equal(t, "/chat/..%2Fmoderation%2Fbans", got.escapedPath,
		"a slash in the id must stay escaped rather than becoming a path separator")
}

func TestKickDeleteMessage_RejectsEmptyID(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusOK, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "tok", "")
	require.Error(t, err)
	assert.Empty(t, got.method, "an empty id must fail before any HTTP request: DELETE /chat/ is not this endpoint")
}

func TestKickDeleteMessage_StatusMapping(t *testing.T) {
	for status, want := range map[int]error{
		http.StatusUnauthorized: ErrKickUnauthorized,
		http.StatusForbidden:    ErrKickForbidden,
	} {
		var got capturedRequest
		c, done := newTestKickClient(t, status, &got)
		err := c.DeleteMessage(context.Background(), "tok", "msg-1")
		assert.ErrorIs(t, err, want, "status %d", status)
		done()
	}
}

func TestKickNonNumericIDIsError(t *testing.T) {
	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusOK, &got)
	defer done()

	err := c.BanUser(context.Background(), "tok", "not-a-number", "999", "")
	require.Error(t, err)
	assert.Empty(t, got.method, "a malformed id must fail before any HTTP request")
}

func TestKickStatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrKickUnauthorized},
		{http.StatusForbidden, ErrKickForbidden},
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestKickClient(t, tc.status, &got)
		err := c.BanUser(context.Background(), "tok", "1", "2", "")
		assert.ErrorIs(t, err, tc.want, "status %d", tc.status)
		done()
	}

	var got capturedRequest
	c, done := newTestKickClient(t, http.StatusInternalServerError, &got)
	defer done()
	err := c.BanUser(context.Background(), "tok", "1", "2", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrKickUnauthorized)
	assert.NotErrorIs(t, err, ErrKickForbidden)
}
