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
 * Altstream monogram: an "A" knocked out of a rounded tile (functional glyph).
 * Altstream's own mark is a raster fox (its icon0.svg only wraps a PNG), so
 * there is no vector path to adapt. Fills with `currentColor`, so set the color
 * via a text-* class (`text-altstream` for the brand orange). Decorative, like
 * DiscordIcon: pair it with visible or sr-only text at the call site.
 */
export function AltstreamIcon({ className = 'h-4 w-4' }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" className={className} aria-hidden="true">
      <path
        fillRule="evenodd"
        d="M6 3h12a3 3 0 0 1 3 3v12a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3zm4.6 3.5h2.8l4.1 11h-2.5l-.93-2.5H9.93L9 17.5H6.5zm.15 6.3h2.5L12 9.46z"
      />
    </svg>
  )
}
