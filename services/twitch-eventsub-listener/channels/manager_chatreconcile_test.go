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

package channels

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// reconcileChatSubscriptions re-asserts the subscription set for chat-active channels. It exists
// because reconcileChatLocked only fires on the want↔ChatActive transition, so a subscription that
// was revoked or failed to create while chat stayed active was never retried — silently killing the
// chat-notice feed for the pod's lifetime (ADR-0046).
func TestReconcileChatSubscriptions_EnsuresOnlyChatActiveChannels(t *testing.T) {
	cb := &recordingCallback{}

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetLeaderFunc(func() bool { return true })

	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "serving", ChatActive: true}
	m.channels["222"] = &Channel{BroadcasterID: "222", BroadcasterName: "nodemand", ChatActive: false}

	m.reconcileChatSubscriptions(context.Background())

	got := cb.snapshot()
	if len(got) != 1 || got[0] != "ensure_chat:111" {
		t.Fatalf("calls = %v, want exactly [ensure_chat:111] — a channel with no chat subscription has nothing to repair", got)
	}
}

// Only the pod holding the subscriptions may recreate them; a standby recreating subscriptions would
// duplicate the leader's and leak them.
func TestReconcileChatSubscriptions_StandbyDoesNothing(t *testing.T) {
	cb := &recordingCallback{}

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetLeaderFunc(func() bool { return false })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "serving", ChatActive: true}

	m.reconcileChatSubscriptions(context.Background())

	if got := cb.snapshot(); len(got) != 0 {
		t.Fatalf("standby issued %v, want no subscription calls", got)
	}
}

// A failing channel must not stop the others from being repaired, and must not propagate an error
// that would look like a sync failure.
func TestReconcileChatSubscriptions_ContinuesPastFailures(t *testing.T) {
	cb := &recordingCallback{err: errors.New("twitch 500")}

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetLeaderFunc(func() bool { return true })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "a", ChatActive: true}
	m.channels["222"] = &Channel{BroadcasterID: "222", BroadcasterName: "b", ChatActive: true}
	m.channels["333"] = &Channel{BroadcasterID: "333", BroadcasterName: "c", ChatActive: true}

	m.reconcileChatSubscriptions(context.Background())

	if got := cb.snapshot(); len(got) != 3 {
		t.Fatalf("attempted %d channels, want all 3 even though every call failed", len(got))
	}
}

// A cancelled context must stop the pass promptly rather than walking every channel during shutdown.
func TestReconcileChatSubscriptions_StopsOnCancelledContext(t *testing.T) {
	cb := &recordingCallback{}

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetLeaderFunc(func() bool { return true })
	for _, id := range []string{"111", "222", "333"} {
		m.channels[id] = &Channel{BroadcasterID: id, BroadcasterName: "ch" + id, ChatActive: true}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.reconcileChatSubscriptions(ctx)

	if got := cb.snapshot(); len(got) != 0 {
		t.Fatalf("issued %v after cancellation, want none", got)
	}
}

// No callback wired (or no channels) must be a safe no-op, not a nil-deref.
func TestReconcileChatSubscriptions_NilSafe(t *testing.T) {
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetLeaderFunc(func() bool { return true })
	m.channels["111"] = &Channel{BroadcasterID: "111", ChatActive: true}

	m.reconcileChatSubscriptions(context.Background()) // no callback set

	cb := &recordingCallback{}
	m2 := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m2.SetSubscriptionCallback(cb.fn)
	m2.SetLeaderFunc(func() bool { return true })
	m2.reconcileChatSubscriptions(context.Background()) // no channels

	if got := cb.snapshot(); len(got) != 0 {
		t.Fatalf("expected no calls with zero channels, got %v", got)
	}
}

// The repair interval must default to the documented constant, and a non-positive override must be
// ignored so a misconfiguration can never turn the repair pass into a busy loop hammering Twitch.
func TestSetChatReconcileInterval(t *testing.T) {
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	if m.chatReconcileInterval != ChatSubscriptionReconcileInterval {
		t.Fatalf("default = %v, want %v", m.chatReconcileInterval, ChatSubscriptionReconcileInterval)
	}

	m.SetChatReconcileInterval(30 * time.Second)
	if m.chatReconcileInterval != 30*time.Second {
		t.Fatalf("override = %v, want 30s", m.chatReconcileInterval)
	}

	for _, bad := range []time.Duration{0, -time.Second} {
		m.SetChatReconcileInterval(bad)
		if m.chatReconcileInterval != 30*time.Second {
			t.Fatalf("interval %v was accepted; non-positive values must be ignored", bad)
		}
	}
}

// TestReconcileChatSubscriptions_ScopeFailureDeactivatesChat pins the mid-life revocation
// contract (council finding, the incident's own shape): a channel that was legitimately
// ChatActive whose grant died underneath it gets the sentinel from ensure_chat. The repair
// pass must flip ChatActive=false — so refreshClaims and heartbeatActiveSources stop
// re-asserting liveness for a channel whose subscription cannot exist — release the
// ownership claim, and publish the re-auth-hinted offline status the frontend renders as
// the red Auth Required indicator.
func TestReconcileChatSubscriptions_ScopeFailureDeactivatesChat(t *testing.T) {
	mr, claims := newClaimTestStore(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	sub := rc.Subscribe(context.Background(), status.PlatformStatusChannel)
	defer sub.Close()
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	msgs := sub.Channel()

	// Seed a live claim as if the channel had been served by EventSub.
	if err := claims.Claim(context.Background(), "revokedchan", "111"); err != nil {
		t.Fatal(err)
	}

	cb := &recordingCallback{err: fmt.Errorf("broadcaster 111: %w", ErrChatScopesMissing)}
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetClaimStore(claims)
	m.SetStatusPublisher(status.NewPublisher(rc, zap.NewNop()))
	m.SetLeaderFunc(func() bool { return true })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "revokedchan", ChatActive: true}

	m.reconcileChatSubscriptions(context.Background())

	if m.channels["111"].ChatActive {
		t.Fatal("a scope-failed channel must be deactivated: refreshClaims and heartbeat would keep re-asserting liveness for a dead subscription")
	}
	if mr.Exists(twitchchat.ClaimKey("revokedchan")) {
		t.Fatal("the ownership claim must be released so the dead channel stops holding IRC off (enforce mode, ADR-0026)")
	}
	select {
	case msg := <-msgs:
		if !strings.Contains(msg.Payload, errChatScopesMissingMessage) {
			t.Fatalf("published status %q does not carry the re-auth hint %q", msg.Payload, errChatScopesMissingMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the re-auth-hinted offline status after a mid-life scope failure")
	}
}

// A transient failure on the repair pass must NOT deactivate the channel: chat itself keeps
// flowing, the next pass retries, and telling the streamer to re-auth over a Twitch 5xx
// would turn a platform hiccup into support tickets.
func TestReconcileChatSubscriptions_TransientFailureKeepsChatActive(t *testing.T) {
	cb := &recordingCallback{err: errors.New("twitch api: 503 service unavailable")}
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetLeaderFunc(func() bool { return true })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "flaky", ChatActive: true}

	m.reconcileChatSubscriptions(context.Background())

	if !m.channels["111"].ChatActive {
		t.Fatal("a transient failure must keep the channel ChatActive — chat still flows and the next pass retries")
	}
}
