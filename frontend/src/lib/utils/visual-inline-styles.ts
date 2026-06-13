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

import type { CSSProperties } from 'react'
import type { VisualSettings } from '@/lib/types/visual-settings'

/**
 * Background / shadow / max-width customizer settings that can't be driven by a
 * blanket CSS rule:
 *
 * - Background colors combine a hex (`*BgColor`) with a 0–1 opacity (`*BgOpacity`)
 *   into a single `rgba()`. A layered `!important` CSS rule for these would clobber
 *   the per-variant Tailwind defaults (normal `bg-slate-900/90`, shared-chat
 *   `bg-purple-900/40`, transparent overlay). Applying them as inline styles *only
 *   when set* leaves those defaults intact when the user hasn't configured them.
 *
 * Returned objects are empty when nothing is configured, so spreading them is a
 * no-op and the existing Tailwind classes win.
 */

/**
 * Combine a hex color (`#rgb` or `#rrggbb`) with a 0–1 opacity string into an
 * `rgba()` string. Returns undefined when the hex is missing/invalid so callers
 * fall back to their default styling.
 */
export function hexToRgba(hex?: string, opacity?: string): string | undefined {
  if (!hex) return undefined
  const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return undefined
  let h = m[1]
  if (h.length === 3) {
    h = h
      .split('')
      .map((c) => c + c)
      .join('')
  }
  const r = parseInt(h.slice(0, 2), 16)
  const g = parseInt(h.slice(2, 4), 16)
  const b = parseInt(h.slice(4, 6), 16)

  const parsed = opacity !== undefined && opacity !== '' ? Number(opacity) : 1
  const a = Number.isFinite(parsed) ? Math.min(1, Math.max(0, parsed)) : 1

  return `rgba(${r}, ${g}, ${b}, ${a})`
}

/** Inline style for the overlay container (background fill + max width). */
export function overlayContainerStyle(vs: Partial<VisualSettings>): CSSProperties {
  const style: CSSProperties = {}
  const bg = hexToRgba(vs.overlayBgColor, vs.overlayBgOpacity)
  if (bg) style.backgroundColor = bg
  if (vs.maxWidth) style.maxWidth = vs.maxWidth
  return style
}

/** Inline style for an individual chat bubble (background fill + shadow). */
export function chatBubbleStyle(vs: Partial<VisualSettings>): CSSProperties {
  const style: CSSProperties = {}
  const bg = hexToRgba(vs.bubbleBgColor, vs.bubbleBgOpacity)
  if (bg) style.backgroundColor = bg
  if (vs.bubbleShadow) style.boxShadow = vs.bubbleShadow
  return style
}
