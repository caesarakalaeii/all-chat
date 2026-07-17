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

// Platform-coded badge with glow dot.
//
// `platform` is typed as `string` (not `Platform`) on purpose: admin surfaces
// render sources whose platform can be `discord` or `shared_overlay`, and the
// set of stored platforms can outgrow the chromatic color map. Any platform
// without a PLATFORM_COLORS entry (and thus without a `--color-*` / glow CSS
// var) falls back to neutral `system` styling instead of dereferencing
// undefined and crashing the page.
function PlatformBadge({
  platform,
  size,
  className,
}: {
  platform: string
  size?: 'default' | 'sm'
  className?: string
}) {
  const resolved: Platform = platform in PLATFORM_COLORS ? (platform as Platform) : 'system'
  const isSystem = resolved === 'system'
  const glowStyle = isSystem
    ? { backgroundColor: 'var(--color-text-dim)', boxShadow: 'none' }
    : {
        backgroundColor: `var(--color-${resolved})`,
        boxShadow: `var(--shadow-glow-${resolved})`,
      }
  const textClass = PLATFORM_COLORS[resolved].text

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
      {platform.replace(/_/g, ' ').toUpperCase()}
    </span>
  )
}

export { Badge, PlatformBadge, badgeVariants }
