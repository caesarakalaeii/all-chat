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
