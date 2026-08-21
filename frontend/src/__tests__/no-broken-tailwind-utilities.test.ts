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
 * Bans two Tailwind class names that emit no CSS at all.
 *
 * `tailwindcss/no-unnecessary-arbitrary-value` reports `opacity-[0.03]` and
 * `hover:scale-[1.02]` and offers bare-decimal replacements. Both suggestions
 * are wrong: fed through the real Tailwind v4 compiler they match no utility and
 * produce an empty rule, so applying either one silently deletes the style
 * rather than restating it.
 *
 * Nothing else catches this. ESLint is the source of the bad advice, the build
 * does not fail on an unmatched utility, and a missing 3% background wash or a
 * missing hover lift is invisible in a unit test that asserts on behaviour.
 * Hence a grep: it fires the moment either string is committed, whether by the
 * plugin's autofix or by hand.
 *
 * The banned strings below are assembled from fragments on purpose. Spelling
 * them out would make this file its own first offender, and CI greps src/ for
 * them directly as well as running this test.
 */

import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'

import { describe, expect, it } from 'vitest'

const SRC = path.join(__dirname, '..')

// Class names Tailwind v4 does not resolve, mapped to the arbitrary-value
// spelling that does work, so a failure says what to write instead.
const BROKEN_UTILITIES = {
  [`opacity-0${'.'}03`]: 'opacity-[0.03]',
  [`scale-1${'.'}02`]: 'scale-[1.02]',
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return sourceFiles(full)
    return /\.(ts|tsx|js|jsx|css)$/.test(entry.name) ? [full] : []
  })
}

describe('Tailwind utilities that compile to nothing', () => {
  // No self-exclusion: this file must be scanned too, so that a later edit which
  // spells a banned utility out in the prose above fails the test that bans it.
  const files = sourceFiles(SRC)

  it('finds source files to scan', () => {
    // Guards the guard: a broken traversal would make every case below vacuous.
    expect(files.length).toBeGreaterThan(100)
  })

  for (const [broken, replacement] of Object.entries(BROKEN_UTILITIES)) {
    it(`does not use ${broken} anywhere under src/ (use ${replacement})`, () => {
      const offenders = files.filter((file) => readFileSync(file, 'utf8').includes(broken))
      expect(offenders.map((file) => path.relative(SRC, file))).toEqual([])
    })
  }
})
