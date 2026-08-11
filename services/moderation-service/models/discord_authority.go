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

// Discord's authority model, expressed as pure decisions over a live permission read.
//
// Discord is ADR-0048's one "platform-attested" platform: every Discord write authenticates
// as the shared bot, so no external party ever re-checks a delegated moderator's standing.
// Whatever these functions decide IS the authorization — there is no platform backstop
// behind them, which is why the decisions live here as total, table-tested functions rather
// than as conditionals scattered through a dispatcher.
//
// Everything here operates on a DiscordMemberAuthority, which is a *snapshot* of a live
// read. Its freshness is a security property, not a tuning knob: because the GUILD_MEMBERS
// privileged intent is off, Discord cannot push us a revocation, so the cache TTL bounds how
// long a removed moderator keeps acting.

// DiscordPermManageGuild permits managing the guild's settings. Together with ADMINISTRATOR
// and guild ownership it is what ADR-0048 accepts as proof that the overlay owner controls
// a guild — the Discord arm of the owner-reach anchor.
const DiscordPermManageGuild uint64 = 1 << 5

// DiscordPermViewChannel permits seeing a channel. Not a moderation permission; named here
// so a member who holds only base permissions can be described without a magic number.
const DiscordPermViewChannel uint64 = 1 << 10

// DiscordMemberAuthority is a point-in-time read of what one Discord user may do in one
// guild: whether they are a member at all, whether they own the guild, their effective
// guild-level permission bits (the OR of every role they hold, including @everyone), and
// the position of their highest role.
//
// The zero value is "not in the guild", which denies everything. That default is
// deliberate: a failed or partial read must never be mistaken for a permissive answer.
type DiscordMemberAuthority struct {
	// InGuild is false when Discord answered 404 for this member. Every decision below
	// returns the denying answer in that case, whatever the other fields hold.
	InGuild bool
	// IsGuildOwner short-circuits permissions (an owner implicitly holds all of them) and
	// makes the member untouchable by member operations.
	IsGuildOwner bool
	// Permissions is the effective guild-level bitfield. Channel overwrites are NOT folded
	// in: they can only further restrict, and Discord enforces them at write time.
	Permissions uint64
	// HighestRolePos is the position of the member's highest role. @everyone is position 0,
	// so a member with no other role sits at 0.
	HighestRolePos int
}

// DiscordMemberActions reports the moderation actions this member could perform themselves,
// natively, in this guild.
//
// A non-member gets nothing. The guild owner gets everything: Discord grants an owner every
// permission implicitly, so reading their bits would understate them.
func DiscordMemberActions(m DiscordMemberAuthority) []Action {
	if !m.InGuild {
		return nil
	}
	if m.IsGuildOwner {
		return []Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban}
	}
	return ActionsForDiscordPermissions(m.Permissions)
}

// DiscordDelegatedActions is the action set a delegated moderator may use in a guild:
// what the BOT can do intersected with what the MODERATOR could do themselves.
//
// Both halves are load-bearing and neither is redundant. The bot performs every write, so
// its permissions bound what is *possible* — a moderator cannot borrow authority the bot
// was never invited with. The moderator's own permissions bound what is *permitted* — the
// point of the intersection is that All-Chat never lets someone do through the bot what
// Discord would refuse them directly.
//
// The result is always non-nil so a caller can serialise it as an empty JSON array rather
// than null.
func DiscordDelegatedActions(bot, mod DiscordMemberAuthority) []Action {
	botActions := DiscordMemberActions(bot)
	modActions := DiscordMemberActions(mod)
	permitted := make(map[Action]bool, len(modActions))
	for _, a := range modActions {
		permitted[a] = true
	}
	out := make([]Action, 0, len(botActions))
	for _, a := range botActions {
		if permitted[a] {
			out = append(out, a)
		}
	}
	return out
}

// DiscordHierarchyApplies reports whether Discord's role-hierarchy rule governs an action.
//
// It governs member operations — timeout and ban — and nothing else. Deleting a message is
// not a member operation, and an unban target is by definition not a guild member, so
// neither has a member record to rank. Applying the rule where Discord does not would deny
// actions a moderator can perform natively, which is its own kind of wrong answer.
func DiscordHierarchyApplies(a Action) bool {
	return a == ActionTimeout || a == ActionBan
}

// DiscordOutranks reports whether actor may perform a member operation on target under
// Discord's role hierarchy.
//
// All-Chat has to evaluate this itself, and that is the whole reason it exists here:
// Discord hierarchy-gates the *actor*, and on a delegated action the actor is the shared
// bot, which typically sits above everyone. Without this check a delegated moderator could
// ban a guild administrator they cannot touch in Discord's own client.
//
// The rules, matching Discord: the guild owner outranks everyone and is outranked by
// nobody; otherwise the actor's highest role must be *strictly* above the target's, so a
// tie is a refusal. ADMINISTRATOR deliberately does NOT bypass this — on Discord an
// administrator still cannot act on a member ranked above them.
func DiscordOutranks(actor, target DiscordMemberAuthority) bool {
	if !actor.InGuild {
		return false
	}
	// The guild owner can never be the target of a timeout or ban, including by another
	// owner — a guild has exactly one, so that case cannot arise, but refusing keeps the
	// rule total rather than leaving a gap for a caller to fall into.
	if target.IsGuildOwner {
		return false
	}
	if actor.IsGuildOwner {
		return true
	}
	// A target who is not a member holds no roles, so nothing ranks above the actor.
	if !target.InGuild {
		return true
	}
	return actor.HighestRolePos > target.HighestRolePos
}

// DiscordOwnerControlsGuild reports whether a member's standing proves they control the
// guild — the Discord arm of ADR-0048's owner-reach anchor, since delegation must never
// exceed what the overlay owner could do themselves.
//
// Guild ownership, ADMINISTRATOR or MANAGE_GUILD count. Moderation permissions deliberately
// do not: they prove someone moderates the guild, not that they run it, and the anchor is
// about control. Note this proves control only — it says nothing about capability, and must
// never be read as authorizing an action.
func DiscordOwnerControlsGuild(m DiscordMemberAuthority) bool {
	if !m.InGuild {
		return false
	}
	if m.IsGuildOwner {
		return true
	}
	return m.Permissions&(DiscordPermAdministrator|DiscordPermManageGuild) != 0
}
