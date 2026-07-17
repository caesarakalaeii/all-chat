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
 * Design-token contrast lock (WCAG 2.2 AA initiative, a11y CI gate).
 *
 * Parses the `@theme` block in globals.css and asserts the contrast pairs the
 * design system promises, so a token tweak can never silently ship an
 * inaccessible text color. This locks TOKEN-level guarantees; in-situ
 * page-level contrast (arbitrary utility combinations, alpha overlays) is
 * covered by the axe checks in the Storybook a11y project and
 * tests/e2e/a11y.spec.ts.
 *
 * Scope note: overlay chat THEMES (broadcast art rendered in OBS) are a
 * product surface with a deliberate lower floor — see
 * tests/e2e/theme-contrast.spec.ts. They are NOT covered here.
 *
 * oklch → sRGB uses the OKLab reference math (Björn Ottosson), the same
 * conversion browsers implement. Sanity-locked below against #020204, the
 * rendered bg the design-system comments base their AA claims on.
 */

import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'

// --- minimal color math (no deps) -------------------------------------------

type LinearRGB = [number, number, number]

function oklchToLinearSrgb(L: number, C: number, Hdeg: number): LinearRGB {
  const h = (Hdeg * Math.PI) / 180
  const a = C * Math.cos(h)
  const b = C * Math.sin(h)
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b
  const s_ = L - 0.0894841775 * a - 1.291485548 * b
  const l = l_ ** 3
  const m = m_ ** 3
  const s = s_ ** 3
  const rgb: LinearRGB = [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ]
  return rgb.map((c) => Math.min(1, Math.max(0, c))) as LinearRGB
}

function hexToLinearSrgb(hex: string): LinearRGB {
  const v = hex.replace('#', '')
  const chan = (i: number) => parseInt(v.slice(i, i + 2), 16) / 255
  const linearize = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return [linearize(chan(0)), linearize(chan(2)), linearize(chan(4))]
}

function toHex(rgb: LinearRGB): string {
  const encode = (c: number) => (c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055)
  return (
    '#' +
    rgb
      .map((c) =>
        Math.round(encode(c) * 255)
          .toString(16)
          .padStart(2, '0')
      )
      .join('')
  )
}

function luminance([r, g, b]: LinearRGB): number {
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: LinearRGB, b: LinearRGB): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (hi + 0.05) / (lo + 0.05)
}

// --- @theme token parser ------------------------------------------------------

/**
 * Resolves `--color-*` tokens from the @theme block. Handles the forms the
 * token system actually uses: hex, absolute oklch(L C H), and var() aliases.
 * Alpha/relative forms (border, badge-bg, nav-bg) are intentionally skipped:
 * they are decorative-by-design and meaningless without a composite backdrop.
 */
function parseThemeTokens(css: string): Map<string, LinearRGB> {
  const themeStart = css.indexOf('@theme {')
  if (themeStart === -1) throw new Error('globals.css: @theme block not found')
  let depth = 0
  let end = themeStart
  for (let i = css.indexOf('{', themeStart); i < css.length; i++) {
    if (css[i] === '{') depth++
    if (css[i] === '}') depth--
    if (depth === 0) {
      end = i
      break
    }
  }
  const block = css.slice(themeStart, end)

  const raw = new Map<string, string>()
  for (const m of block.matchAll(/(--color-[\w-]+)\s*:\s*([^;]+);/g)) {
    raw.set(m[1], m[2].trim())
  }

  const resolved = new Map<string, LinearRGB>()
  const resolve = (name: string, seen: Set<string> = new Set()): LinearRGB | null => {
    if (resolved.has(name)) return resolved.get(name) ?? null
    if (seen.has(name)) throw new Error(`circular var() reference at ${name}`)
    seen.add(name)
    const value = raw.get(name)
    if (!value) return null

    const varRef = value.match(/^var\((--color-[\w-]+)\)$/)
    if (varRef) {
      const target = resolve(varRef[1], seen)
      if (target) resolved.set(name, target)
      return target
    }
    const hex = value.match(/^#([0-9a-fA-F]{6})$/)
    if (hex) {
      const rgb = hexToLinearSrgb(value)
      resolved.set(name, rgb)
      return rgb
    }
    const oklch = value.match(/^oklch\(([\d.]+)\s+([\d.]+)\s+([\d.]+)\)$/)
    if (oklch) {
      const rgb = oklchToLinearSrgb(Number(oklch[1]), Number(oklch[2]), Number(oklch[3]))
      resolved.set(name, rgb)
      return rgb
    }
    // Alpha/relative/color-mix forms: skipped by design (see docblock).
    return null
  }
  for (const name of raw.keys()) resolve(name)
  return resolved
}

const css = readFileSync(join(__dirname, '..', 'globals.css'), 'utf-8')
const tokens = parseThemeTokens(css)

function token(name: string): LinearRGB {
  const rgb = tokens.get(name)
  if (!rgb) throw new Error(`token ${name} missing or unresolvable — did the @theme block change?`)
  return rgb
}

// --- assertions ---------------------------------------------------------------

const AA_TEXT = 4.5
const AA_UI = 3.0

const BACKDROPS = ['--color-bg', '--color-surface', '--color-surface-2'] as const
const TEXT_TOKENS = ['--color-text', '--color-text-sub', '--color-text-dim'] as const
const PLATFORMS = [
  '--color-twitch',
  '--color-youtube',
  '--color-kick',
  '--color-tiktok',
  '--color-discord',
] as const

describe('design token contrast (WCAG 2.2 AA)', () => {
  it('oklch conversion matches the rendered bg the design system documents', () => {
    // The platform-color comment claims AA "on dark backgrounds (#020204)" —
    // the browser-rendered value of --color-bg. If this fails, the conversion
    // math drifted and every other assertion here is meaningless.
    expect(toHex(token('--color-bg'))).toBe('#020204')
  })

  for (const text of TEXT_TOKENS) {
    for (const backdrop of BACKDROPS) {
      it(`${text} on ${backdrop} ≥ ${AA_TEXT}:1`, () => {
        expect(contrast(token(text), token(backdrop))).toBeGreaterThanOrEqual(AA_TEXT)
      })
    }
  }

  it('--color-text-dim stays a tier below --color-text-sub', () => {
    // Visual hierarchy contract from the token comments: dim < sub < text.
    const bg = token('--color-bg')
    expect(contrast(token('--color-text-dim'), bg)).toBeLessThan(
      contrast(token('--color-text-sub'), bg)
    )
  })

  for (const platform of PLATFORMS) {
    it(`${platform} as UI component color on --color-bg ≥ ${AA_UI}:1`, () => {
      expect(contrast(token(platform), token('--color-bg'))).toBeGreaterThanOrEqual(AA_UI)
    })
  }

  // The token block claims text-level AA for these four ("Twitch: lightened
  // ... for 4.5:1", "Others already pass"). Discord is deliberately absent:
  // it sits at ~4.5:1 exactly and is only used as a badge/brand color.
  for (const platform of PLATFORMS.filter((p) => p !== '--color-discord')) {
    it(`${platform} as text on --color-bg ≥ ${AA_TEXT}:1`, () => {
      expect(contrast(token(platform), token('--color-bg'))).toBeGreaterThanOrEqual(AA_TEXT)
    })
  }
})
