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
 * Ambient-locale formatting gate (#799).
 *
 * `toLocaleString()`, `toLocaleDateString()`, `toLocaleTimeString()` and a bare
 * `new Intl.*` all format with "whatever locale this machine happens to have".
 * That made the same overlay render different date text on two machines and put
 * German month abbreviations inside English copy, because OBS and a browser do
 * not agree on a default. formatDateTime and formatNumber in @/lib/i18n pin the
 * UI locale instead, so output is deterministic wherever it renders.
 *
 * This is a source-text gate rather than a render assertion because the failure
 * it guards is environmental: a render test asserting '10:00:00 AM' passes on
 * an en-US CI runner whether or not the call site pins anything. Grepping the
 * source catches it on every machine. Repo convention for source-text gates is
 * token-contrast.test.ts.
 *
 * Tests are excluded: a test may legitimately construct an Intl formatter to
 * assert what a pinned one should produce, and format.ts itself is the one
 * place the constructors belong.
 */

import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

const SRC = join(__dirname, '..', '..')
const ROOTS = ['app', 'components']

/** Every .ts/.tsx file under the given roots, excluding __tests__ directories. */
function sourceFiles(): string[] {
  const found: string[] = []
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir)) {
      const full = join(dir, entry)
      if (statSync(full).isDirectory()) {
        if (entry !== '__tests__') walk(full)
      } else if (entry.endsWith('.ts') || entry.endsWith('.tsx')) {
        found.push(full)
      }
    }
  }
  for (const root of ROOTS) walk(join(SRC, root))
  return found
}

/** Reports `path:line` for every line matching, across all source files. */
function hits(pattern: RegExp): string[] {
  const found: string[] = []
  for (const file of sourceFiles()) {
    const relative = file.slice(SRC.length + 1)
    readFileSync(file, 'utf-8')
      .split('\n')
      .forEach((line, index) => {
        if (pattern.test(line)) found.push(`${relative}:${index + 1}`)
      })
  }
  return found
}

describe('no rendering formats with the ambient locale', () => {
  it('finds source files to check, so an empty pass is impossible', () => {
    // Without this the suite would go green if the walk silently returned
    // nothing -- a renamed directory, say -- and report no violations at all.
    // 166 files at the time of writing; the bound is loose because the point is
    // to catch zero, not to pin a count that every new component would break.
    expect(sourceFiles().length).toBeGreaterThan(100)
  })

  it('has no toLocaleString / toLocaleDateString / toLocaleTimeString call', () => {
    expect(hits(/toLocale(?:String|DateString|TimeString)\s*\(/)).toEqual([])
  })

  it('has no bare Intl constructor', () => {
    // format.ts owns the constructors and caches them; anywhere else is a
    // formatter built per render with no locale pinned.
    expect(hits(/new Intl\./)).toEqual([])
  })
})
