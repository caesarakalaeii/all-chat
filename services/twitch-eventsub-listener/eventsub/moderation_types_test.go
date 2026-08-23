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

package eventsub

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// A channel.moderate timeout must decode with the action name, a populated Timeout sub-object and a
// nil Ban — the conversion code branches on exactly that, and a Ban that decoded to a zero value
// instead of nil would make a timeout indistinguishable from a permanent ban.
// Payload is Twitch's own channel.moderate v2 reference example.
func TestChannelModerateTimeoutDecodes(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"moderator_user_id": "1339",
		"moderator_user_login": "modgirl",
		"moderator_user_name": "ModGirl",
		"action": "timeout",
		"followers": null,
		"slow": null,
		"vip": null,
		"unvip": null,
		"mod": null,
		"unmod": null,
		"ban": null,
		"unban": null,
		"timeout": {
			"user_id": "9876",
			"user_login": "rudeviewer",
			"user_name": "RudeViewer",
			"reason": "Does not follow the rules",
			"expires_at": "2022-03-15T02:00:28.17369185Z"
		},
		"untimeout": null,
		"raid": null,
		"unraid": null,
		"delete": null,
		"automod_terms": null,
		"unban_request": null,
		"warn": null
	}`

	var event ChannelModerateEvent
	require.NoError(t, json.Unmarshal([]byte(payload), &event))

	require.Equal(t, "timeout", event.Action)
	require.Equal(t, "modgirl", event.ModeratorUserLogin)
	require.Nil(t, event.Ban, "a timeout must not decode as a ban")
	require.NotNil(t, event.Timeout)
	require.Equal(t, "rudeviewer", event.Timeout.UserLogin)
	require.Equal(t, "Does not follow the rules", event.Timeout.Reason)
	require.Equal(t, 2022, event.Timeout.ExpiresAt.Year())
}

// Twitch adds moderator actions over time. An action this build has never heard of must still
// decode — dropping the whole event would silently lose the actions we DO understand in the same
// feed once Twitch ships the next one.
func TestChannelModerateUnknownActionStillDecodes(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"moderator_user_id": "1339",
		"moderator_user_login": "modgirl",
		"moderator_user_name": "ModGirl",
		"action": "some_action_twitch_added_later",
		"some_action_twitch_added_later": {"whatever": 1}
	}`

	var event ChannelModerateEvent
	require.NoError(t, json.Unmarshal([]byte(payload), &event))
	require.Equal(t, "some_action_twitch_added_later", event.Action)
	require.Nil(t, event.Timeout)
}

// An automod.message.hold carries the held text, the category that triggered AutoMod and the
// severity level. Those three are what the moderation view shows for a held message.
// Payload is Twitch's own automod.message.hold v2 reference example.
func TestAutoModMessageHoldDecodes(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"user_id": "9876",
		"user_login": "rudeviewer",
		"user_name": "RudeViewer",
		"message_id": "d6b1b0e0-0000-4000-8000-00000000abcd",
		"message": {
			"text": "some offensive text",
			"fragments": [
				{"type": "text", "text": "some offensive text", "cheermote": null, "emote": null}
			]
		},
		"category": "aggressive",
		"level": 4,
		"held_at": "2022-12-02T15:00:00.00Z"
	}`

	var event AutoModMessageHoldEvent
	require.NoError(t, json.Unmarshal([]byte(payload), &event))

	require.Equal(t, "some offensive text", event.Message.Text)
	require.Equal(t, "aggressive", event.Category)
	require.Equal(t, 4, event.Level)
	require.Equal(t, "rudeviewer", event.UserLogin)
	require.Equal(t, 2022, event.HeldAt.Year())
	require.Len(t, event.Message.Fragments, 1)
}

// An automod.message.update carries the resolution and who made it. Both are needed: the status
// alone cannot say whether a moderator approved the message or it simply expired.
// Payload is Twitch's own automod.message.update v2 reference example.
func TestAutoModMessageUpdateDecodes(t *testing.T) {
	const payload = `{
		"broadcaster_user_id": "1971641",
		"broadcaster_user_login": "streamer",
		"broadcaster_user_name": "Streamer",
		"user_id": "9876",
		"user_login": "rudeviewer",
		"user_name": "RudeViewer",
		"moderator_user_id": "1339",
		"moderator_user_login": "modgirl",
		"moderator_user_name": "ModGirl",
		"message_id": "d6b1b0e0-0000-4000-8000-00000000abcd",
		"message": {
			"text": "some offensive text",
			"fragments": [
				{"type": "text", "text": "some offensive text", "cheermote": null, "emote": null}
			]
		},
		"category": "aggressive",
		"level": 4,
		"status": "approved",
		"held_at": "2022-12-02T15:00:00.00Z"
	}`

	var event AutoModMessageUpdateEvent
	require.NoError(t, json.Unmarshal([]byte(payload), &event))

	require.Equal(t, "approved", event.Status)
	require.Equal(t, "modgirl", event.ModeratorUserLogin)
	require.Equal(t, "d6b1b0e0-0000-4000-8000-00000000abcd", event.MessageID)
}
