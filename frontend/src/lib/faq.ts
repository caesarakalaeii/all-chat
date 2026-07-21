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
 * Landing-page FAQ — single source of truth for both the visible <FaqSection/>
 * and the FAQPage JSON-LD on the home route. Google requires the structured-data
 * text to match the visible answers verbatim, so both consume this array.
 *
 * ⚠️ MARKETING COPY — please review/approve wording before merge. Every claim below
 * is grounded in current product facts: supported platforms (Twitch/YouTube/Kick/
 * TikTok/Discord), OBS browser source, free & open source (AGPL-3.0), 7TV/BTTV/FFZ +
 * native emotes, 16 built-in themes + custom CSS, cookieless self-hosted analytics
 * with ~1h chat retention, the browser extension, and the premium gating reasons
 * (TTS users bring their own ElevenLabs API key — the gate is the audio streams,
 * which cost far more to deliver than chat websocket traffic; YouTube API quota for
 * moderation actions; stream selection is fully InnerTube (no quota cost) and gated
 * as a power-user feature very few need; chat send quota for poll/prediction
 * announcements; abuse prevention for sharing; cosmetics as supporter perks — see
 * feature_gates migrations 044-076).
 */
export interface FaqItem {
  question: string
  answer: string
}

export const FAQ_ITEMS: FaqItem[] = [
  {
    question: 'Which platforms can I combine?',
    answer:
      'Twitch, YouTube, Kick, TikTok, and Discord — in any combination, all in a single overlay.',
  },
  {
    question: 'How do I add All-Chat to OBS?',
    answer:
      'Create an overlay, add your chat sources, then paste the overlay URL into an OBS Browser Source. No plugins or bots required.',
  },
  {
    question: 'Is All-Chat free?',
    answer: 'Yes. All-Chat is free and open source under the AGPL-3.0 license.',
  },
  {
    question: 'Why are some features premium?',
    answer:
      'Premium covers what costs real money or scarce quota to run, plus a few power-user extras: text-to-speech streams audio to your overlay, which is far more expensive to deliver than regular chat messages; YouTube moderation actions and poll announcements posted to chat consume strictly limited platform quotas; YouTube stream selection is an advanced option very few channels need; shared chat is gated to prevent abuse; and viewer flairs are cosmetic perks for supporters. Premium is funded through Patreon and keeps All-Chat running for everyone.',
  },
  {
    question: 'Which emotes are supported?',
    answer:
      '7TV, BTTV, and FFZ, alongside native Twitch and YouTube emotes — they all render correctly in your overlay.',
  },
  {
    question: 'Can I customize how the overlay looks?',
    answer:
      'Yes. Choose from 16 built-in themes or write your own CSS for full control over fonts, colors, and layout.',
  },
  {
    question: 'Does All-Chat track my viewers or use cookies?',
    answer:
      'No. Usage analytics are cookieless and self-hosted, and chat messages are automatically deleted after about an hour.',
  },
  {
    question: 'Is there a browser extension?',
    answer:
      'Yes. The All-Chat browser extension replaces native Twitch, YouTube, and Kick chat so your viewers can follow along across platforms.',
  },
]
