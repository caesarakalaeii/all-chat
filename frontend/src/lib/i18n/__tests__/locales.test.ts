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

import { DEFAULT_LOCALE, LOCALE_NATIVE_NAMES, SUPPORTED_LOCALES } from '@/lib/i18n/config'
import { deMessages } from '@/lib/i18n/messages/de'
import { enMessages } from '@/lib/i18n/messages/en'
import { esMessages } from '@/lib/i18n/messages/es'
import { frMessages } from '@/lib/i18n/messages/fr'

const translated = { de: deMessages, es: esMessages, fr: frMessages }

/** Every string leaf as [dotted key, value]. */
function leaves(node: object, prefix = ''): [string, string][] {
  return Object.entries(node).flatMap(([key, value]): [string, string][] => {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') return [[path, value]]
    return typeof value === 'object' && value !== null ? leaves(value, path) : []
  })
}

const placeholders = (template: string) =>
  [...template.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort()

it('has a translated catalog and a native name for every supported locale', () => {
  const nonDefault = SUPPORTED_LOCALES.filter((locale) => locale !== DEFAULT_LOCALE)
  expect(Object.keys(translated).sort()).toEqual([...nonDefault].sort())
  expect(LOCALE_NATIVE_NAMES).toMatchObject({ fr: 'Français', es: 'Español' })
})

describe.each(Object.entries(translated))('the %s catalog', (_locale, messages) => {
  it('translates every key of every namespace except the binding legal text', () => {
    // The namespace files are typed complete, but the barrel is not: a namespace
    // dropped from it would silently render English. legal.* stays English until
    // a legal review signs off a translated privacy policy and terms.
    const expected = leaves(enMessages)
      .map(([key]) => key)
      .filter((key) => !key.startsWith('legal.'))
    expect(
      leaves(messages)
        .map(([key]) => key)
        .sort()
    ).toEqual(expected.sort())
  })

  it('keeps exactly the placeholders of the English string', () => {
    // A renamed or dropped placeholder is not a type error; it renders as a
    // literal {name}, or loses the value, on someone's screen.
    const en = new Map(leaves(enMessages))
    const mismatches = leaves(messages)
      .filter(([key, value]) => {
        const source = en.get(key)
        return source === undefined || placeholders(source).join() !== placeholders(value).join()
      })
      .map(([key]) => key)
    expect(mismatches).toEqual([])
  })

  it('uses the REQUIRED "Search KLIPY" placeholder verbatim', () => {
    // docs.klipy.com/attribution: "Set 'Search KLIPY' as the default
    // placeholder text in the search input field - REQUIRED". Mandated
    // wording, so it is not translated.
    expect(messages.overlayEditor?.alertsEditor?.gifSearchPlaceholder).toBe('Search KLIPY')
  })
})

describe('the French catalog', () => {
  it('keeps the wording our volunteer translators submitted', () => {
    // Samples from the /translate contributor tool (ADR-0063).
    expect(frMessages.errors?.networkError?.title).toBe('Erreur de connexion')
    expect(frMessages.maintenanceBanner?.popoverScheduledHeading).toBe('Maintenance programmée')
  })
})
