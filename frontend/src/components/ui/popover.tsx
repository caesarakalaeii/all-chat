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
 * Accessible popover primitive (Base UI Popover): focus management, Escape,
 * outside-press dismissal and anchor positioning for free. Use for anchored
 * pickers/panels (color swatch pickers, small config flyouts) — for
 * confirmation flows use ui/dialog or ui/alert-dialog instead.
 */

import { Popover as PopoverPrimitive } from '@base-ui/react/popover'
import { cn } from '@/lib/utils'

function PopoverContent({
  className,
  sideOffset = 8,
  children,
  ...props
}: React.ComponentPropsWithoutRef<typeof PopoverPrimitive.Popup> & {
  sideOffset?: number
}) {
  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner sideOffset={sideOffset} className="z-50">
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={cn(
            'rounded-lg border border-border-md bg-surface-2 p-3 text-text shadow-xl',
            'focus-visible:outline-none',
            className
          )}
          {...props}
        >
          {children}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  )
}

function PopoverTitle({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof PopoverPrimitive.Title>) {
  return (
    <PopoverPrimitive.Title
      className={cn('text-sm font-semibold text-text', className)}
      {...props}
    />
  )
}

export const Popover = {
  Root: PopoverPrimitive.Root,
  Trigger: PopoverPrimitive.Trigger,
  Close: PopoverPrimitive.Close,
  Title: PopoverTitle,
  Content: PopoverContent,
}
