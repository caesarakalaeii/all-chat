/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Adapted from frontend/src/lib/types/message.ts — kept narrow to the fields
 * needed by marketing scenes. Keep this file in sync if the unified message
 * format gains/loses fields that marketing wants to showcase.
 */

export type Platform = 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'system';

export interface Badge {
  name: string;
  version: string;
  icon_url: string;
}

export interface Emote {
  code: string;
  provider: 'twitch' | '7tv' | 'bttv' | 'ffz' | 'youtube';
  url: string;
  positions: number[][];
}

export interface UserInfo {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  badges: Badge[];
  color?: string;
}

export interface MessageInfo {
  text: string;
  emotes: Emote[];
}

export interface ChatMessage {
  id: string;
  platform: Platform;
  channel_name: string;
  user: UserInfo;
  message: MessageInfo;
  timestamp: string;
  /** Frame at which this message should appear in the scene timeline. */
  revealAt: number;
}
