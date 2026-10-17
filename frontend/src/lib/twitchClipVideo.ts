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

// Twitch's own web client ID, public in every twitch.tv page. The GQL API
// accepts it from any origin without a user token.
const TWITCH_WEB_CLIENT_ID = 'kimne78kx3ncx6brgo4mv6wki5h1ko'

const CLIP_VIDEO_QUERY = `query($slug: ID!) {
  clip(slug: $slug) {
    playbackAccessToken(params: { platform: "web", playerBackend: "mediaplayer", playerType: "site" }) {
      signature
      value
    }
    videoQualities { sourceURL }
  }
}`

/**
 * The signed MP4 URL of a Twitch clip, best quality first, from Twitch's web
 * GQL API (ADR-0076). Twitch's clip embed always autoplays muted; the MP4 in
 * our own <video> plays with sound in OBS. Null when the lookup fails in any
 * way, so the caller falls back to the embed.
 */
export async function fetchClipVideoUrl(slug: string): Promise<string | null> {
  try {
    const res = await fetch('https://gql.twitch.tv/gql', {
      method: 'POST',
      headers: { 'Client-Id': TWITCH_WEB_CLIENT_ID },
      body: JSON.stringify({ query: CLIP_VIDEO_QUERY, variables: { slug } }),
    })
    if (!res.ok) return null
    const clip = (await res.json())?.data?.clip
    const source = clip?.videoQualities?.[0]?.sourceURL
    const token = clip?.playbackAccessToken
    if (!source || !token) return null
    return `${source}?sig=${token.signature}&token=${encodeURIComponent(token.value)}`
  } catch {
    return null
  }
}
