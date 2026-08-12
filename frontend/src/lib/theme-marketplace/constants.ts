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
 * Sample messages for theme previews.
 *
 * Spans all four platforms and deliberately uses five distinct, viewer-style
 * name colours (purple, blue, green, pink, gold) so previews showcase that every
 * chatter keeps their own name colour — a core promise (and the reason themes
 * must not override username colours; gradient names are a premium feature).
 *
 * One entry is an EVENT (a gifted sub), not chat. Events used to be invisible in
 * every preview surface, so a theme could look perfect in the marketplace card
 * and then have a gold glowing card drop into the live overlay. Previewing an
 * event alongside chat is what makes that mismatch visible before it ships.
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
      color: '#A970FF',
    },
    message: {
      text: 'Welcome in, everyone! 🎉',
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
      username: 'pixelpenguin',
      display_name: 'PixelPenguin',
      avatar_url: 'https://i.pravatar.cc/100?img=5',
      badges: [],
      color: '#2EA8FF',
    },
    message: {
      text: 'this overlay looks so clean 🔥',
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
      username: 'modsquad',
      display_name: 'ModSquad',
      avatar_url: 'https://i.pravatar.cc/100?img=3',
      badges: [
        {
          name: 'moderator',
          version: '1',
          icon_url: 'https://static-cdn.jtvnw.net/badges/v1/3267646d-33f0-4b17-b3df-f923a41db1d0/1',
        },
      ],
      color: '#22C55E',
    },
    message: {
      text: 'love the customization 💚',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
  {
    id: 'preview-4',
    overlay_id: 'preview',
    platform: 'tiktok',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-4',
      username: 'novabyte',
      display_name: 'NovaByte',
      avatar_url: 'https://i.pravatar.cc/100?img=8',
      badges: [],
      color: '#FF5C8A',
    },
    message: {
      text: 'looks amazing on stream ✨',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
  {
    id: 'preview-5',
    overlay_id: 'preview',
    platform: 'twitch',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-5',
      username: 'sunnybeats',
      display_name: 'SunnyBeats',
      avatar_url: 'https://i.pravatar.cc/100?img=9',
      badges: [],
      color: '#F5A623',
    },
    message: {
      text: 'how do I set this up?? 👀',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
  },
  {
    id: 'preview-6',
    overlay_id: 'preview',
    platform: 'twitch',
    channel_id: 'preview',
    channel_name: 'Preview',
    user: {
      id: 'user-6',
      username: 'lumenwave',
      display_name: 'LumenWave',
      avatar_url: 'https://i.pravatar.cc/100?img=12',
      badges: [],
      color: '#C084FC',
    },
    message: {
      text: '',
      emotes: [],
    },
    timestamp: new Date().toISOString(),
    metadata: {},
    // Medium tier: the most common event a viewer actually sees, so it's the
    // right one to hold every theme's event styling to.
    event: {
      type: 'gift_subscription',
      tier: 'medium',
      value: { amount: 5, currency: 'USD', display_text: 'x5' },
      duration: 8,
      is_update: false,
      metadata: { gift_count: 5 },
    },
  },
]
