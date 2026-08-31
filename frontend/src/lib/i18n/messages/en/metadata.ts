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
 * Route `export const metadata` copy: SEO titles and descriptions.
 *
 * Read from Server Components, so via `getTranslations()`.
 */

export const metadata = {
  // The generated social card. Its text ships as an image, so the alt is the
  // only version a screen reader ever reaches; both are copy.
  socialCard: {
    alt: 'All-Chat — Every chat. One overlay.',
    title: 'All-Chat',
    subtitle: 'Every chat. One overlay.',
    emoteProviders: '7TV + BTTV + FFZ Emotes',
    tagline: 'One overlay. Every chat. All platforms.',
  },
  // layout.tsx, inherited by every route that does not override it.
  site: {
    titleDefault: 'All-Chat — Every chat. One overlay.',
    // Copy, not configuration: it places the page title relative to the brand.
    titleTemplate: '%s | All-Chat',
    description:
      'See all your Twitch, YouTube, Kick, TikTok, and Discord chat in one OBS chat overlay. Drop it into OBS and go. 7TV, BTTV, and FFZ emotes built in. Free and open source.',
    // openGraph and twitter carried identical strings, so one key each.
    socialTitle: 'All-Chat — Every chat. One overlay.',
    socialDescription:
      'All your Twitch, YouTube, Kick, TikTok, and Discord chat in one OBS overlay. 7TV, BTTV, and FFZ emotes built in.',
  },
  home: {
    title: 'Multi-Platform Chat Overlay for Twitch, YouTube, Kick & TikTok | All-Chat',
    description:
      'Free multi-platform chat overlay for OBS. Merge your Twitch, YouTube, Kick, TikTok, and Discord chat into one overlay, with 7TV, BTTV, and FFZ emotes built in. Open source, no install.',
  },
  docs: {
    title: 'Documentation',
    description:
      'Get your All-Chat overlay live in OBS, pick from 16 built-in themes, and make it your own — no CSS required to start, full CSS control when you want it.',
  },
  docsApi: {
    title: 'Developer API',
    description:
      'Connect third-party tools to the All-Chat unified chat WebSocket stream: message format, platform events, status messages and reconnection (Twitch, YouTube, Kick, TikTok, Discord).',
  },
  upgrade: {
    title: 'Upgrade to Premium | All-Chat',
    description:
      'Back All-Chat on Patreon to unlock premium features: moderate from your overlay, ElevenLabs TTS, YouTube stream selection, shared chat, and viewer flairs.',
  },
  // The OBS browser-source routes. Seen in the browser-source title bar and in
  // a tab title, so still copy.
  overlay: {
    title: 'All-Chat Overlay',
    description: 'Chat overlay for OBS Browser Source',
  },
  overlayMonitor: {
    title: 'All-Chat Monitor',
    description: 'Readable chat & activity monitor for streamers',
  },
  impressum: {
    title: 'Impressum | All-Chat',
    description: 'Legal notice (Impressum) as required by § 5 DDG.',
  },
  privacy: {
    title: 'Privacy Policy | All-Chat',
    description: 'Learn how All-Chat collects, processes, and protects your information.',
  },
  terms: {
    title: 'Terms of Service | All-Chat',
    description: 'Understand the rules and responsibilities for using All-Chat.',
  },
} as const
