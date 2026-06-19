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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDiscordClient returns a DiscordClient pointed at a server that records the
// request and replies with the given status.
func newTestDiscordClient(t *testing.T, status int, captured *capturedRequest) (*DiscordClient, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.auth = r.Header.Get("Authorization")
		captured.client = r.Header.Get("User-Agent")
		w.WriteHeader(status)
	}))
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}
	return c, srv.Close
}

func TestDiscordDeleteMessage(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusNoContent, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "chan-1", "msg-1")
	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, got.method)
	assert.Equal(t, "/channels/chan-1/messages/msg-1", got.path)
	assert.Equal(t, "Bot bot-tok", got.auth, "Discord uses the Bot token scheme, not Bearer")
	assert.NotEmpty(t, got.client, "Discord requires a User-Agent header")
}

func TestDiscordDelete_NotFoundIsIdempotentSuccess(t *testing.T) {
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusNotFound, &got)
	defer done()

	err := c.DeleteMessage(context.Background(), "chan-1", "gone")
	require.NoError(t, err, "a 404 (already deleted) is treated as success: DELETE is idempotent")
}

func TestDiscordDelete_StatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrDiscordUnauthorized},
		{http.StatusForbidden, ErrDiscordForbidden},
	}
	for _, tc := range cases {
		var got capturedRequest
		c, done := newTestDiscordClient(t, tc.status, &got)
		err := c.DeleteMessage(context.Background(), "c", "m")
		assert.ErrorIs(t, err, tc.want, "status %d", tc.status)
		done()
	}

	// Unexpected statuses surface a descriptive error (not a sentinel).
	var got capturedRequest
	c, done := newTestDiscordClient(t, http.StatusInternalServerError, &got)
	defer done()
	err := c.DeleteMessage(context.Background(), "c", "m")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrDiscordForbidden)
	assert.NotErrorIs(t, err, ErrDiscordUnauthorized)
}
