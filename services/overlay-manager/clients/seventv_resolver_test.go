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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestSevenTVResolver_Resolve_BareID(t *testing.T) {
	const setID = "0123456789abcdef01234567"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		w.Write([]byte(`{"id": "` + setID + `", "name": "MySet", "emotes": [{"id": "a"}, {"id": "b"}]}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), setID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
	assert.Equal(t, "MySet", resolved.Name)
	assert.Equal(t, 2, resolved.EmoteCount)
}

func TestSevenTVResolver_Resolve_EmptyInput(t *testing.T) {
	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	// Empty input should be a clean no-op without hitting the network.
	resolver.baseURL = "http://invalid.invalid"

	resolved, err := resolver.Resolve(context.Background(), "   ")
	require.NoError(t, err)
	assert.Equal(t, "", resolved.EmoteSetID)
}

func TestSevenTVResolver_Resolve_EmoteSetURL(t *testing.T) {
	const setID = "abcdef0123456789abcdef01"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		w.Write([]byte(`{"id": "` + setID + `", "name": "URLSet", "emotes": []}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), "https://7tv.app/emote-sets/"+setID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
}

func TestSevenTVResolver_Resolve_UserConnectionURL(t *testing.T) {
	const setID = "999999999999999999999999"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/v3/users/twitch/71092938"):
			w.Write([]byte(`{"emote_set": {"id": "` + setID + `"}}`))
		case strings.Contains(r.URL.Path, "/v3/emote-sets/"+setID):
			w.Write([]byte(`{"id": "` + setID + `", "name": "ResolvedFromUser", "emotes": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), "https://7tv.app/users/twitch/71092938")
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
	assert.Equal(t, "ResolvedFromUser", resolved.Name)
}

func TestSevenTVResolver_Resolve_InvalidInput(t *testing.T) {
	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = "http://invalid.invalid"

	tests := []struct {
		name    string
		input   string
		errPart string
	}{
		{"random gibberish", "not-an-id", "not a 24-char"},
		{"too short hex", "deadbeef", "not a 24-char"},
		{"foreign host", "https://example.com/emote-sets/0123456789abcdef01234567", "unsupported host"},
		{"7tv URL with bad path", "https://7tv.app/store", "does not point to"},
		{"7tv emote-sets URL with bad id", "https://7tv.app/emote-sets/short", "24-char hex"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.Resolve(context.Background(), tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errPart)
		})
	}
}

func TestSevenTVResolver_Resolve_API404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	_, err := resolver.Resolve(context.Background(), "0123456789abcdef01234567")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
