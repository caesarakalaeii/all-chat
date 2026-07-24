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
 * in the initial server HTML for crawlers, independent of client hydration.
 */

import type { Metadata } from 'next'
import HomeClient from './HomeClient'
import { JsonLd } from '@/components/JsonLd'
import { FAQ_ITEMS } from '@/lib/faq'

export const metadata: Metadata = {
  // The homepage targets the category search intent, not just the brand.
  // `absolute` overrides the layout title template ("%s | All-Chat") so the
  // query terms lead the <title>, and a keyword-led description overrides the
  // layout default for this page specifically.
  title: {
    absolute: 'Multi-Platform Chat Overlay for Twitch, YouTube, Kick & TikTok | All-Chat',
  },
  description:
    'Free multi-platform chat overlay for OBS. Merge your Twitch, YouTube, Kick, TikTok, and Discord chat into one overlay, with 7TV, BTTV, and FFZ emotes built in. Open source, no install.',
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
  description:
    'See all your Twitch, YouTube, Kick, TikTok, and Discord chat in one overlay. Drop it into OBS and go. 7TV, BTTV, and FFZ emotes built in.',
  featureList: [
    'Twitch IRC chat',
    'YouTube Live chat',
    'Kick chat',
    'TikTok Live chat',
    'Discord chat relay',
    '7TV, BTTV, FFZ emote support',
    'OBS Browser Source overlay',
  ],
}

const faqLd = {
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  mainEntity: FAQ_ITEMS.map((item) => ({
    '@type': 'Question',
    name: item.question,
    acceptedAnswer: { '@type': 'Answer', text: item.answer },
  })),
}

export default function HomePage() {
  return (
    <>
      <JsonLd data={softwareApplicationLd} />
      <JsonLd data={faqLd} />
      <HomeClient />
    </>
  )
}
