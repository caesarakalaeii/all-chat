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

import Link from 'next/link'
import { cn } from '@/lib/utils'

/**
 * Inline link that sends users to the premium upsell page (`/upgrade`).
 *
 * Use this anywhere a feature is locked behind premium so every gate funnels to
 * the same upsell → Patreon flow. Defaults to the text "Upgrade to Premium";
 * pass children to fit it into a sentence (e.g. "Premium" or "Upgrade your
 * account").
 */
export function PremiumUpsellLink({
  children = 'Upgrade to Premium',
  className,
}: {
  children?: React.ReactNode
  className?: string
}) {
  return (
    <Link
      href="/upgrade"
      className={cn(
        'font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
        className
      )}
    >
      {children}
    </Link>
  )
}
