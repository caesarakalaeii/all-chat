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
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// newClaimTestHandler builds a Handler wired to an in-memory Redis with a chat-ownership claim
// store. db/metrics/status are nil — the chat-message path does not touch them.
func newClaimTestHandler(t *testing.T) (*miniredis.Miniredis, *Handler) {
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

	reg := registry.NewRedisRegistry(rc, time.Hour)
	h := NewHandler("secret", rc, nil, pub, nil, nil, twitchchat.NewClaimStore(rc), reg, zap.NewNop())
	return mr, h
}

func chatEventJSON(t *testing.T, login, broadcasterID, msgID string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(eventsub.ChatMessageEvent{
		BroadcasterUserID:    broadcasterID,
		BroadcasterUserLogin: login,
		ChatterUserID:        "999",
		ChatterUserLogin:     "viewer",
		MessageID:            msgID,
		Message:              eventsub.ChatMessageBody{Text: "hello"},
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return b
}

// A delivered chat message must create the chat-ownership claim so the IRC listener excludes the
// channel (ADR-0015). The claim key is the lower-cased login.
func TestHandleChatMessage_CreatesOwnershipClaim(t *testing.T) {
	mr, h := newClaimTestHandler(t)

	if err := h.handleChatMessage(context.Background(), chatEventJSON(t, "CaesarLP", "67241623", "m1")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}

	val, err := mr.Get("eventsub:chat:owner:caesarlp")
	if err != nil {
		t.Fatalf("expected ownership claim to exist after delivered chat: %v", err)
	}
	if val != "67241623" {
		t.Fatalf("claim value = %q, want broadcaster id 67241623", val)
	}
	if ttl := mr.TTL("eventsub:chat:owner:caesarlp"); ttl <= 0 {
		t.Fatalf("claim must have a TTL so it lapses when EventSub stops delivering; got %v", ttl)
	}
}

// The per-channel refresh is throttled, but a second message for the SAME channel must keep the
// claim alive (idempotent), and a message for a DIFFERENT channel must create its own claim.
func TestHandleChatMessage_ClaimPerChannel(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	ctx := context.Background()

	_ = h.handleChatMessage(ctx, chatEventJSON(t, "chanA", "1", "a1"))
	_ = h.handleChatMessage(ctx, chatEventJSON(t, "chanA", "1", "a2")) // throttled refresh, still claimed
	_ = h.handleChatMessage(ctx, chatEventJSON(t, "chanB", "2", "b1"))

	if !mr.Exists("eventsub:chat:owner:chana") {
		t.Error("chanA should remain claimed after a second (throttled) message")
	}
	if !mr.Exists("eventsub:chat:owner:chanb") {
		t.Error("chanB should be claimed independently")
	}
}

// A nil claim store must not panic and must not block message handling (claims are an optimisation
// layered on top of delivery).
func TestHandleChatMessage_NilClaimStoreIsSafe(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	pub := publisher.NewStreamPublisher(rc, zap.NewNop())
	t.Cleanup(pub.Stop)

	h := NewHandler("secret", rc, nil, pub, nil, nil, nil /* no claim store */, nil /* no registry */, zap.NewNop())
	if err := h.handleChatMessage(context.Background(), chatEventJSON(t, "chan", "1", "m1")); err != nil {
		t.Fatalf("handleChatMessage with nil claim store: %v", err)
	}
}

// A message missing required fields must be dropped without creating a claim.
func TestHandleChatMessage_MissingFieldsNoClaim(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	// Missing MessageID and ChatterUserLogin → handler returns nil and drops it.
	bad := chatEventJSON(t, "lonelychan", "1", "")
	if err := h.handleChatMessage(context.Background(), bad); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}
	if mr.Exists("eventsub:chat:owner:lonelychan") {
		t.Fatal("a dropped (invalid) message must not create an ownership claim")
	}
}
