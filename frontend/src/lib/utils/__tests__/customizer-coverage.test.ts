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

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

import { BUNDLED_THEMES } from '@/lib/theme-marketplace/bundled-themes.generated'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { OVERRIDDEN_FIELDS, PROPERTY_MAP } from '../visual-settings-to-css'

/**
 * Regression guard for the "the UI setting does nothing" class of bug.
 *
 * `visualSettingsToCss` writes every set field as a `--chat-*` / `--platform-*`
 * custom property on `:root`. A custom property paints nothing on its own — it
 * only reaches the pixels if something CONSUMES it. Three properties had no
 * consumer at all and were shipped as UI controls anyway:
 *
 *  - `--chat-text-shadow` and `--chat-bubble-shadow` were applied as normal
 *    inline styles on the overlay pages, which lose to the
 *    `text-shadow`/`box-shadow` `!important` declarations every bundled theme
 *    carries. Text Shadow (Soft / Strong / Outline) was inert on every theme.
 *  - `--platform-*-accent` had no consumer anywhere: badges take their colour
 *    from hardcoded Tailwind classes and SVG `fill` attributes, so the whole
 *    Platform Colors editor section did nothing.
 *
 * Each of the three is now forced by an `!important` rule inside
 * `@layer visual-customizer` (OVERRIDE_RULES in visual-settings-to-css.ts).
 * This file asserts nothing slips back into "emitted but unconsumed".
 */

const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

/**
 * The stylesheets that consume customizer variables, with all whitespace
 * stripped: prettier breaks long declarations across lines, so `var(` and the
 * variable name are not always adjacent in the source.
 */
const STYLESHEETS = [read('../../../styles/events.css'), read('../../../app/globals.css')]
  .join('\n')
  .replace(/\s+/g, '')

/**
 * Fields whose value is deliberately delivered by React rather than by a CSS
 * rule keyed on the variable. Each entry is a claim about how the setting
 * reaches the pixels; if that stops being true, the setting is dead and the
 * entry must go (or move into OVERRIDE_RULES).
 */
const REACT_DELIVERED: Partial<Record<keyof VisualSettings, string>> = {
  // ADR-0047 priority chain. resolveUsernameColor() renders
  // `var(--chat-username-color, <auto colour>)` inline so a viewer's own colour
  // short-circuits it; no CSS rule may force it, or themes would repaint
  // viewer-defined names.
  usernameColor: 'resolveUsernameColor() inline, ADR-0047',
  // overlayContainerStyle() / chatBubbleStyle() apply these inline and ONLY when
  // set, so the per-variant Tailwind defaults (bg-slate-900/90 vs
  // bg-purple-900/40, transparent overlay) survive an unset control. Themes
  // cooperate by reading the variable, and theme-css-parser back-fills the field
  // from their `var()` fallbacks — which is exactly why these must not be forced.
  overlayBgColor: 'overlayContainerStyle() inline; themes read the var',
  overlayBgOpacity: 'overlayContainerStyle() inline (legacy pre-ADR-0050 alpha)',
  bubbleBgColor: 'chatBubbleStyle() inline; themes read the var',
  bubbleBgOpacity: 'chatBubbleStyle() inline (legacy pre-ADR-0050 alpha)',
  maxWidth: 'overlayContainerStyle() inline; no bundled theme sets max-width',
  // Conditionally rendered, not styled away: the live overlay reads these into
  // React state (showPlatformBadge / showPlatformIndicators).
  showPlatformBadge: 'conditional render on both overlay surfaces',
  showPlatformIndicators: 'conditional render on both overlay surfaces',
}

describe('visual customizer property coverage', () => {
  it('has a consumer for every emitted customizer property', () => {
    const orphans = PROPERTY_MAP.filter(([field, cssVar]) => {
      if (STYLESHEETS.includes(`var(${cssVar}`)) return false
      if (OVERRIDDEN_FIELDS.has(field)) return false
      if (field in REACT_DELIVERED) return false
      return true
    }).map(([field, cssVar]) => `${String(field)} (${cssVar})`)

    expect(
      orphans,
      `These settings are written to :root but nothing consumes them, so the ` +
        `control does nothing on either overlay surface. Consume the variable in ` +
        `events.css, add the field to OVERRIDE_RULES in visual-settings-to-css.ts, ` +
        `or document a React delivery path in REACT_DELIVERED above.`
    ).toEqual([])
  })

  /**
   * The forced rules are `!important` inside a cascade layer, so nothing a theme
   * writes can beat them. That is only safe while "the field is set" means "the
   * user set it". theme-css-parser back-fills fields from a theme's `--chat-*`
   * declarations AND from its `var(--chat-*, fallback)` usages, so the moment a
   * bundled theme mentions one of these variables, loading that theme would pin
   * every overlay to the theme's own default with no way to override it.
   */
  it('never forces a variable any bundled theme declares or reads', () => {
    const forcedVars = PROPERTY_MAP.filter(([field]) => OVERRIDDEN_FIELDS.has(field)).map(
      ([, cssVar]) => cssVar
    )
    expect(forcedVars.length).toBeGreaterThan(0)

    const collisions: string[] = []
    for (const theme of BUNDLED_THEMES) {
      for (const cssVar of forcedVars) {
        if (theme.css.includes(cssVar)) collisions.push(`${theme.id} mentions ${cssVar}`)
      }
    }

    expect(
      collisions,
      `A bundled theme reads or declares a variable that OVERRIDE_RULES forces ` +
        `with layered !important. theme-css-parser would back-fill the field from ` +
        `the theme, and the theme's own value would then be unbeatable. Either ` +
        `drop the theme's reference or drop the field from OVERRIDE_RULES.`
    ).toEqual([])
  })

  it('forces the three settings that had no consumer', () => {
    for (const field of [
      'textShadow',
      'bubbleShadow',
      'twitchAccent',
      'youtubeAccent',
      'kickAccent',
      'tiktokAccent',
      'discordAccent',
    ] as const) {
      expect(OVERRIDDEN_FIELDS.has(field), `${field} must stay forced`).toBe(true)
    }
  })
})
