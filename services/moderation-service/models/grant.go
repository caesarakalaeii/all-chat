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
	"errors"
	"fmt"
	"strings"
	"time"
)

// Lifecycle states of a delegation grant (ADR-0048). Only `active` authorizes anything;
// ResolveOverlayAccess ignores every other state.
const (
	// GrantStatusPending: an invite exists but nobody has redeemed it, so it is bound to no
	// account yet.
	GrantStatusPending = "pending"
	// GrantStatusActive: redeemed and live.
	GrantStatusActive = "active"
	// GrantStatusSuspended: dormancy-suspended after 90 idle days. Only the OWNER reactivates —
	// if re-consenting lifted it, the suspension would be a speed bump rather than a control.
	GrantStatusSuspended = "suspended"
	// GrantStatusRevoked: terminal. The row is kept for history.
	GrantStatusRevoked = "revoked"
)

// ModeratorsPerOverlayCap bounds how many live grants an overlay may carry.
//
// Enforced at invite time only (the 11th invite is refused with 409), never retroactively: a
// smaller cap must not silently cut a working mod team off mid-stream. Admins bypass it.
const ModeratorsPerOverlayCap = 10

// ErrNoActions reports an explicitly empty action list. Distinct from an absent list, which means
// "use the default" — collapsing the two would grant actions the caller never asked for.
var ErrNoActions = errors.New("a grant must delegate at least one action")

// DefaultDelegatedActions is what an invite grants when the streamer does not choose: the pair a
// volunteer moderator needs day to day. Ban and unban stay opt-in.
var DefaultDelegatedActions = []string{string(ActionDelete), string(ActionTimeout)}

// delegatableActions is the closed allowlist of verbs a grant may carry, in canonical order.
//
// This is the ONLY admission point for a grant's actions, and it is load-bearing beyond tidiness:
// ModerationScopesForActions downstream also accepts "engagement", which maps to
// channel:read:polls / channel:read:predictions. An unfiltered string reaching it would widen a
// moderator's consent screen to scopes on their own channel that have nothing to do with
// moderation (an ADR-0012 regression). Chat send is absent by decision — it is a distinct,
// higher-trust capability and stays owner-only in v1.
var delegatableActions = []string{
	string(ActionDelete), string(ActionTimeout), string(ActionBan), string(ActionUnban),
}

// IsDelegatableAction reports whether action may be carried by a delegation grant.
func IsDelegatableAction(action string) bool {
	for _, a := range delegatableActions {
		if a == action {
			return true
		}
	}
	return false
}

// NormalizeDelegatedActions validates a requested action set and returns it deduplicated in
// canonical order, so the stored array, the API response and the UI never disagree about order.
//
// A nil list yields DefaultDelegatedActions; an empty-but-present list is ErrNoActions.
func NormalizeDelegatedActions(requested []string) ([]string, error) {
	if requested == nil {
		return append([]string(nil), DefaultDelegatedActions...), nil
	}
	if len(requested) == 0 {
		return nil, ErrNoActions
	}

	wanted := make(map[string]bool, len(requested))
	for _, action := range requested {
		if !IsDelegatableAction(action) {
			return nil, fmt.Errorf("%q is not a delegatable moderation action", action)
		}
		wanted[action] = true
	}

	out := make([]string, 0, len(wanted))
	for _, a := range delegatableActions {
		if wanted[a] {
			out = append(out, a)
		}
	}
	return out, nil
}

// NormalizeDelegatedPlatforms validates the per-platform legs to enable and returns them
// deduplicated in the order given.
//
// A nil or empty list is legitimate and means "no platform enabled yet": absence is disablement
// (migration 080), so a grant delegates nothing until the streamer opts a platform in. That is
// what keeps Discord — the one platform with no external authority behind it — off by default.
func NormalizeDelegatedPlatforms(requested []string) ([]string, error) {
	out := make([]string, 0, len(requested))
	seen := make(map[string]bool, len(requested))
	for _, platform := range requested {
		// PlatformSupported already excludes TikTok (no moderation API at all) and
		// shared_overlay (someone else's channel), which is exactly the exclusion a grant needs.
		if !PlatformSupported(platform) {
			return nil, fmt.Errorf("%q cannot be moderated, so it cannot be delegated", platform)
		}
		if seen[platform] {
			continue
		}
		seen[platform] = true
		out = append(out, platform)
	}
	return out, nil
}

// PreBindablePlatform reports whether an invite may be pre-bound to an account on this platform.
//
// Pre-binding is only honest where acceptance can actually resolve the redeeming user's id on that
// platform and compare it. Twitch is the only one today — it is where the "pick from your
// moderators" flow reads its ids from, and an All-Chat account carries a verified Twitch id. For
// the others we would be storing a constraint that silently does nothing, so the API refuses it
// rather than pretending.
func PreBindablePlatform(platform string) bool { return platform == "twitch" }

// maxInviteeLabelLen mirrors overlay_moderators.invitee_label. Trimmed rather than rejected: the
// label is display-only and a truncated name beats a failed invite.
const maxInviteeLabelLen = 120

// TrimInviteeLabel normalizes the streamer-supplied display label for an invitee.
func TrimInviteeLabel(label string) string {
	label = strings.TrimSpace(label)
	if len(label) > maxInviteeLabelLen {
		label = strings.ToValidUTF8(label[:maxInviteeLabelLen], "")
	}
	return label
}

// ---------------------------------------------------------------------------
// Owner-facing request/response DTOs.
// ---------------------------------------------------------------------------

// CreateInviteRequest mints an invite for one moderator.
type CreateInviteRequest struct {
	// Actions to delegate. Absent = DefaultDelegatedActions; [] = 400.
	Actions []string `json:"actions"`
	// Platforms whose legs start enabled. Absent = none, and the grant delegates nothing until
	// the streamer enables one.
	Platforms []string `json:"platforms"`
	// InviteeLabel is display-only ("Sarah, my Twitch mod"), so the list is readable before
	// anyone accepts.
	InviteeLabel string `json:"invitee_label"`
	// ExpectedPlatform / ExpectedPlatformUserID optionally bind the invite to one platform
	// account, so redeeming it from the wrong account fails with an explanation instead of
	// silently granting the wrong person.
	ExpectedPlatform       string `json:"expected_platform"`
	ExpectedPlatformUserID string `json:"expected_platform_user_id"`
}

// InviteCreated is the response to a fresh invite. InviteToken appears here and nowhere else,
// ever again.
type InviteCreated struct {
	GrantID string `json:"grant_id"`
	// InviteToken is the single-use secret. It is not stored in retrievable form, so a streamer
	// who loses it mints a new invite; there is no "show again".
	InviteToken  string    `json:"invite_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Actions      []string  `json:"actions"`
	Platforms    []string  `json:"platforms"`
	InviteeLabel string    `json:"invitee_label,omitempty"`
}

// GrantPlatformLeg is one platform's enablement on a grant, plus its last known readiness.
type GrantPlatformLeg struct {
	Platform string `json:"platform"`
	Enabled  bool   `json:"enabled"`
	// Verification is TELEMETRY for the owner's panel — the platform's answer at action time is
	// the authority. It is never consulted when authorizing, so a transient 403 cannot lock out
	// a legitimate moderator.
	Verification string     `json:"verification"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
}

// ModeratorGrant is one delegation as the overlay owner sees it.
type ModeratorGrant struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// ModeratorUserID is empty while the invite is unredeemed.
	ModeratorUserID string `json:"moderator_user_id,omitempty"`
	// DisplayName is captured at accept time so the list still names the person after their
	// account is gone.
	DisplayName  string             `json:"display_name,omitempty"`
	InviteeLabel string             `json:"invitee_label,omitempty"`
	Actions      []string           `json:"actions"`
	Platforms    []GrantPlatformLeg `json:"platforms"`
	// ExpectedPlatform / ExpectedAccount echo a pre-binding so the panel can show who the
	// outstanding invite is for.
	ExpectedPlatform string     `json:"expected_platform,omitempty"`
	ExpectedAccount  string     `json:"expected_account,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	AcceptedAt       *time.Time `json:"accepted_at,omitempty"`
	// InviteExpiresAt is set only while the invite is outstanding.
	InviteExpiresAt *time.Time `json:"invite_expires_at,omitempty"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	LastActionAt    *time.Time `json:"last_action_at,omitempty"`
}

// ModeratorList is the owner's Moderators panel payload. Cap and Used are reported so the UI can
// explain a refused invite before the streamer hits it.
type ModeratorList struct {
	Moderators []ModeratorGrant `json:"moderators"`
	Cap        int              `json:"cap"`
	Used       int              `json:"used"`
}

// UpdateGrantRequest changes what an existing grant may do. Both fields are optional; an absent
// field leaves that dimension untouched.
type UpdateGrantRequest struct {
	Actions []string `json:"actions"`
	// Platforms maps a platform to its enablement. Only the listed platforms change, so the UI
	// can send one toggle without restating the rest.
	Platforms map[string]bool `json:"platforms"`
}

// ---------------------------------------------------------------------------
// Moderator-facing DTOs.
// ---------------------------------------------------------------------------

// InviteTokenRequest carries an invite secret. It is a POST body rather than a path parameter on
// purpose: a secret in a URL ends up in access logs, proxy logs and Referer headers.
type InviteTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

// InvitePreview is what an invite holder is shown before they accept: who is asking, for which
// overlay, and what they are agreeing to do.
//
// The overlay id is deliberately absent. An overlay UUID already grants chat READ to anyone
// holding it, so it is disclosed at acceptance rather than to everyone who merely opens the link.
type InvitePreview struct {
	OverlayName      string             `json:"overlay_name"`
	OwnerDisplayName string             `json:"owner_display_name"`
	Actions          []string           `json:"actions"`
	Platforms        []GrantPlatformLeg `json:"platforms"`
	ExpiresAt        time.Time          `json:"expires_at"`
	InviteeLabel     string             `json:"invitee_label,omitempty"`
	// ExpectedPlatform / ExpectedAccount are set when the invite is pre-bound, so the page can
	// say "this invite is for @sarah" before the redeem attempt fails.
	ExpectedPlatform string `json:"expected_platform,omitempty"`
	ExpectedAccount  string `json:"expected_account,omitempty"`
}

// InviteAccepted confirms a redeemed invite and hands over the overlay the moderator may now act
// on.
type InviteAccepted struct {
	GrantID          string             `json:"grant_id"`
	OverlayID        string             `json:"overlay_id"`
	OverlayName      string             `json:"overlay_name"`
	OwnerDisplayName string             `json:"owner_display_name"`
	Actions          []string           `json:"actions"`
	Platforms        []GrantPlatformLeg `json:"platforms"`
}
