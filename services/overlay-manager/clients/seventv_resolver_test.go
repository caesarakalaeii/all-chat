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

func TestSevenTVResolver_Resolve_BareHexID(t *testing.T) {
	const setID = "0123456789abcdef01234567"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "MySet", "emotes": [{"id": "a"}, {"id": "b"}]}`))
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

func TestSevenTVResolver_Resolve_BareULID(t *testing.T) {
	const setID = "01K0BT1KXDYA24WQJD80CRZC75" // real 7TV ULID format

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "coom", "emotes": []}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), setID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
	assert.Equal(t, "coom", resolved.Name)
}

func TestSevenTVResolver_Resolve_EmptyInput(t *testing.T) {
	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	// Empty input should be a clean no-op without hitting the network.
	resolver.baseURL = "http://invalid.invalid"

	resolved, err := resolver.Resolve(context.Background(), "   ")
	require.NoError(t, err)
	assert.Equal(t, "", resolved.EmoteSetID)
}

func TestSevenTVResolver_Resolve_EmoteSetURL_Hex(t *testing.T) {
	const setID = "abcdef0123456789abcdef01"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "URLSet", "emotes": []}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), "https://7tv.app/emote-sets/"+setID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
}

func TestSevenTVResolver_Resolve_EmoteSetURL_ULID(t *testing.T) {
	const setID = "01K0BT1KXDYA24WQJD80CRZC75"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/v3/emote-sets/"+setID)
		_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "ULIDSet", "emotes": [{"id":"x"}]}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), "https://7tv.app/emote-sets/"+setID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
	assert.Equal(t, 1, resolved.EmoteCount)
}

func TestSevenTVResolver_Resolve_UserConnectionURL(t *testing.T) {
	const setID = "999999999999999999999999"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/v3/users/twitch/71092938"):
			_, _ = w.Write([]byte(`{"emote_set_id": "` + setID + `", "emote_set": {"id": "` + setID + `"}}`))
		case strings.Contains(r.URL.Path, "/v3/emote-sets/"+setID):
			_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "ResolvedFromUser", "emotes": []}`))
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

// TestSevenTVResolver_Resolve_UserULID_EmoteSetsURL is the regression test for
// the broken-import bug — the 7TV web UI links streamers' emote-set pages as
// /users/{user_id}/emote-sets and the old resolver mis-routed that to
// /v3/users/{user_id}/emote-sets (which 7TV returns 400 "invalid platform" for).
// It must now be treated as the user form and resolve via the user's Twitch
// connection's active emote set.
func TestSevenTVResolver_Resolve_UserULID_EmoteSetsURL(t *testing.T) {
	const (
		userID = "01K0BSRG70F5HE3TYNDYCPFR70"
		setID  = "01K0BT1KXDYA24WQJD80CRZC75"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v3/users/"+userID:
			// Mirror the real 7TV response shape: emote_sets[] and connections[].
			_, _ = w.Write([]byte(`{
				"id": "` + userID + `",
				"username": "tuffaholicc",
				"emote_sets": [{"id": "` + setID + `", "name": "coom"}],
				"connections": [{
					"platform": "TWITCH",
					"emote_set_id": "` + setID + `",
					"emote_set": {"id": "` + setID + `"}
				}]
			}`))
		case r.URL.Path == "/v3/emote-sets/"+setID:
			_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "coom", "emotes": [{"id":"a"},{"id":"b"},{"id":"c"}]}`))
		default:
			t.Logf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	// Both the bare /users/{id} form and the /users/{id}/emote-sets SPA form
	// should resolve identically — that's the whole point of the fix.
	for _, input := range []string{
		"https://7tv.app/users/" + userID,
		"https://7tv.app/users/" + userID + "/emote-sets",
	} {
		t.Run(input, func(t *testing.T) {
			resolved, err := resolver.Resolve(context.Background(), input)
			require.NoError(t, err)
			assert.Equal(t, setID, resolved.EmoteSetID)
			assert.Equal(t, "coom", resolved.Name)
			assert.Equal(t, 3, resolved.EmoteCount)
		})
	}
}

// TestSevenTVResolver_Resolve_UserNoTwitchConnection covers a user whose
// active set is reachable only via emote_sets[] or a non-Twitch connection.
// We should still resolve to an active set, not error out.
func TestSevenTVResolver_Resolve_UserNoTwitchConnection(t *testing.T) {
	const (
		userID = "01K0BSRG70F5HE3TYNDYCPFR70"
		setID  = "01K0BT1KXDYA24WQJD80CRZC76"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v3/users/"+userID:
			_, _ = w.Write([]byte(`{
				"id": "` + userID + `",
				"emote_sets": [{"id": "` + setID + `", "name": "fallback"}],
				"connections": [{"platform": "KICK", "emote_set_id": "` + setID + `"}]
			}`))
		case r.URL.Path == "/v3/emote-sets/"+setID:
			_, _ = w.Write([]byte(`{"id": "` + setID + `", "name": "fallback", "emotes": []}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	resolved, err := resolver.Resolve(context.Background(), "https://7tv.app/users/"+userID)
	require.NoError(t, err)
	assert.Equal(t, setID, resolved.EmoteSetID)
}

func TestSevenTVResolver_Resolve_UserWithoutSets(t *testing.T) {
	const userID = "01K0BSRG70F5HE3TYNDYCPFR70"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v3/users/"+userID, r.URL.Path)
		_, _ = w.Write([]byte(`{"id": "` + userID + `", "emote_sets": [], "connections": []}`))
	}))
	defer server.Close()

	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = server.URL

	_, err := resolver.Resolve(context.Background(), "https://7tv.app/users/"+userID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active emote set")
}

func TestSevenTVResolver_Resolve_InvalidInput(t *testing.T) {
	resolver := NewSevenTVResolver(zaptest.NewLogger(t))
	resolver.baseURL = "http://invalid.invalid"

	tests := []struct {
		name    string
		input   string
		errPart string
	}{
		{"random gibberish", "not-an-id", "not a 7TV emote-set id"},
		{"too short hex", "deadbeef", "not a 7TV emote-set id"},
		{"foreign host", "https://example.com/emote-sets/0123456789abcdef01234567", "unsupported host"},
		{"7tv URL with bad path", "https://7tv.app/store", "does not point to"},
		{"7tv emote-sets URL with bad id", "https://7tv.app/emote-sets/short", "24-char hex or 26-char ULID"},
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
