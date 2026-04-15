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

import type { VisualSettings } from '@/lib/types/visual-settings'
import { PROPERTY_MAP } from '@/lib/utils/visual-settings-to-css'

/**
 * Reverse map from CSS custom property name to VisualSettings field name.
 * Built once at module load from PROPERTY_MAP — Map is never mutated.
 */
const REVERSE_MAP = new Map<string, keyof VisualSettings>(
  PROPERTY_MAP.map(([field, cssVar]) => [cssVar, field])
)

/**
 * Parses a CSS string and returns a Partial<VisualSettings> by reverse-mapping
 * --chat-* and --platform-* custom property declarations to VisualSettings fields.
 *
 * Supports two patterns:
 * 1. Direct declaration:  --chat-font-size: 22px;
 * 2. var() fallback:      font-size: var(--chat-font-size, 22px)
 *
 * - Known CSS vars are mapped to their VisualSettings field with trimmed value
 * - Unknown CSS vars produce a console.warn and are excluded from the result
 * - Empty input returns {}
 * - Regexes are defined inside the function to avoid stale lastIndex across calls
 */
export function parseCssToVisualSettings(css: string): Partial<VisualSettings> {
  const result: Partial<VisualSettings> = {}

  // Pattern 1: direct CSS custom property declaration  --chat-*: value;
  const DIRECT_REGEX = /(--(chat|platform)-[\w-]+)\s*:\s*([^;}\n]+?)\s*;/g
  let match: RegExpExecArray | null
  while ((match = DIRECT_REGEX.exec(css)) !== null) {
    const cssVar = match[1]
    const value = match[3]
    const field = REVERSE_MAP.get(cssVar)
    if (field !== undefined) {
      ;(result as Record<string, string>)[field] = value
    } else {
      console.warn('[theme-css-parser] Unknown CSS variable: ' + cssVar)
    }
  }

  // Pattern 2: var(--chat-*, fallback) usage — extract fallback as the default value.
  // Only fills fields not already set by Pattern 1.
  // Skips fallbacks with unbalanced parentheses (e.g. partial linear-gradient extractions)
  // because unbalanced parens in a CSS custom property value corrupt subsequent declarations.
  const VAR_FALLBACK_REGEX = /var\((--(chat|platform)-[\w-]+)\s*,\s*([^)]+?)\s*\)/g
  while ((match = VAR_FALLBACK_REGEX.exec(css)) !== null) {
    const cssVar = match[1]
    const fallback = match[3].trim()
    // Skip values with unbalanced parentheses — they are partial extractions of
    // complex CSS functions like linear-gradient() or rgba() and would corrupt
    // the CSS layer block when written to :root {}
    const openParens = (fallback.match(/\(/g) ?? []).length
    const closeParens = (fallback.match(/\)/g) ?? []).length
    if (openParens !== closeParens) continue
    const field = REVERSE_MAP.get(cssVar)
    if (field !== undefined && !(field in result)) {
      // Reject 'auto' for sizing fields — it makes images render at natural
      // resolution (e.g. 300x300 Twitch avatars) instead of the intended size.
      if (fallback === 'auto' && (field === 'avatarSize' || field === 'badgeSize')) continue
      ;(result as Record<string, string>)[field] = fallback
    }
  }

  return result
}
