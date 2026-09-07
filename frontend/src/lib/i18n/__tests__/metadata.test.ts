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
 * Copy lock for the route metadata: SEO titles, descriptions and the social
 * card.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. The strings that
 * were at the render sites are pinned here instead, transcribed from the
 * pre-migration source. If a key's text drifts, this fails and names the key.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('social card copy', () => {
  it('keeps the alt text and every line the card draws', () => {
    // The card is a generated PNG, so its text ships as an image and its alt is
    // the only version a screen reader ever reaches. Both are copy.
    expect(t('metadata.socialCard.alt')).toBe('All-Chat \u2014 Every chat. One overlay.')
    expect(t('metadata.socialCard.title')).toBe('All-Chat')
    expect(t('metadata.socialCard.subtitle')).toBe('Every chat. One overlay.')
    expect(t('metadata.socialCard.emoteProviders')).toBe('7TV + BTTV + FFZ Emotes')
    expect(t('metadata.socialCard.tagline')).toBe('One overlay. Every chat. All platforms.')
  })
})

describe('site-wide metadata (layout.tsx)', () => {
  it('keeps the default title, the template and the site description', () => {
    expect(t('metadata.site.titleDefault')).toBe('All-Chat \u2014 Every chat. One overlay.')
    // The template is copy: '%s | All-Chat' places the page title relative to
    // the brand, and a language that puts the brand first has to move it.
    expect(t('metadata.site.titleTemplate')).toBe('%s | All-Chat')
    expect(t('metadata.site.description')).toBe(
      'See all your Twitch, YouTube, Kick, TikTok, and Discord chat in one OBS chat overlay. Drop it into OBS and go. 7TV, BTTV, and FFZ emotes built in. Free and open source.'
    )
  })

  it('keeps the shared OpenGraph and Twitter card copy', () => {
    // openGraph and twitter carried byte-identical title and description, so
    // one key each rather than two pairs that can drift apart.
    expect(t('metadata.site.socialTitle')).toBe('All-Chat \u2014 Every chat. One overlay.')
    expect(t('metadata.site.socialDescription')).toBe(
      'All your Twitch, YouTube, Kick, TikTok, and Discord chat in one OBS overlay. 7TV, BTTV, and FFZ emotes built in.'
    )
  })
})

describe('homepage metadata', () => {
  it('keeps the keyword-led absolute title and description', () => {
    expect(t('metadata.home.title')).toBe(
      'Multi-Platform Chat Overlay for Twitch, YouTube, Kick & TikTok | All-Chat'
    )
    expect(t('metadata.home.description')).toBe(
      'Free multi-platform chat overlay for OBS. Merge your Twitch, YouTube, Kick, TikTok, and Discord chat into one overlay, with 7TV, BTTV, and FFZ emotes built in. Open source, no install.'
    )
  })
})

describe('docs metadata', () => {
  it('keeps the streamer guide title and description', () => {
    expect(t('metadata.docs.title')).toBe('Documentation')
    expect(t('metadata.docs.description')).toBe(
      'Get your All-Chat overlay live in OBS, pick from 16 built-in themes, and make it your own \u2014 no CSS required to start, full CSS control when you want it.'
    )
  })

  it('keeps the developer API title and description', () => {
    expect(t('metadata.docsApi.title')).toBe('Developer API')
    expect(t('metadata.docsApi.description')).toBe(
      'Connect third-party tools to the All-Chat unified chat WebSocket stream: message format, platform events, status messages and reconnection (Twitch, YouTube, Kick, TikTok, Discord).'
    )
  })
})

describe('upgrade metadata', () => {
  it('keeps the upgrade title and description', () => {
    expect(t('metadata.upgrade.title')).toBe('Upgrade to Premium | All-Chat')
    expect(t('metadata.upgrade.description')).toBe(
      'Back All-Chat on Patreon to unlock premium features: moderate from your overlay, ElevenLabs TTS, YouTube stream selection, shared chat, and viewer flairs.'
    )
  })
})

describe('overlay metadata', () => {
  it('keeps the OBS browser-source titles and descriptions', () => {
    expect(t('metadata.overlay.title')).toBe('All-Chat Overlay')
    expect(t('metadata.overlay.description')).toBe('Chat overlay for OBS Browser Source')
    expect(t('metadata.overlayMonitor.title')).toBe('All-Chat Monitor')
    expect(t('metadata.overlayMonitor.description')).toBe(
      'Readable chat & activity monitor for streamers'
    )
  })
})
