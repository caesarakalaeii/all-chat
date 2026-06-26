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

import { NameGradient } from '@/lib/types/message';

/**
 * Hex color allow-list for gradient stops (L31 defense-in-depth). Accepts
 * 3/4/6/8-digit hex (#rgb, #rgba, #rrggbb, #rrggbbaa). Any other value is
 * rejected so attacker-controlled gradient stops cannot break out of the
 * CSS `linear-gradient()` value and inject arbitrary styles.
 */
const GRADIENT_COLOR_RE = /^#[0-9a-fA-F]{3,8}$/;

function isValidGradient(g: NameGradient): boolean {
  if (!g || g.type !== 'linear') return false;
  if (!Array.isArray(g.colors) || g.colors.length === 0) return false;
  if (!g.colors.every((c) => typeof c === 'string' && GRADIENT_COLOR_RE.test(c))) return false;
  // angle must be a finite number; NaN/Infinity would render as "NaNdeg".
  if (typeof g.angle !== 'number' || !Number.isFinite(g.angle)) return false;
  return true;
}

/**
 * Converts a NameGradient definition into a CSS linear-gradient() string.
 *
 * Returns an empty string when the gradient fails validation so that no
 * attacker-controlled value can be interpolated into the CSS output (L31).
 *
 * @example
 * buildGradientCSS({ type: 'linear', colors: ['#ff0000', '#0000ff'], angle: 90 })
 * // => "linear-gradient(90deg, #ff0000, #0000ff)"
 */
export function buildGradientCSS(g: NameGradient): string {
  if (!isValidGradient(g)) return '';
  // angle clamped to [0, 360) to normalize; colors already validated as hex.
  const angle = ((g.angle % 360) + 360) % 360;
  return `linear-gradient(${angle}deg, ${g.colors.join(', ')})`;
}
