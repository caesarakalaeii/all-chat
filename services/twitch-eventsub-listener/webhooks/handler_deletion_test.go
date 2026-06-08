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

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/redis/go-redis/v9"
)

// firstStreamRawMessage decodes the first entry the publisher wrote to chat:raw. The ring-buffer
// publisher writes synchronously on success, so the entry is present once the handler returns.
func firstStreamRawMessage(t *testing.T, rc *redis.Client) models.RawChatMessage {
	t.Helper()
	res, err := rc.XRange(context.Background(), "chat:raw", "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange chat:raw: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("no message published to chat:raw")
	}
	data, _ := res[0].Values["data"].(string)
	var rm models.RawChatMessage
	if err := json.Unmarshal([]byte(data), &rm); err != nil {
		t.Fatalf("unmarshal RawChatMessage: %v", err)
	}
	return rm
}

// --- pure builders: exact shape matches twitch-listener's CLEARMSG/CLEARCHAT parsing ---

func TestBuildSingleDeletion(t *testing.T) {
	got := buildSingleDeletion(&eventsub.ChatMessageDeleteEvent{
		BroadcasterUserLogin: "CaesarLP",
		TargetUserLogin:      "Spammer",
		MessageID:            "native-123",
	})
	if got == nil {
		t.Fatal("buildSingleDeletion returned nil for a valid event")
	}
	if got.EventType != "message_deletion" {
		t.Errorf("EventType = %q, want message_deletion", got.EventType)
	}
	if got.Platform != "twitch" {
		t.Errorf("Platform = %q, want twitch", got.Platform)
	}
	if got.ChannelID != "caesarlp" {
		t.Errorf("ChannelID = %q, want lowercased login caesarlp", got.ChannelID)
	}
	if got.Username != "spammer" {
		t.Errorf("Username = %q, want lowercased target spammer", got.Username)
	}
	if got.EventData["deletion_type"] != "single" {
		t.Errorf("deletion_type = %v, want single", got.EventData["deletion_type"])
	}
	if got.EventData["target_msg_id"] != "native-123" {
		t.Errorf("target_msg_id = %v, want native-123", got.EventData["target_msg_id"])
	}
	if got.MessageID == "" {
		t.Error("deletion event must carry its own MessageID (UUID)")
	}
}

func TestBuildSingleDeletion_MissingFieldsReturnsNil(t *testing.T) {
	if buildSingleDeletion(&eventsub.ChatMessageDeleteEvent{BroadcasterUserLogin: "c"}) != nil {
		t.Error("missing message_id must return nil")
	}
	if buildSingleDeletion(&eventsub.ChatMessageDeleteEvent{MessageID: "m"}) != nil {
		t.Error("missing broadcaster login must return nil")
	}
}

func TestBuildBatchDeletion(t *testing.T) {
	got := buildBatchDeletion(&eventsub.ChatClearUserMessagesEvent{
		BroadcasterUserLogin: "CaesarLP",
		TargetUserID:         "u123",
		TargetUserLogin:      "Spammer",
	})
	if got == nil {
		t.Fatal("buildBatchDeletion returned nil for a valid event")
	}
	if got.ChannelID != "caesarlp" {
		t.Errorf("ChannelID = %q, want caesarlp", got.ChannelID)
	}
	if got.UserID != "u123" {
		t.Errorf("UserID = %q, want u123", got.UserID)
	}
	if got.EventData["deletion_type"] != "batch" {
		t.Errorf("deletion_type = %v, want batch", got.EventData["deletion_type"])
	}
	if got.EventData["target_user_id"] != "u123" {
		t.Errorf("target_user_id = %v, want u123", got.EventData["target_user_id"])
	}
	if got.EventData["target_username"] != "spammer" {
		t.Errorf("target_username = %v, want spammer", got.EventData["target_username"])
	}
	// Twitch omits the duration on clear_user_messages; the key must be ABSENT so downstream reads
	// it as a ban (a present ban_duration would falsely signal a timeout).
	if _, ok := got.EventData["ban_duration"]; ok {
		t.Error("ban_duration must be absent — EventSub does not provide it for clear_user_messages")
	}
}

func TestBuildBatchDeletion_MissingFieldsReturnsNil(t *testing.T) {
	if buildBatchDeletion(&eventsub.ChatClearUserMessagesEvent{BroadcasterUserLogin: "c"}) != nil {
		t.Error("missing target_user_id must return nil")
	}
	if buildBatchDeletion(&eventsub.ChatClearUserMessagesEvent{TargetUserID: "u"}) != nil {
		t.Error("missing broadcaster login must return nil")
	}
}

func TestBuildClearDeletion(t *testing.T) {
	got := buildClearDeletion(&eventsub.ChatClearEvent{BroadcasterUserLogin: "CaesarLP"})
	if got == nil {
		t.Fatal("buildClearDeletion returned nil for a valid event")
	}
	if got.ChannelID != "caesarlp" {
		t.Errorf("ChannelID = %q, want caesarlp", got.ChannelID)
	}
	if got.EventData["deletion_type"] != "clear" {
		t.Errorf("deletion_type = %v, want clear", got.EventData["deletion_type"])
	}
	if buildClearDeletion(&eventsub.ChatClearEvent{}) != nil {
		t.Error("missing broadcaster login must return nil")
	}
}

// --- routing + publish: routeEvent dispatches each deletion type onto chat:raw ---

func TestRouteEvent_SingleDeletionPublished(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ev, _ := json.Marshal(eventsub.ChatMessageDeleteEvent{
		BroadcasterUserLogin: "CaesarLP",
		TargetUserLogin:      "spammer",
		MessageID:            "native-123",
	})
	if err := h.routeEvent(context.Background(), "channel.chat.message_delete", ev); err != nil {
		t.Fatalf("routeEvent: %v", err)
	}

	rm := firstStreamRawMessage(t, rc)
	if rm.EventType != "message_deletion" || rm.EventData["deletion_type"] != "single" {
		t.Fatalf("published event = %q/%v, want message_deletion/single", rm.EventType, rm.EventData["deletion_type"])
	}
	if rm.EventData["target_msg_id"] != "native-123" {
		t.Errorf("target_msg_id = %v, want native-123", rm.EventData["target_msg_id"])
	}
}

func TestRouteEvent_BatchDeletionPublished(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ev, _ := json.Marshal(eventsub.ChatClearUserMessagesEvent{
		BroadcasterUserLogin: "CaesarLP",
		TargetUserID:         "u123",
		TargetUserLogin:      "spammer",
	})
	if err := h.routeEvent(context.Background(), "channel.chat.clear_user_messages", ev); err != nil {
		t.Fatalf("routeEvent: %v", err)
	}

	rm := firstStreamRawMessage(t, rc)
	if rm.EventData["deletion_type"] != "batch" || rm.EventData["target_user_id"] != "u123" {
		t.Fatalf("published event = %v/%v, want batch/u123", rm.EventData["deletion_type"], rm.EventData["target_user_id"])
	}
}

func TestRouteEvent_ClearDeletionPublished(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ev, _ := json.Marshal(eventsub.ChatClearEvent{BroadcasterUserLogin: "CaesarLP"})
	if err := h.routeEvent(context.Background(), "channel.chat.clear", ev); err != nil {
		t.Fatalf("routeEvent: %v", err)
	}

	rm := firstStreamRawMessage(t, rc)
	if rm.EventData["deletion_type"] != "clear" {
		t.Fatalf("deletion_type = %v, want clear", rm.EventData["deletion_type"])
	}
	if rm.ChannelID != "caesarlp" {
		t.Errorf("ChannelID = %q, want caesarlp", rm.ChannelID)
	}
}

// A delivered chat message must register native-id → internal-UUID, and the registered UUID must
// equal the published message's UUID — otherwise a later single-message delete resolves to the
// wrong (or no) overlay message. This is the registration the IRC listener does at capture and the
// EventSub listener previously omitted.
func TestHandleChatMessage_RegistersNativeIDForSingleDelete(t *testing.T) {
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	ctx := context.Background()

	if err := h.handleChatMessage(ctx, chatEventJSON(t, "CaesarLP", "67241623", "native-abc")); err != nil {
		t.Fatalf("handleChatMessage: %v", err)
	}

	// Registry resolves the native id to the internal UUID, keyed by the lower-cased login.
	internalUUID, err := h.registry.Lookup(ctx, "twitch", "caesarlp", "native-abc")
	if err != nil {
		t.Fatalf("expected registry mapping for native-abc: %v", err)
	}

	// The registered UUID must be the UUID actually delivered to the overlay.
	rm := firstStreamRawMessage(t, rc)
	if rm.MessageID != internalUUID {
		t.Fatalf("registry UUID %q != published MessageID %q — a single delete would miss the message",
			internalUUID, rm.MessageID)
	}
}
