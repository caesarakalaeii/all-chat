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
 * Bundled overlay themes.
 *
 * The themes ship inside the frontend build (generated from
 * docs/overlay-themes/*.css by scripts/generate-bundled-themes.mjs) instead of
 * being fetched from GitHub at runtime or copied into each overlay's DB row.
 * Resolving theme CSS from this bundle at render time is what lets a theme fix
 * reach every overlay on the next deploy — no per-overlay data migration.
 */

import type { Theme } from './types'
import { BUNDLED_THEMES } from './bundled-themes.generated'

/** All bundled themes (stable order: by id). */
export function getBundledThemes(): Theme[] {
  return BUNDLED_THEMES
}

/** Look up a single bundled theme by id (e.g. "modern-dark-theme"). */
export function getBundledTheme(id: string): Theme | undefined {
  return BUNDLED_THEMES.find((t) => t.id === id)
}

export { BUNDLED_THEMES }
