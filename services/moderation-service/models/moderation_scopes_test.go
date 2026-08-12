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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionsForTwitchScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   []Action
	}{
		{"no scopes", nil, nil},
		{"unrelated scopes only", []string{"user:read:chat", "channel:bot"}, nil},
		{
			"delete only",
			[]string{ScopeTwitchManageMessages},
			[]Action{ActionDelete},
		},
		{
			"banned-users only",
			[]string{ScopeTwitchManageBannedUsers},
			[]Action{ActionTimeout, ActionBan, ActionUnban},
		},
		{
			"both, mixed with login scopes",
			[]string{"user:read:chat", ScopeTwitchManageBannedUsers, ScopeTwitchManageMessages},
			[]Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, ActionsForTwitchScopes(tt.scopes))
		})
	}
}

func TestActionsForTwitchScopes_NeverExceedsPlatformSupport(t *testing.T) {
	got := ActionsForTwitchScopes([]string{ScopeTwitchManageMessages, ScopeTwitchManageBannedUsers})
	for _, a := range got {
		assert.True(t, SupportsAction("twitch", a), "scope-derived action %q must be a supported twitch action", a)
	}
}

func TestRequiredTwitchScope(t *testing.T) {
	assert.Equal(t, ScopeTwitchManageMessages, RequiredTwitchScope(ActionDelete))
	assert.Equal(t, ScopeTwitchManageBannedUsers, RequiredTwitchScope(ActionTimeout))
	assert.Equal(t, ScopeTwitchManageBannedUsers, RequiredTwitchScope(ActionBan))
	assert.Equal(t, ScopeTwitchManageBannedUsers, RequiredTwitchScope(ActionUnban))
	assert.Equal(t, "", RequiredTwitchScope(Action("bogus")))
}

func TestActionsForKickScopes(t *testing.T) {
	assert.Empty(t, ActionsForKickScopes(nil))
	assert.Empty(t, ActionsForKickScopes([]string{"user:read"}), "Kick login scope grants no moderation")
	assert.ElementsMatch(t,
		[]Action{ActionTimeout, ActionBan, ActionUnban},
		ActionsForKickScopes([]string{"user:read", ScopeKickModeration}))
}

// Kick splits moderation across two scopes, so each must grant only its own actions. A
// streamer who consented before delete existed holds moderation:ban alone, and must keep
// timeout/ban/unban without silently gaining a delete their token cannot perform.
func TestActionsForKickScopes_TheTwoScopesAreIndependent(t *testing.T) {
	assert.Equal(t,
		[]Action{ActionDelete},
		ActionsForKickScopes([]string{ScopeKickChatMessageManage}),
		"the message scope grants delete alone")
	assert.NotContains(t,
		ActionsForKickScopes([]string{ScopeKickModeration}), ActionDelete,
		"the ban scope must never grant delete")
	assert.ElementsMatch(t,
		[]Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban},
		ActionsForKickScopes([]string{ScopeKickModeration, ScopeKickChatMessageManage}),
		"both scopes together grant the full Kick set")
}

func TestActionsForKickScopes_NeverExceedsPlatformSupport(t *testing.T) {
	for _, a := range ActionsForKickScopes([]string{ScopeKickModeration, ScopeKickChatMessageManage}) {
		assert.True(t, SupportsAction("kick", a), "scope-derived action %q must be a supported kick action", a)
	}
}

func TestRequiredKickScope(t *testing.T) {
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionTimeout))
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionBan))
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionUnban))
	assert.Equal(t, ScopeKickChatMessageManage, RequiredKickScope(ActionDelete),
		"delete is gated behind its own Kick scope, not the ban scope")
	assert.Equal(t, "", RequiredKickScope(Action("bogus")))
}

// YouTube's single force-ssl scope covers both supported actions, because timeout and ban are the
// same liveChatBans.insert call with a different ban type.
func TestActionsForYouTubeScopes(t *testing.T) {
	assert.Empty(t, ActionsForYouTubeScopes(nil))
	assert.Empty(t, ActionsForYouTubeScopes([]string{"https://www.googleapis.com/auth/youtube.readonly"}),
		"the listener's readonly scope grants no moderation")
	assert.Equal(t,
		[]Action{ActionTimeout, ActionBan},
		ActionsForYouTubeScopes([]string{ScopeYouTubeModeration}))
}

func TestRequiredYouTubeScope(t *testing.T) {
	assert.Equal(t, ScopeYouTubeModeration, RequiredYouTubeScope(ActionTimeout))
	assert.Equal(t, ScopeYouTubeModeration, RequiredYouTubeScope(ActionBan))
	assert.Equal(t, "", RequiredYouTubeScope(ActionDelete))
	assert.Equal(t, "", RequiredYouTubeScope(ActionUnban))
}

// Delete and unban are absent from YouTube for lack of an ID, not for lack of a scope — so no scope
// may ever produce them, and the capability must not offer a control whose call cannot be built.
// (delete needs a Data API message id where production holds InnerTube renderer ids; unban needs the
// ban resource id returned by insert, which nothing persists.)
func TestYouTubeNeverOffersDeleteOrUnban(t *testing.T) {
	for _, a := range ActionsForYouTubeScopes([]string{ScopeYouTubeModeration}) {
		assert.True(t, SupportsAction("youtube", a), "scope-derived action %q must be supported", a)
	}
	assert.False(t, SupportsAction("youtube", ActionDelete))
	assert.False(t, SupportsAction("youtube", ActionUnban))
}

func TestActionsForDiscordPermissions(t *testing.T) {
	tests := []struct {
		name  string
		perms uint64
		want  []Action
	}{
		{"none", 0, nil},
		{"manage messages only", DiscordPermManageMessages, []Action{ActionDelete}},
		{"moderate members only", DiscordPermModerateMembers, []Action{ActionTimeout}},
		{"ban members grants ban+unban", DiscordPermBanMembers, []Action{ActionBan, ActionUnban}},
		{
			"full moderation set",
			ModerationBotPermissions,
			[]Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban},
		},
		{
			"administrator implies everything",
			DiscordPermAdministrator,
			[]Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.want, ActionsForDiscordPermissions(tt.perms))
		})
	}
}

func TestActionsForDiscordPermissions_NeverExceedsPlatformSupport(t *testing.T) {
	for _, a := range ActionsForDiscordPermissions(ModerationBotPermissions | DiscordPermAdministrator) {
		assert.True(t, SupportsAction("discord", a), "permission-derived action %q must be a supported discord action", a)
	}
}

func TestRequiredDiscordPermission(t *testing.T) {
	assert.Equal(t, DiscordPermManageMessages, RequiredDiscordPermission(ActionDelete))
	assert.Equal(t, DiscordPermModerateMembers, RequiredDiscordPermission(ActionTimeout))
	assert.Equal(t, DiscordPermBanMembers, RequiredDiscordPermission(ActionBan))
	assert.Equal(t, DiscordPermBanMembers, RequiredDiscordPermission(ActionUnban))
	assert.Equal(t, uint64(0), RequiredDiscordPermission(Action("bogus")))
}
