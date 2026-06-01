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
 * Visibility helper for the OBS overlay page.
 *
 * VisualSettings encode show/hide toggles as a CSS `display` value
 * ('block' | 'inline' | 'flex' | 'none'). The preview/embed surfaces consume
 * these via the `.overlay-preview-body`-scoped `@layer visual-customizer`
 * rules in events.css. The live OBS overlay does NOT carry that scope hook, so
 * it must interpret the same toggles in React instead. This pure helper is the
 * single source of truth for "is this element visible?".
 *
 * `undefined` means the setting was never configured → visible by default,
 * matching the CSS defaults (e.g. `var(--chat-show-timestamps, block)`).
 */
export function isDisplayVisible(value: string | undefined): boolean {
  return value !== 'none'
}
