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

package chatcmd

import (
	"testing"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	cmds := []models.ChatCommand{
		{Trigger: "clip that", Kind: models.TriggerKeyword},
		{Trigger: "!hello", Kind: models.TriggerCommand},
		{Trigger: "!so", Kind: models.TriggerCommand},
	}

	m, ok := Match(cmds, "  !HELLO   @bob  there ")
	require.True(t, ok)
	assert.Equal(t, "!hello", m.Command.Trigger)
	assert.Equal(t, []string{"@bob", "there"}, m.Args)
	assert.Equal(t, "@bob  there", m.Query)

	// A command word beats a keyword in the same message.
	m, ok = Match(cmds, "!so clip that")
	require.True(t, ok)
	assert.Equal(t, "!so", m.Command.Trigger)

	m, ok = Match(cmds, "lol CLIP THAT now")
	require.True(t, ok)
	assert.Equal(t, "clip that", m.Command.Trigger)
	assert.Equal(t, "lol CLIP THAT now", m.Query)
	assert.Equal(t, []string{"lol", "CLIP", "THAT", "now"}, m.Args)

	// The command word must be the whole first word.
	_, ok = Match(cmds, "!helloworld")
	assert.False(t, ok)
	_, ok = Match(cmds, "say !hello")
	assert.False(t, ok)
	_, ok = Match(cmds, "")
	assert.False(t, ok)
}

func TestHighestRole(t *testing.T) {
	assert.Equal(t, RoleEveryone, HighestRole(nil))
	assert.Equal(t, RoleEveryone, HighestRole([]string{"founder"}))
	assert.Equal(t, RoleSubscriber, HighestRole([]string{"subscriber"}))
	assert.Equal(t, RoleModerator, HighestRole([]string{"subscriber", "moderator", "vip"}))
	assert.Equal(t, RoleBroadcaster, HighestRole([]string{"broadcaster"}))
}

func TestRoleAllows(t *testing.T) {
	assert.True(t, RoleEveryone.Allows(RoleEveryone))
	assert.True(t, RoleModerator.Allows(RoleVIP))
	assert.False(t, RoleSubscriber.Allows(RoleVIP))
	assert.True(t, RoleBroadcaster.Allows(RoleModerator))
}

func TestParseRole(t *testing.T) {
	for _, s := range []string{"everyone", "subscriber", "vip", "moderator", "broadcaster"} {
		r, ok := ParseRole(s)
		require.True(t, ok, s)
		assert.Equal(t, s, r.String())
	}
	_, ok := ParseRole("admin")
	assert.False(t, ok)
}
