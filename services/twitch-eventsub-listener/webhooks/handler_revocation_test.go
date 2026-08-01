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

package webhooks

import (
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// revocationBody builds the payload Twitch sends when it revokes a subscription.
func revocationBody(t *testing.T, subType, broadcasterID string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"subscription": map[string]interface{}{
			"id":      "sub-1",
			"type":    subType,
			"status":  "authorization_revoked",
			"version": "1",
			"condition": map[string]interface{}{
				"broadcaster_user_id": broadcasterID,
				"user_id":             broadcasterID,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type forgetRecorder struct {
	mu    sync.Mutex
	calls []string // "broadcasterID:subType"
}

func (f *forgetRecorder) forget(broadcasterID, subType string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, broadcasterID+":"+subType)
	return true
}

func (f *forgetRecorder) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// A revocation must evict the cached subscription id for EVERY type. The cached id is stale the
// moment Twitch revokes it, and because the subscribe path returns early on a cache hit, leaving it
// behind makes every future re-subscribe a silent no-op — the subscription then stays dead for the
// pod's lifetime. Eviction is what lets the channel manager's repair pass recreate it (ADR-0046).
func TestHandleRevocation_EvictsCachedSubscription(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, subType := range []string{
		"channel.chat.notification",
		"channel.chat.message_delete",
		"channel.chat.clear_user_messages",
		"channel.chat.clear",
		"channel.chat.message",
		"channel.follow",
	} {
		t.Run(subType, func(t *testing.T) {
			_, _, h := newStatusTestHandler(t)
			rec := &forgetRecorder{}
			h.SetSubscriptionForgetter(rec.forget)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			h.handleRevocation(c, revocationBody(t, subType, "12345"))

			got := rec.snapshot()
			if len(got) != 1 || got[0] != "12345:"+subType {
				t.Fatalf("forget calls = %v, want [12345:%s]", got, subType)
			}
		})
	}
}

// Without a broadcaster id in the condition there is no cache key to evict; the handler must not
// invent one (a "" key would evict nothing and hide the real problem).
func TestHandleRevocation_NoBroadcasterIDDoesNotEvict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, h := newStatusTestHandler(t)
	rec := &forgetRecorder{}
	h.SetSubscriptionForgetter(rec.forget)

	body, err := json.Marshal(map[string]interface{}{
		"subscription": map[string]interface{}{
			"id":        "sub-1",
			"type":      "channel.chat.notification",
			"status":    "authorization_revoked",
			"condition": map[string]interface{}{}, // no broadcaster_user_id
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	h.handleRevocation(c, body)

	if got := rec.snapshot(); len(got) != 0 {
		t.Fatalf("forget calls = %v, want none", got)
	}
}

// The forgetter is optional; a handler without one must still process revocations rather than panic.
func TestHandleRevocation_NilForgetterIsSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, h := newStatusTestHandler(t) // no SetSubscriptionForgetter

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	h.handleRevocation(c, revocationBody(t, "channel.chat.notification", "12345"))

	if w.Code >= 500 {
		t.Fatalf("status = %d, want a non-5xx response", w.Code)
	}
}

// A malformed revocation body must not reach the forgetter (nothing reliable to evict) and must not
// panic — Twitch retries on 5xx, and a revocation storm answered with 5xx risks further revocations.
func TestHandleRevocation_MalformedBodyIsSafe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, h := newStatusTestHandler(t)
	rec := &forgetRecorder{}
	h.SetSubscriptionForgetter(rec.forget)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	h.handleRevocation(c, []byte("{not json"))

	if got := rec.snapshot(); len(got) != 0 {
		t.Fatalf("forget calls = %v, want none for an unparseable body", got)
	}
}
