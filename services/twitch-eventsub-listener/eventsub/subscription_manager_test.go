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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestSubscribeToStreamOffline verifies that SubscribeToStreamOffline can
// be called on the SubscriptionManager (EXPIRY-02).
// Wave 0: RED stub — method does not exist yet.
func TestSubscribeToStreamOffline(t *testing.T) {
	// RED: SubscribeToStreamOffline method does not exist yet.
	log, _ := zap.NewDevelopment()
	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	require.NotNil(t, sm)

	// This will fail to compile until Wave 2 adds the method.
	_, err := sm.SubscribeToStreamOffline(context.Background(), "broadcaster-123")
	// We expect an error (no real Twitch API in test), but the call must exist.
	_ = err
}

// TestSubscribeToChatDeletionEvents verifies the chat-moderation subscription methods exist and are
// callable. Each shares channel.chat.message's own-channel condition + user:read:chat scope, so the
// listener can honor deletions (single message, user timeout/ban, full clear) on EventSub-owned
// channels. No real Twitch API in tests, so getAccessToken errors out before the POST — we only
// assert the methods exist and surface that error rather than panicking.
func TestSubscribeToChatDeletionEvents(t *testing.T) {
	log, _ := zap.NewDevelopment()
	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	require.NotNil(t, sm)

	ctx := context.Background()
	if _, err := sm.SubscribeToChatMessageDelete(ctx, "broadcaster-123"); err == nil {
		t.Error("expected an error without a real Twitch token endpoint")
	}
	if _, err := sm.SubscribeToChatClearUserMessages(ctx, "broadcaster-123"); err == nil {
		t.Error("expected an error without a real Twitch token endpoint")
	}
	if _, err := sm.SubscribeToChatClear(ctx, "broadcaster-123"); err == nil {
		t.Error("expected an error without a real Twitch token endpoint")
	}
}

// Test409RepopulatesCacheAndAllowsUnsubscribe verifies the M5 fix: when Twitch returns HTTP 409
// ("subscription already exists") for a POST, the manager reconciles the existing subscription id
// via a GET and repopulates the in-memory cache, so a subsequent UnsubscribeType actually issues
// the DELETE (rather than becoming a silent no-op that leaks the live subscription).
func Test409RepopulatesCacheAndAllowsUnsubscribe(t *testing.T) {
	log, _ := zap.NewDevelopment()

	var mu sync.Mutex
	var deletedIDs []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/token"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"app-token","expires_in":3600,"token_type":"bearer"}`))
		case strings.Contains(r.URL.Path, "/eventsub/subscriptions"):
			switch r.Method {
			case http.MethodPost:
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"Conflict","status":409,"message":"subscription already exists"}`))
			case http.MethodGet:
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":[{"id":"sub-existing-123","status":"enabled","type":"channel.poll.begin"}],"total":1}`))
			case http.MethodDelete:
				mu.Lock()
				deletedIDs = append(deletedIDs, r.URL.Query().Get("id"))
				mu.Unlock()
				w.WriteHeader(http.StatusNoContent)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	sm.apiURL = server.URL + "/helix/eventsub/subscriptions"
	sm.tokenURL = server.URL + "/oauth2/token"

	ctx := context.Background()

	id, err := sm.Subscribe(ctx, "channel.poll.begin", "b-1", "1")
	require.NoError(t, err)
	require.Equal(t, "sub-existing-123", id)

	sm.mu.RLock()
	cached := sm.subscriptions["b-1:channel.poll.begin"]
	sm.mu.RUnlock()
	require.Equal(t, "sub-existing-123", cached, "409 must repopulate the cache with the existing subscription id")

	err = sm.UnsubscribeType(ctx, "b-1", "channel.poll.begin")
	require.NoError(t, err)

	mu.Lock()
	gotDeletes := append([]string(nil), deletedIDs...)
	mu.Unlock()
	require.Contains(t, gotDeletes, "sub-existing-123", "UnsubscribeType must issue a DELETE for the reconciled subscription id")

	sm.mu.RLock()
	_, stillCached := sm.subscriptions["b-1:channel.poll.begin"]
	sm.mu.RUnlock()
	require.False(t, stillCached, "cache entry must be removed after UnsubscribeType")
}

// Test409GetExistingFails verifies that if the reconciliation GET fails after a 409, Subscribe
// surfaces an error and does NOT populate the cache (so a later teardown won't falsely believe it
// owns a subscription id).
func Test409GetExistingFails(t *testing.T) {
	log, _ := zap.NewDevelopment()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/oauth2/token"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"app-token","expires_in":3600,"token_type":"bearer"}`))
		case strings.Contains(r.URL.Path, "/eventsub/subscriptions"):
			switch r.Method {
			case http.MethodPost:
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"error":"Conflict","status":409,"message":"subscription already exists"}`))
			case http.MethodGet:
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(``))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	sm.apiURL = server.URL + "/helix/eventsub/subscriptions"
	sm.tokenURL = server.URL + "/oauth2/token"

	ctx := context.Background()

	_, err := sm.Subscribe(ctx, "channel.poll.begin", "b-1", "1")
	require.Error(t, err)

	sm.mu.RLock()
	_, cached := sm.subscriptions["b-1:channel.poll.begin"]
	sm.mu.RUnlock()
	require.False(t, cached, "cache must not be populated when reconciliation GET fails")
}

// ForgetSubscription drops a revoked subscription's cached id WITHOUT calling Twitch. This is what
// makes a revocation recoverable: subscribeChatScoped/Subscribe return early on a cache hit, so a
// stale entry turns every future re-subscribe into a silent no-op and the subscription can never
// come back while the pod lives (ADR-0046).
func TestForgetSubscription(t *testing.T) {
	log := zap.NewNop()

	var deleteCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalls++
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", log)
	sm.apiURL = server.URL + "/helix/eventsub/subscriptions"
	sm.tokenURL = server.URL + "/oauth2/token"

	sm.mu.Lock()
	sm.subscriptions["b-1:channel.chat.notification"] = "sub-abc"
	sm.mu.Unlock()

	require.True(t, sm.HasSubscription("b-1", "channel.chat.notification"))

	require.True(t, sm.ForgetSubscription("b-1", "channel.chat.notification"),
		"forgetting a cached subscription must report that an entry was removed")
	require.False(t, sm.HasSubscription("b-1", "channel.chat.notification"),
		"the cache entry must be gone so the repair pass can recreate the subscription")
	require.Equal(t, 0, deleteCalls,
		"a revoked subscription no longer exists on Twitch — ForgetSubscription must not issue a DELETE")

	// Idempotent, and never invents entries for unknown keys.
	require.False(t, sm.ForgetSubscription("b-1", "channel.chat.notification"))
	require.False(t, sm.ForgetSubscription("", "channel.chat.notification"))
	require.False(t, sm.ForgetSubscription("b-1", ""))
	require.False(t, sm.HasSubscription("b-1", ""))
	require.False(t, sm.HasSubscription("", ""))
}

// HasSubscription must be scoped per (broadcaster, type) so repairing one channel's notice feed
// never masks another channel's missing one.
func TestHasSubscriptionIsScopedPerBroadcasterAndType(t *testing.T) {
	sm := NewSubscriptionManager("client-id", "client-secret", "webhook-secret", "https://example.com/callback", zap.NewNop())

	sm.mu.Lock()
	sm.subscriptions["b-1:channel.chat.message"] = "sub-1"
	sm.mu.Unlock()

	require.True(t, sm.HasSubscription("b-1", "channel.chat.message"))
	require.False(t, sm.HasSubscription("b-1", "channel.chat.notification"), "different type")
	require.False(t, sm.HasSubscription("b-2", "channel.chat.message"), "different broadcaster")
}
