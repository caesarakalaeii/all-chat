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
 * Converts a NameGradient definition into a CSS linear-gradient() string.
 *
 * @example
 * buildGradientCSS({ type: 'linear', colors: ['#ff0000', '#0000ff'], angle: 90 })
 * // => "linear-gradient(90deg, #ff0000, #0000ff)"
 */
export function buildGradientCSS(g: NameGradient): string {
  return `linear-gradient(${g.angle}deg, ${g.colors.join(', ')})`;
}
