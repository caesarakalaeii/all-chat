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

/** Why a source cannot be moderated, when `moderatable` is false. */
export type ModerationUnavailableReason = 'unsupported_platform' | 'missing_scope'

/** Per-source moderation capability, as returned by the capabilities endpoint. */
export interface SourceCapability {
  platform: ModerationPlatform
  channel_id: string
  channel_name: string
  moderatable: boolean
  reason?: ModerationUnavailableReason
  actions: ModerationAction[]
}

/** Overlay-wide moderation capabilities for the logged-in viewer. */
export interface ModerationCapabilities {
  is_owner: boolean
  /**
   * Whether the moderation feature gate (ADR-0008) is open for this viewer. When
   * false, the owner is outside the rollout cohort: controls are hidden and the
   * action endpoints would 403. Always false for non-owners.
   */
  enabled: boolean
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
