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

package models_test

import (
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
)

// Discord is the one platform in ADR-0048 with no external authority behind it: nobody at
// the platform re-checks a delegated moderator, because every write authenticates as the
// shared bot. These tests are therefore the authorization check itself, not a convenience
// layer over one.

func member(perms uint64, pos int) models.DiscordMemberAuthority {
	return models.DiscordMemberAuthority{InGuild: true, Permissions: perms, HighestRolePos: pos}
}

func TestDiscordMemberActions(t *testing.T) {
	tests := []struct {
		name string
		m    models.DiscordMemberAuthority
		want []models.Action
	}{
		{
			name: "not in the guild yields nothing",
			m:    models.DiscordMemberAuthority{InGuild: false, Permissions: models.DiscordPermAdministrator},
			want: nil,
		},
		{
			name: "guild owner holds every action regardless of permission bits",
			m:    models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true, Permissions: 0},
			want: []models.Action{models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban},
		},
		{
			name: "ADMINISTRATOR short-circuits to every action",
			m:    member(models.DiscordPermAdministrator, 5),
			want: []models.Action{models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban},
		},
		{
			name: "MANAGE_MESSAGES alone yields delete only",
			m:    member(models.DiscordPermManageMessages, 5),
			want: []models.Action{models.ActionDelete},
		},
		{
			name: "MODERATE_MEMBERS alone yields timeout only",
			m:    member(models.DiscordPermModerateMembers, 5),
			want: []models.Action{models.ActionTimeout},
		},
		{
			name: "BAN_MEMBERS yields ban and unban together",
			m:    member(models.DiscordPermBanMembers, 5),
			want: []models.Action{models.ActionBan, models.ActionUnban},
		},
		{
			name: "no moderation bits yields nothing",
			m:    member(models.DiscordPermViewChannel, 5),
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, models.DiscordMemberActions(tc.m))
		})
	}
}

// TestDiscordDelegatedActions_IsBotIntersectModerator pins ADR-0048's rule: the effective
// set is ActionsForDiscordPermissions(botBits) ∩ ActionsForDiscordPermissions(modBits).
// Neither side alone is sufficient — the bot performs the write, so its permissions bound
// what is possible; the moderator's bound what is permitted.
func TestDiscordDelegatedActions_IsBotIntersectModerator(t *testing.T) {
	tests := []struct {
		name     string
		bot, mod models.DiscordMemberAuthority
		want     []models.Action
	}{
		{
			name: "both hold everything",
			bot:  member(models.ModerationBotPermissions, 10),
			mod:  member(models.ModerationBotPermissions, 5),
			want: []models.Action{models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban},
		},
		{
			name: "the bot is the ceiling: a moderator cannot exceed what the bot can do",
			bot:  member(models.DiscordPermManageMessages, 10),
			mod:  member(models.ModerationBotPermissions, 5),
			want: []models.Action{models.ActionDelete},
		},
		{
			name: "the moderator is the floor: a permissive bot grants a moderator nothing extra",
			bot:  member(models.ModerationBotPermissions, 10),
			mod:  member(models.DiscordPermModerateMembers, 5),
			want: []models.Action{models.ActionTimeout},
		},
		{
			name: "an ADMINISTRATOR moderator is still bounded by the bot",
			bot:  member(models.DiscordPermBanMembers, 10),
			mod:  member(models.DiscordPermAdministrator, 5),
			want: []models.Action{models.ActionBan, models.ActionUnban},
		},
		{
			name: "a guild-owner moderator is still bounded by the bot",
			bot:  member(models.DiscordPermManageMessages, 10),
			mod:  models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true},
			want: []models.Action{models.ActionDelete},
		},
		{
			name: "disjoint permissions yield nothing",
			bot:  member(models.DiscordPermManageMessages, 10),
			mod:  member(models.DiscordPermBanMembers, 5),
			want: []models.Action{},
		},
		{
			name: "a moderator who left the guild gets nothing even from an ADMINISTRATOR bot",
			bot:  member(models.DiscordPermAdministrator, 10),
			mod:  models.DiscordMemberAuthority{InGuild: false, Permissions: models.DiscordPermAdministrator},
			want: []models.Action{},
		},
		{
			name: "a bot removed from the guild grants nothing to anyone",
			bot:  models.DiscordMemberAuthority{InGuild: false},
			mod:  member(models.DiscordPermAdministrator, 5),
			want: []models.Action{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, models.DiscordDelegatedActions(tc.bot, tc.mod))
		})
	}
}

// TestDiscordHierarchyApplies: Discord hierarchy-gates the ACTOR on member operations, and
// the bot is always the actor, so All-Chat must apply the gate itself — but only where
// Discord would. Message deletion is not a member operation, and an unban target is by
// definition not a member, so neither has a member record to rank.
func TestDiscordHierarchyApplies(t *testing.T) {
	assert.True(t, models.DiscordHierarchyApplies(models.ActionTimeout))
	assert.True(t, models.DiscordHierarchyApplies(models.ActionBan))
	assert.False(t, models.DiscordHierarchyApplies(models.ActionDelete),
		"deleting a message is not a member operation; Discord applies no hierarchy check to it")
	assert.False(t, models.DiscordHierarchyApplies(models.ActionUnban),
		"an unban target is not a guild member, so there is no member record to rank")
}

func TestDiscordOutranks(t *testing.T) {
	tests := []struct {
		name          string
		actor, target models.DiscordMemberAuthority
		want          bool
		why           string
	}{
		{
			name:   "a strictly higher role outranks",
			actor:  member(models.DiscordPermBanMembers, 10),
			target: member(0, 5),
			want:   true,
		},
		{
			name:   "equal positions do NOT outrank",
			actor:  member(models.DiscordPermBanMembers, 5),
			target: member(0, 5),
			want:   false,
			why:    "Discord requires strictly greater; a tie means neither can act on the other",
		},
		{
			name:   "a lower role does not outrank",
			actor:  member(models.DiscordPermBanMembers, 3),
			target: member(0, 7),
			want:   false,
		},
		{
			name:   "ADMINISTRATOR does not bypass hierarchy",
			actor:  member(models.DiscordPermAdministrator, 3),
			target: member(0, 7),
			want:   false,
			why:    "on Discord an administrator still cannot act on a member ranked above them",
		},
		{
			name:   "the guild owner outranks everyone",
			actor:  models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true, HighestRolePos: 0},
			target: member(0, 99),
			want:   true,
		},
		{
			name:   "nobody outranks the guild owner",
			actor:  member(models.DiscordPermAdministrator, 99),
			target: models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true, HighestRolePos: 0},
			want:   false,
			why:    "the guild owner cannot be timed out or banned by anyone",
		},
		{
			name:   "an owner acting on an owner is still refused",
			actor:  models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true},
			target: models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true},
			want:   false,
			why:    "a guild has one owner, so this cannot arise; refusing keeps the rule total",
		},
		{
			name:   "an actor outside the guild outranks nobody",
			actor:  models.DiscordMemberAuthority{InGuild: false, HighestRolePos: 99},
			target: member(0, 1),
			want:   false,
		},
		{
			name:   "a target outside the guild is outranked by any member",
			actor:  member(models.DiscordPermBanMembers, 1),
			target: models.DiscordMemberAuthority{InGuild: false},
			want:   true,
			why:    "a non-member holds no roles, so a timeout/ban of them is not blocked by hierarchy",
		},
		{
			name:   "@everyone-only actor cannot act on an @everyone-only target",
			actor:  member(models.DiscordPermBanMembers, 0),
			target: member(0, 0),
			want:   false,
			why:    "both sit at position 0, which is a tie, not authority",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, models.DiscordOutranks(tc.actor, tc.target), tc.why)
		})
	}
}

// TestDiscordOwnerControlsGuild covers the owner-reach anchor's Discord predicate: the
// overlay owner must demonstrably control the guild, which the ADR defines as being the
// guild owner or holding ADMINISTRATOR or MANAGE_GUILD.
func TestDiscordOwnerControlsGuild(t *testing.T) {
	tests := []struct {
		name string
		m    models.DiscordMemberAuthority
		want bool
	}{
		{"guild owner controls it", models.DiscordMemberAuthority{InGuild: true, IsGuildOwner: true}, true},
		{"ADMINISTRATOR controls it", member(models.DiscordPermAdministrator, 1), true},
		{"MANAGE_GUILD controls it", member(models.DiscordPermManageGuild, 1), true},
		{"a moderation-only member does not", member(models.ModerationBotPermissions, 1), false},
		{"a plain member does not", member(models.DiscordPermViewChannel, 1), false},
		{"someone outside the guild does not, whatever bits we were handed", models.DiscordMemberAuthority{InGuild: false, Permissions: models.DiscordPermAdministrator}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, models.DiscordOwnerControlsGuild(tc.m))
		})
	}
}

// TestDiscordPermManageGuild_BitValue guards the constant against a typo: a wrong bit here
// would silently widen or narrow who counts as controlling a guild.
func TestDiscordPermManageGuild_BitValue(t *testing.T) {
	assert.Equal(t, uint64(1)<<5, models.DiscordPermManageGuild, "MANAGE_GUILD is 1 << 5 (32)")
	assert.Equal(t, uint64(1)<<10, models.DiscordPermViewChannel, "VIEW_CHANNEL is 1 << 10 (1024)")
}
