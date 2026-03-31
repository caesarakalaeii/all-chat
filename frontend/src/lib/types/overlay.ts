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
  created_at: string
  updated_at: string
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
}

export type StreamSelectionStrategy = 'first_found' | 'most_viewers' | 'fewest_viewers' | 'title_match' | 'all'

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
