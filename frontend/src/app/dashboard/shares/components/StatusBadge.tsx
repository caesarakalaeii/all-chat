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
 * StatusBadge Component
 *
 * Color-coded status indicator for share requests.
 */

import clsx from 'clsx'

import { useTranslations } from '@/lib/i18n'

interface StatusBadgeProps {
  status: 'pending' | 'accepted' | 'rejected' | 'expired' | 'revoked'
  size?: 'sm' | 'md'
}

export function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  const t = useTranslations()
  // messageStem names the catalog key for the label; icon is a symbol, not copy.
  // `as const` on the literal, not on the indexed result: it keeps each stem a
  // string literal, so a typo fails tsc at the t() call.
  const config = (
    {
      pending: {
        messageStem: 'statusPending',
        className: 'text-amber-300',
        icon: '⏳',
      },
      accepted: {
        messageStem: 'statusAccepted',
        className: 'text-kick',
        icon: '✓',
      },
      expired: {
        messageStem: 'statusExpired',
        className: 'text-dim',
        icon: '⏱',
      },
      revoked: {
        messageStem: 'statusRevoked',
        className: 'text-youtube',
        icon: '✗',
      },
      rejected: {
        messageStem: 'statusRejected',
        className: 'text-youtube',
        icon: '✗',
      },
    } as const
  )[status]

  const sizeClasses = size === 'sm' ? 'text-xs px-2 py-0.5' : 'text-xs px-2.5 py-0.5'

  return (
    <span className={clsx('lanes-chip', config.className, sizeClasses)}>
      <span>{config.icon}</span>
      <span>{t(`dashboard.shares.${config.messageStem}`)}</span>
    </span>
  )
}
