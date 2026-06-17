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
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func newClaimTestStore(t *testing.T) (*miniredis.Miniredis, *twitchchat.ClaimStore) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return mr, twitchchat.NewClaimStore(rc)
}

// TestRefreshClaims_KeepsActiveChatChannelsClaimed verifies the leader refreshes a claim for every
// channel holding a live chat subscription — independent of chat volume. This is the fix for the
// IRC↔EventSub flapping a low-traffic channel exhibited when its claim was only refreshed on
// delivered chat (ADR-0015).
func TestRefreshClaims_KeepsActiveChatChannelsClaimed(t *testing.T) {
	mr, claims := newClaimTestStore(t)

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetClaimStore(claims)
	m.SetLeaderFunc(func() bool { return true })

	// Mixed-case login proves the claim key is lowercased; an inactive channel proves we only
	// claim channels EventSub is actually serving.
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "CaesarLP", ChatActive: true}
	m.channels["222"] = &Channel{BroadcasterID: "222", BroadcasterName: "quietnochat", ChatActive: false}

	m.refreshClaims(context.Background())

	if !mr.Exists(twitchchat.ClaimKey("caesarlp")) {
		t.Fatal("active chat channel should hold a live claim after refresh")
	}
	if v, err := mr.Get(twitchchat.ClaimKey("caesarlp")); err != nil || v != "111" {
		t.Fatalf("claim value = %q (err %v), want broadcaster id 111", v, err)
	}
	if mr.Exists(twitchchat.ClaimKey("quietnochat")) {
		t.Fatal("channel without an active chat subscription must not be claimed")
	}
}

// TestRefreshClaims_StandbyDoesNotWrite verifies a non-leader never writes claims: only the pod
// that actually owns the subscriptions may keep a channel off IRC.
func TestRefreshClaims_StandbyDoesNotWrite(t *testing.T) {
	mr, claims := newClaimTestStore(t)

	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetClaimStore(claims)
	m.SetLeaderFunc(func() bool { return false })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "caesarlp", ChatActive: true}

	m.refreshClaims(context.Background())

	if mr.Exists(twitchchat.ClaimKey("caesarlp")) {
		t.Fatal("standby pod must not write chat-ownership claims")
	}
}

// TestReconcileChat_ReleasesClaimOnTeardown verifies that when a chat subscription is torn down
// (demand lost) the leader drops the claim immediately so IRC resumes the channel without waiting
// out the TTL.
func TestReconcileChat_ReleasesClaimOnTeardown(t *testing.T) {
	mr, claims := newClaimTestStore(t)

	// Seed a live claim as if the channel had been served by EventSub.
	if err := claims.Claim(context.Background(), "caesarlp", "111"); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists(twitchchat.ClaimKey("caesarlp")) {
		t.Fatal("precondition: claim should exist")
	}

	cb := &recordingCallback{}
	m := NewManager(nil, zap.NewNop(), &countingResolver{}, nil, time.Minute)
	m.SetSubscriptionCallback(cb.fn)
	m.SetClaimStore(claims)
	m.SetLeaderFunc(func() bool { return true })
	m.channels["111"] = &Channel{BroadcasterID: "111", BroadcasterName: "CaesarLP", SourceIDs: []string{"s1"}, HasChatScope: true, ChatActive: true}

	// Empty (non-nil) demand → channel loses demand → chat unsubscribed → claim released.
	m.mu.Lock()
	m.reconcileChatLocked(map[string]listener.DemandedSource{})
	m.mu.Unlock()

	if mr.Exists(twitchchat.ClaimKey("caesarlp")) {
		t.Fatal("claim should be released when the chat subscription is torn down")
	}
}
