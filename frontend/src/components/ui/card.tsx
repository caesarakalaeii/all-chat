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

import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '@/lib/utils'

const cardVariants = cva('rounded-xl border border-border bg-surface text-text transition-all', {
  variants: {
    interactive: {
      // eslint-disable-next-line tailwindcss/no-unnecessary-arbitrary-value -- the bare-decimal scale utility the plugin suggests matches nothing in Tailwind v4 and emits no CSS at all, so taking the fix would delete this hover lift outright rather than restate it. See src/__tests__/no-broken-tailwind-utilities.test.ts
      true: 'hover:scale-[1.02] hover:shadow-lg cursor-pointer',
      false: '',
    },
  },
  defaultVariants: {
    interactive: false,
  },
})

function Card({
  className,
  interactive,
  ...props
}: React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof cardVariants>) {
  return (
    <div data-slot="card" className={cn(cardVariants({ interactive, className }))} {...props} />
  )
}

export { Card, cardVariants }
