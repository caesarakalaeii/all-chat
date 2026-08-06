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
const (
	// ReasonUnsupportedPlatform: the platform has no usable moderation API (TikTok).
	ReasonUnsupportedPlatform = "unsupported_platform"
	// ReasonMissingScope: the owner has not granted the moderation OAuth scopes for
	// this platform/account yet (covers both "never opted in" and "partially granted").
	// Cleared via the opt-in re-consent flow.
	ReasonMissingScope = "missing_scope"
	// ReasonNotOwner: the requester does not own this overlay.
	ReasonNotOwner = "not_owner"
)

// PlatformActions is the moderation support matrix per platform in All-Chat.
// TikTok is intentionally absent: it has no official moderation API, so it is
// always reported unsupported rather than offered a button that cannot work.
// shared_overlay is absent by design: a recipient must not moderate the original
// streamer's channel (least-privilege / owner-only authorization).
var PlatformActions = map[string][]Action{
	"twitch": {ActionDelete, ActionTimeout, ActionBan, ActionUnban},
	"kick":   {ActionTimeout, ActionBan, ActionUnban},
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
	for _, supported := range PlatformActions[platform] {
		if supported == a {
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

// Kick moderation OAuth scope. Kick's public API gates ban/timeout/unban behind a
// single scope; there is no single-message delete endpoint, so Kick never grants
// ActionDelete (see PlatformActions["kick"]). Requested only through the opt-in
// re-consent flow (never bundled into login/add-source), per ADR-0017.
const ScopeKickModeration = "moderation:ban"

// ActionsForKickScopes maps a Kick token's granted scopes to moderation actions. The
// result is a subset of PlatformActions["kick"], so the UI only enables what the
// granted scope allows.
func ActionsForKickScopes(scopes []string) []Action {
	for _, s := range scopes {
		if s == ScopeKickModeration {
			return []Action{ActionTimeout, ActionBan, ActionUnban}
		}
	}
	return nil
}

// RequiredKickScope returns the Kick OAuth scope an action needs, or "" if the action
// is not a Kick moderation action. Used to pre-check a resolved token before calling
// the Kick API and to populate missing_scopes on a re-consent prompt.
func RequiredKickScope(a Action) string {
	switch a {
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
	// DispatchNoCredential: the owner holds no moderator credential for this channel.
	// The handler returns 422.
	DispatchNoCredential
)

// DispatchResult is what a Dispatcher reports back to the handler.
type DispatchResult struct {
	Outcome        DispatchOutcome
	MissingScopes  []string // populated on DispatchReauthRequired
	PlatformStatus string   // platform detail for the audit row
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

// Capabilities is the response of the capabilities endpoint: whether the caller
// owns the overlay, whether the moderation feature gate is open for them, and the
// per-source moderation capability.
type Capabilities struct {
	IsOwner bool `json:"is_owner"`
	// Enabled reports whether the moderation feature gate (ADR-0008) is open for
	// this user. When false the owner is outside the rollout cohort: the dashboard
	// hides moderation controls (the action endpoints are independently gated and
	// would 403). Always false for non-owners.
	Enabled bool               `json:"enabled"`
	Sources []SourceCapability `json:"sources"`
}
