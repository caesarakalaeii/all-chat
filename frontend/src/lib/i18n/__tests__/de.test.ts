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

import { deMessages } from '@/lib/i18n/messages/de'
import { enMessages } from '@/lib/i18n/messages/en'

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

describe('the German catalog', () => {
  it('translates every namespace except the binding legal text', () => {
    // The namespace files are typed complete, but the barrel is not: a namespace
    // dropped from it would silently render English. legal.* stays English until
    // a legal review signs off a German privacy policy and terms.
    const expected = Object.keys(enMessages).filter((ns) => ns !== 'legal')
    expect(Object.keys(deMessages).sort()).toEqual(expected.sort())
  })

  it('keeps exactly the placeholders of the English string', () => {
    // A renamed or dropped placeholder is not a type error; it renders as a
    // literal {name}, or loses the value, on someone's screen.
    const en = new Map(leaves(enMessages))
    const mismatches = leaves(deMessages)
      .filter(([key, value]) => {
        const source = en.get(key)
        return source === undefined || placeholders(source).join() !== placeholders(value).join()
      })
      .map(([key]) => key)
    expect(mismatches).toEqual([])
  })
})
