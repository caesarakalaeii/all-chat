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

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'
import { creditRollValue } from '@/lib/utils/creditRollValue'
import {
  creditFontSize,
  creditTextPlaceholders,
  withCreditTexts,
} from '@/lib/utils/creditRollCustomization'

const t = getTranslations()

describe('withCreditTexts', () => {
  it('replaces a credit roll message and fills its placeholders', () => {
    const ct = withCreditTexts(t, { session: 'Session du {day}/{month}/{year}' })
    expect(
      ct('viewerOverlay.credits.session', { date: 'x', day: '17', month: '10', year: 2026 })
    ).toBe('Session du 17/10/2026')
  })

  it('keeps the catalog for blank overrides and for messages outside the credit roll', () => {
    const ct = withCreditTexts(t, { heading: '   ', loading: 'Chargement' })
    expect(ct('viewerOverlay.credits.heading')).toBe(t('viewerOverlay.credits.heading'))
    // Same leaf name, different namespace: never overridden.
    expect(ct('overlayEditor.credits.heading')).toBe(t('overlayEditor.credits.heading'))
  })

  it('reaches the leaderboard totals, so units can be translated too', () => {
    const ct = withCreditTexts(t, { valueBitsMany: '{count} bits offerts' })
    const entry = {
      rank: 1,
      user_id: 'u',
      display_name: 'V',
      avatar_url: '',
      platform: 'twitch',
      total_value: 500,
      metadata: { currency: 'bits' },
    }
    expect(creditRollValue(ct, entry)).toBe('500 bits offerts')
  })

  it('leaves an unknown placeholder visible rather than dropping text', () => {
    const ct = withCreditTexts(t, { thanks: 'Merci {name} !' })
    expect(ct('viewerOverlay.credits.thanks')).toBe('Merci {name} !')
  })
})

describe('creditTextPlaceholders', () => {
  it('offers the extra date and duration parts beside the default ones', () => {
    expect(creditTextPlaceholders('session')).toEqual(['date', 'day', 'month', 'year'])
    expect(creditTextPlaceholders('duration')).toEqual(['duration', 'hours', 'minutes'])
    expect(creditTextPlaceholders('heading')).toEqual([])
  })
})

describe('creditFontSize', () => {
  it('scales an element and its sub-parts from the stored percentage', () => {
    expect(creditFontSize({ session: 200 }, 'session')).toBe('2rem')
    expect(creditFontSize({ entry: 150 }, 'entry', 1.2)).toBe('2.25rem')
  })

  it('leaves an element at its default size to its class and theme CSS', () => {
    expect(creditFontSize(undefined, 'heading')).toBeUndefined()
    expect(creditFontSize({ heading: 100 }, 'heading')).toBeUndefined()
  })
})
