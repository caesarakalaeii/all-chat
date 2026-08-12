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

// Package audit records every moderation command to the moderation_actions table
// (migration 060) for abuse forensics and accountability. A row is written for every
// command regardless of outcome (allowed, denied, dry-run, or platform failure).
package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Outcome values recorded for a moderation command.
const (
	OutcomeSuccess        = "success"         // platform accepted the action
	OutcomeDryRun         = "dry_run"         // reflect-back emitted, no platform call (no client wired)
	OutcomeDenied         = "denied"          // authorization failed (not owner, not a source, etc.)
	OutcomePlatformError  = "platform_error"  // platform API rejected/failed the action
	OutcomeReauthRequired = "reauth_required" // owner's token lacks the moderation scope; needs opt-in re-consent
	OutcomeNoCredential   = "no_credential"   // the actor holds no moderator credential for the channel
	// OutcomeOwnerUnverified: the overlay owner cannot be shown to control the channel, so there
	// was nothing for them to delegate on it (ADR-0048's owner-reach anchor).
	OutcomeOwnerUnverified = "owner_unverified"
	// OutcomeNotPlatformModerator: the platform says the delegated moderator does not moderate
	// that channel. Distinct from a denial: All-Chat allowed it and the platform did not, which is
	// exactly the authority split the design intends — and a spike of these is one of the signals
	// that a grant has gone stale.
	OutcomeNotPlatformModerator = "not_platform_moderator"
	// OutcomeDelegationUnsupported: a delegated action on a platform whose delegated path is not
	// built yet. Recorded rather than dropped — it is how a moderator hitting a wall becomes
	// visible instead of just looking broken.
	OutcomeDelegationUnsupported = "delegation_unsupported"
	// The Discord refusals. They are kept apart rather than collapsed into OutcomeDenied because
	// on Discord no platform message accompanies a refusal — All-Chat's own check is the only
	// authority (ADR-0048), so the audit row is the entire record of WHY something was refused,
	// and the five causes have five different remedies.
	//
	// OutcomeDiscordLinkRequired: the moderator has no Discord account linked, so their guild
	// permissions could not be read at all.
	OutcomeDiscordLinkRequired = "discord_link_required"
	// OutcomeModNotInGuild: the moderator is not a member of the guild.
	OutcomeModNotInGuild = "mod_not_in_guild"
	// OutcomeModLacksPermission: the moderator is in the guild but could not perform the action
	// themselves, so All-Chat refused to lend them the bot's authority.
	OutcomeModLacksPermission = "mod_lacks_permission"
	// OutcomeModBelowTarget: Discord's role hierarchy refused the member operation. Worth its own
	// value because a run of these against one target is what an escalation attempt looks like.
	OutcomeModBelowTarget = "mod_below_target"
	// OutcomeBotMissingPermission: the bot was never invited with the permission, so nobody can
	// borrow it. Distinct from a platform 403 on the owner path: this one is known before any call.
	OutcomeBotMissingPermission = "bot_missing_permission"
)

// Entry is one audited moderation command.
//
// Five identities are kept distinguishable forever (ADR-0048), because a delegated action has
// more of them than an owner action does and collapsing any pair would destroy the trail:
// who acted, in what role, for whom, with whose credential, and as which platform id.
type Entry struct {
	OverlayID string
	// ActorUserID is the human who performed the action — the owner when they moderate their own
	// overlay, the delegated moderator when one acts. It is NOT necessarily whose credential ran
	// the platform call (see CredentialUserID) and NOT necessarily the overlay owner.
	ActorUserID string
	// ActorRole is owner | moderator. Empty on legacy rows, where the actor was always the owner.
	ActorRole string
	// OnBehalfOfUserID is the overlay owner the action was performed for. Equals ActorUserID when
	// the owner acted themselves.
	OnBehalfOfUserID string
	// CredentialUserID is whose OAuth token actually performed the platform call. This is the
	// machine-checkable proof that a delegated action never fell back to the owner's credential:
	// for a delegated row it must equal ActorUserID, never OnBehalfOfUserID. Empty where no
	// per-user credential acts (Discord's shared bot, and dry runs).
	CredentialUserID string
	// PlatformActorID is the id sent as the platform's moderator field, so a row can be
	// reconciled against the platform's own moderator log.
	PlatformActorID string
	// GrantID is the delegation the action was performed under. Empty for an owner action.
	GrantID         string
	ImpersonatedBy  string // real admin when acting under impersonation; "" otherwise
	Platform        string
	ChannelID       string
	Action          string
	TargetUserID    string // timeout/ban/unban
	TargetMessageID string // delete (native platform message id)
	Reason          string
	Outcome         string
	PlatformStatus  string // platform API status/detail, if any
}

// Store writes audit rows.
type Store struct {
	db *pgxpool.Pool
}

// New creates an audit Store over the given pool.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Record inserts one audit row. Optional string fields are stored as NULL when
// empty (notably impersonated_by and the ADR-0048 attribution columns, which are UUID columns and
// must be NULL — not the empty string — when they do not apply).
func (s *Store) Record(ctx context.Context, e Entry) error {
	const query = `
		INSERT INTO moderation_actions
			(overlay_id, actor_user_id, impersonated_by, platform, channel_id, action,
			 target_user_id, target_message_id, reason, outcome, platform_status,
			 actor_role, on_behalf_of_user_id, credential_user_id, platform_actor_id, grant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`
	_, err := s.db.Exec(ctx, query,
		e.OverlayID,
		e.ActorUserID,
		nullIfEmpty(e.ImpersonatedBy),
		e.Platform,
		e.ChannelID,
		e.Action,
		nullIfEmpty(e.TargetUserID),
		nullIfEmpty(e.TargetMessageID),
		nullIfEmpty(e.Reason),
		e.Outcome,
		nullIfEmpty(e.PlatformStatus),
		nullIfEmpty(e.ActorRole),
		nullIfEmpty(e.OnBehalfOfUserID),
		nullIfEmpty(e.CredentialUserID),
		nullIfEmpty(e.PlatformActorID),
		nullIfEmpty(e.GrantID),
	)
	if err != nil {
		return fmt.Errorf("record moderation action: %w", err)
	}
	return nil
}

// nullIfEmpty returns nil for an empty string so the column is stored as SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
