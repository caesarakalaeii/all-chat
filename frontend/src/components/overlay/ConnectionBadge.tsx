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

import clsx from 'clsx'

import type { ConnectionStatus } from '@/hooks/useOverlayStream'

const STATUS_META: Record<ConnectionStatus, { label: string; dot: string }> = {
  open: { label: 'Live', dot: 'bg-kick' },
  connecting: { label: 'Connecting', dot: 'bg-amber-400' },
  reconnecting: { label: 'Reconnecting', dot: 'bg-amber-400' },
}

/** Static (no-animation) connection-state pill for the view header. */
export function ConnectionBadge({ status }: { status: ConnectionStatus }) {
  const meta = STATUS_META[status]
  return (
    <span className="connection-status flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub">
      <span className={clsx('h-2 w-2 rounded-full', meta.dot)} aria-hidden />
      {meta.label}
    </span>
  )
}
