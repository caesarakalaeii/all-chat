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
 * SpotlightCard — a card whose soft accent glow follows the pointer, revealed
 * only while hovered. A restrained take on the old landing "magnetic glow": no
 * global pointer listener and no bespoke layout — just per-card CSS variables
 * updated on hover, so any existing card can opt in. The glow sits behind the
 * content and is inert to pointer + assistive tech.
 */

'use client'

import { useRef } from 'react'
import { cn } from '@/lib/utils'

interface SpotlightCardProps {
  children: React.ReactNode
  className?: string
  /** Glow diameter in px. */
  radius?: number
  /** Glow colour — any CSS colour; defaults to the brand violet at low alpha. */
  glow?: string
}

export function SpotlightCard({
  children,
  className,
  radius = 300,
  glow = 'color-mix(in oklch, var(--color-twitch) 28%, transparent)',
}: SpotlightCardProps) {
  const ref = useRef<HTMLDivElement>(null)

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const el = ref.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    el.style.setProperty('--spot-x', `${e.clientX - rect.left}px`)
    el.style.setProperty('--spot-y', `${e.clientY - rect.top}px`)
  }

  return (
    <div
      ref={ref}
      onPointerMove={handlePointerMove}
      className={cn('group/spot relative overflow-hidden', className)}
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 opacity-0 transition-opacity duration-300 group-hover/spot:opacity-100 motion-reduce:transition-none"
        style={{
          background: `radial-gradient(${radius}px circle at var(--spot-x, 50%) var(--spot-y, 50%), ${glow}, transparent 70%)`,
        }}
      />
      <div className="relative">{children}</div>
    </div>
  )
}
