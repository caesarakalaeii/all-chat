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

// Package models holds the request/response DTOs and the per-platform capability
// model for chat moderation. The frontend reads Capabilities to decide which
// controls to enable, so the support matrix lives here as the single source of truth.
package models

// Action is a moderation verb the dashboard can request.
type Action string

const (
	ActionDelete  Action = "delete"  // remove a single message
	ActionTimeout Action = "timeout" // remove a user's messages for a duration
	ActionBan     Action = "ban"     // permanently remove a user
	ActionUnban   Action = "unban"   // lift a ban/timeout
)

// Reasons explaining why a source is not (currently) moderatable.
//
// The first two can be reported to either role; the last three only ever describe a delegated
// moderator's source (ADR-0048), because they are about the grant or about the moderator's own
// consent rather than about the streamer's credential.
const (
	// ReasonUnsupportedPlatform: the platform has no usable moderation API (TikTok).
	ReasonUnsupportedPlatform = "unsupported_platform"
	// ReasonMissingScope: the owner has not granted the moderation OAuth scopes for
	// this platform/account yet (covers both "never opted in" and "partially granted").
	// Cleared via the opt-in re-consent flow.
	ReasonMissingScope = "missing_scope"
	// ReasonNotDelegated: the overlay owner did not delegate this platform to this moderator —
	// either its leg is disabled (absence IS disablement, migration 080) or none of the granted
	// actions exist on this platform. Only the streamer can clear it.
	ReasonNotDelegated = "not_delegated"
	// ReasonNeedsConsent: the moderator has not yet granted All-Chat their OWN moderation scopes
	// for this platform. This is the "Connect to moderate" state, and it is the one the moderator
	// can clear themselves (ADR-0048 defers consent to first use).
	ReasonNeedsConsent = "needs_consent"
	// ReasonNeedsDiscordLink: Discord has no per-user moderation API, so the shared bot acts and
	// All-Chat must know which Discord account the MODERATOR is before it will check their server
	// permissions. Theirs to clear, via the Discord account link.
	ReasonNeedsDiscordLink = "needs_discord_link"
	// ReasonOwnerChannelUnverified: the overlay OWNER cannot be shown to control this channel, so
	// there is nothing for them to delegate on it (ADR-0048's owner-reach anchor). Kept apart from
	// ReasonNeedsDiscordLink even though the same missing thing — a Discord account link — can
	// cause both: the moderator can clear one and only the streamer can clear the other, and
	// pointing a volunteer at a link flow that would change nothing is the dead end this whole
	// vocabulary exists to avoid. Matches the action path's owner_channel_unverified code.
	ReasonOwnerChannelUnverified = "owner_channel_unverified"
	// ReasonBotMissingPermission: the All-Chat bot was invited to this Discord server without the
	// permissions this grant covers, so nobody can borrow them. Cleared by the streamer
	// re-inviting the bot — never by any OAuth re-consent, which cannot touch guild permissions.
	ReasonBotMissingPermission = "bot_missing_permission"
)

// PlatformActions is the moderation support matrix per platform in All-Chat.
// TikTok is intentionally absent: it has no official moderation API, so it is
// always reported unsupported rather than offered a button that cannot work.
// shared_overlay is absent by design: a recipient must not moderate the original
// streamer's channel (least-privilege / owner-only authorization).
var PlatformActions = map[string][]Action{
	"twitch": {ActionDelete, ActionTimeout, ActionBan, ActionUnban},
	// Kick supports the full set, across two scopes: delete needs
	// moderation:chat_message:manage, the rest moderation:ban. Delete was believed absent
	// until ADR-0048 found the endpoint; a streamer who consented before then holds only
	// the ban scope, which is why the scope→action mapping decides per scope rather than
	// per platform (see ActionsForKickScopes).
	"kick": {ActionDelete, ActionTimeout, ActionBan, ActionUnban},
	// Discord supports the full set via guild-level bot permissions (MANAGE_MESSAGES
	// for delete, MODERATE_MEMBERS for timeout, BAN_MEMBERS for ban/unban). Which of
	// these a given source can actually use depends on the bot's effective permissions
	// in that guild (see ActionsForDiscordPermissions) — granted at invite time, so a
	// bot invited without the elevated permissions reports only what it holds.
	"discord": {ActionDelete, ActionTimeout, ActionBan, ActionUnban},
	// YouTube is ban-only for v1: liveChatBans.delete (unban) keys on the ban resource
	// id returned by insert, which All-Chat does not persist, so unban is deferred.
	"youtube": {ActionBan},
}

// SupportsAction reports whether the platform supports the given moderation action.
func SupportsAction(platform string, a Action) bool {
	return ActionsInclude(PlatformActions[platform], a)
}

// ActionsInclude reports whether an action set contains an action. The sets it is asked about are
// at most four elements long, so a linear scan is the whole implementation.
func ActionsInclude(actions []Action, a Action) bool {
	for _, candidate := range actions {
		if candidate == a {
			return true
		}
	}
	return false
}

// Twitch moderation OAuth scopes. These are requested only through the dedicated
// opt-in re-consent flow (never bundled into login/add-source), minimized to the
// actions the streamer enables (ADR-0017, least privilege).
const (
	// ScopeTwitchManageMessages permits deleting individual chat messages.
	ScopeTwitchManageMessages = "moderator:manage:chat_messages"
	// ScopeTwitchManageBannedUsers permits timeout/ban/unban.
	ScopeTwitchManageBannedUsers = "moderator:manage:banned_users"
)

// ActionsForTwitchScopes maps a Twitch token's granted scopes to the moderation
// actions it can perform. The result is inherently a subset of
// PlatformActions["twitch"], so the UI only enables what the minimal granted
// scopes actually allow.
func ActionsForTwitchScopes(scopes []string) []Action {
	has := func(want string) bool {
		for _, s := range scopes {
			if s == want {
				return true
			}
		}
		return false
	}
	var out []Action
	if has(ScopeTwitchManageMessages) {
		out = append(out, ActionDelete)
	}
	if has(ScopeTwitchManageBannedUsers) {
		out = append(out, ActionTimeout, ActionBan, ActionUnban)
	}
	return out
}

// RequiredTwitchScope returns the single Twitch OAuth scope an action needs, or ""
// if the action is not a Twitch moderation action. Used to pre-check a resolved
// token before calling Helix and to populate missing_scopes on a re-consent prompt.
func RequiredTwitchScope(a Action) string {
	switch a {
	case ActionDelete:
		return ScopeTwitchManageMessages
	case ActionTimeout, ActionBan, ActionUnban:
		return ScopeTwitchManageBannedUsers
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Chat-send capability (advanced-controls opt-in).
//
// Sending a chat message from the monitor view needs a DIFFERENT OAuth scope than
// moderation: Twitch user:write:chat, Kick chat:write. YouTube reuses the force-ssl
// scope (ScopeYouTubeModeration) it already grants for bans. These scopes ride along
// on the same opt-in re-consent that grants moderation, so a source is "sendable"
// exactly when its granted scopes include the send scope.
// ---------------------------------------------------------------------------

// ScopeTwitchSend authorizes the Helix Send Chat Message API (user token).
const ScopeTwitchSend = "user:write:chat"

// ScopeKickSend authorizes the Kick public Send Chat Message API.
const ScopeKickSend = "chat:write"

// CanSendForTwitchScopes reports whether a Twitch token's scopes allow sending chat.
func CanSendForTwitchScopes(scopes []string) bool { return scopesContain(scopes, ScopeTwitchSend) }

// CanSendForKickScopes reports whether a Kick token's scopes allow sending chat.
func CanSendForKickScopes(scopes []string) bool { return scopesContain(scopes, ScopeKickSend) }

// CanSendForYouTubeScopes reports whether a YouTube token's scopes allow live-chat
// send (force-ssl — the same scope that authorizes bans).
func CanSendForYouTubeScopes(scopes []string) bool {
	return scopesContain(scopes, ScopeYouTubeModeration)
}

// scopesContain reports whether want is present in scopes.
func scopesContain(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// Kick moderation OAuth scopes. Kick splits moderation across two: ban/timeout/unban sit
// behind moderation:ban, and single-message delete behind moderation:chat_message:manage.
// Both are requested only through the opt-in re-consent flow (never bundled into
// login/add-source), per ADR-0017.
const (
	// ScopeKickModeration permits timeout/ban/unban ("Execute ban/unban actions on users").
	ScopeKickModeration = "moderation:ban"
	// ScopeKickChatMessageManage permits deleting individual chat messages ("Execute
	// moderation actions on chat messages"). Kick's single-message delete was long recorded
	// here as nonexistent; the endpoint is DELETE /public/v1/chat/{message_id} and this is
	// the scope that opens it.
	ScopeKickChatMessageManage = "moderation:chat_message:manage"
)

// ActionsForKickScopes maps a Kick token's granted scopes to moderation actions. The
// result is a subset of PlatformActions["kick"], so the UI only enables what the
// granted scopes allow.
//
// Per scope, not per platform: the two Kick scopes are granted independently, and every
// streamer who consented before delete existed holds only moderation:ban. Reporting
// delete for them would enable a button whose call Kick refuses.
func ActionsForKickScopes(scopes []string) []Action {
	var out []Action
	if scopesContain(scopes, ScopeKickChatMessageManage) {
		out = append(out, ActionDelete)
	}
	if scopesContain(scopes, ScopeKickModeration) {
		out = append(out, ActionTimeout, ActionBan, ActionUnban)
	}
	return out
}

// RequiredKickScope returns the Kick OAuth scope an action needs, or "" if the action
// is not a Kick moderation action. Used to pre-check a resolved token before calling
// the Kick API and to populate missing_scopes on a re-consent prompt.
func RequiredKickScope(a Action) string {
	switch a {
	case ActionDelete:
		return ScopeKickChatMessageManage
	case ActionTimeout, ActionBan, ActionUnban:
		return ScopeKickModeration
	default:
		return ""
	}
}

// YouTube moderation OAuth scope. force-ssl grants live-chat write access (incl.
// bans). It was dropped from login (ADR-0012) and is re-added ONLY through the opt-in
// re-consent flow (ADR-0017). The full URL is the scope identifier Google expects.
const ScopeYouTubeModeration = "https://www.googleapis.com/auth/youtube.force-ssl"

// ActionsForYouTubeScopes maps a YouTube token's granted scopes to moderation actions.
// v1 is ban-only (see PlatformActions["youtube"]).
func ActionsForYouTubeScopes(scopes []string) []Action {
	for _, s := range scopes {
		if s == ScopeYouTubeModeration {
			return []Action{ActionBan}
		}
	}
	return nil
}

// RequiredYouTubeScope returns the YouTube OAuth scope an action needs, or "" if the
// action is not a YouTube moderation action.
func RequiredYouTubeScope(a Action) string {
	if a == ActionBan {
		return ScopeYouTubeModeration
	}
	return ""
}

// Discord moderation permission bits. Unlike Twitch/Kick/YouTube, Discord moderation
// authority is a GUILD-level bot permission (granted at invite time), not a per-user
// OAuth scope — so the "opt-in" is choosing the elevated invite. These bit values are
// from the Discord permissions reference (https://discord.com/developers/docs/topics/permissions).
const (
	// DiscordPermAdministrator implicitly grants every permission.
	DiscordPermAdministrator uint64 = 1 << 3
	// DiscordPermBanMembers permits ban + unban.
	DiscordPermBanMembers uint64 = 1 << 2
	// DiscordPermManageMessages permits deleting other users' messages.
	DiscordPermManageMessages uint64 = 1 << 13
	// DiscordPermModerateMembers permits timeout (communication_disabled_until).
	DiscordPermModerateMembers uint64 = 1 << 40
)

// ModerationBotPermissions is the set of permissions the bot needs to perform every
// supported Discord moderation action. The opt-in re-invite URL requests exactly these
// (on top of the base listener permissions).
const ModerationBotPermissions = DiscordPermManageMessages | DiscordPermModerateMembers | DiscordPermBanMembers

// ActionsForDiscordPermissions maps the bot's EFFECTIVE guild permission bits to the
// moderation actions it can perform. ADMINISTRATOR short-circuits to the full set. The
// result is a subset of PlatformActions["discord"], so the UI only enables what the
// bot's invite actually granted (a bot invited with the legacy listener-only permission
// reports nothing → the source shows the re-invite prompt).
func ActionsForDiscordPermissions(perms uint64) []Action {
	if perms&DiscordPermAdministrator != 0 {
		return []Action{ActionDelete, ActionTimeout, ActionBan, ActionUnban}
	}
	var out []Action
	if perms&DiscordPermManageMessages != 0 {
		out = append(out, ActionDelete)
	}
	if perms&DiscordPermModerateMembers != 0 {
		out = append(out, ActionTimeout)
	}
	if perms&DiscordPermBanMembers != 0 {
		out = append(out, ActionBan, ActionUnban)
	}
	return out
}

// RequiredDiscordPermission returns the permission bit an action needs, or 0 if the
// action is not a Discord moderation action.
func RequiredDiscordPermission(a Action) uint64 {
	switch a {
	case ActionDelete:
		return DiscordPermManageMessages
	case ActionTimeout:
		return DiscordPermModerateMembers
	case ActionBan, ActionUnban:
		return DiscordPermBanMembers
	default:
		return 0
	}
}

// Actor is who is performing a moderation action, and on whose behalf.
//
// It travels with every dispatch because *whose credential acts* is the load-bearing
// question of ADR-0048, and it must never be inferred from the caller's id alone: an owner acts
// with their own broadcaster credential, a delegated moderator with their own moderator
// credential against the OWNER's channel, and there is no fallback between them. Passing this
// explicitly is what makes a dispatcher that ignores the distinction a compile-time change rather
// than a silent one.
type Actor struct {
	// UserID is the human who pressed the button, and whose credential must perform the call.
	UserID string
	// Role is RoleOwner or RoleModerator as resolved on the overlay.
	Role string
	// OwnerUserID is the overlay owner the action is performed for. Equals UserID when the owner
	// acts themselves; for a moderator it is whose channel is being moderated, and it is the only
	// identity the owner-reach anchor may be resolved against.
	OwnerUserID string
	// GrantID is the delegation the moderator is acting under. Empty for an owner.
	GrantID string
}

// IsModerator reports whether a delegated moderator is acting.
func (a Actor) IsModerator() bool { return a.Role == RoleModerator }

// Roles an Actor can hold. These mirror repository.Role* — duplicated rather than imported
// because models must not depend on the repository (the dependency runs the other way).
const (
	RoleOwner     = "owner"
	RoleModerator = "moderator"
)

// DispatchRequest carries the identifiers a platform moderation call needs. It is
// the platform-agnostic input the handler hands to a Dispatcher.
type DispatchRequest struct {
	Platform        string
	ChannelID       string
	NativeMessageID string // delete: the platform's own message id
	TargetUserID    string // timeout/ban/unban
	DurationSeconds int    // timeout (0 => permanent ban)
	Reason          string
}

// DispatchOutcome is the result classification of a platform dispatch.
type DispatchOutcome int

const (
	// DispatchDryRun: no real client is wired for the platform yet, so no platform
	// call was made. The handler still emits the reflect-back event.
	DispatchDryRun DispatchOutcome = iota
	// DispatchPerformed: the platform accepted the action.
	DispatchPerformed
	// DispatchReauthRequired: the owner's token lacks the required scope (or the
	// platform rejected it as unauthorized). The handler returns 403 + missing_scopes.
	DispatchReauthRequired
	// DispatchNoCredential: the actor holds no moderator credential for this channel.
	// The handler returns 422.
	DispatchNoCredential
	// DispatchOwnerUnverified: the overlay owner cannot prove they control this channel, so
	// there is nothing for them to delegate (ADR-0048's owner-reach anchor). Delegation never
	// exceeds what the owner could do themselves. The handler returns 403 with copy aimed at the
	// OWNER — reconnecting their account is the fix, and the moderator cannot perform it.
	DispatchOwnerUnverified
	// DispatchNotPlatformModerator: the platform accepted the credential and refused the action
	// because this person is not a moderator of that channel. Only reported for a delegated
	// moderator, and only once the scope pre-check has already passed — which is what makes the
	// inference sound. It is the platform answering the question ADR-0048 defers to it, and the
	// remediation is the streamer adding them in the platform's own tools, never a re-consent.
	DispatchNotPlatformModerator
	// DispatchDelegationUnsupported: this platform's delegated path is not built yet, so a
	// delegated action must be refused rather than fall through to whatever credential the
	// dispatcher would otherwise use. Load-bearing for Discord in particular, where the actor is
	// always the shared bot: without this, a delegated action would execute with the bot's full
	// guild authority and no check that the moderator holds any of it.
	DispatchDelegationUnsupported

	// The four outcomes below are Discord's, and they exist because Discord is the one platform
	// where no external party re-checks a delegated moderator (ADR-0048's platform-attested
	// model). Everywhere else a refusal comes back from the platform and needs no vocabulary of
	// its own; here All-Chat is the authority, so each way its own check can refuse has to be
	// nameable — otherwise a volunteer is told "it didn't work" with no route to a fix, and the
	// fixes genuinely differ.

	// DispatchModNotLinked: the acting moderator has not linked a Discord account, so there is no
	// snowflake to read their guild permissions against. Theirs to clear, and the only Discord
	// outcome that is — which is why it is not folded into DispatchNoCredential: that one means a
	// missing OAuth *credential*, and the Discord link deliberately stores no token at all, so the
	// remedy is a different flow.
	DispatchModNotLinked
	// DispatchModNotInGuild: Discord answered 404 for the moderator in that guild. They are not a
	// member, so they hold no permissions there and All-Chat must not lend them the bot's. Only
	// the streamer can clear it, by inviting them to the server.
	DispatchModNotInGuild
	// DispatchModLacksPermission: the moderator is in the guild but could not perform this action
	// themselves. The bot could — that is the point of the intersection: All-Chat never lets
	// someone do through the bot what Discord would refuse them directly. The streamer clears it
	// by giving them a role that carries the permission.
	DispatchModLacksPermission
	// DispatchModBelowTarget: Discord's role hierarchy refuses this member operation, because the
	// moderator's highest role does not sit strictly above the target's. All-Chat has to evaluate
	// this itself: Discord hierarchy-gates the *actor*, and on a delegated action the actor is the
	// bot, which typically outranks everyone.
	DispatchModBelowTarget
	// DispatchBotMissingPermission: the bot itself was never invited with the permission this
	// action needs, so no moderator can borrow it. The streamer clears it by re-inviting the bot —
	// never an OAuth re-consent, which cannot touch guild permissions.
	DispatchBotMissingPermission
)

// DispatchResult is what a Dispatcher reports back to the handler.
type DispatchResult struct {
	Outcome        DispatchOutcome
	MissingScopes  []string // populated on DispatchReauthRequired
	PlatformStatus string   // platform detail for the audit row
	// CredentialUserID is whose OAuth token actually performed the call, and PlatformActorID the
	// id sent as the platform's moderator field. Together they are the machine-checkable proof
	// that a delegated action never fell back to the owner's credential (ADR-0048), which is why
	// they are reported back rather than assumed by the handler. Both are empty where no per-user
	// credential acts — Discord, where the shared bot is always the actor, and dry runs.
	CredentialUserID string
	PlatformActorID  string
}

// PlatformSupported reports whether the platform has any moderation support at all.
func PlatformSupported(platform string) bool {
	return len(PlatformActions[platform]) > 0
}

// DeleteRequest removes a single message. native_message_id is the platform's own
// message id (used for the platform API call); target_uuid is the internal id the
// frontend optimistically marks (the reflect-back event re-resolves it via the registry).
type DeleteRequest struct {
	Platform        string `json:"platform" binding:"required"`
	ChannelID       string `json:"channel_id" binding:"required"`
	NativeMessageID string `json:"native_message_id" binding:"required"`
	TargetUUID      string `json:"target_uuid"`
}

// TimeoutRequest removes a user's messages for DurationSeconds.
type TimeoutRequest struct {
	Platform        string `json:"platform" binding:"required"`
	ChannelID       string `json:"channel_id" binding:"required"`
	TargetUserID    string `json:"target_user_id" binding:"required"`
	TargetUsername  string `json:"target_username"`
	DurationSeconds int    `json:"duration_seconds" binding:"required,gt=0"`
	Reason          string `json:"reason"`
}

// BanRequest permanently removes a user.
type BanRequest struct {
	Platform       string `json:"platform" binding:"required"`
	ChannelID      string `json:"channel_id" binding:"required"`
	TargetUserID   string `json:"target_user_id" binding:"required"`
	TargetUsername string `json:"target_username"`
	Reason         string `json:"reason"`
}

// UnbanRequest lifts a ban or timeout.
type UnbanRequest struct {
	Platform     string `json:"platform" binding:"required"`
	ChannelID    string `json:"channel_id" binding:"required"`
	TargetUserID string `json:"target_user_id" binding:"required"`
}

// SourceCapability describes whether one overlay source can be moderated and how.
type SourceCapability struct {
	Platform    string `json:"platform"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Moderatable bool   `json:"moderatable"`
	// CanSend reports whether the owner can send chat to this source from the monitor
	// view (the chat-send scope is granted). Independent of Moderatable/Actions.
	CanSend bool     `json:"can_send"`
	Reason  string   `json:"reason,omitempty"`
	Actions []Action `json:"actions"`
}

// Capabilities is the response of the capabilities endpoint: which role the caller holds on the
// overlay, whether the moderation feature gate is open for it, and the per-source capability.
type Capabilities struct {
	// Role is the caller's role: owner, moderator or none (ADR-0048). A caller with no role and a
	// caller asking about an overlay that does not exist get the identical body, so this must
	// never be read as evidence that an overlay exists.
	Role string `json:"role"`
	// IsOwner is Role == owner, kept as its own field because it is the flag the dashboard has
	// always branched on and the two must never drift apart.
	IsOwner bool `json:"is_owner"`
	// Enabled reports whether the moderation feature gate (ADR-0008) is open. It is keyed on the
	// overlay OWNER, never the caller: a premium streamer's moderators moderate for free, and the
	// action path decides the same way. False for a caller with no role.
	Enabled bool `json:"enabled"`
	// CanModerate is the single flag the UI switches its controls on: the caller holds a role AND
	// the owner's plan has moderation open. Which actions are actually available per source is
	// still decided source by source.
	CanModerate bool `json:"can_moderate"`
	// DelegatedActions is the grant's action set, present for a moderator only.
	//
	// It is what a source's "Connect to moderate" must request scopes for: a source that needs
	// consent reports no usable actions yet, so without this the consent screen would have to
	// guess — and guessing high would ask a volunteer for ban scope the streamer never delegated.
	DelegatedActions []Action           `json:"delegated_actions,omitempty"`
	Sources          []SourceCapability `json:"sources"`
}

// ActionsForModeratorScopes maps a delegated moderator's OWN granted scopes to the actions they
// can perform on a platform.
//
// This is the same scope→action mapping the owner path uses; what differs is whose credential the
// scopes came from. Discord is absent by construction: its authority is the shared bot's guild
// permissions plus a link to the moderator's Discord account, neither of which is an OAuth scope
// the moderator grants (see ReasonNeedsDiscordLink).
func ActionsForModeratorScopes(platform string, scopes []string) []Action {
	switch platform {
	case "twitch":
		return ActionsForTwitchScopes(scopes)
	case "kick":
		return ActionsForKickScopes(scopes)
	case "youtube":
		return ActionsForYouTubeScopes(scopes)
	default:
		return nil
	}
}

// IntersectActions returns the actions present in both sets, in the order of the first.
//
// A delegated moderator's usable actions are the intersection of what the platform supports, what
// their own scopes allow, and what the streamer delegated — so this is applied, not assumed, and
// the result can legitimately be empty.
func IntersectActions(actions []Action, allowed []string) []Action {
	permitted := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		permitted[a] = true
	}
	out := make([]Action, 0, len(actions))
	for _, a := range actions {
		if permitted[string(a)] {
			out = append(out, a)
		}
	}
	return out
}
