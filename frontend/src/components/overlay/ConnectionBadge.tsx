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

// After this many consecutive failed reconnects the link is treated as a real
// outage ("Offline", red) rather than a transient blip ("Reconnecting", amber),
// so the streamer can tell a momentary hiccup from a connection that's actually
// down. With the exponential backoff this is roughly ~13s of failing retries.
const OFFLINE_THRESHOLD = 4

/**
 * Static (no-animation) connection-state pill for the view header. Reflects the
 * live socket state from useOverlayStream; `attempts` lets a sustained
 * reconnect loop surface as a distinct "Offline" state.
 */
export function ConnectionBadge({
  status,
  attempts = 0,
}: {
  status: ConnectionStatus
  attempts?: number
}) {
  const offline = status === 'reconnecting' && attempts >= OFFLINE_THRESHOLD
  const meta = offline ? { label: 'Offline', dot: 'bg-red-500' } : STATUS_META[status]
  const title =
    status === 'reconnecting' && attempts > 0
      ? `${meta.label} — ${attempts} failed attempt${attempts === 1 ? '' : 's'}`
      : meta.label

  return (
    <span
      className="connection-status flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub"
      role="status"
      aria-live="polite"
      title={title}
    >
      <span className={clsx('h-2 w-2 rounded-full', meta.dot)} aria-hidden />
      {meta.label}
    </span>
  )
}
