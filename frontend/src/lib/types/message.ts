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
 * Chat Message Types
 *
 * These types match the unified message format from the Message Processor.
 * Used for WebSocket messages and chat rendering.
 */

export type MessageType = 'chat' | 'event';

export type EventType =
  // Twitch
  | 'subscription' | 'resubscription' | 'gift_subscription' | 'mystery_gift'
  | 'bits' | 'raid' | 'channel_points' | 'ritual'
  // Twitch chat notices (channel.chat.notification, ADR-0046). watch_streak and
  // announcement carry the chatter's own message text in message.text.
  | 'watch_streak' | 'announcement' | 'unraid' | 'modiversary' | 'bits_badge_tier'
  | 'charity_donation' | 'gift_paid_upgrade' | 'prime_paid_upgrade'
  | 'pay_it_forward' | 'twitch_notice'
  // YouTube
  | 'super_chat' | 'super_sticker' | 'new_sponsor'
  | 'member_milestone' | 'membership_gift' | 'gift_received'
  | 'message_deleted' | 'user_banned'
  // Kick
  | 'kick_subscription' | 'kick_gift_subscription' | 'kick_donation'
  // TikTok
  | 'gift' | 'follow' | 'like_aggregate' | 'share'
  // System
  | 'token_expiration_warning'
  | 'source_permission_error'
  | 'listener_deprecation_notice'
  // Deletion events
  | 'message_deletion';

export type EventTier = 'low' | 'medium' | 'high';

export type DeletionType = 'single' | 'batch' | 'clear';

export interface DeletionMetadata {
  deletion_type: DeletionType;
  // Single message deletion
  target_uuid?: string;        // Internal message UUID to delete
  target_msg_id?: string;      // Platform message ID (debugging)
  // Batch deletion (user timeout/ban)
  target_user_id?: string;     // User ID to delete all messages from
  target_username?: string;    // Username (display purposes)
  ban_duration?: number;       // Timeout duration in seconds (0 = permanent ban)
  // Full clear has no additional metadata
}

export interface EventInfo {
  type: EventType;
  tier: EventTier;
  value?: {
    amount: number;
    currency: string;
    display_text: string;
  };
  duration: number; // Display duration in seconds
  aggregation_id?: string; // For TikTok like updates
  is_update: boolean; // True for TikTok like aggregate updates
  metadata: Record<string, unknown>; // Use DeletionMetadata interface for message_deletion events
}

export interface ChatMessage {
  id: string;
  overlay_id: string;
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'system'; // Primary platform
  /**
   * All platforms this message was delivered to. Length > 1 for a streamer's
   * "send to all" echo collapsed into one message; absent/length ≤ 1 ⇒ render
   * just the singular `platform`.
   */
  platforms?: string[];
  channel_id: string;
  channel_name: string;
  user: UserInfo;
  message: MessageInfo;
  timestamp: string;
  metadata: Record<string, unknown>;
  event?: EventInfo; // Present for events, absent for regular chat
}

// Phase 29: NameGradient represents a CSS linear-gradient definition stored server-side.
export interface NameGradient {
  type: 'linear';
  colors: string[];
  angle: number;
}

export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: Badge[];
  color?: string;
  name_gradient?: NameGradient; // Phase 29: premium gradient replaces color when set
  source_badges?: Badge[];  // Badges from source channel (shared chat)
  source_user_id?: string;  // User ID in source channel (shared chat)
  avatar_frame_url?: string;  // Phase 30: URL of selected avatar frame
  avatar_flair_url?: string;  // Phase 30: URL of selected avatar flair
  pronouns?: string;  // Phase 9: e.g. "she/her", "they/them" — absent if no pronouns set
}

export interface Badge {
  name: string;
  version: string;
  icon_url: string;
}

export interface MessageInfo {
  text: string;
  emotes: Emote[];
  attachments?: Attachment[];
}

export interface Emote {
  code: string;
  provider: 'twitch' | '7tv' | 'bttv' | 'ffz' | 'youtube' | 'discord';
  url: string;
  positions: number[][];
}

/**
 * A renderable image/GIF/video shared in a chat message (Discord uploads and
 * Tenor/Giphy link previews, and Twitch chat GIFs — see ADR-0037). `type` is
 * 'image' or 'video'; GIFs arrive as images that animate natively. `thumb_url` is
 * an optional poster frame for videos; `spoiler` marks media the sender flagged so
 * the overlay can blur it. `filename` doubles as the render alt text.
 */
export interface Attachment {
  type: 'image' | 'video';
  url: string;
  content_type?: string;
  width?: number;
  height?: number;
  thumb_url?: string;
  spoiler?: boolean;
  filename?: string;
}

export interface PlatformStatus {
  platform: 'youtube' | 'twitch' | 'kick' | 'tiktok' | 'discord';
  channel_id: string;
  channel_name?: string;
  status: 'connected' | 'reconnecting' | 'offline' | 'quota_exceeded' | 'error' | 'paused';
  next_retry_at?: string; // ISO 8601 timestamp
  error_message?: string;
}

export interface WebSocketMessage {
  // 'poll_update' / 'prediction_update' carry engagement snapshots (issue #523). The
  // gateway fans them out on the overlay socket; clients treat them as a "refetch now"
  // signal and pull the authoritative state from the HTTP endpoint (which applies the
  // All-Chat-over-native display precedence), so the payload body isn't relied on.
  type:
    | 'chat_message'
    | 'message_update'
    | 'ping'
    | 'pong'
    | 'error'
    | 'platform_status'
    | 'poll_update'
    | 'prediction_update';
  data?: ChatMessage | PlatformStatus;
  timestamp?: string;
  error?: string;
}
