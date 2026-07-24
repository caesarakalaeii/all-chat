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
 * A single CSS diagnostic surfaced from Monaco's built-in CSS language service,
 * flattened to a shape the editor UI can render without importing monaco types.
 */
export interface CssIssue {
  line: number
  column: number
  message: string
  severity: 'error' | 'warning' | 'info'
}

/**
 * Maps a Monaco MarkerSeverity to our coarse level. Monaco's enum values are
 * Hint=1, Info=2, Warning=4, Error=8 (kept inline to avoid importing the whole
 * monaco-editor module just for an enum).
 */
export function markerSeverityToLevel(severity: number): CssIssue['severity'] {
  if (severity >= 8) return 'error'
  if (severity >= 4) return 'warning'
  return 'info'
}

/**
 * Custom-CSS fork model (ADR-0043).
 *
 * The overlay editor PRELOADS the applied bundled theme's CSS into the Advanced
 * editor so it is visible and directly editable. To keep theme fixes propagating
 * on deploy, that preloaded copy is persisted to `custom_css` only when the user
 * actually diverges from the pristine theme ("fork-on-edit"). An untouched theme
 * (or an editor still equal to its pristine theme) saves an empty override so the
 * overlay stays linked to the bundle.
 */

/**
 * True when the editor content represents a user fork: it has content AND differs
 * from the pristine bundled theme CSS. When there is no theme (pristine === ''),
 * any non-empty content is a fork.
 */
export function isCustomCssForked(customCss: string, pristineThemeCss: string): boolean {
  return customCss.trim().length > 0 && customCss !== pristineThemeCss
}

/**
 * The value to persist to `custom_css`: the editor content when forked, else ''
 * (which re-links the overlay to its bundled theme_id).
 */
export function persistedCustomCss(customCss: string, pristineThemeCss: string): string {
  return isCustomCssForked(customCss, pristineThemeCss) ? customCss : ''
}
