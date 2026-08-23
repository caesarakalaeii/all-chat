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

package eventsub

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// capturedSubscribeRequest is one POST /eventsub/subscriptions body seen by the fake Twitch API.
type capturedSubscribeRequest struct {
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
}

// newFakeTwitchAPI stands up an httptest server that answers the token endpoint and records every
// subscription POST body, returning the manager pointed at it plus an accessor for the captures.
func newFakeTwitchAPI(t *testing.T) (*SubscriptionManager, func() []capturedSubscribeRequest) {
	t.Helper()

	var mu sync.Mutex
	var captured []capturedSubscribeRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/token"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"app-token","expires_in":3600,"token_type":"bearer"}`))
		case strings.Contains(r.URL.Path, "/eventsub/subscriptions") && r.Method == http.MethodPost:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var req capturedSubscribeRequest
			if err := json.Unmarshal(body, &req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			mu.Lock()
			captured = append(captured, req)
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"data":[{"id":"sub-new","status":"webhook_callback_verification_pending","type":"` + req.Type + `"}],"total":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", zap.NewNop())
	sm.apiURL = server.URL + "/helix/eventsub/subscriptions"
	sm.tokenURL = server.URL + "/oauth2/token"

	return sm, func() []capturedSubscribeRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedSubscribeRequest(nil), captured...)
	}
}

// The moderation subscriptions all carry BOTH broadcaster_user_id and moderator_user_id. A condition
// with only broadcaster_user_id is rejected by Twitch at creation time with a 400, so asserting the
// exact key set is the whole point of this test — a silent drop of moderator_user_id would leave the
// moderation log with no events and nothing but a 400 in a log line to explain it.
func TestModerationSubscriptionsSendBothConditionKeys(t *testing.T) {
	sm, captures := newFakeTwitchAPI(t)
	ctx := context.Background()

	for _, tc := range []struct {
		wantType  string
		subscribe func(context.Context, string) (string, error)
	}{
		{"channel.moderate", sm.SubscribeToChannelModerate},
		{"automod.message.hold", sm.SubscribeToAutoModMessageHold},
		{"automod.message.update", sm.SubscribeToAutoModMessageUpdate},
	} {
		t.Run(tc.wantType, func(t *testing.T) {
			id, err := tc.subscribe(ctx, "b-1")
			require.NoError(t, err)
			require.Equal(t, "sub-new", id)
		})
	}

	got := captures()
	require.Len(t, got, 3, "each function must issue exactly one subscription POST")

	byType := make(map[string]capturedSubscribeRequest, len(got))
	for _, req := range got {
		byType[req.Type] = req
	}

	for _, wantType := range []string{"channel.moderate", "automod.message.hold", "automod.message.update"} {
		req, ok := byType[wantType]
		require.True(t, ok, "no subscription POST for %s", wantType)
		require.Equal(t, "2", req.Version, "%s must be created at version 2", wantType)
		require.Equal(t, map[string]string{
			"broadcaster_user_id": "b-1",
			"moderator_user_id":   "b-1",
		}, req.Condition, "%s condition must carry exactly the broadcaster/moderator pair", wantType)
	}
}

// A cached subscription id short-circuits the POST, exactly as the other subscribe helpers do —
// otherwise every channel resync would burn Twitch's subscription-creation budget on 409s.
func TestSubscribeToAutoModMessageHoldReturnsCachedIDWithoutHTTP(t *testing.T) {
	sm, captures := newFakeTwitchAPI(t)

	sm.mu.Lock()
	sm.subscriptions["b-1:automod.message.hold"] = "sub-x"
	sm.mu.Unlock()

	id, err := sm.SubscribeToAutoModMessageHold(context.Background(), "b-1")
	require.NoError(t, err)
	require.Equal(t, "sub-x", id)
	require.Empty(t, captures(), "a cache hit must not call Twitch")
}
