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
 * Static platform color mapping for Tailwind JIT safety.
 *
 * Uses complete literal class strings — no dynamic construction.
 * Tailwind JIT requires full class names to be present as static strings.
 */
export const PLATFORM_COLORS = {
  twitch: { text: 'text-twitch', bg: 'bg-twitch' },
  youtube: { text: 'text-youtube', bg: 'bg-youtube' },
  kick: { text: 'text-kick', bg: 'bg-kick' },
  tiktok: { text: 'text-tiktok', bg: 'bg-tiktok' },
  discord: { text: 'text-discord', bg: 'bg-discord' },
  system: { text: 'text-text-sub', bg: 'bg-surface' },
} as const

export type Platform = keyof typeof PLATFORM_COLORS
