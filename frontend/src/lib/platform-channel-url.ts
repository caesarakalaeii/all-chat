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
 * Resolve a chat source to its public channel page on the origin platform.
 *
 * Grounded in how the backend stores each platform's channel identifier
 * (see services/overlay-manager/handlers/sources.go):
 *   - twitch: `channelId` is the lowercased login  -> twitch.tv/{login}
 *   - kick:   `channelId` is the slug (non-numeric) -> kick.com/{slug}
 *   - tiktok: `channelId` is the username           -> tiktok.com/@{username}
 *   - youtube: ambiguous. Prefer a real @handle when we have one; otherwise
 *     only build a /channel/ link when `channelId` is a genuine `UC…` channel
 *     id. The account-linked `youtube_id` elsewhere is a Google account id, not
 *     a channel id, so callers must pass the SOURCE channel id here, never that.
 *   - discord: `channelId` is a numeric snowflake (not web-addressable) -> null
 *   - shared_overlay / anything else: not a real channel -> null
 *
 * Returns `null` when no trustworthy public URL can be built; callers render
 * plain text in that case.
 */
export function channelUrl(
  platform: string,
  channelId: string,
  channelHandle?: string | null
): string | null {
  const id = (channelId ?? '').trim()
  const handle = (channelHandle ?? '').trim()

  switch (platform) {
    case 'twitch': {
      const login = (id || handle).toLowerCase()
      return login ? `https://twitch.tv/${encodeURIComponent(login)}` : null
    }
    case 'kick': {
      const slug = id || handle
      return slug ? `https://kick.com/${encodeURIComponent(slug)}` : null
    }
    case 'tiktok': {
      const username = (id || handle).replace(/^@+/, '')
      return username ? `https://www.tiktok.com/@${encodeURIComponent(username)}` : null
    }
    case 'youtube': {
      if (handle) {
        const h = handle.replace(/^@+/, '')
        return h ? `https://www.youtube.com/@${encodeURIComponent(h)}` : null
      }
      // Only trust a /channel/ link for a canonical UC… channel id.
      if (/^UC[A-Za-z0-9_-]{20,}$/.test(id)) {
        return `https://www.youtube.com/channel/${encodeURIComponent(id)}`
      }
      return null
    }
    default:
      // discord (snowflake), shared_overlay (overlay UUID), unknown.
      return null
  }
}
