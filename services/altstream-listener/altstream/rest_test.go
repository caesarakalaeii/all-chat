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

package altstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFetchEmotes_ResolvesSiteRelativeURLs(t *testing.T) {
	body, err := os.ReadFile("testdata/channel_emotes.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/channels/copygirl/emotes", r.URL.Path)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, nil, zap.NewNop())
	cat, err := c.fetchEmotes(context.Background(), "copygirl")
	require.NoError(t, err)
	assert.Len(t, cat, 8)
	assert.Equal(t, srv.URL+"/api/media/stickers/copygirl_1792322555448_b51313fd3353.png", cat["copyPeek"])
	assert.Equal(t, srv.URL+"/api/media/stickers/copygirl_1792322601534_c8d7e093cc65.webp", cat["copyVibe"])
}

func TestEmoteStore_ResolvesAcrossWatchedChannels(t *testing.T) {
	s := newEmoteStore()
	s.set("copygirl", map[string]string{"copyPeek": "https://a/peek.png", "copyHeck": "https://a/heck.png"})
	s.set("fletchmakes", map[string]string{"fletchlaugh": "https://a/laugh.png"})

	// Unknown tokens stay text; repeats are listed once, in first-seen order;
	// a token of another watched channel resolves too.
	got := s.resolve(":fletchlaugh: hi :copyPeek: :nope: :copyPeek: copyHeck :copyHeck:")
	assert.Equal(t, []Emote{
		{Name: "fletchlaugh", URL: "https://a/laugh.png"},
		{Name: "copyPeek", URL: "https://a/peek.png"},
		{Name: "copyHeck", URL: "https://a/heck.png"},
	}, got)

	// A channel that is no longer watched no longer contributes.
	s.set("fletchmakes", nil)
	assert.Equal(t, []Emote{}, s.resolve(":fletchlaugh:"))
	assert.Equal(t, []Emote{}, s.resolve("no emotes here: at all"))
}

func TestLatestID_NewestOfOldestFirstHistory(t *testing.T) {
	body, err := os.ReadFile("testdata/rest_recent_chat.json")
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "achie7240", r.URL.Query().Get("channel"))
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, nil, zap.NewNop())
	assert.Equal(t, "952046758", c.latestID(context.Background(), "achie7240"))
}
