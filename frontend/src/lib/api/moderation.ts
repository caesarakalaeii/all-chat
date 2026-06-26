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

import { apiClient } from './client'
import type { ModerationAction, ModerationCapabilities, ModerationPlatform } from '@/lib/types/moderation'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

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
}
