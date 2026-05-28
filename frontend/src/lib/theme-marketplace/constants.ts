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

import type { Theme, ChatMessagePreview } from './types'

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

/**
 * Embedded fallback themes
 * Used when GitHub API fails or cache is empty
 */
export const EMBEDDED_FALLBACK_THEMES: Theme[] = [
  {
    id: 'minimal-theme',
    filename: 'minimal-theme.css',
    name: 'Minimal Clean Theme',
    description: 'Inline layout with colorful usernames, no backgrounds or avatars',
    tags: ['minimal', 'clean', 'inline', 'simple'],
    author: 'All-Chat Team',
    version: '1.0.1',
    updated: '2026-05-28',
    css: `/* Minimal theme CSS will be fetched from GitHub */
body {
  background: transparent !important;
  font-family: 'Roboto', Arial, sans-serif !important;
}
.space-y-3 > div {
  background: transparent !important;
  border: none !important;
  padding: 0 !important;
}
.flex-shrink-0 {
  display: none !important;
}
.flex.items-center.gap-2 {
  display: inline !important;
}
.font-semibold.text-sm {
  font-size: 16px !important;
  font-weight: 700 !important;
  paint-order: stroke fill !important;
  -webkit-text-stroke: 3px #000 !important;
}
.text-white.break-words {
  font-size: 16px !important;
  color: #ffffff !important;
  paint-order: stroke fill !important;
  -webkit-text-stroke: 3px #000 !important;
}`,
  },
  {
    id: 'win98-theme',
    filename: 'win98-theme.css',
    name: 'Windows 98 Retro Theme',
    description: 'Nostalgic Windows 98 styling with 3D borders and inset containers',
    tags: ['retro', 'nostalgic', '90s', 'classic'],
    author: 'All-Chat Team',
    version: '1.0.0',
    updated: '2026-01-08',
    css: `/* Windows 98 theme CSS will be fetched from GitHub */
body {
  background: #008080 !important;
  font-family: 'MS Sans Serif', Arial, sans-serif !important;
}
.space-y-3 > div {
  background: #c0c0c0 !important;
  border: 2px outset #ffffff !important;
  padding: 8px !important;
  box-shadow: inset -1px -1px 0 #808080, inset 1px 1px 0 #ffffff !important;
}`,
  },
]
