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
 * Home route (server component)
 *
 * Owns the homepage canonical URL and JSON-LD structured data, then renders the
 * interactive landing UI (`HomeClient`). Emitting the structured data here puts it
 * in the initial server HTML for crawlers, independent of client hydration. The
 * landing stats are read here for the same reason and handed down as initial data.
 */

import type { Metadata } from 'next'
import HomeClient from './HomeClient'
import { JsonLd } from '@/components/JsonLd'
import { fetchLandingStats } from '@/lib/api/stats'
import { FAQ_MESSAGE_STEMS } from '@/lib/faq'
import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

export const metadata: Metadata = {
  // The homepage targets the category search intent, not just the brand.
  // `absolute` overrides the layout title template ("%s | All-Chat") so the
  // query terms lead the <title>, and a keyword-led description overrides the
  // layout default for this page specifically.
  title: { absolute: t('metadata.home.title') },
  description: t('metadata.home.description'),
  alternates: { canonical: '/' },
}

const softwareApplicationLd = {
  '@context': 'https://schema.org',
  '@type': 'SoftwareApplication',
  name: 'All-Chat',
  url: 'https://allch.at',
  applicationCategory: 'MultimediaApplication',
  operatingSystem: 'Web',
  offers: { '@type': 'Offer', price: '0', priceCurrency: 'EUR' },
  isAccessibleForFree: true,
  license: 'https://www.gnu.org/licenses/agpl-3.0.html',
  softwareHelp: { '@type': 'CreativeWork', url: 'https://allch.at/docs' },
  description:
    'See all your Twitch, YouTube, Kick, TikTok, and Discord chat in one overlay. Drop it into OBS as a browser source, and open the chat monitor as an OBS dock. 7TV, BTTV, and FFZ emotes built in.',
  featureList: [
    'Twitch chat (EventSub)',
    'YouTube Live chat',
    'Kick chat',
    'TikTok LIVE chat',
    'Discord channel relay',
    '7TV, BTTV, FFZ and native emotes',
    'OBS Browser Source overlay',
    'Chat monitor usable as an OBS dock',
    'Multiple overlays per account',
    '16 built-in themes and custom CSS',
    'Cross-platform polls, predictions and viewer points',
    'Events feed and credit roll',
    'Moderation from the chat monitor (premium)',
    'Text-to-speech (premium)',
    'Browser extension for Chrome and Firefox',
    'Public developer WebSocket API',
  ],
}

const faqLd = {
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  mainEntity: FAQ_MESSAGE_STEMS.map((stem) => ({
    '@type': 'Question',
    name: t(`marketing.faq.${stem}Question`),
    acceptedAnswer: { '@type': 'Answer', text: t(`marketing.faq.${stem}Answer`) },
  })),
}

export default async function HomePage() {
  const initialStats = await fetchLandingStats()
  return (
    <>
      <JsonLd data={softwareApplicationLd} />
      <JsonLd data={faqLd} />
      <HomeClient initialStats={initialStats} />
    </>
  )
}
