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
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newStatusTestHandler builds a Handler wired to an in-memory Redis with a REAL status publisher
// (so the platform:status pub/sub writes are observable). db/metrics are nil — the chat-message
// path does not touch them. The claim store is wired so refreshChatClaim stays on its normal path.
func newStatusTestHandler(t *testing.T) (*miniredis.Miniredis, *redis.Client, *Handler) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	pub := publisher.NewStreamPublisher(rc, zap.NewNop())
	t.Cleanup(pub.Stop)
	statusPub := status.NewPublisher(rc, zap.NewNop())
	reg := registry.NewRedisRegistry(rc, time.Hour)
	h := NewHandler("secret", rc, nil, pub, nil, statusPub, twitchchat.NewClaimStore(rc), reg, zap.NewNop())
	return mr, rc, h
}

// subscribePlatformStatus subscribes to the platform:status Redis channel and returns the message
// channel. The subscription is confirmed (Receive) before returning so callers can publish
// immediately without racing the subscription registration.
func subscribePlatformStatus(t *testing.T, rc *redis.Client) <-chan *redis.Message {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pubsub := rc.Subscribe(ctx, status.PlatformStatusChannel)
	t.Cleanup(func() { _ = pubsub.Close() })
	if _, err := pubsub.Receive(ctx); err != nil {
		t.Fatalf("subscribe to %s: %v", status.PlatformStatusChannel, err)
	}
	return pubsub.Channel()
}

// A delivered chat message is the definitive proof that the channel.chat.message subscription is
// live, so the handler must republish platform:status "connected" (keyed by the lowercased login
// that overlay_chat_sources uses). This is what keeps the overlay indicator green across
// api-gateway / listener restarts — the challenge-time publish only fires once, when the
// subscription is first created.
func TestHandleChatMessage_PublishesConnectedStatus(t *testing.T) {
	_, rc, h := newStatusTestHandler(t)
	ch := subscribePlatformStatus(t, rc)

	if err := h.handleChatMessage(context.Background(), chatEventJSON(t, "CaesarLP", "67241623", "m1")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}

	select {
	case m := <-ch:
		var s status.Message
		if err := json.Unmarshal([]byte(m.Payload), &s); err != nil {
			t.Fatalf("unmarshal status payload: %v (payload=%q)", err, m.Payload)
		}
		if s.Platform != "twitch" || s.ChannelID != "caesarlp" || s.Status != "connected" {
			t.Fatalf("status = %+v, want platform=twitch channel_id=caesarlp status=connected", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a platform:status connected publish on chat message, timed out")
	}
}

// The connected heartbeat is throttled per channel: a second message within the heartbeat interval
// must NOT trigger a second publish, but once the interval elapses the next message republishes
// (so a freshly-restarted api-gateway rehydrates its indicator state).
func TestHandleChatMessage_ConnectedStatusThrottled(t *testing.T) {
	_, rc, h := newStatusTestHandler(t)
	ch := subscribePlatformStatus(t, rc)
	ctx := context.Background()

	// First message publishes connected.
	if err := h.handleChatMessage(ctx, chatEventJSON(t, "chanA", "1", "a1")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}
	// Second message within the heartbeat interval is throttled — no second publish.
	if err := h.handleChatMessage(ctx, chatEventJSON(t, "chanA", "1", "a2")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}

	// Expect exactly the first publish.
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("expected first connected publish, timed out")
	}
	// No second publish should arrive.
	select {
	case m := <-ch:
		t.Fatalf("expected throttling, but received a second publish: %s", m.Payload)
	case <-time.After(300 * time.Millisecond):
		// good — throttled
	}

	// Expire the throttle and assert the next message republishes connected.
	h.statusMu.Lock()
	h.statusPublishedAt["chana"] = time.Now().Add(-2 * statusHeartbeatInterval)
	h.statusMu.Unlock()

	if err := h.handleChatMessage(ctx, chatEventJSON(t, "chanA", "1", "a3")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}
	select {
	case m := <-ch:
		var s status.Message
		if err := json.Unmarshal([]byte(m.Payload), &s); err != nil {
			t.Fatalf("unmarshal status payload: %v", err)
		}
		if s.ChannelID != "chana" || s.Status != "connected" {
			t.Fatalf("status = %+v, want channel_id=chana status=connected", s)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected a republish after throttle expiry, timed out")
	}
}

// A nil status publisher must not panic and must not block message handling (the heartbeat is an
// optimisation layered on top of delivery).
func TestHandleChatMessage_NilStatusPublisherIsSafe(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	pub := publisher.NewStreamPublisher(rc, zap.NewNop())
	t.Cleanup(pub.Stop)

	h := NewHandler("secret", rc, nil, pub, nil, nil /* no status publisher */, nil, nil, zap.NewNop())
	if err := h.handleChatMessage(context.Background(), chatEventJSON(t, "chan", "1", "m1")); err != nil {
		t.Fatalf("handleChatMessage with nil status publisher: %v", err)
	}
}
