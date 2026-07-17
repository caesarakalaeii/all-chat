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
 * Accessible switch primitive (Base UI Switch).
 *
 * The root is 24px tall — the WCAG 2.5.8 minimum target size. Always give a
 * switch an accessible name: render it inside a `Field.Root` with a
 * `Field.Label` (auto-associated), or pass `aria-label` / `aria-labelledby`.
 */

import { Switch as SwitchPrimitive } from '@base-ui/react/switch'
import { cn } from '@/lib/utils'

function SwitchRoot({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      className={cn(
        'inline-flex h-6 w-10 shrink-0 items-center rounded-full p-0.5 transition-colors duration-200',
        'bg-surface-2 data-[checked]:bg-twitch',
        'focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:outline-none',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className
      )}
      {...props}
    />
  )
}

function SwitchThumb({
  className,
  ...props
}: React.ComponentPropsWithoutRef<typeof SwitchPrimitive.Thumb>) {
  return (
    <SwitchPrimitive.Thumb
      data-slot="switch-thumb"
      className={cn(
        'block h-5 w-5 rounded-full bg-white shadow transition-transform duration-200 ease-in-out',
        'data-[checked]:translate-x-4',
        className
      )}
      {...props}
    />
  )
}

export const Switch = {
  Root: SwitchRoot,
  Thumb: SwitchThumb,
}
