'use client'

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

import React from 'react'
import { ChevronRight } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface AdvancedDisclosureProps {
  /** Number of controls inside — shown as "Advanced (n)" */
  count: number
  children: React.ReactNode
  className?: string
}

/**
 * Per-section disclosure for low-traffic settings (ADR-0042). Collapsed by
 * default and intentionally NOT persisted: advanced settings should stay out
 * of the default eye-line on every visit. A native <details> so the settings
 * search can force it open (`details.open = true`) when jumping to an
 * anchored control inside.
 */
export function AdvancedDisclosure({
  count,
  children,
  className,
}: AdvancedDisclosureProps): React.ReactElement {
  return (
    <details className={cn('group mt-5 border-t border-border', className)}>
      <summary className="flex cursor-pointer list-none items-center gap-1.5 py-2.5 text-[11px] font-medium tracking-widest text-text-sub uppercase select-none hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none [&::-webkit-details-marker]:hidden">
        <ChevronRight
          aria-hidden="true"
          className="size-3 transition-transform duration-150 group-open:rotate-90"
        />
        Advanced ({count})
      </summary>
      <div className="space-y-5 pt-1 pb-1">{children}</div>
    </details>
  )
}
