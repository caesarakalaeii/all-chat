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
 * Scope owner/marketplace-authored CSS to a preview container.
 *
 * Shared by the editor preview iframe (`/overlays/[id]/preview/embed`) and the
 * marketplace/landing `ThemePreview` card — they each carried their own copy of
 * this function, which is exactly the kind of duplication that drifts.
 *
 * NOTE (M11): this prefixes selectors so owner-authored CSS is scoped to the
 * preview root. It is NOT a full CSS sanitizer: `@import`, `url(...)`,
 * `expression()`, or escaped-selector tricks could still escape. A complete CSS
 * sanitiser is large and out of scope here; the blast radius is capped by the
 * CSP `style-src` directive added in next.config.js (M10), which blocks
 * external stylesheets and inline style injection vectors that would otherwise
 * be reachable via url()/@import.
 */

/**
 * Strip `/* … *​/` comments.
 *
 * Not cosmetic: the selector regex below treats everything between `}` and `{`
 * as a selector list and splits it on commas, so a comment sitting above a rule
 * was absorbed into that rule's selector. A comment containing a comma — most
 * of them do — then split into fragments, and the rule that followed came out
 * WITHOUT the scope prefix, leaking theme CSS out of the preview and onto the
 * surrounding page. Removing comments first makes selector detection see only
 * selectors.
 *
 * Best-effort like the rest of this function: a `/*` inside a quoted `content`
 * value would be mis-detected, which no bundled theme does.
 */
const stripComments = (css: string): string => css.replace(/\/\*[\s\S]*?\*\//g, '')

export const scopeCustomCss = (
  css: string,
  scopeSelector: string,
  bodySelector: string
): string => {
  if (!css.trim()) {
    return ''
  }

  const replaceBody = stripComments(css)
    .replace(/:root/gi, scopeSelector)
    .replace(/\bbody\b/gi, bodySelector)

  return replaceBody.replace(/(^|}|{)\s*([^@}{]+)\s*{/g, (match, prefix, selectorGroup) => {
    const trimmed = selectorGroup.trim()
    if (!trimmed) {
      return match
    }

    const isKeyframeStep =
      ['from', 'to'].includes(trimmed.toLowerCase()) || /^\d+\.?\d*%$/i.test(trimmed)
    if (isKeyframeStep) {
      return `${prefix} ${trimmed} {`
    }

    const scopedSelectors = trimmed
      .split(',')
      .map((selector: string) => {
        const sel = selector.trim()
        if (!sel || sel.startsWith(scopeSelector) || sel.startsWith(bodySelector)) {
          return sel
        }
        return `${scopeSelector} ${sel}`
      })
      .filter(Boolean)
      .join(', ')

    return `${prefix} ${scopedSelectors} {`
  })
}
