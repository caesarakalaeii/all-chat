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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTwitchTokens struct {
	cred       *tokens.TwitchCredential
	err        error
	refreshErr error
	refreshed  int
	gotUser    string
	gotChannel string
}

func (f *fakeTwitchTokens) Resolve(_ context.Context, userID, channelID string) (*tokens.TwitchCredential, error) {
	f.gotUser, f.gotChannel = userID, channelID
	return f.cred, f.err
}

func (f *fakeTwitchTokens) Refresh(_ context.Context, cred *tokens.TwitchCredential) error {
	f.refreshed++
	if f.refreshErr == nil {
		cred.AccessToken = "fresh"
	}
	return f.refreshErr
}

func helix(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	return srv.URL
}

var twitchGoal = repository.PolledGoal{OverlayID: "ov", OwnerUserID: "user-1", Metric: "followers", Platform: "twitch", ChannelID: "Streamer"}

// The owner's own credential for the source channel (ADR-0016 resolution),
// the broadcaster id from it, and Helix's total.
func TestTwitchCounter_ReadsTheFollowerTotal(t *testing.T) {
	src := &fakeTwitchTokens{cred: &tokens.TwitchCredential{AccessToken: "tok", BroadcasterID: "9001"}}
	url := helix(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/channels/followers", r.URL.Path)
		assert.Equal(t, "9001", r.URL.Query().Get("broadcaster_id"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "client-id", r.Header.Get("Client-Id"))
		_, _ = w.Write([]byte(`{"total":1234,"data":[]}`))
	})
	n, err := (&TwitchCounter{Tokens: src, ClientID: "client-id", BaseURL: url}).Count(context.Background(), twitchGoal)
	require.NoError(t, err)
	assert.EqualValues(t, 1234, n)
	assert.Equal(t, "user-1", src.gotUser)
	assert.Equal(t, "Streamer", src.gotChannel)
}

func TestTwitchCounter_RefreshesOnceOn401(t *testing.T) {
	src := &fakeTwitchTokens{cred: &tokens.TwitchCredential{AccessToken: "stale", BroadcasterID: "9001"}}
	url := helix(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"total":7}`))
	})
	n, err := (&TwitchCounter{Tokens: src, ClientID: "c", BaseURL: url}).Count(context.Background(), twitchGoal)
	require.NoError(t, err)
	assert.EqualValues(t, 7, n)
	assert.Equal(t, 1, src.refreshed)
}

func TestTwitchCounter_DegradesToReconnect(t *testing.T) {
	url := helix(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })

	_, err := (&TwitchCounter{Tokens: &fakeTwitchTokens{err: tokens.ErrNoCredential}, BaseURL: url}).Count(context.Background(), twitchGoal)
	assert.ErrorIs(t, err, ErrReconnect, "no linked credential")

	src := &fakeTwitchTokens{cred: &tokens.TwitchCredential{AccessToken: "x"}, refreshErr: errors.New("invalid_grant")}
	_, err = (&TwitchCounter{Tokens: src, BaseURL: url}).Count(context.Background(), twitchGoal)
	assert.ErrorIs(t, err, ErrReconnect, "revoked credential")

	src = &fakeTwitchTokens{cred: &tokens.TwitchCredential{AccessToken: "x"}}
	_, err = (&TwitchCounter{Tokens: src, BaseURL: url}).Count(context.Background(), twitchGoal)
	assert.ErrorIs(t, err, ErrReconnect, "still unauthorized after a refresh")
}

func TestTwitchCounter_ServerErrorIsTransient(t *testing.T) {
	url := helix(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) })
	src := &fakeTwitchTokens{cred: &tokens.TwitchCredential{AccessToken: "x"}}
	_, err := (&TwitchCounter{Tokens: src, BaseURL: url}).Count(context.Background(), twitchGoal)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrReconnect)
}
