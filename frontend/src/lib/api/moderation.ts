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
 * Moderation API client (Phase 1).
 *
 * Mirrors the shape of `lib/api/discord.ts`: thin wrappers over the shared
 * `apiClient` (which attaches `Authorization: Bearer <jwt>`). Every mutating
 * call sends a fresh `Idempotency-Key` (a UUID) so a double-click dedupes
 * server-side and only performs the action once.
 */

import { ApiError, apiClient } from './client'
import type {
  CreateInviteRequest,
  DelegatablePlatform,
  DelegationErrorCode,
  InviteCreated,
  ModerationAction,
  ModerationCapabilities,
  ModerationPlatform,
  ModeratorGrant,
  ModeratorList,
  UpdateGrantRequest,
} from '@/lib/types/moderation'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

/**
 * Whether a failed moderation action needs the streamer to re-authorize the platform.
 *
 * The moderation-service returns 403 with `requires_reauth: true` whenever the stored
 * broadcaster token can no longer perform the action — the scope was never granted, the
 * grant lapsed, or Helix rejected the token (401) even after a refresh. In every one of
 * those cases the only fix is a fresh opt-in re-consent (force_verify superset), so the
 * monitor view must surface the re-consent CTA rather than a dead-end "action failed".
 *
 * Keyed off the body flag, NOT the status code: a 403 without the flag (or the 422
 * "you are not the broadcaster" case) is a genuine authorization failure that re-consent
 * would not resolve, so it must not trigger a re-auth prompt.
 */
export function isModerationReauthError(err: unknown): boolean {
  return err instanceof ApiError && err.data?.requires_reauth === true
}

/** Standard success envelope for every moderation POST. */
export interface ModerationResult {
  status: 'ok'
  dry_run?: boolean
}

export interface DeleteMessageRequest {
  platform: ModerationPlatform
  channel_id: string
  native_message_id: string
  target_uuid: string
}

export interface TimeoutUserRequest {
  platform: ModerationPlatform
  channel_id: string
  target_user_id: string
  target_username: string
  duration_seconds: number
  reason?: string
}

export interface BanUserRequest {
  platform: ModerationPlatform
  channel_id: string
  target_user_id: string
  target_username: string
  reason?: string
}

export interface UnbanUserRequest {
  platform: ModerationPlatform
  channel_id: string
  target_user_id: string
}

/** A fresh idempotency header for one mutating request. */
function idempotencyHeader(): Record<string, string> {
  return { 'Idempotency-Key': crypto.randomUUID() }
}

/** Best available display name for a chat user. */
function targetName(item: ViewItem): string {
  return item.user?.display_name || item.user?.username || ''
}

// --- Request builders (pure) -------------------------------------------------
// Derive a request body straight off a rendered ChatMessage. Kept pure so the
// monitor page and its tests construct identical payloads.

export function buildDeleteRequest(item: ViewItem): DeleteMessageRequest {
  // The platform delete API needs the platform-native message id, not our internal
  // UUID (Twitch Helix wants the IRC/EventSub `Tags["id"]`). The Twitch normalizer
  // preserves it at metadata.twitch_message_id on every live message; target_msg_id
  // only appears on deletion-echo events. Fall back to the internal id as a last
  // resort (e.g. a platform whose native id isn't yet threaded through).
  const nativeId =
    (item.metadata?.twitch_message_id as string | undefined) ??
    (item.metadata?.target_msg_id as string | undefined) ??
    item.id
  return {
    platform: item.platform,
    channel_id: item.channel_id,
    native_message_id: nativeId,
    target_uuid: item.id,
  }
}

export function buildTimeoutRequest(item: ViewItem, durationSeconds: number): TimeoutUserRequest {
  return {
    platform: item.platform,
    channel_id: item.channel_id,
    target_user_id: item.user?.id ?? '',
    target_username: targetName(item),
    duration_seconds: durationSeconds,
  }
}

export function buildBanRequest(item: ViewItem): BanUserRequest {
  return {
    platform: item.platform,
    channel_id: item.channel_id,
    target_user_id: item.user?.id ?? '',
    target_username: targetName(item),
  }
}

export function buildUnbanRequest(item: ViewItem): UnbanUserRequest {
  return {
    platform: item.platform,
    channel_id: item.channel_id,
    target_user_id: item.user?.id ?? '',
  }
}

/**
 * The machine-readable `code` on a delegation error body (ADR-0048), or undefined.
 *
 * Always branch on this rather than on the human-readable `error` string: the copy
 * differs by role and is free to change. Undefined covers both a non-delegation
 * failure and the deliberately code-less owner-only 403, which is identical for an
 * unauthorized caller, a delegated moderator, and an overlay that does not exist —
 * so it must never be read as evidence of a role.
 */
export function delegationErrorCode(err: unknown): DelegationErrorCode | undefined {
  if (!(err instanceof ApiError)) return undefined
  const code = err.data?.code
  return typeof code === 'string' ? (code as DelegationErrorCode) : undefined
}

/**
 * The account a pre-bound invite belongs to, when acceptance was refused because the
 * signed-in user is someone else. Turns a dead end into an instruction.
 */
export function boundInviteAccount(err: unknown): { account: string; platform: string } | null {
  if (delegationErrorCode(err) !== 'invite_bound_to_other_account') return null
  const data = (err as ApiError).data
  return {
    account: typeof data?.expected_account === 'string' ? data.expected_account : '',
    platform: typeof data?.expected_platform === 'string' ? data.expected_platform : '',
  }
}

/** Response of the moderation re-consent endpoint. */
interface ConsentUrlResponse {
  auth_url: string
}

export const moderationApi = {
  getCapabilities(overlayId: string): Promise<ModerationCapabilities> {
    return apiClient.get<ModerationCapabilities>(
      `/api/v1/moderation/overlays/${overlayId}/capabilities`
    )
  },

  /**
   * Fetch the Twitch OAuth re-consent URL to enable moderation, requesting only the
   * scopes for the given actions (least privilege, ADR-0017). The endpoint is on the
   * auth-service (which applies its own JWT); the caller redirects the browser to the
   * returned auth_url. Twitch-only in Phase 1.
   */
  async getTwitchConsentUrl(overlayId: string, actions: ModerationAction[]): Promise<string> {
    const res = await apiClient.get<ConsentUrlResponse>(
      `/api/v1/auth/twitch/moderation/${overlayId}?actions=${actions.join(',')}`
    )
    return res.auth_url
  },

  /**
   * Fetch the Kick OAuth re-consent URL to enable moderation. Kick gates ban/timeout/
   * unban behind a single scope (moderation:ban) and has no single-message delete, so
   * the requested actions are timeout/ban/unban (ADR-0017, least privilege).
   */
  async getKickConsentUrl(overlayId: string, actions: ModerationAction[]): Promise<string> {
    const res = await apiClient.get<ConsentUrlResponse>(
      `/api/v1/auth/kick/moderation/${overlayId}?actions=${actions.join(',')}`
    )
    return res.auth_url
  },

  /**
   * Fetch the YouTube OAuth re-consent URL to enable moderation. YouTube moderation is
   * ban-only (force-ssl scope), re-added only via this opt-in flow (ADR-0017 amends
   * ADR-0012).
   */
  async getYouTubeConsentUrl(overlayId: string, actions: ModerationAction[]): Promise<string> {
    const res = await apiClient.get<ConsentUrlResponse>(
      `/api/v1/auth/youtube/moderation/${overlayId}?actions=${actions.join(',')}`
    )
    return res.auth_url
  },

  deleteMessage(overlayId: string, body: DeleteMessageRequest): Promise<ModerationResult> {
    return apiClient.post<ModerationResult>(
      `/api/v1/moderation/overlays/${overlayId}/delete`,
      body,
      idempotencyHeader()
    )
  },

  timeoutUser(overlayId: string, body: TimeoutUserRequest): Promise<ModerationResult> {
    return apiClient.post<ModerationResult>(
      `/api/v1/moderation/overlays/${overlayId}/timeout`,
      body,
      idempotencyHeader()
    )
  },

  banUser(overlayId: string, body: BanUserRequest): Promise<ModerationResult> {
    return apiClient.post<ModerationResult>(
      `/api/v1/moderation/overlays/${overlayId}/ban`,
      body,
      idempotencyHeader()
    )
  },

  unbanUser(overlayId: string, body: UnbanUserRequest): Promise<ModerationResult> {
    return apiClient.post<ModerationResult>(
      `/api/v1/moderation/overlays/${overlayId}/unban`,
      body,
      idempotencyHeader()
    )
  },

  /**
   * Force the youtube-listener to re-discover the overlay's live stream. Recovers the
   * "platform shows connected but no chat" case where YouTube keeps reporting an
   * ended/crashed stream as live. Owner-only (any owner — not a premium gate); the
   * channel(s) are resolved server-side, so no body is needed. A 429 means the
   * per-channel cooldown is still active.
   */
  forceYouTubeRediscover(overlayId: string): Promise<ModerationResult> {
    return apiClient.post<ModerationResult>(
      `/api/v1/moderation/overlays/${overlayId}/youtube/rediscover`,
      {},
      idempotencyHeader()
    )
  },

  // --- Delegated moderators (ADR-0048) ------------------------------------
  // Owner-only. Every one of these answers a non-owner with the same 403 body an
  // unknown overlay gets, so a failure here says nothing about who the caller is.

  /** The overlay's delegation roster, plus `used`/`cap` for the invite button. */
  listModerators(overlayId: string): Promise<ModeratorList> {
    return apiClient.get<ModeratorList>(`/api/v1/moderation/overlays/${overlayId}/moderators`)
  },

  /**
   * Mint an invite. The returned `invite_token` is the ONLY time the secret exists
   * outside the streamer's clipboard — only its digest is stored, so there is no
   * endpoint that can show it again.
   */
  createInvite(overlayId: string, body: CreateInviteRequest): Promise<InviteCreated> {
    return apiClient.post<InviteCreated>(
      `/api/v1/moderation/overlays/${overlayId}/moderators`,
      body,
      idempotencyHeader()
    )
  },

  /**
   * Narrow or widen a grant. Omit a field to leave that dimension alone; `platforms`
   * is a partial map, so one toggle sends one key.
   */
  updateGrant(
    overlayId: string,
    grantId: string,
    body: UpdateGrantRequest
  ): Promise<ModeratorGrant> {
    return apiClient.patch<ModeratorGrant>(
      `/api/v1/moderation/overlays/${overlayId}/moderators/${grantId}`,
      body
    )
  },

  /** Revoke one grant. Takes effect on the moderator's very next request. */
  revokeGrant(overlayId: string, grantId: string): Promise<{ revoked: boolean }> {
    return apiClient.deleteJson<{ revoked: boolean }>(
      `/api/v1/moderation/overlays/${overlayId}/moderators/${grantId}`
    )
  },

  /**
   * The kill switch: revoke every grant on the overlay, unredeemed invites included.
   * Deliberately still available when the delegation gate is closed.
   */
  revokeAllModerators(overlayId: string): Promise<{ revoked: number }> {
    return apiClient.deleteJson<{ revoked: number }>(
      `/api/v1/moderation/overlays/${overlayId}/moderators`
    )
  },
}
