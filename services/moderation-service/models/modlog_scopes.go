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

// ModLogTwitchScopes are the nine Twitch scopes the mod-log opt-in grants: the eight
// moderator:read:* that channel.moderate v2 requires, plus moderator:manage:automod,
// which Twitch demands to create an automod.message.hold subscription even though
// All-Chat only reads those events.
//
// The source of truth is the consent flow that requests them:
// services/auth-service/oauth/twitch.go (the "modlog" entry of
// twitchModerationScopesByAction). This is a deliberate copy, not an import —
// auth-service and moderation-service are separate Go modules, and a cross-service
// import for one string slice would couple two deployables. TestModLogScopesCount
// pins the count so a scope added there and not here is caught here.
var ModLogTwitchScopes = []string{
	"moderator:read:blocked_terms",
	"moderator:read:chat_settings",
	"moderator:read:unban_requests",
	"moderator:read:banned_users",
	"moderator:read:chat_messages",
	"moderator:read:warnings",
	"moderator:read:moderators",
	"moderator:read:vips",
	"moderator:manage:automod",
}

// ModLogGranted reports whether a Twitch token's granted scopes cover the whole mod-log
// opt-in.
//
// All nine or nothing: a credential missing one of them cannot create the EventSub
// subscriptions the mod log is made of, so a partial grant is the same situation as no
// grant — the streamer still has to run the consent.
func ModLogGranted(scopes []string) bool {
	for _, want := range ModLogTwitchScopes {
		if !scopesContain(scopes, want) {
			return false
		}
	}
	return true
}
