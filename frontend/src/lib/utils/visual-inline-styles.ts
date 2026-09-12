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
import type { UserInfo } from '@/lib/types/message'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { buildGradientCSS } from '@/lib/utils/gradient'
import { normalizeHex } from '@/lib/utils/hex-alpha'

/**
 * Background / shadow / max-width customizer settings that can't be driven by a
 * blanket CSS rule:
 *
 * - Background colors resolve to a single `rgba()`. The opacity rides in the
 *   color itself as an 8-digit hex (ADR-0050); the legacy sibling `*BgOpacity`
 *   field is still honoured for settings saved before that. A layered
 *   `!important` CSS rule for these would clobber the per-variant Tailwind
 *   defaults (normal `bg-slate-900/90`, shared-chat `bg-purple-900/40`,
 *   transparent overlay). Applying them as inline styles *only when set* leaves
 *   those defaults intact when the user hasn't configured them.
 *
 * Returned objects are empty when nothing is configured, so spreading them is a
 * no-op and the existing Tailwind classes win.
 */

/**
 * Resolve a hex color (`#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`) to an
 * `rgba()` string. An alpha channel on the color wins; `opacity` (a legacy
 * "0"–"1" string) applies only to colors without one. Returns undefined when
 * the hex is missing/invalid so callers fall back to their default styling.
 */
export function hexToRgba(hex?: string, opacity?: string): string | undefined {
  if (!hex) return undefined
  const normalized = normalizeHex(hex)
  if (normalized === undefined) return undefined

  const r = parseInt(normalized.slice(1, 3), 16)
  const g = parseInt(normalized.slice(3, 5), 16)
  const b = parseInt(normalized.slice(5, 7), 16)

  let a: number
  if (normalized.length === 9) {
    a = parseInt(normalized.slice(7, 9), 16) / 255
  } else {
    const parsed = opacity !== undefined && opacity !== '' ? Number(opacity) : 1
    a = Number.isFinite(parsed) ? Math.min(1, Math.max(0, parsed)) : 1
  }

  // Round so an 8-bit alpha channel reads as 0.502, not 0.5019607843137255.
  return `rgba(${r}, ${g}, ${b}, ${Math.round(a * 1000) / 1000})`
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

/**
 * Per-row custom properties carrying a chatter's username colour onto their
 * bubble. The receiver is the [data-user-bubble] rule userBubbleRules emits in
 * the visual-customizer layer (see visual-settings-to-css): these inline
 * values are only declarations, so the generated rule's consumption wins the
 * cascade the moment the row carries the attribute.
 *
 * Gradients are applied opaque: buildGradientCSS cannot fade a whole gradient,
 * so bubbleUserColorOpacity is honoured for flat colours only.
 */
export function userBubbleStyle(
  user: UserInfo | undefined,
  vs: Partial<VisualSettings>,
  mode: 'background' | 'border'
): CSSProperties {
  const gradient =
    user?.name_gradient && buildGradientCSS(user.name_gradient) !== ''
      ? user.name_gradient
      : undefined
  if (mode === 'background') {
    if (gradient) return { '--row-user-bg-image': buildGradientCSS(gradient) } as CSSProperties
    const fill = hexToRgba(user?.color || user?.auto_color, vs.bubbleUserColorOpacity)
    if (fill) return { '--row-user-bg': fill } as CSSProperties
    return {}
  }
  // Border mode. A gradient border has no faithful outline equivalent, so the
  // first stop stands in for the gradient; it is the colour the viewer most
  // associates with the name. Width is deliberately this mode's own 2px, not
  // bubbleBorderWidth — a border nobody sized would never appear.
  const borderColor = gradient ? gradient.colors[0] : user?.color || user?.auto_color
  if (!borderColor || !normalizeHex(borderColor)) return {}
  return {
    '--row-user-border-color': borderColor,
    '--row-user-border-width': '2px',
  } as CSSProperties
}
