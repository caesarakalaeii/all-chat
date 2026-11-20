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
	"fmt"
	"strings"
	"time"
)

// Kick's platform events, as its realtime feed names them. Seen live on
// 2026-11-20 (Centrifugo): subscriptions and hosts on chatrooms.<id>.v2,
// gifted subscriptions only on chatroom_<id>. Follows carry no username on any
// public channel, and KICKs gifts sit on channel_<channel id>, which this
// listener does not read.
const (
	kickSubscriptionEvent = "App\\Events\\SubscriptionEvent"
	kickStreamHostEvent   = "App\\Events\\StreamHostEvent"
	kickGiftedSubsEvent   = "GiftedSubscriptionsEvent"

	// kickGiftChannelFormat is the chatroom alias that carries gifted subs.
	kickGiftChannelFormat = "chatroom_%d"
	kickGiftChannelPrefix = "chatroom_"
)

// KickEvent is a Kick platform event flattened to what the pipeline needs.
// Type and the Data keys are the contract with the message-processor's
// KickNormalizer.NormalizeEvent.
type KickEvent struct {
	Type       string // kick_subscription, kick_gift_subscription, kick_host
	ID         string
	ChatroomID int
	Username   string // who subscribed, gifted or hosted
	Text       string // the host's optional message
	Data       map[string]any
}

// EventHandler is called for each Kick platform event.
type EventHandler func(event *KickEvent)

// parseKickEvent returns the platform event in a realtime frame, or nil when
// the frame is not one, comes from the wrong chatroom alias, or names nobody.
func parseKickEvent(event, channel string, data json.RawMessage) *KickEvent {
	giftAlias := strings.HasPrefix(channel, kickGiftChannelPrefix)
	var p struct {
		ChatroomID      int      `json:"chatroom_id"`
		Username        string   `json:"username"`
		Months          int      `json:"months"`
		HostUsername    string   `json:"host_username"`
		NumberViewers   int      `json:"number_viewers"`
		OptionalMessage *string  `json:"optional_message"`
		GifterUsername  string   `json:"gifter_username"`
		GiftedUsernames []string `json:"gifted_usernames"`
		GiftedTotal     int      `json:"gifted_total"`
		CorrelationID   string   `json:"correlation_id"`
		ChunkDetails    *struct {
			ChunkIndex int `json:"chunk_index"`
		} `json:"chunk_details"`
	}
	known := event == kickSubscriptionEvent || event == kickStreamHostEvent || event == kickGiftedSubsEvent
	if !known || giftAlias != (event == kickGiftedSubsEvent) || decodeEventData(data, &p) != nil {
		return nil
	}

	out := &KickEvent{ChatroomID: p.ChatroomID}
	switch event {
	case kickSubscriptionEvent:
		out.Type, out.Username = "kick_subscription", p.Username
		out.Data = map[string]any{"months": p.Months}
	case kickStreamHostEvent:
		out.Type, out.Username = "kick_host", p.HostUsername
		out.Data = map[string]any{"viewers": p.NumberViewers}
		if p.OptionalMessage != nil {
			out.Text = *p.OptionalMessage
		}
	case kickGiftedSubsEvent:
		// A large gift is split across frames; the first carries the total.
		if p.ChunkDetails != nil && p.ChunkDetails.ChunkIndex > 0 {
			return nil
		}
		count := max(p.GiftedTotal, len(p.GiftedUsernames))
		out.Type, out.Username = "kick_gift_subscription", p.GifterUsername
		out.Data = map[string]any{"gift_count": count}
		if p.CorrelationID != "" {
			out.ID = "kick-gift-" + p.CorrelationID
		}
	}
	if out.Username == "" || out.ChatroomID == 0 {
		return nil
	}
	if out.ID == "" {
		out.ID = fmt.Sprintf("%s-%d-%s-%d", out.Type, out.ChatroomID, out.Username, time.Now().UnixNano())
	}
	return out
}
