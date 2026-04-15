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
 * Theme Marketplace Type Definitions
 */

export interface ThemeMetadata {
  name: string
  description: string
  tags: string[]
  author?: string
  version?: string
  updated?: string
}

export interface Theme extends ThemeMetadata {
  id: string
  filename: string
  css: string
}

export interface ThemeCacheData {
  timestamp: number
  ttl: number
  themes: Theme[]
}

export interface ChatMessagePreview {
  id: string
  overlay_id: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  user: {
    id: string
    username: string
    display_name: string
    avatar_url: string
    badges: Array<{
      name: string
      version: string
      icon_url: string
    }>
    color: string
  }
  message: {
    text: string
    emotes: Array<{
      code: string
      provider: string
      url: string
      positions: number[][]
    }>
  }
  timestamp: string
  metadata: Record<string, any>
}
