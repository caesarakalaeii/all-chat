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

import { enMessages } from '@/lib/i18n/messages/en'
import type { MessageCatalog } from '@/lib/i18n/translate'

/**
 * The namespace files the migration batches write into, one file each under
 * messages/en/. Pinned here because the barrel and this list are the contract
 * every later batch is written against: without it two batches invent two names
 * for the same surface and collide.
 */
const EXPECTED_NAMESPACES = [
  'a11y',
  'admin',
  'auth',
  'common',
  'dashboard',
  'docs',
  'errors',
  'legal',
  'maintenanceBanner',
  'marketing',
  'metadata',
  'moderation',
  'onboarding',
  'overlayEditor',
  'settings',
  'viewerOverlay',
] as const

/** Every dotted path to a string leaf, as segment arrays. */
function collectKeyPaths(catalog: MessageCatalog, prefix: string[] = []): string[][] {
  return Object.entries(catalog).flatMap(([key, value]) =>
    typeof value === 'string' ? [[...prefix, key]] : collectKeyPaths(value, [...prefix, key])
  )
}

function leafAt(path: string[]): unknown {
  return path.reduce<unknown>((node, key) => (node as MessageCatalog)[key], enMessages)
}

describe('the composed English catalog', () => {
  it('exposes exactly the pinned namespaces', () => {
    expect(Object.keys(enMessages).sort()).toEqual([...EXPECTED_NAMESPACES])
  })

  it('bottoms out in strings only', () => {
    for (const path of collectKeyPaths(enMessages)) {
      expect(typeof leafAt(path)).toBe('string')
    }
  })

  it('nests at most three levels deep', () => {
    // docs/frontend/I18N.md states the limit. Without an assertion it drifts,
    // and a four-level key is not something a reviewer spots in a 200-key diff.
    const tooDeep = collectKeyPaths(enMessages)
      .filter((path) => path.length > 3)
      .map((path) => path.join('.'))
    expect(tooDeep).toEqual([])
  })

  it('has no blank string', () => {
    // A blank leaf renders as nothing and looks migrated. Trailing or leading
    // whitespace is allowed: copy is migrated byte-identically, spacing included.
    const blank = collectKeyPaths(enMessages)
      .filter((path) => (leafAt(path) as string).trim() === '')
      .map((path) => path.join('.'))
    expect(blank).toEqual([])
  })
})
