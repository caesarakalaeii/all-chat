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
 * Fails when a design-token utility compiles to nothing (ADR-0056).
 *
 * Tailwind has no such thing as an unknown-class error. Write `bg-primary`
 * when no `--color-primary` token exists and the compiler simply emits no rule:
 * the build passes, the types pass, ESLint passes, and the element renders with
 * no background. Nothing in the toolchain reports it, and in review the class
 * name reads exactly like a working one.
 *
 * That is not hypothetical. It is how this repo ended up shipping a `<Button>`
 * whose default variant had no background (27 dead classes in `ui/button.tsx`
 * alone), error copy that was never red (`text-destructive`, 35 places), and
 * two background tokens that never existed in any form (`surface-alt` and
 * `subtle`, named here without their `bg-` prefix on purpose: this file is
 * scanned like every other, so spelling a dead utility out in full would make
 * the test its own first offender).
 * ADR-0056 fixed the cause; this test is what keeps it fixed.
 *
 * How it works: the real Tailwind compiler and the real Tailwind source scanner
 * — not a regex approximation of either — so the verdict is exactly what the
 * production build would emit. Every scanned candidate that is SHAPED like a
 * token utility (`bg-*`, `text-*`, `border-*`, `ring-*`, …) is compiled, and
 * any that produces no CSS is a failure.
 *
 * There is deliberately no suppressions baseline. The check is at zero today
 * and a dead token is always a bug, never debt worth carrying — see
 * frontend/DESIGN_SYSTEM.md.
 *
 * If this test fails, the fix is one of:
 *   1. the token is misspelled       -> correct the class name
 *   2. the token should exist        -> add it to @theme in src/app/globals.css
 *                                       (and to the contrast lock in
 *                                       src/app/__tests__/token-contrast.test.ts)
 *   3. it is not a class name at all -> add it to CSS_PROPERTY_NAMES below
 */

import { readFileSync } from 'node:fs'
import path from 'node:path'

import { Scanner } from '@tailwindcss/oxide'
import { compile } from 'tailwindcss'
import { describe, expect, it } from 'vitest'

const ROOT = path.join(__dirname, '..', '..')
const SRC = path.join(ROOT, 'src')
const CSS_ENTRY = path.join(SRC, 'app', 'globals.css')

/**
 * Utility prefixes whose value is a design token. A candidate starting with one
 * of these (after any number of `hover:` / `data-[…]:` style variants) is held
 * to the rule; anything else the scanner turned up is ignored, because the
 * scanner is intentionally over-broad and returns plain JS identifiers too.
 */
const TOKEN_PREFIXES = [
  'bg',
  'text',
  'border',
  'ring',
  'outline',
  'fill',
  'stroke',
  'from',
  'via',
  'to',
  'decoration',
  'caret',
  'placeholder',
  'divide',
  'shadow',
  'accent',
  'rounded',
]

const TOKEN_SHAPED = new RegExp(`^(?:[-\\w[\\]().%/]+:)*(?:${TOKEN_PREFIXES.join('|')})-`)

/**
 * CSS property names that happen to start with a token prefix. These reach the
 * scanner from CSS *written as a string* — bundled overlay themes, the Monaco
 * editor's completion data, custom-CSS docs — where they are property names,
 * not class names. Tailwind ignores them for the same reason.
 */
const CSS_PROPERTY_NAMES = new Set([
  'accent-cycling',
  'border-bottom',
  'border-bottom-color',
  'border-box',
  'border-color',
  'border-image',
  'border-left',
  'border-left-color',
  'border-left-width',
  'border-radius',
  'border-right',
  'border-right-color',
  'border-top-color',
  'border-width',
  'fill-opacity',
  'fill-rule',
  'stroke-linecap',
  'stroke-width',
  'text-align',
  'text-level',
  'text-rendering',
  'text-shadow',
  'text-to-speech',
  'text-transform',
])

/** Escapes a candidate the way Tailwind escapes it into a class selector. */
function toSelector(candidate: string): string {
  return '.' + candidate.replace(/[^\w-]/g, (ch) => '\\' + ch)
}

async function compileWith(css: string) {
  const base = path.dirname(CSS_ENTRY)
  return compile(css, {
    base,
    loadStylesheet: async (id: string, importBase: string) => {
      // globals.css imports `tailwindcss` and `tw-animate-css`; everything else
      // resolves relative to the importing sheet.
      const resolved = id.startsWith('tailwindcss')
        ? path.join(ROOT, 'node_modules', 'tailwindcss', 'index.css')
        : id === 'tw-animate-css'
          ? path.join(ROOT, 'node_modules', 'tw-animate-css', 'dist', 'tw-animate.css')
          : path.resolve(importBase, id)
      return {
        path: resolved,
        base: path.dirname(resolved),
        content: readFileSync(resolved, 'utf8'),
      }
    },
    // globals.css loads no JS plugin or config; the signature still has to be
    // satisfied for compile() to accept the options object.
    loadModule: async () => ({ path: CSS_ENTRY, base, module: {} }),
  })
}

describe('design tokens resolve to real CSS', () => {
  const candidates = new Scanner({
    sources: [{ base: SRC, pattern: '**/*', negated: false }],
  }).scan()

  const tokenShaped = candidates.filter(
    (candidate) =>
      TOKEN_SHAPED.test(candidate) &&
      !candidate.startsWith('!') &&
      !CSS_PROPERTY_NAMES.has(candidate)
  )

  it('finds token utilities to check', () => {
    // Guards the guard: a broken scan would make the assertion below vacuous.
    expect(tokenShaped.length).toBeGreaterThan(100)
  })

  it('every token utility used in src/ emits CSS', async () => {
    const compiler = await compileWith(readFileSync(CSS_ENTRY, 'utf8'))
    const built = compiler.build(tokenShaped)
    const dead = tokenShaped.filter((candidate) => !built.includes(toSelector(candidate)))

    // Sorted so a failure reads as a stable, reviewable list.
    expect(dead.sort()).toEqual([])
  })

  it('detects a dead token when one is introduced', async () => {
    // Proves the assertion above can actually fail. Without this, a change that
    // silently broke compilation or escaping would turn the real check green.
    //
    // Assembled from fragments for the same reason as the docblock above: a
    // literal dead class here would be scanned out of this very file and fail
    // the real assertion.
    const dead = ['bg', 'token', 'that', 'does', 'not', 'exist'].join('-')
    const compiler = await compileWith(readFileSync(CSS_ENTRY, 'utf8'))
    const built = compiler.build(['bg-surface', dead])

    expect(built).toContain(toSelector('bg-surface'))
    expect(built).not.toContain(toSelector(dead))
  })
})
