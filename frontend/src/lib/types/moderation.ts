/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Chat-moderation types (Phase 1).
 *
 * Mirrors the backend moderation contract: capabilities describe, per source,
 * whether the logged-in viewer owns the overlay and which actions each platform
 * source supports. Re-uses `ModKind`/`ModEntry`/`DeletionMetadata` from the
 * overlay view-model / message types — moderation actions are expressed through
 * those existing structures, not duplicated here.
 */

/** Platforms a chat source can report. Mirrors ChatMessage['platform']. */
export type ModerationPlatform = 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'system'

/** A single moderation action the backend can perform on a source. */
export type ModerationAction = 'delete' | 'timeout' | 'ban' | 'unban'

/**
 * Why a source cannot be moderated, when `moderatable` is false.
 *
 * The first two can describe either role. The last three only ever describe a
 * delegated moderator's source (ADR-0048), and they differ in who can clear
 * them: `not_delegated` needs the streamer, `needs_consent` needs the
 * moderator, and `needs_discord_link` needs neither — nothing can clear it yet.
 */
export type ModerationUnavailableReason =
  | 'unsupported_platform'
  | 'missing_scope'
  | 'not_delegated'
  | 'needs_consent'
  | 'needs_discord_link'

/** The caller's role on an overlay's moderation write-path. */
export type ModerationRole = 'owner' | 'moderator' | 'none'

/** Per-source moderation capability, as returned by the capabilities endpoint. */
export interface SourceCapability {
  platform: ModerationPlatform
  channel_id: string
  channel_name: string
  moderatable: boolean
  reason?: ModerationUnavailableReason
  actions: ModerationAction[]
  /**
   * Whether the streamer can send chat messages on this source from the monitor.
   * Optional; absent ⇒ treat as false (no send scope granted / unsupported).
   */
  can_send?: boolean
}

/** Overlay-wide moderation capabilities for the logged-in viewer. */
export interface ModerationCapabilities {
  /**
   * The caller's role. A caller with no role and an overlay that does not exist
   * produce the identical payload, so `none` must never be read as proof that
   * the overlay is real.
   */
  role: ModerationRole
  is_owner: boolean
  /**
   * Whether the moderation feature gate (ADR-0008) is open. Keyed on the overlay
   * OWNER, never the caller: a premium streamer's moderators moderate for free,
   * so a moderator seeing `false` here must be shown the streamer's plan as the
   * cause, never an upgrade prompt.
   */
  enabled: boolean
  /**
   * The single flag the controls switch on: the caller holds a role AND the
   * owner's plan has moderation open. Which actions are actually available is
   * still decided source by source.
   */
  can_moderate: boolean
  /**
   * The grant's action set, present for a moderator only.
   *
   * What a `needs_consent` source's "Connect to moderate" requests scopes for: such a
   * source reports no usable actions yet, and guessing high would ask a volunteer for
   * ban scope the streamer never delegated.
   */
  delegated_actions?: ModerationAction[]
  sources: SourceCapability[]
}

/**
 * Platforms that expose a moderation API at all. A source whose platform is not
 * in this set renders disabled controls regardless of `actions` (e.g. TikTok).
 */
export const MODERATABLE_PLATFORMS: ReadonlySet<string> = new Set([
  'twitch',
  'youtube',
  'kick',
  'discord',
])

/** Preset timeout durations offered in the per-user moderation popover. */
export const TIMEOUT_PRESETS: ReadonlyArray<{ label: string; seconds: number }> = [
  { label: '1m', seconds: 60 },
  { label: '10m', seconds: 600 },
  { label: '1h', seconds: 3600 },
]

// ---------------------------------------------------------------------------
// Delegated moderators (ADR-0048)
// ---------------------------------------------------------------------------

/**
 * Lifecycle state of a delegation grant. `revoked` never reaches the client —
 * the roster endpoint excludes it — but it is in the union because the backend
 * column allows it and a future activity view will show it.
 */
export type GrantStatus = 'pending' | 'active' | 'suspended' | 'revoked'

/**
 * Last known platform moderator status for one leg of a grant.
 *
 * TELEMETRY ONLY. The platform's own answer at action time is the authority, so
 * this must be rendered as advisory readiness and never as an authorization
 * verdict: a single transient 403 can set `not_a_moderator`, and treating that
 * as a denial would lock out a legitimate moderator.
 */
export type GrantVerification =
  | 'unverified'
  | 'verified'
  | 'not_a_moderator'
  | 'needs_consent'
  | 'needs_discord_link'
  | 'unavailable'

/** Platforms a grant can delegate. TikTok has no moderation API, so it is absent. */
export type DelegatablePlatform = 'twitch' | 'youtube' | 'kick' | 'discord'

/** One platform's enablement on a grant. An absent leg means disabled. */
export interface GrantPlatformLeg {
  platform: DelegatablePlatform
  enabled: boolean
  verification: GrantVerification
  verified_at?: string
}

/** One delegation grant as the overlay owner sees it. */
export interface ModeratorGrant {
  id: string
  status: GrantStatus
  /** Absent while the invite is unredeemed. */
  moderator_user_id?: string
  /** Captured at accept time, so it survives the moderator deleting their account. */
  display_name?: string
  /** What the streamer typed when creating the invite. Display only. */
  invitee_label?: string
  actions: ModerationAction[]
  platforms: GrantPlatformLeg[]
  /** Set when the invite is pre-bound to one platform account. */
  expected_platform?: DelegatablePlatform
  expected_account?: string
  created_at: string
  accepted_at?: string
  /** Present only while an invite is still outstanding. */
  invite_expires_at?: string
  suspended_at?: string
  last_action_at?: string
}

/**
 * The owner's Moderators roster. `used`/`cap` let the UI refuse an over-cap
 * invite before the request, rather than explaining a 409 afterwards.
 */
export interface ModeratorList {
  moderators: ModeratorGrant[]
  cap: number
  used: number
}

/** Body for creating an invite. Omit a field to leave it at its default. */
export interface CreateInviteRequest {
  /**
   * Absent means the backend default (`delete` + `timeout`). An explicitly empty
   * array is a 400 rather than being widened, so never send `[]` to mean
   * "unchanged" — omit the key.
   */
  actions?: ModerationAction[]
  /** Absent enables nothing: absence IS disablement, so the grant does nothing yet. */
  platforms?: DelegatablePlatform[]
  invitee_label?: string
  /** Twitch only — the one platform where acceptance can verify the account. */
  expected_platform?: 'twitch'
  expected_platform_user_id?: string
}

/**
 * A freshly minted invite. `invite_token` is returned exactly once and stored
 * only as a SHA-256 digest, so it can never be re-displayed: a lost invite is
 * re-minted, not recovered.
 */
export interface InviteCreated {
  grant_id: string
  invite_token: string
  expires_at: string
  actions: ModerationAction[]
  platforms: DelegatablePlatform[]
  invitee_label?: string
}

/**
 * What an invite holder is shown before agreeing to moderate.
 *
 * Deliberately carries NO `overlay_id`: an overlay UUID already grants chat read
 * to anyone holding it, so it is disclosed on acceptance rather than to everyone
 * who merely opens the link.
 */
export interface InvitePreview {
  overlay_name: string
  owner_display_name: string
  actions: ModerationAction[]
  platforms: GrantPlatformLeg[]
  expires_at: string
  invitee_label?: string
  /** Set when the invite is pre-bound to one platform account. */
  expected_platform?: DelegatablePlatform
  expected_account?: string
}

/** A redeemed invite, with the overlay the moderator may now act on. */
export interface InviteAccepted {
  grant_id: string
  overlay_id: string
  overlay_name: string
  owner_display_name: string
  actions: ModerationAction[]
  platforms: GrantPlatformLeg[]
}

/** Body for narrowing or widening an existing grant. */
export interface UpdateGrantRequest {
  /** Omit to leave the action set untouched. */
  actions?: ModerationAction[]
  /** Partial map: only the platforms named here change. */
  platforms?: Partial<Record<DelegatablePlatform, boolean>>
}

/**
 * Machine-readable `code` on every delegation error body. Switch on these
 * rather than parsing the human-readable `error` string.
 */
export type DelegationErrorCode =
  | 'moderator_cap_reached'
  | 'delegation_unavailable'
  | 'grant_not_found'
  | 'invalid_request'
  | 'invite_not_found'
  | 'invite_expired'
  | 'already_moderator'
  | 'owner_cannot_accept'
  | 'invite_bound_to_other_account'

/**
 * One channel a moderator has been handed, as the MODERATOR sees it.
 *
 * The mirror image of `ModeratorGrant`: the owner's view names the moderator,
 * this one names the streamer. It carries no user ids — a volunteer learns who
 * delegated to them, not who else moderates there.
 */
export interface Delegation {
  grant_id: string
  /** The only way to reach the overlay: `GET /api/v1/overlays` is owner-filtered. */
  overlay_id: string
  overlay_name: string
  owner_display_name: string
  /** `active` or `suspended`. A suspended grant is listed, not hidden. */
  status: GrantStatus
  actions: ModerationAction[]
  platforms: GrantPlatformLeg[]
  /**
   * Whether the streamer's plan currently has delegated moderation open. False
   * means the streamer's plan lapsed — say so, and never link a volunteer to
   * `/upgrade` for a plan that is not theirs to buy.
   */
  available: boolean
  accepted_at?: string
  last_action_at?: string
}

/** The "channels I moderate" payload. */
export interface DelegationList {
  delegations: Delegation[]
}

/** Platforms a grant leg may cover, in display order. */
export const DELEGATABLE_PLATFORMS: ReadonlyArray<DelegatablePlatform> = [
  'twitch',
  'youtube',
  'kick',
  'discord',
]

/** Every action a grant may delegate, in the backend's canonical order. */
export const DELEGATABLE_ACTIONS: ReadonlyArray<ModerationAction> = [
  'delete',
  'timeout',
  'ban',
  'unban',
]
