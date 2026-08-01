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
	"testing"
	"time"

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
