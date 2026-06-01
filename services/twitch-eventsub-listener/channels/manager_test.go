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
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// recordingCallback records the (broadcasterID, action) pairs the manager invokes, in order.
type recordingCallback struct {
	mu      sync.Mutex
	actions []string // "action:broadcasterID"
}

func (r *recordingCallback) fn(broadcasterID, _ string, action string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions = append(r.actions, action+":"+broadcasterID)
	return nil
}

func (r *recordingCallback) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.actions))
	copy(out, r.actions)
	return out
}

func demandFor(sourceIDs ...string) map[string]listener.DemandedSource {
	d := make(map[string]listener.DemandedSource, len(sourceIDs))
	for _, s := range sourceIDs {
		d[s] = listener.DemandedSource{SourceID: s}
	}
	return d
}

// TestReconcileChat_SubscribesWhenScopedAndDemanded verifies a chat subscription is created
// only when the channel has the chat scope AND live-overlay demand.
func TestReconcileChat_SubscribesWhenScopedAndDemanded(t *testing.T) {
	cb := &recordingCallback{}
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)

	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "scoped", SourceIDs: []string{"s1"}, HasChatScope: true}
	m.channels["222"] = &Channel{BroadcasterID: "222", BroadcasterName: "noscope", SourceIDs: []string{"s2"}, HasChatScope: false}

	m.mu.Lock()
	m.reconcileChatLocked(demandFor("s1", "s2"))
	m.mu.Unlock()

	got := cb.snapshot()
	if len(got) != 1 || got[0] != "subscribe_chat:111" {
		t.Fatalf("want exactly [subscribe_chat:111], got %v", got)
	}
	if !m.channels["111"].ChatActive {
		t.Fatal("scoped+demanded channel should be marked ChatActive")
	}
	if m.channels["222"].ChatActive {
		t.Fatal("unscoped channel must not become ChatActive")
	}
}

// TestReconcileChat_UnsubscribesAndPublishesOffline verifies that when demand drops, the chat
// subscription is torn down AND an "offline" platform:status is published keyed by the
// lowercased login, so the overlay indicator clears.
func TestReconcileChat_UnsubscribesAndPublishesOffline(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	// Subscribe before triggering so we don't miss the message.
	sub := rc.Subscribe(context.Background(), status.PlatformStatusChannel)
	defer sub.Close()
	if _, err := sub.Receive(context.Background()); err != nil {
		t.Fatal(err)
	}
	msgs := sub.Channel()

	cb := &recordingCallback{}
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetStatusPublisher(status.NewPublisher(rc, zap.NewNop()))

	// Channel currently reading chat ("CaesarLP" mixed-case to prove lowercasing) but demand gone.
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "CaesarLP", SourceIDs: []string{"s1"}, HasChatScope: true, ChatActive: true}

	// An explicit empty (non-nil) demand map means "no demand" (nil would fail open).
	m.mu.Lock()
	m.reconcileChatLocked(map[string]listener.DemandedSource{})
	m.mu.Unlock()

	if got := cb.snapshot(); len(got) != 1 || got[0] != "unsubscribe_chat:111" {
		t.Fatalf("want exactly [unsubscribe_chat:111], got %v", got)
	}
	if m.channels["111"].ChatActive {
		t.Fatal("channel should no longer be ChatActive after teardown")
	}

	select {
	case msg := <-msgs:
		var sm status.Message
		if err := json.Unmarshal([]byte(msg.Payload), &sm); err != nil {
			t.Fatalf("bad status payload: %v", err)
		}
		if sm.Platform != "twitch" || sm.ChannelID != "caesarlp" || sm.Status != "offline" {
			t.Fatalf("unexpected status: %+v", sm)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected an offline platform:status publish, got none")
	}
}

// countingResolver counts GetUserIDByLogin calls to verify caching.
type countingResolver struct {
	calls atomic.Int64
	id    string
	err   error
}

func (r *countingResolver) GetUserIDByLogin(_ context.Context, _ string) (string, error) {
	r.calls.Add(1)
	return r.id, r.err
}

// TestResolveBroadcasterID_CachesPositive verifies a successful login→id resolution is
// cached, so repeated syncs don't re-hit the Twitch API for the same channel.
func TestResolveBroadcasterID_CachesPositive(t *testing.T) {
	r := &countingResolver{id: "12345"}
	m := NewManager(nil, zap.NewNop(), r, nil, time.Minute)

	for i := 0; i < 3; i++ {
		id, err := m.resolveBroadcasterID(context.Background(), "caesarlp")
		if err != nil || id != "12345" {
			t.Fatalf("resolve %d: id=%q err=%v", i, id, err)
		}
	}
	if got := r.calls.Load(); got != 1 {
		t.Fatalf("resolver called %d times, want 1 (positive cache)", got)
	}
}

// TestResolveBroadcasterID_NegativeCache verifies an unresolvable login (deleted account)
// is negatively cached, so it isn't re-resolved every sync.
func TestResolveBroadcasterID_NegativeCache(t *testing.T) {
	r := &countingResolver{err: errors.New("user not found")}
	m := NewManager(nil, zap.NewNop(), r, nil, time.Minute)

	for i := 0; i < 3; i++ {
		if _, err := m.resolveBroadcasterID(context.Background(), "deleteduser"); err == nil {
			t.Fatalf("expected error on resolve %d", i)
		}
	}
	if got := r.calls.Load(); got != 1 {
		t.Fatalf("resolver called %d times, want 1 (negative cache)", got)
	}
}
