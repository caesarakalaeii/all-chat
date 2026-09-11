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
 * wrapThemeCss: inject a bundled theme under `@layer marketplace-themes`.
 *
 * The overlay precedence hierarchy is (highest first):
 *
 *   1. manual custom CSS — unlayered
 *   2. GUI visual settings — `@layer visual-customizer` (top computed layer)
 *   3. theme — `@layer marketplace-themes` (this wrapper)
 *   4. Tailwind utilities + app defaults — their own layers
 *
 * CSS Cascade 5 ranks layered declarations after unlayered ones, so GUI rules
 * in the top layer beat a theme in a lower layer at NORMAL weight — but a
 * theme declaration kept `!important` would still beat GUI-normal rules AND
 * the user's unlayered-important manual CSS (layered-important outranks
 * unlayered-important). Every `!important` is therefore stripped from the
 * theme on injection; the layer alone provides the ranking.
 *
 * `@import` statements are hoisted above the layer body because the CSS
 * grammar requires them to precede all other rules. Run this AFTER
 * {@link rewriteThemeFontImports} so the proxy rewrite sees the original URLs.
 */
export const THEME_LAYER_NAME = 'marketplace-themes'

function stripImportant(css: string): string {
  return css.replace(/\s*!important/gi, '')
}

function extractImports(css: string): { imports: string[]; rest: string } {
  const imports: string[] = []
  const rest = css.replace(/@import[^;]+;/g, (statement) => {
    imports.push(statement.trim())
    return ''
  })
  return { imports, rest }
}

export function wrapThemeCss(css: string): string {
  if (!css.trim()) return ''

  const { imports, rest } = extractImports(stripImportant(css))
  const parts = [...imports, `@layer ${THEME_LAYER_NAME} {`, rest.trim(), '}']
  return parts.filter((part) => part !== '').join('\n')
}
