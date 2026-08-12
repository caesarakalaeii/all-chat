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

package dispatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"go.uber.org/zap"
)

// discordAPI is the subset of clients.DiscordClient the dispatcher calls. Member ops
// (timeout/ban/unban) are guild-scoped; the guild id is resolved from the channel id by
// guilds, not threaded through the platform-agnostic DispatchRequest.
type discordAPI interface {
	DeleteMessage(ctx context.Context, channelID, messageID string) error
	TimeoutMember(ctx context.Context, guildID, userID string, until time.Time) error
	BanMember(ctx context.Context, guildID, userID string) error
	UnbanMember(ctx context.Context, guildID, userID string) error
}

// discordGuildResolver answers the three cached Discord reads authorization needs: which guild a
// channel belongs to, what the bot may do there, and what one member may do there.
// (*clients.DiscordGuildResolver satisfies it.)
type discordGuildResolver interface {
	GuildID(ctx context.Context, channelID string) (string, error)
	GuildBotPermissions(ctx context.Context, guildID string) (uint64, error)
	MemberAuthority(ctx context.Context, guildID, userID string) (clients.DiscordMember, error)
}

// discordAuthorityStore is the database half of the Discord authority model: who someone is on
// Discord, and which guilds the overlay owner connected. (*repository.Repository satisfies it.)
type discordAuthorityStore interface {
	DiscordIdentity(ctx context.Context, userID string) (string, bool, error)
	DiscordGuildConnectedBy(ctx context.Context, userID, guildID string) (bool, error)
}

// Discord dispatches moderation (delete + member ban/timeout/unban) to the Discord bot REST API.
//
// It is the one dispatcher that resolves no per-user credential: Discord has no per-user
// moderation API, so every write authenticates as the shared bot whoever asked for it. That makes
// this file's authorization checks load-bearing in a way no other dispatcher's are — ADR-0048 calls
// the model "platform-attested", meaning Discord attests the facts (via live bot-token reads) and
// All-Chat alone decides on them. Twitch/Kick/YouTube re-check a moderator's role on every call and
// will refuse an action we wrongly allowed; here nothing will.
//
// Two consequences run through everything below. Every read that feeds the decision fails CLOSED,
// because a swallowed error would be an authorization decision made on no information. And the
// role-hierarchy rule is enforced here rather than left to Discord: Discord hierarchy-gates the
// *actor*, and the actor is always the bot, which typically outranks everyone in the guild.
//
// A bot permission the guild never granted still surfaces as a dispatch error (502, no
// reflect-back) on the owner path, fixed by RE-INVITING the bot rather than by an OAuth
// re-consent; on the delegated path the bot's ceiling is known before the call and refuses cleanly.
type Discord struct {
	api    discordAPI
	guilds discordGuildResolver
	store  discordAuthorityStore
	logger *zap.Logger
}

// NewDiscord wires a Discord dispatcher over the bot REST client, the cached guild/member
// resolver, and the store that answers the owner anchor and the Discord account links.
//
// store is a constructor parameter rather than an opt-in setter (as Twitch's mod source is)
// because on Discord it is not an enhancement: without it nothing can be checked, and "cannot
// check" must never degrade into "act with the streamer's full guild authority". A nil store
// therefore refuses every action rather than allowing any.
func NewDiscord(api discordAPI, guilds discordGuildResolver, store discordAuthorityStore, logger *zap.Logger) *Discord {
	return &Discord{api: api, guilds: guilds, store: store, logger: logger}
}

// Dispatch authorizes and performs a Discord moderation action.
//
// delete is channel-scoped and the member ops are guild-scoped, but authority is per-guild for all
// of them, so the guild is resolved first in every case — that resolution is also the ADR's
// requirement that a source's channel actually belong to the guild being anchored against.
func (d *Discord) Dispatch(ctx context.Context, actor models.Actor, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if req.Platform != "discord" {
		return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
	}
	if d.store == nil {
		// A wiring bug, not a user state: loud, and refusing.
		return models.DispatchResult{}, errors.New("discord authority store not wired; refusing rather than acting with the bot's guild authority")
	}

	guildID, err := d.guilds.GuildID(ctx, req.ChannelID)
	if err != nil {
		return d.result(action, req.ChannelID, fmt.Errorf("resolve guild for channel: %w", err))
	}

	res, authorized, err := d.authorize(ctx, actor, action, guildID, req)
	if err != nil || !authorized {
		return res, err
	}
	return d.perform(ctx, action, guildID, req)
}

// authorize decides whether this actor may perform this action in this guild. A false second
// return means the decision is already made and res carries it; an error means a read failed and
// nothing may be inferred.
func (d *Discord) authorize(
	ctx context.Context, actor models.Actor, action models.Action, guildID string, req models.DispatchRequest,
) (models.DispatchResult, bool, error) {
	// The owner-reach anchor, on both paths: nothing delegated may exceed what the overlay owner
	// could do themselves, and an owner may only act on a guild they actually connected. The row
	// is itself platform-attested — Discord only lets someone add a bot where they hold Manage
	// Server — so on the owner path it is the whole anchor (ADR-0048, "Discord anchor strength").
	connected, err := d.store.DiscordGuildConnectedBy(ctx, actor.OwnerUserID, guildID)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read owner guild anchor: %w", err)
	}
	if !connected {
		return models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, false, nil
	}
	if !actor.IsModerator() {
		return models.DispatchResult{}, true, nil
	}

	// Everything from here is the delegated path, ordered so that the most actionable answer wins
	// when several checks would fail: the streamer's own blockers first (they gate every moderator
	// on the overlay), then this moderator's.
	if res, ok, err := d.ownerStillControlsGuild(ctx, actor.OwnerUserID, guildID); err != nil || !ok {
		return res, false, err
	}

	modSnowflake, linked, err := d.store.DiscordIdentity(ctx, actor.UserID)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read moderator discord identity: %w", err)
	}
	if !linked {
		// The one Discord blocker the moderator can clear themselves. Consent is deferred to first
		// use, so this is the normal state of a fresh grant rather than a fault.
		return models.DispatchResult{Outcome: models.DispatchModNotLinked}, false, nil
	}

	mod, err := d.memberAuthority(ctx, guildID, modSnowflake)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read moderator guild standing: %w", err)
	}
	if !mod.InGuild {
		return models.DispatchResult{Outcome: models.DispatchModNotInGuild}, false, nil
	}

	// The bot bounds what is POSSIBLE, the moderator what is PERMITTED, and neither half is
	// redundant: a moderator cannot borrow authority the bot was never invited with, and All-Chat
	// must never let someone do through the bot what Discord would refuse them directly.
	botBits, err := d.guilds.GuildBotPermissions(ctx, guildID)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read bot guild permissions: %w", err)
	}
	// The bot's own hierarchy position is deliberately not read: it bounds nothing here, because
	// Discord enforces the actor's hierarchy itself and the actor IS the bot. A bot cannot own a
	// guild either, and if one somehow did, reading only its permission bits understates it —
	// which errs toward refusal.
	bot := models.DiscordMemberAuthority{InGuild: true, Permissions: botBits}
	if !models.ActionsInclude(models.DiscordDelegatedActions(bot, mod), action) {
		// Which side is missing it decides who is told to do what: re-invite the bot, or give the
		// moderator a role. The bot is reported first when both are missing, since no moderator can
		// perform an action the bot cannot.
		if !models.ActionsInclude(models.DiscordMemberActions(bot), action) {
			d.logger.Warn("discord delegated action refused: the bot lacks the permission",
				zap.String("action", string(action)), zap.String("guild_id", guildID))
			return models.DispatchResult{Outcome: models.DispatchBotMissingPermission}, false, nil
		}
		return models.DispatchResult{Outcome: models.DispatchModLacksPermission}, false, nil
	}

	if !models.DiscordHierarchyApplies(action) {
		// Delete is not a member operation and an unban target is by definition not a member, so
		// neither has a member record to rank. Ranking them anyway would deny actions the
		// moderator can perform natively.
		return models.DispatchResult{}, true, nil
	}
	target, err := d.memberAuthority(ctx, guildID, req.TargetUserID)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read target guild standing: %w", err)
	}
	if !models.DiscordOutranks(mod, target) {
		return models.DispatchResult{Outcome: models.DispatchModBelowTarget}, false, nil
	}
	return models.DispatchResult{}, true, nil
}

// ownerStillControlsGuild is the delegated path's live half of the anchor. The connected-guild row
// records that the owner controlled the guild when they invited the bot; only this read notices
// they have since lost that standing, which matters precisely because a third party is about to
// act on the strength of the owner's reach.
//
// An owner who has never linked a Discord account cannot be read at all. That is an owner-side
// blocker — only the streamer can clear it — so it reports as owner-unverified rather than sending
// the moderator to a link flow that would change nothing.
func (d *Discord) ownerStillControlsGuild(ctx context.Context, ownerUserID, guildID string) (models.DispatchResult, bool, error) {
	ownerSnowflake, linked, err := d.store.DiscordIdentity(ctx, ownerUserID)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read owner discord identity: %w", err)
	}
	if !linked {
		return models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, false, nil
	}
	owner, err := d.memberAuthority(ctx, guildID, ownerSnowflake)
	if err != nil {
		return models.DispatchResult{}, false, fmt.Errorf("read owner guild standing: %w", err)
	}
	if !models.DiscordOwnerControlsGuild(owner) {
		d.logger.Warn("discord delegation refused: the overlay owner no longer controls the guild",
			zap.String("guild_id", guildID), zap.String("owner_user_id", ownerUserID))
		return models.DispatchResult{Outcome: models.DispatchOwnerUnverified}, false, nil
	}
	return models.DispatchResult{}, true, nil
}

// memberAuthority reads a member's live standing and converts it into the domain type the decision
// functions take. clients deliberately holds no dependency on models — the same reason models.Actor
// duplicates repository.Role* — so the one-line conversion lives here, in the layer importing both.
func (d *Discord) memberAuthority(ctx context.Context, guildID, userID string) (models.DiscordMemberAuthority, error) {
	m, err := d.guilds.MemberAuthority(ctx, guildID, userID)
	if err != nil {
		return models.DiscordMemberAuthority{}, err
	}
	return models.DiscordMemberAuthority{
		InGuild:        m.InGuild,
		IsGuildOwner:   m.IsGuildOwner,
		Permissions:    m.Permissions,
		HighestRolePos: m.HighestRolePos,
	}, nil
}

// perform makes the authorized call. No Discord result is reported back as attribution: the shared
// bot is always the actor, so there is no per-user credential and no platform moderator id, and
// claiming either would put a fiction in the audit row.
func (d *Discord) perform(ctx context.Context, action models.Action, guildID string, req models.DispatchRequest) (models.DispatchResult, error) {
	switch action {
	case models.ActionDelete:
		return d.result(action, req.ChannelID, d.api.DeleteMessage(ctx, req.ChannelID, req.NativeMessageID))
	case models.ActionTimeout:
		until := time.Now().Add(time.Duration(req.DurationSeconds) * time.Second)
		return d.result(action, req.ChannelID, d.api.TimeoutMember(ctx, guildID, req.TargetUserID, until))
	case models.ActionBan:
		return d.result(action, req.ChannelID, d.api.BanMember(ctx, guildID, req.TargetUserID))
	case models.ActionUnban:
		return d.result(action, req.ChannelID, d.api.UnbanMember(ctx, guildID, req.TargetUserID))
	default:
		return models.DispatchResult{}, fmt.Errorf("dispatch: unsupported discord action %q", action)
	}
}

// result classifies a Discord API outcome. A 403 (bot lacks the moderation permission)
// is returned as an error with a re-invite hint — never a false reflect-back. nil is
// success (DispatchPerformed).
func (d *Discord) result(action models.Action, channelID string, err error) (models.DispatchResult, error) {
	switch {
	case err == nil:
		return models.DispatchResult{Outcome: models.DispatchPerformed}, nil
	case errors.Is(err, clients.ErrDiscordForbidden):
		d.logger.Warn("discord moderation forbidden; the bot lacks the required permission — re-invite it with moderation permissions",
			zap.String("action", string(action)), zap.String("channel_id", channelID))
		return models.DispatchResult{}, fmt.Errorf("discord bot lacks the permission for %s; re-invite the bot with moderation permissions: %w", action, err)
	default:
		// Unauthorized bot token or any other error: a real failure. Returning an error
		// keeps the reflect-back from firing for an action that did not land.
		return models.DispatchResult{}, fmt.Errorf("discord %s failed: %w", action, err)
	}
}
