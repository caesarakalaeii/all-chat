/**
 * Chat Message Types
 *
 * These types match the unified message format from the Message Processor.
 * Used for WebSocket messages and chat rendering.
 */

export interface ChatMessage {
  id: string;
  overlay_id: string;
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok';
  channel_id: string;
  channel_name: string;
  user: UserInfo;
  message: MessageInfo;
  timestamp: string;
  metadata: Record<string, unknown>;
}

export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: string[];
  color?: string;
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

export interface WebSocketMessage {
  type: 'chat_message' | 'ping' | 'pong' | 'error';
  data?: ChatMessage;
  timestamp?: string;
  error?: string;
}
