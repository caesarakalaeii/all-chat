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

/**
 * Added unmodified from the shadcn `base-nova` registry (ADR-0056).
 *
 * Keep it that way: local styling belongs in the call site's `className`, not
 * in here, so `npx shadcn@4 diff` stays meaningful and an upstream fix can be
 * taken without a merge. If a change really must live in the primitive, note
 * why here — that note is what tells the next person the diff is intentional.
 */

import { Separator as SeparatorPrimitive } from '@base-ui/react/separator'

import { cn } from '@/lib/utils'

function Separator({ className, orientation = 'horizontal', ...props }: SeparatorPrimitive.Props) {
  return (
    <SeparatorPrimitive
      data-slot="separator"
      orientation={orientation}
      className={cn(
        'shrink-0 bg-border data-horizontal:h-px data-horizontal:w-full data-vertical:w-px data-vertical:self-stretch',
        className
      )}
      {...props}
    />
  )
}

export { Separator }
