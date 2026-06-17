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
 * Theme Marketplace Constants
 *
 * Contains fallback themes and sample messages.
 */

import type { ChatMessagePreview } from './types'

/**
 * Sample messages for theme previews
 * Showcases different platforms with diverse content
 */
export const SAMPLE_PREVIEW_MESSAGES: ChatMessagePreview[] = [
  {
    id: 'preview-1',
    overlay_id: 'preview',
    platform: 'twitch',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-1',
      username: 'streamerpro',
      display_name: 'StreamerPro',
      avatar_url: 'https://i.pravatar.cc/100?img=1',
      badges: [
        {
          name: 'broadcaster',
          version: '1',
          icon_url: 'https://static-cdn.jtvnw.net/badges/v1/5527c58c-fb7d-422d-b71b-f309dcb85cc1/1',
        },
      ],
      color: '#9146ff',
    },
    message: {
      text: 'Welcome everyone! 🎉',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
  {
    id: 'preview-2',
    overlay_id: 'preview',
    platform: 'youtube',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-2',
      username: 'viewer123',
      display_name: 'Viewer123',
      avatar_url: 'https://i.pravatar.cc/100?img=2',
      badges: [],
      color: '#ff0000',
    },
    message: {
      text: 'This theme looks amazing! 🔥',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
  {
    id: 'preview-3',
    overlay_id: 'preview',
    platform: 'kick',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-3',
      username: 'modhelper',
      display_name: 'ModHelper',
      avatar_url: 'https://i.pravatar.cc/100?img=3',
      badges: [
        {
          name: 'moderator',
          version: '1',
          icon_url: 'https://static-cdn.jtvnw.net/badges/v1/3267646d-33f0-4b17-b3df-f923a41db1d0/1',
        },
      ],
      color: '#00ff00',
    },
    message: {
      text: 'Love the customization! 💜',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
]
