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
 * Alpha-carrying hex colors for the visual customizer (ADR-0050).
 *
 * Every color setting stores its own opacity in the value itself as an 8-digit
 * hex (`#rrggbbaa`) rather than in a sibling `*Opacity` field. The alpha then
 * travels with the color into every consumer — the inline bubble/overlay
 * styles, the `--chat-*` custom properties, and any theme or custom CSS that
 * reads them — so an `!important` theme rule such as
 * `background: var(--chat-bubble-bg-color, …)` honours it too. Sibling opacity
 * fields could not: themes only ever read the color variable.
 *
 * Fully opaque colors stay 6-digit, so settings written before this change and
 * hand-authored theme CSS keep exactly the shape everyone already writes.
 */

/** `#rgb`, `#rgba`, `#rrggbb` or `#rrggbbaa`, with or without the leading `#`. */
const HEX_RE = /^#?([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i

function clamp01(value: number): number {
  if (!Number.isFinite(value)) return 1
  return Math.min(1, Math.max(0, value))
}

/**
 * Normalizes a hex color to lowercase `#rrggbb` / `#rrggbbaa`, expanding
 * shorthand. Returns undefined when the input is not a hex color (callers then
 * pass the value through untouched — it may be a keyword or a gradient).
 */
export function normalizeHex(hex: string): string | undefined {
  const match = HEX_RE.exec(hex.trim())
  if (!match) return undefined
  const digits = match[1].toLowerCase()
  if (digits.length > 4) return `#${digits}`
  return `#${digits
    .split('')
    .map((c) => c + c)
    .join('')}`
}

/** Alpha channel of a hex color as 0–1; fully opaque when it carries none. */
export function alphaFromHex(hex: string): number {
  const normalized = normalizeHex(hex)
  if (normalized === undefined || normalized.length !== 9) return 1
  return parseInt(normalized.slice(7, 9), 16) / 255
}

/** The color without its alpha channel (`#rrggbb`). */
export function stripAlpha(hex: string): string {
  const normalized = normalizeHex(hex)
  if (normalized === undefined) return hex
  return normalized.slice(0, 7)
}

/**
 * Sets the alpha channel of a hex color. Full opacity yields a plain 6-digit
 * hex so untouched colors never grow a redundant `ff` suffix.
 */
export function hexWithAlpha(hex: string, alpha: number): string {
  const normalized = normalizeHex(hex)
  if (normalized === undefined) return hex
  const rgb = normalized.slice(0, 7)
  const byte = Math.round(clamp01(alpha) * 255)
  if (byte === 255) return rgb
  return `${rgb}${byte.toString(16).padStart(2, '0')}`
}

/**
 * Folds a legacy sibling opacity field (`overlayBgOpacity`, `bubbleBgOpacity` —
 * stored as a "0"–"1" string) into the color value. An alpha channel already
 * carried by the color wins, so re-saved settings are never double-dimmed.
 */
export function withLegacyOpacity(hex: string, opacity?: string): string {
  const normalized = normalizeHex(hex)
  if (normalized === undefined || normalized.length === 9) return normalized ?? hex
  if (opacity === undefined || opacity.trim() === '') return normalized
  const parsed = Number(opacity)
  if (!Number.isFinite(parsed)) return normalized
  return hexWithAlpha(normalized, parsed)
}
