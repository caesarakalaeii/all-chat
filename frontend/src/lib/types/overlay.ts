/**
 * Overlay and Chat Source Types
 *
 * These types match the backend API responses from the Overlay Manager.
 * Includes types for overlays, configurations, and chat sources.
 */

export interface Overlay {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface OverlayConfig {
  id: string;
  overlay_id: string;
  display_settings: DisplaySettings;
  filter_settings: FilterSettings;
  enable_7tv: boolean;
  enable_bttv: boolean;
  enable_ffz: boolean;
  created_at: string;
  updated_at: string;
}

export interface DisplaySettings {
  font_family?: string;
  font_size?: number;
  message_duration?: number;
  max_messages?: number;
  show_badges?: boolean;
  show_avatars?: boolean;
  animation?: 'slide' | 'fade' | 'none';
}

export interface FilterSettings {
  banned_words?: string[];
  banned_users?: string[];
  min_message_length?: number;
  hide_commands?: boolean;
}

export interface ChatSource {
  id: string;
  overlay_id: string;
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok';
  channel_id: string;
  config?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateOverlayRequest {
  name: string;
  description?: string;
}

export interface UpdateOverlayRequest {
  name?: string;
  description?: string;
  is_active?: boolean;
}

export interface AddSourceRequest {
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok';
  channel_id: string;
  config?: Record<string, unknown>;
}
