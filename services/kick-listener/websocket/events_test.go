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

package websocket

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

// Payloads below were captured from Kick's Centrifugo feed on 2026-11-20
// (usernames shortened). Kick sends them as JSON-encoded strings.
const (
	subPayload  = `{"chatroom_id":4338066,"username":"barbecue","months":14}`
	hostPayload = `{"chatroom_id":17627848,"optional_message":null,"number_viewers":12,"host_username":"IBrahem"}`
	giftPayload = `{"chatroom_id":4338066,"correlation_id":"pi_3Udz","gifted_usernames":["a","b","c"],"gifter_username":"Dr_M","gifted_total":50,"gifter_total":651,"chunk_details":null}`
	// chatroom_<id> also carries this differently shaped sibling of the chat
	// event; it must never reach the chat handler.
	infoPayload = `{"message":{"id":"f9c5","message":"","type":"info","chatroom_id":4338066,"action":"subscribe","months_subscribed":14},"user":{"id":38497319,"username":"barbecue"}}`
)

func quoted(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return b
}

func TestParseKickEvent(t *testing.T) {
	tests := []struct {
		name, event, channel, payload string
		want                          *KickEvent
	}{
		{"subscription", kickSubscriptionEvent, "chatrooms.4338066.v2", subPayload,
			&KickEvent{Type: "kick_subscription", ChatroomID: 4338066, Username: "barbecue", Data: map[string]any{"months": 14}}},
		{"host", kickStreamHostEvent, "chatrooms.17627848.v2", hostPayload,
			&KickEvent{Type: "kick_host", ChatroomID: 17627848, Username: "IBrahem", Data: map[string]any{"viewers": 12}}},
		{"gifted subs count the whole gift, not the names in this frame", kickGiftedSubsEvent, "chatroom_4338066", giftPayload,
			&KickEvent{Type: "kick_gift_subscription", ChatroomID: 4338066, Username: "Dr_M", ID: "kick-gift-pi_3Udz", Data: map[string]any{"gift_count": 50}}},
		// Kick mirrors events across its chatroom aliases; each is read from one.
		{"subscription on the gift alias is ignored", kickSubscriptionEvent, "chatroom_4338066", subPayload, nil},
		{"gifted subs on the chat alias are ignored", kickGiftedSubsEvent, "chatrooms.4338066.v2", giftPayload, nil},
		{"later chunks of one gift are ignored", kickGiftedSubsEvent, "chatroom_1",
			`{"chatroom_id":1,"correlation_id":"x","gifted_usernames":["a"],"gifter_username":"g","gifted_total":150,"chunk_details":{"correlation_id":"x","chunk_index":1,"total_chunks":2}}`, nil},
		{"unknown event", "App\\Events\\PollUpdateEvent", "chatrooms.1.v2", `{}`, nil},
		{"subscription without a name", kickSubscriptionEvent, "chatrooms.1.v2", `{"chatroom_id":1,"months":1}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseKickEvent(tt.event, tt.channel, quoted(tt.payload))
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			if got == nil {
				return
			}
			if got.Type != tt.want.Type || got.ChatroomID != tt.want.ChatroomID || got.Username != tt.want.Username {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
			if tt.want.ID != "" && got.ID != tt.want.ID {
				t.Errorf("id = %q, want %q", got.ID, tt.want.ID)
			}
			for k, v := range tt.want.Data {
				if got.Data[k] != v {
					t.Errorf("data[%s] = %v, want %v", k, got.Data[k], v)
				}
			}
		})
	}
}

func TestDispatchKeepsTheGiftAliasOutOfChat(t *testing.T) {
	var chats, events int
	onMessage := func(string, *KickChatMessage) { chats++ }
	onEvent := func(*KickEvent) { events++ }
	dispatchKickEvent(zap.NewNop(), kickChatMessageEvent, "chatroom_4338066", quoted(infoPayload), onMessage, nil, onEvent)
	dispatchKickEvent(zap.NewNop(), kickGiftedSubsEvent, "chatroom_4338066", quoted(giftPayload), onMessage, nil, onEvent)
	dispatchKickEvent(zap.NewNop(), kickSubscriptionEvent, "chatrooms.4338066.v2", quoted(subPayload), onMessage, nil, onEvent)
	if chats != 0 || events != 2 {
		t.Fatalf("chats = %d, events = %d; want 0 and 2", chats, events)
	}
}
