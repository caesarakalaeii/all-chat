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
	"testing"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
)

// channel.subscribe and channel.subscription.gift carry no chatter message. The overlay renders
// Text as the chatter's own words next to the value pill, so a synthetic placeholder showed up as
// "Tier 1Subscribed at 1000" / "1 gift subsGifted 1 subs" on stream.
func TestBuildSubscribeHasNoPlaceholderText(t *testing.T) {
	msg := buildSubscribe(&eventsub.SubscribeEvent{
		UserID: "1", UserLogin: "fluxduo", BroadcasterUserLogin: "streamer", Tier: "1000",
	}, "evt-1")
	if msg.Text != "" {
		t.Errorf("Text = %q, want empty", msg.Text)
	}
	if msg.EventType != "subscription" || msg.EventData["tier"] != "1000" || msg.EventData["eventsub_message_id"] != "evt-1" {
		t.Errorf("got type %q data %v", msg.EventType, msg.EventData)
	}
}

func TestBuildSubscriptionGiftHasNoPlaceholderText(t *testing.T) {
	msg := buildSubscriptionGift(&eventsub.SubscriptionGiftEvent{
		UserID: "1", UserLogin: "gifter", BroadcasterUserLogin: "streamer", Tier: "1000", Total: 1,
	}, "evt-2")
	if msg.Text != "" {
		t.Errorf("Text = %q, want empty", msg.Text)
	}
	if msg.EventType != "mystery_gift" || msg.EventData["total"] != 1 {
		t.Errorf("got type %q data %v", msg.EventType, msg.EventData)
	}
}
