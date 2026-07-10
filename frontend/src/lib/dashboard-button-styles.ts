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
 * Provider-coloured "Go to Dashboard" button styles.
 *
 * Kept as an explicit static map (never `bg-${provider}`) so Tailwind's JIT can
 * see every class string at build time — ENFORCE-03 forbids template-literal
 * classNames. Shared by the landing hero (HomeClient) and the sticky HomeHeader
 * so both render the returning user's provider colour identically.
 */
export const DASHBOARD_BUTTON_STYLES: Record<string, { bg: string; ring: string; text: string }> = {
  twitch: { bg: 'bg-twitch', ring: 'focus-visible:ring-twitch', text: 'text-white' },
  youtube: { bg: 'bg-youtube', ring: 'focus-visible:ring-youtube', text: 'text-white' },
  kick: { bg: 'bg-kick', ring: 'focus-visible:ring-kick', text: 'text-bg' },
}

/** Resolve the button style for an auth provider, defaulting to Twitch. */
export const dashStyleFor = (provider?: string) =>
  DASHBOARD_BUTTON_STYLES[provider ?? ''] ?? DASHBOARD_BUTTON_STYLES.twitch
