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

/**
 * Retired theme ids that must keep resolving (ADR-0053).
 *
 * A bundled theme id is stored in `overlay_configs.theme_id`, so deleting a
 * theme file would leave every overlay on that id with NO theme CSS at all.
 * Retiring one therefore means aliasing it, not removing it: the alias keeps
 * already-configured overlays rendering while the id disappears from the picker.
 * A migration rewrites the stored ids; the alias is what covers rows written
 * before it ran (and any client still holding the old id).
 */
export const THEME_ALIASES: Readonly<Record<string, string>> = {
  // Was a bugfix fork of minimal-theme that shipped alongside it instead of
  // replacing it; the two drifted into near-duplicates and were consolidated.
  'minimal-theme-fixed': 'minimal-theme',
}

/** Resolve a possibly-retired theme id to the id that is actually bundled. */
export function resolveThemeId(id: string): string {
  return THEME_ALIASES[id] ?? id
}

/** Look up a single bundled theme by id (e.g. "modern-dark-theme"). */
export function getBundledTheme(id: string): Theme | undefined {
  const resolved = resolveThemeId(id)
  return BUNDLED_THEMES.find((t) => t.id === resolved)
}

export { BUNDLED_THEMES }
