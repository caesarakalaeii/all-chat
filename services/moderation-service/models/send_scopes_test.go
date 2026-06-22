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

import "testing"

func TestCanSendForTwitchScopes(t *testing.T) {
	if !CanSendForTwitchScopes([]string{"user:read:chat", ScopeTwitchSend}) {
		t.Error("user:write:chat should grant Twitch send")
	}
	if CanSendForTwitchScopes([]string{"user:read:chat", "moderator:manage:banned_users"}) {
		t.Error("Twitch send must require user:write:chat, not a read/mod scope")
	}
}

func TestCanSendForKickScopes(t *testing.T) {
	if !CanSendForKickScopes([]string{ScopeKickSend}) {
		t.Error("chat:write should grant Kick send")
	}
	if CanSendForKickScopes([]string{"user:read", ScopeKickModeration}) {
		t.Error("Kick send must require chat:write, not user:read/moderation:ban")
	}
}

func TestCanSendForYouTubeScopes(t *testing.T) {
	// YouTube send reuses the same force-ssl scope that authorizes bans.
	if !CanSendForYouTubeScopes([]string{ScopeYouTubeModeration}) {
		t.Error("force-ssl should grant YouTube send")
	}
	if CanSendForYouTubeScopes([]string{"https://www.googleapis.com/auth/youtube.readonly"}) {
		t.Error("YouTube send must require force-ssl, not readonly")
	}
}
