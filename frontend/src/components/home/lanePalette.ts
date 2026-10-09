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
 * Lane fill colors, top lane to bottom (twitch, youtube, tiktok, kick,
 * discord). A plain module rather than part of useLaneWaves so the server
 * social card can read it too. The CSS fallback fills in globals.css
 * (--lanes-* and --lanes-pride-*) mirror these; CSS cannot import them.
 */

/** Brand colors, desaturated ~25% so five bands do not read as neon. */
export const LANE_COLORS = ['#8464d6', '#d95c50', '#62aeb4', '#56b847', '#6a72c9'] as const

/**
 * Pride month: red, orange, yellow, green, violet, a five-band rainbow.
 * Desaturated like the brand set; each keeps the near-black marquee ink at
 * WCAG AA.
 */
export const PRIDE_LANE_COLORS = ['#d95c50', '#e08a3c', '#dcc24a', '#56b847', '#8464d6'] as const
