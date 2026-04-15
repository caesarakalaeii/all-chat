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

interface StatusBadgeProps {
  status: 'pending' | 'accepted' | 'rejected' | 'expired' | 'revoked'
  size?: 'sm' | 'md'
}

export function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  const config = {
    pending: {
      label: 'Pending',
      className: 'bg-amber-500/10 text-amber-400 border border-amber-500/20',
      icon: '⏳',
    },
    accepted: {
      label: 'Active',
      className: 'bg-green-500/10 text-green-400 border border-green-500/20',
      icon: '✓',
    },
    expired: {
      label: 'Expired',
      className: 'bg-surface-2/40 text-text-sub border border-border',
      icon: '⏱',
    },
    revoked: {
      label: 'Revoked',
      className: 'bg-red-500/10 text-red-400 border border-red-500/20',
      icon: '✗',
    },
    rejected: {
      label: 'Rejected',
      className: 'bg-red-500/10 text-red-400 border border-red-500/20',
      icon: '✗',
    },
  }[status]

  const sizeClasses = size === 'sm' ? 'text-xs px-2 py-0.5' : 'text-xs px-2.5 py-0.5'

  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1 rounded-full font-medium',
        config.className,
        sizeClasses
      )}
    >
      <span>{config.icon}</span>
      <span>{config.label}</span>
    </span>
  )
}
