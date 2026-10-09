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
 * Bodies of /llms.txt and /llms-full.txt (llmstxt.org format).
 *
 * Machine-facing English, not UI copy, so it lives here rather than in the
 * i18n catalog. Where the catalog already holds the exact answer (the landing
 * FAQ), it is read from there so the two cannot drift apart.
 */

import {
  DISCORD_INVITE_URL,
  EXTENSION_CHROME_URL,
  EXTENSION_FIREFOX_URL,
  PATREON_PAGE_URL,
} from '@/lib/constants'
import { FAQ_MESSAGE_STEMS } from '@/lib/faq'
import { getTranslations } from '@/lib/i18n'

const SITE_URL = 'https://allch.at'
const REPO_URL = 'https://github.com/caesarakalaeii/all-chat'

const OVERLAY_URL = `${SITE_URL}/overlay/<overlay-id>`
const MONITOR_URL = `${OVERLAY_URL}/view`
const DOCK_URL = `${MONITOR_URL}?dock=1`

const HEADER = `# All-Chat

> All-Chat is a free, open-source (AGPL-3.0) chat overlay for OBS. It merges live chat from Twitch, YouTube, Kick, TikTok and Discord into one feed, with 7TV, BTTV and FFZ emotes. The hosted service runs at ${SITE_URL}; the source code is on GitHub.`

function overview(): string {
  return `Sign in at ${SITE_URL} with a Twitch, YouTube or Kick account, create an overlay, add the chat sources you want, and paste the overlay URL into an OBS Browser Source. One account can have several overlays, each with its own mix of chat sources and its own theme.

- OBS overlay: ${OVERLAY_URL}, added to OBS as a Browser Source. The background is transparent.
- Chat monitor: ${MONITOR_URL}, a readable view of the same chat for the streamer, with an activity feed beside it. Sending messages, moderation, polls and predictions are run from here. It requires signing in.
- OBS dock: ${DOCK_URL}, the chat monitor laid out for a narrow OBS panel, added under View > Docks > Custom Browser Docks. Streamlabs custom browser docks take the same URL.
- Emotes: 7TV, BTTV and FFZ, plus native Twitch and YouTube emotes.
- Look: 16 built-in themes, or your own CSS.
- Engagement: cross-platform polls, predictions and per-overlay viewer points, an events feed (subs, bits, raids, Super Chats, memberships, follows) and an end-of-stream credit roll.
- Browser extension for Chrome and Firefox that replaces native Twitch, YouTube and Kick chat with the All-Chat feed.
- Developer API: a public WebSocket stream of the normalized chat, documented at ${SITE_URL}/docs/api.
- Demand-driven: a platform is read only while an overlay that uses it is connected. With "Shutdown source when not visible" ticked in OBS, hiding the source stops the reading after a grace period.
- Privacy: no cookies and no viewer tracking. Usage analytics are cookieless and self-hosted, and chat messages are deleted after about an hour.`
}

const PLATFORMS = `- Twitch: read through Twitch EventSub.
- YouTube: read through the YouTube Data API or through an InnerTube poller that uses no API quota.
- Kick: read through Kick's Pusher WebSocket.
- TikTok: TikTok has no public API for LIVE chat. Every tool that shows TikTok LIVE chat, All-Chat included, reads it the way TikTok's own web player does. All-Chat signs those connections with its own open-source signing service (${REPO_URL}/tree/main/services/tiktok-signer); no third-party sign server is involved. When TikTok started requiring browser-grade sessions on 2026-09-09, All-Chat adapted within a week. For premium streamers, a second delivery path takes over when TikTok refuses the primary connection.
- Discord: the All-Chat Discord bot relays messages from a channel you pick. Connect your Discord server under Settings first.`

const FREE_AND_PREMIUM = `The merged chat overlay is free for every platform above, together with themes, custom CSS, the chat monitor and dock, viewer points, the events feed, the credit roll, the browser extension and the developer API.

Premium is funded through Patreon and covers: moderation from the chat monitor, delegated moderators, text-to-speech with ElevenLabs voices (you bring your own ElevenLabs API key), YouTube stream selection, shared chat between overlays, starting polls and predictions, and viewer flairs. The current list is at ${SITE_URL}/upgrade.`

const LIMITS = `- YouTube: only public streams that are live right now. Unlisted, private and scheduled streams do not work.
- TikTok is the platform most likely to see short interruptions when TikTok changes its web player.
- TikTok has no moderation or send API, so neither works for TikTok. Sending messages from the chat monitor works for Twitch, YouTube and Kick.
- Chat messages are deleted after about an hour. All-Chat is not a chat archive.
- Custom alerts are in development and not available yet.`

const LINKS = `## Guides

- [OBS chat overlay](${SITE_URL}/obs-chat-overlay): add merged chat to OBS as a Browser Source
- [OBS chat dock](${SITE_URL}/obs-chat-dock): put the chat monitor into an OBS custom browser dock
- [TikTok LIVE chat overlay](${SITE_URL}/tiktok-live-chat-overlay): how TikTok LIVE chat is read, and its limits
- [Multistream chat](${SITE_URL}/multistream-chat): one chat for Twitch, YouTube, Kick, TikTok and Discord at once
- [Compare](${SITE_URL}/compare): All-Chat next to other multistream chat tools

## Docs

- [Get your overlay live](${SITE_URL}/docs#getting-started): sign in, create an overlay, add it to OBS
- [TikTok: how it works and its limits](${SITE_URL}/docs#tiktok)
- [Chat monitor](${SITE_URL}/docs#monitor): sending messages, display settings, YouTube re-discovery
- [Moderation](${SITE_URL}/docs#moderation): delete, timeout, ban and unban per platform
- [Polls, predictions and points](${SITE_URL}/docs#engagement)
- [Events and credit roll](${SITE_URL}/docs#events-credits)
- [Share an overlay](${SITE_URL}/docs#sharing)
- [Themes](${SITE_URL}/docs#themes) and [custom CSS](${SITE_URL}/docs#custom-css)
- [24/7 and IRL streams](${SITE_URL}/docs#24-7-irl)
- [Developer API](${SITE_URL}/docs/api): WebSocket stream of normalized chat messages and events for third-party tools
- [Premium](${SITE_URL}/upgrade): what Patreon supporters get

## Source code

- [GitHub repository](${REPO_URL})
- [License: AGPL-3.0](${REPO_URL}/blob/main/LICENSE)
- [Self-hosting](${REPO_URL}#self-hosting): run your own instance with Docker Compose
- [Browser extension source](https://github.com/caesarakalaeii/all-chat-extension)

## Optional

- [Browser extension for Chrome](${EXTENSION_CHROME_URL})
- [Browser extension for Firefox](${EXTENSION_FIREFOX_URL})
- [Patreon](${PATREON_PAGE_URL})
- [Discord community](${DISCORD_INVITE_URL})
- [Privacy policy](${SITE_URL}/legal/privacy)`

export function llmsTxt(): string {
  return `${HEADER}

${overview()}

**Platforms and how each one is read**

${PLATFORMS}

**Free and premium**

${FREE_AND_PREMIUM}

**Limits**

${LIMITS}

${LINKS}
`
}

const SETUP = `## Set up an overlay in OBS

1. Go to ${SITE_URL} and sign in with Twitch, YouTube or Kick.
2. In the dashboard, create an overlay.
3. Add a chat source for each platform you stream on. Discord sources need your Discord server connected under Settings first.
4. Pick a theme, or adjust fonts, spacing and colors in the Appearance panel. Custom CSS is under Advanced > Custom CSS.
5. Copy the overlay URL. It has the form ${OVERLAY_URL}.
6. In OBS, add a Browser Source and paste the URL. A size around 400 x 800 suits a chat panel.
7. Optional: tick "Shutdown source when not visible" in the Browser Source properties. All-Chat then stops reading the platforms while the source is hidden, after a grace period, and starts again when it is visible.

## Add the chat monitor as an OBS dock

1. In the overlay editor, copy the dock link. It has the form ${DOCK_URL}.
2. In OBS, open View > Docks > Custom Browser Docks.
3. Give the dock a name, paste the link into the URL field, click Apply and close the dialog.
4. Sign in once inside the dock. It keeps its own sign-in, separate from your browser.

Streamlabs custom browser docks take the same URL and the same steps. Without ?dock=1, ${MONITOR_URL} opens the full-width monitor in a browser tab.

## 24/7 and IRL streams

For an OBS instance that runs around the clock, add ?passive=true to the Browser Source URL. A passive overlay shows chat like a normal one but does not start YouTube capture on its own, so YouTube discovery cannot time out while you are offline. When you go live, open the chat monitor and press Rediscover if chat is not flowing yet. Capture keeps running while the monitor stays open.

## Moderation

Moderation runs from the chat monitor and is a premium feature. Twitch, Kick and Discord support delete, timeout, ban and unban. YouTube supports timeout and ban. TikTok has no moderation API. A streamer can invite delegated moderators, who act with their own platform accounts, so Twitch, YouTube and Kick check their moderator role on every action. On Discord the All-Chat bot acts, after checking that the moderator's own Discord roles allow the action.

## Polls, predictions and viewer points

Polls, predictions and viewer points work across every connected platform, not just Twitch. Turn them on in the overlay editor's Engagement section. Viewers join from chat (for example !vote 2) or on a participation page. Poll and prediction widgets are separate Browser Sources. Starting a poll or a prediction is premium, because the announcement is posted to chat and uses the platform's send quota. Earning points and taking part are free.

## Events and credit roll

The overlay can show events such as subs, resubs, gift subs, bits, raids, Super Chats, memberships and follows, chosen per platform. The credit roll is its own Browser Source: a scrolling thank-you to top subscribers, gifters, cheerers, raiders and new followers at the end of a stream.`

function faq(): string {
  const t = getTranslations()
  const entries = FAQ_MESSAGE_STEMS.map(
    (stem) => `### ${t(`marketing.faq.${stem}Question`)}\n\n${t(`marketing.faq.${stem}Answer`)}`
  )
  return `## FAQ\n\n${entries.join('\n\n')}`
}

export function llmsFullTxt(): string {
  return `${HEADER}

${overview()}

## Platforms and how each one is read

${PLATFORMS}

${SETUP}

## Free and premium

${FREE_AND_PREMIUM}

## Limits

${LIMITS}

${faq()}

${LINKS}
`
}
