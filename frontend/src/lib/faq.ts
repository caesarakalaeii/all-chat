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
 * Landing-page FAQ — the question order shared by the visible <FaqSection/> and
 * the FAQPage JSON-LD on the home route. Google requires the structured-data
 * text to match the visible answers verbatim, so both resolve the same
 * `marketing.faq.*` catalog keys from this one list.
 *
 * ⚠️ MARKETING COPY — the answers now live in `marketing.faq.*`; please
 * review/approve any wording change there before merge. Every claim is
 * grounded in current product facts: supported platforms (Twitch/YouTube/Kick/
 * TikTok/Discord), OBS browser source, free & open source (AGPL-3.0), 7TV/BTTV/FFZ +
 * native emotes, 16 built-in themes + custom CSS, cookieless self-hosted analytics
 * with ~1h chat retention, the browser extension, and the premium gating reasons
 * (TTS users bring their own ElevenLabs API key — the gate is the audio streams,
 * which cost far more to deliver than chat websocket traffic; YouTube API quota for
 * moderation actions; stream selection is fully InnerTube (no quota cost) and gated
 * as a power-user feature very few need; chat send quota for poll/prediction
 * announcements; abuse prevention for sharing; cosmetics as supporter perks — see
 * feature_gates migrations 044-076).
 *
 * Each entry below names its pair of `marketing.faq.*` leaves rather than
 * carrying the copy. `as const satisfies` rather than a plain annotation: an
 * annotation widens the stems to string, and a typo would then resolve to a
 * missing key at runtime instead of failing tsc at the call site.
 */
export const FAQ_MESSAGE_STEMS = [
  'platforms',
  'obs',
  'free',
  'premium',
  'emotes',
  'customize',
  'privacy',
  'extension',
] as const satisfies ReadonlyArray<string>
