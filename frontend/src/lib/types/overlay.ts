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

import type { ChatMessage } from './message'
import type { VisualSettings } from './visual-settings'

/**
 * Overlay and Chat Source Types
 *
 * These types match the backend API responses from the Overlay Manager.
 * Includes types for overlays, configurations, and chat sources.
 */

export interface Overlay {
  id: string
  user_id: string
  name: string
  description?: string
  is_active: boolean
  is_public_for_viewers: boolean
  created_at: string
  updated_at: string
}

export interface OverlayConfig {
  id: string
  overlay_id: string
  display_settings: DisplaySettings
  filter_settings: FilterSettings
  enable_7tv: boolean
  enable_bttv: boolean
  enable_ffz: boolean
  custom_css?: string
  visual_settings?: Partial<VisualSettings>
  /** Optional 7TV emote-set ID override; lets users attach a 7TV set independent of source platforms. */
  seventv_emote_set_id?: string
  /**
   * Bundled marketplace theme id (e.g. "modern-dark-theme"). The renderer
   * resolves the theme CSS fresh from the build bundle so theme fixes
   * propagate on deploy; custom_css is only the user's raw overrides.
   */
  theme_id?: string
  created_at: string
  updated_at: string
}

export interface SevenTVResolvedSet {
  emote_set_id: string
  name?: string
  emote_count?: number
}

/** One source's status as exposed by the public config endpoint. */
export interface PublicSourceStatus {
  platform: string
  channel_id: string
  channel_name?: string
  is_active: boolean
}

/**
 * Shape of `GET /api/v1/overlays/public/:id/config` — the unauthenticated subset
 * served to overlay renderers (no auth, no ownership). Mirrors
 * `ConfigHandler.HandleGetPublicConfig` in overlay-manager.
 */
export interface PublicOverlayConfig {
  display_settings?: DisplaySettings
  filter_settings?: FilterSettings
  custom_css?: string
  visual_settings?: Partial<VisualSettings>
  seventv_emote_set_id?: string
  /** Bundled theme id; renderer resolves its CSS from the build bundle. */
  theme_id?: string
  sources?: PublicSourceStatus[]
}

/**
 * Per-overlay event toggles — mirrors `models.EventSettings` in overlay-manager.
 * Served by `GET /api/v1/overlays/public/:id/event-settings` (public) when the
 * gateway route is enabled; the observability view degrades gracefully without it.
 */
export interface EventSettings {
  id: string
  overlay_id: string
  created_at?: string
  updated_at?: string
  // Twitch
  enable_twitch_subs: boolean
  enable_twitch_resubs: boolean
  enable_twitch_gift_subs: boolean
  enable_twitch_bits: boolean
  enable_twitch_raids: boolean
  enable_twitch_channel_points: boolean
  enable_twitch_follows: boolean
  // YouTube
  enable_youtube_super_chat: boolean
  enable_youtube_super_sticker: boolean
  enable_youtube_members: boolean
  enable_youtube_member_milestones: boolean
  enable_youtube_member_gifts: boolean
  // Kick
  enable_kick_subs: boolean
  enable_kick_gifts: boolean
  // TikTok
  enable_tiktok_likes: boolean
  enable_tiktok_gifts: boolean
  enable_tiktok_follows: boolean
  enable_tiktok_shares: boolean
  // System
  enable_token_warnings: boolean
  // Aggregation
  tiktok_like_aggregation_window_seconds: number
  event_display_duration_multiplier: number
}

export interface DisplaySettings {
  font_family?: string
  font_size?: number
  message_duration?: number
  max_messages?: number
  show_badges?: boolean
  show_avatars?: boolean
  animation?: 'slide' | 'fade' | 'none'
  disable_message_fade?: boolean
  invert_message_order?: boolean
  show_platform_badge?: boolean
  platform_badge_position?: 'before' | 'after'
  platform_badge_style?: 'text' | 'icon'
  show_pronouns?: boolean
  pronoun_position?: 'before' | 'after'
  pronoun_color?: string
  notification_sound_enabled?: boolean
  notification_sound_preset?: string
  notification_sound_url?: string
  notification_sound_volume?: number
  notification_sound_cooldown?: number

  // --- Phase 13: Text-to-speech (D-24) ---

  // Core
  tts_enabled?: boolean
  tts_provider?: 'browser' | 'elevenlabs'
  tts_volume?: number

  // Web Speech API options
  tts_voice_uri?: string
  tts_rate?: number
  tts_pitch?: number

  // Message selection / throttling
  tts_filter_mode?: 'all' | 'sample' | 'priority_only'
  tts_sample_rate?: number
  tts_max_queue?: number
  tts_messages_per_minute?: number
  tts_user_cooldown_seconds?: number
  tts_staleness_seconds?: number

  // Priority overrides
  tts_priority_events?: boolean
  tts_priority_bits_min?: number

  // Content formatting
  tts_read_username?: boolean
  tts_read_platform?: boolean
  tts_max_message_chars?: number
  tts_skip_emote_only?: boolean
  tts_skip_links?: boolean
  tts_enabled_platforms?: string[]
}

export interface FilterSettings {
  banned_words?: string[]
  banned_users?: string[]
  min_message_length?: number
  hide_commands?: boolean
}

export interface ChatSource {
  id: string
  overlay_id: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'shared_overlay' | 'discord'
  channel_id: string
  channel_name?: string
  config?: Record<string, unknown>
  created_at: string
  updated_at: string
  is_active: boolean
  share_status?: 'accepted' | 'revoked' | 'expired' // Only present for shared_overlay sources
  /** Twitch only: true when the channel owner granted user:read:chat, so chat is read via
   *  EventSub instead of IRC. Computed server-side on the overlay source list. */
  chat_via_eventsub?: boolean
}

export type StreamSelectionStrategy =
  | 'first_found'
  | 'most_viewers'
  | 'fewest_viewers'
  | 'title_match'
  | 'title_match_all'
  | 'all'

export interface YouTubeSourceConfig {
  stream_select?: StreamSelectionStrategy
  stream_match?: string
  [key: string]: unknown
}

export interface DiscordSourceConfig {
  guild_id: string
  inbound_channel_id: string
  relay_enabled: boolean
  relay_channel_id: string | null
  [key: string]: unknown
}

export interface CreateOverlayRequest {
  name: string
  description?: string
}

export interface UpdateOverlayRequest {
  name?: string
  description?: string
  is_active?: boolean
  is_public_for_viewers?: boolean
}

export interface AddSourceRequest {
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'shared_overlay' | 'discord'
  channel_id: string
  channel_name?: string
  config?: Record<string, unknown>
}

export interface MockMessagePayload {
  platform?: ChatMessage['platform']
  channel_id?: string
  channel_name?: string
  text: string
  username?: string
  display_name?: string
  avatar_url?: string
  color?: string
  badges?: Array<{
    name: string
    version: string
    icon_url: string
  }>
  event?: import('./message').EventInfo
  metadata?: Record<string, unknown>
}

export interface CreditRollConfig {
  id: string
  overlay_id: string
  enabled: boolean
  include_subs: boolean
  include_resubs: boolean
  include_gift_subs: boolean
  include_bits: boolean
  include_raids: boolean
  include_channel_points: boolean
  include_super_chats: boolean
  include_memberships: boolean
  include_follows: boolean
  leaderboard_top_n: number
  leaderboard_sort_by: 'value' | 'count'
  scroll_speed: number
  display_duration_seconds: number
  background_opacity: number
  theme: 'classic' | 'cinematic' | 'modern'
  clips_enabled: boolean
  clips_max_count: number
  clips_fallback_days: number
  clips_muted: boolean
  custom_css?: string
  created_at: string
  updated_at: string
}

export interface LeaderboardEntry {
  rank: number
  user_id: string
  display_name: string
  avatar_url: string
  platform: string
  count?: number
  total_value?: number
  metadata?: Record<string, any>
}

export interface Leaderboards {
  subs?: LeaderboardEntry[]
  bits?: LeaderboardEntry[]
  raids?: LeaderboardEntry[]
  super_chats?: LeaderboardEntry[]
  follows?: LeaderboardEntry[]
  gifts?: LeaderboardEntry[]
  points?: LeaderboardEntry[]
}

export interface Clip {
  id: string
  url: string
  embed_url: string
  title: string
  view_count: number
  created_at: string
  thumbnail_url: string
  duration: number
}

export interface CreditRollResponse {
  overlay_id: string
  session_id: string
  session_started_at: string
  session_duration_seconds: number
  leaderboards: Leaderboards
  clips: Clip[]
  clips_is_fallback: boolean
}
