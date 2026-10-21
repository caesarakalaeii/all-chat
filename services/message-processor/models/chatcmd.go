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

package models

import "strings"

// Streamer-defined chat commands (ADR-0079). engagement-service keeps the set of
// channels that have at least one enabled command in ChatCommandChannelsKey; the hot
// path holds an in-process copy and forwards every chat message from those channels
// to StreamChatCommands as a CommandJob carrying the chatter's roles.
const (
	StreamChatCommands     = "chatcmd:jobs"
	ChatCommandChannelsKey = "chatcmd:channels"
)

// ChatCommandChannelMember is one member of ChatCommandChannelsKey. Lowercased on
// both sides for the same reason as EngagementActiveKey.
func ChatCommandChannelMember(platform, channelID string) string {
	return strings.ToLower(platform) + ":" + strings.ToLower(channelID)
}

// roleBadges maps the badge names Twitch and Kick put in Tags["badges"] to the role
// names a chat command's min_role is checked against. Other badges carry no role.
var roleBadges = map[string]string{
	"broadcaster":    "broadcaster",
	"moderator":      "moderator",
	"lead_moderator": "moderator",
	"vip":            "vip",
	"subscriber":     "subscriber",
	"founder":        "subscriber",
}

// ChatRoles derives the chatter's roles from the listener tags the normalizers also
// read: Twitch "broadcaster/1,subscriber/12", Kick "moderator,og", YouTube is_* flags.
// Platforms whose listeners send no role tags (TikTok, Discord, ...) yield none, so
// their chatters count as everyone.
func ChatRoles(raw *RawChatMessage) []string {
	var roles []string
	switch raw.Platform {
	case "twitch", "kick":
		for _, b := range strings.Split(raw.Tags["badges"], ",") {
			name, _, _ := strings.Cut(strings.TrimSpace(b), "/")
			if role, ok := roleBadges[strings.ToLower(name)]; ok {
				roles = append(roles, role)
			}
		}
	case "youtube":
		if raw.Tags["is_owner"] == "true" {
			roles = append(roles, "broadcaster")
		}
		if raw.Tags["is_moderator"] == "true" {
			roles = append(roles, "moderator")
		}
		if raw.Tags["badge_member_url"] != "" || raw.Tags["is_sponsor"] == "true" {
			roles = append(roles, "subscriber")
		}
	}
	return roles
}

// ChatDisplayName is the chatter's display name: Twitch's display-name tag when
// present, else the username every listener sets.
func ChatDisplayName(raw *RawChatMessage) string {
	if dn := raw.Tags["display-name"]; dn != "" {
		return dn
	}
	return raw.Username
}
