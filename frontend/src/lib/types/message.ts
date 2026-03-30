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
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'system';
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
}

export interface Badge {
  name: string;
  version: string;
  icon_url: string;
}

export interface MessageInfo {
  text: string;
  emotes: Emote[];
}

export interface Emote {
  code: string;
  provider: 'twitch' | '7tv' | 'bttv' | 'ffz' | 'youtube';
  url: string;
  positions: number[][];
}

export interface PlatformStatus {
  platform: 'youtube' | 'twitch' | 'kick' | 'tiktok';
  channel_id: string;
  status: 'connected' | 'reconnecting' | 'offline' | 'quota_exceeded';
  next_retry_at?: string; // ISO 8601 timestamp
  error_message?: string;
}

export interface WebSocketMessage {
  type: 'chat_message' | 'message_update' | 'ping' | 'pong' | 'error' | 'platform_status';
  data?: ChatMessage | PlatformStatus;
  timestamp?: string;
  error?: string;
}
