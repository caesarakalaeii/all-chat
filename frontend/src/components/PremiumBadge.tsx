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

export function PremiumBadge({ size = 18, title }: { size?: number; title?: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 18 18"
      fill="none"
      aria-label="Premium badge"
      className="inline-block shrink-0"
    >
      {title && <title>{title}</title>}
      <polygon points="5,2 13,2 16,7 9,16 2,7" fill="#a855f7" stroke="#7c3aed" strokeWidth="1" />
      <line x1="2" y1="7" x2="16" y2="7" stroke="#7c3aed" strokeWidth="0.8" />
      <line x1="5" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
      <line x1="13" y1="2" x2="9" y2="7" stroke="#c084fc" strokeWidth="0.6" />
    </svg>
  )
}
