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

func TestActionsForKickScopes_NeverExceedsPlatformSupport(t *testing.T) {
	for _, a := range ActionsForKickScopes([]string{ScopeKickModeration}) {
		assert.True(t, SupportsAction("kick", a), "scope-derived action %q must be a supported kick action", a)
	}
	// Kick has no single-message delete, so the scope must never grant it.
	assert.NotContains(t, ActionsForKickScopes([]string{ScopeKickModeration}), ActionDelete)
}

func TestRequiredKickScope(t *testing.T) {
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionTimeout))
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionBan))
	assert.Equal(t, ScopeKickModeration, RequiredKickScope(ActionUnban))
	assert.Equal(t, "", RequiredKickScope(ActionDelete), "Kick does not support single-message delete")
}
