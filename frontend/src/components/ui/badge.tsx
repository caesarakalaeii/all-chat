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
import { PLATFORM_COLORS, type Platform } from '@/lib/platform-colors'

const badgeVariants = cva(
  'inline-flex items-center rounded-full bg-badge-bg font-mono text-text whitespace-nowrap',
  {
    variants: {
      size: {
        default: 'px-2.5 py-0.5 text-xs',
        sm: 'px-2 py-0.5 text-[0.65rem]',
      },
    },
    defaultVariants: { size: 'default' },
  }
)

// Generic badge (for non-platform use)
function Badge({
  className,
  size,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span data-slot="badge" className={cn(badgeVariants({ size, className }))} {...props} />
}

// Platform-coded badge with glow dot
function PlatformBadge({
  platform,
  size,
  className,
}: {
  platform: Platform
  size?: 'default' | 'sm'
  className?: string
}) {
  const isSystem = platform === 'system'
  const glowStyle = isSystem
    ? { backgroundColor: 'var(--color-text-dim)', boxShadow: 'none' }
    : {
        backgroundColor: `var(--color-${platform})`,
        boxShadow: `var(--shadow-glow-${platform})`,
      }
  const textClass = PLATFORM_COLORS[platform].text

  return (
    <span
      data-slot="platform-badge"
      data-platform={platform}
      className={cn(badgeVariants({ size }), textClass, className)}
    >
      <span
        className="mr-1.5 inline-block h-1.5 w-1.5 shrink-0 rounded-full"
        style={glowStyle}
        aria-hidden="true"
      />
      {platform.toUpperCase()}
    </span>
  )
}

export { Badge, PlatformBadge, badgeVariants }
