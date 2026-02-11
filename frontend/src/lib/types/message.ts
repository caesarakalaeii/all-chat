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
  | 'token_expiration_warning';

export type EventTier = 'low' | 'medium' | 'high';

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
  metadata: Record<string, unknown>;
}

export interface ChatMessage {
  id: string;
  overlay_id: string;
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'system';
  channel_id: string;
  channel_name: string;
  user: UserInfo;
  message: MessageInfo;
  timestamp: string;
  metadata: Record<string, unknown>;
  event?: EventInfo; // Present for events, absent for regular chat
}

export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: Badge[];
  color?: string;
  source_badges?: Badge[];  // Badges from source channel (shared chat)
  source_user_id?: string;  // User ID in source channel (shared chat)
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
  provider: 'twitch' | '7tv' | 'bttv' | 'ffz';
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
