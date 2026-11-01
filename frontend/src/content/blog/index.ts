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
 * The engineering blog's posts. Each one is a .tsx module exporting `meta` and
 * a default `Body` component; this list is what /blog, /blog/<slug> and the
 * sitemap read.
 *
 * Posts are authored, dated, first-person English prose, so they live outside
 * the UI string catalog and this directory is exempt from the i18n lint gate
 * (eslint.i18n.config.mjs). Routing them through the catalog would also put
 * them in front of the /translate contributor tool, and a translated copy of a
 * dated post is a second version nobody keeps in step with the first.
 */

import { formatDateTime } from '@/lib/i18n'
import CveStorm, { meta as cveStormMeta } from './becoming-ready-for-the-cve-storm'
import DeafTiktokRoomSlowTwitchRestart, {
  meta as deafTiktokRoomSlowTwitchRestartMeta,
} from './one-deaf-tiktok-room-and-a-slow-twitch-restart'
import TiktokSignServer, {
  meta as tiktokSignServerMeta,
} from './tiktok-live-chat-without-a-sign-server'
import TwitchChatWentQuiet, {
  meta as twitchChatWentQuietMeta,
} from './twitch-chat-went-quiet-without-an-error'
import type { BlogPost } from './types'

/** Newest first. */
export const POSTS: readonly BlogPost[] = [
  { meta: cveStormMeta, Body: CveStorm },
  { meta: deafTiktokRoomSlowTwitchRestartMeta, Body: DeafTiktokRoomSlowTwitchRestart },
  { meta: twitchChatWentQuietMeta, Body: TwitchChatWentQuiet },
  { meta: tiktokSignServerMeta, Body: TiktokSignServer },
]

export function findPost(slug: string): BlogPost | undefined {
  return POSTS.find((p) => p.meta.slug === slug)
}

// A post date is a calendar day, not an instant: pin UTC so a server in
// another zone cannot render the day before.
const POST_DATE: Intl.DateTimeFormatOptions = { dateStyle: 'long', timeZone: 'UTC' }

export function formatPostDate(date: string): string {
  return formatDateTime(new Date(`${date}T00:00:00Z`), POST_DATE)
}

export type { BlogPost, BlogPostMeta } from './types'
