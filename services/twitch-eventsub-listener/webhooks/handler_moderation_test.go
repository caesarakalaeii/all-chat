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
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// heldMessageID is the Twitch message id shared by the automod.message.hold payload and the
// automod.message.update payload below. The two tests assert the same value because held_message_id
// is the join key the frontend uses to update a held row in place when it is resolved.
const heldMessageID = "1a2b3c4d-0000-4000-8000-0123456789ab"

// moderateTimeoutPayload is Twitch's channel.moderate (v2) reference payload for a timeout, with an
// absolute expires_at as Twitch sends it. The expiry is computed at call time so ban_duration stays
// positive however long the test binary has been running.
func moderateTimeoutPayload(expiresAt time.Time) string {
	return fmt.Sprintf(`{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "Streamer",
		"broadcaster_user_name": "Streamer",
		"moderator_user_id": "1339",
		"moderator_user_login": "ModPerson",
		"moderator_user_name": "ModPerson",
		"action": "timeout",
		"timeout": {
			"user_id": "9001",
			"user_login": "Spammer",
			"user_name": "Spammer",
			"reason": "stop that",
			"expires_at": "%s"
		}
	}`, expiresAt.Format(time.RFC3339Nano))
}

const automodHoldPayload = `{
	"broadcaster_user_id": "1971641",
	"broadcaster_user_login": "Streamer",
	"broadcaster_user_name": "Streamer",
	"user_id": "9001",
	"user_login": "Spammer",
	"user_name": "Spammer",
	"message_id": "` + heldMessageID + `",
	"message": {
		"text": "a message automod did not like",
		"fragments": [{"type": "text", "text": "a message automod did not like"}]
	},
	"category": "aggression",
	"level": 4,
	"held_at": "2024-11-04T21:00:00.000000000Z"
}`

const automodApprovedPayload = `{
	"broadcaster_user_id": "1971641",
	"broadcaster_user_login": "Streamer",
	"broadcaster_user_name": "Streamer",
	"user_id": "9001",
	"user_login": "Spammer",
	"user_name": "Spammer",
	"moderator_user_id": "1339",
	"moderator_user_login": "ModPerson",
	"moderator_user_name": "ModPerson",
	"message_id": "` + heldMessageID + `",
	"message": {
		"text": "a message automod did not like",
		"fragments": [{"type": "text", "text": "a message automod did not like"}]
	},
	"status": "approved",
	"held_at": "2024-11-04T21:00:00.000000000Z"
}`

// routeModerationEvent runs one raw Twitch payload through routeEvent and returns the RawChatMessage
// the handler published to chat:raw.
func routeModerationEvent(t *testing.T, subscriptionType, payload string) map[string]interface{} {
	t.Helper()
	mr, h := newClaimTestHandler(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	if err := h.routeEvent(context.Background(), subscriptionType, json.RawMessage(payload), ""); err != nil {
		t.Fatalf("routeEvent(%s): %v", subscriptionType, err)
	}
	rm := firstStreamRawMessage(t, rc)
	if rm.EventType != "mod_action" {
		t.Fatalf("EventType = %q, want mod_action for every moderation/AutoMod event", rm.EventType)
	}
	if rm.Text == "" {
		t.Error("Text must carry a human-readable summary; an empty text trips the empty-text metric")
	}
	if rm.ChannelID != "streamer" {
		t.Errorf("ChannelID = %q, want the lower-cased broadcaster login streamer", rm.ChannelID)
	}
	return rm.EventData
}

// A timeout must arrive with the ACTING MODERATOR — the one thing the existing deletion path cannot
// provide, and the reason channel.moderate is subscribed at all.
func TestRouteEvent_ChannelModerateTimeout(t *testing.T) {
	expiresAt := time.Now().UTC().Add(600 * time.Second)
	data := routeModerationEvent(t, "channel.moderate", moderateTimeoutPayload(expiresAt))

	if data["action"] != "timeout" {
		t.Errorf("action = %v, want timeout", data["action"])
	}
	if data["moderator_login"] != "modperson" {
		t.Errorf("moderator_login = %v, want modperson", data["moderator_login"])
	}
	if data["moderator_id"] != "1339" {
		t.Errorf("moderator_id = %v, want 1339", data["moderator_id"])
	}
	if data["target_login"] != "spammer" {
		t.Errorf("target_login = %v, want spammer", data["target_login"])
	}
	if data["target_user_id"] != "9001" {
		t.Errorf("target_user_id = %v, want 9001", data["target_user_id"])
	}
	if data["reason"] != "stop that" {
		t.Errorf("reason = %v, want %q", data["reason"], "stop that")
	}
	// Twitch sends an absolute expires_at; the frontend renders a duration, so the handler converts.
	duration, ok := data["ban_duration"].(int)
	if !ok {
		t.Fatalf("ban_duration = %#v, want an int number of seconds", data["ban_duration"])
	}
	if duration <= 0 {
		t.Errorf("ban_duration = %d, want a positive number of seconds", duration)
	}
}

// Twitch adds moderation actions over time. An action this code has never seen must still reach the
// overlay as an unrecognised-but-visible row rather than be dropped by a whitelist.
func TestRouteEvent_ChannelModerateUnknownActionPassesThrough(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "Streamer",
		"broadcaster_user_name": "Streamer",
		"moderator_user_id": "1339",
		"moderator_user_login": "ModPerson",
		"moderator_user_name": "ModPerson",
		"action": "future_action"
	}`

	data := routeModerationEvent(t, "channel.moderate", payload)

	if data["action"] != "future_action" {
		t.Errorf("action = %v, want the verbatim future_action", data["action"])
	}
	if data["moderator_login"] != "modperson" {
		t.Errorf("moderator_login = %v, want modperson", data["moderator_login"])
	}
}

func TestRouteEvent_AutoModHold(t *testing.T) {
	data := routeModerationEvent(t, "automod.message.hold", automodHoldPayload)

	if data["action"] != "automod_hold" {
		t.Errorf("action = %v, want automod_hold", data["action"])
	}
	if data["held_text"] != "a message automod did not like" {
		t.Errorf("held_text = %v, want the held message text", data["held_text"])
	}
	if data["automod_category"] != "aggression" {
		t.Errorf("automod_category = %v, want aggression", data["automod_category"])
	}
	if data["automod_level"] != 4 {
		t.Errorf("automod_level = %#v, want int 4", data["automod_level"])
	}
	if data["held_message_id"] != heldMessageID {
		t.Errorf("held_message_id = %v, want %q", data["held_message_id"], heldMessageID)
	}
	if data["target_login"] != "spammer" {
		t.Errorf("target_login = %v, want spammer", data["target_login"])
	}
	// AutoMod has no human actor. A placeholder moderator here would later read as a real one.
	if _, ok := data["moderator_login"]; ok {
		t.Error("moderator_login must be absent for automod_hold — AutoMod is not a moderator")
	}
	if _, ok := data["moderator_id"]; ok {
		t.Error("moderator_id must be absent for automod_hold — AutoMod is not a moderator")
	}
}

func TestRouteEvent_AutoModUpdateApproved(t *testing.T) {
	data := routeModerationEvent(t, "automod.message.update", automodApprovedPayload)

	if data["action"] != "automod_resolved" {
		t.Errorf("action = %v, want automod_resolved", data["action"])
	}
	if data["resolution"] != "approved" {
		t.Errorf("resolution = %v, want approved", data["resolution"])
	}
	if data["resolved_by"] != "modperson" {
		t.Errorf("resolved_by = %v, want modperson", data["resolved_by"])
	}
	if data["moderator_login"] != "modperson" {
		t.Errorf("moderator_login = %v, want modperson", data["moderator_login"])
	}
	// The hold and its resolution must agree on held_message_id or the frontend cannot pair them.
	if data["held_message_id"] != heldMessageID {
		t.Errorf("held_message_id = %v, want %q — a mismatch means a resolved hold never updates in place",
			data["held_message_id"], heldMessageID)
	}
}
