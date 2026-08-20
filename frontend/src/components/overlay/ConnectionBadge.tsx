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

import { connectionStatusDisplay } from '@/lib/utils/connectionStatusLabel'

import type { ConnectionStatus } from '@/hooks/useOverlayStream'

/**
 * Static (no-animation) connection-state pill for the view header. Reflects the
 * live socket state from useOverlayStream; `attempts` lets a sustained
 * reconnect loop escalate the dot from amber to red.
 *
 * All copy and colour lives in `connectionStatusLabel`, which is a pure
 * function so the wording can be covered by the node-environment `unit` vitest
 * project. This component is the render, nothing else.
 */
export function ConnectionBadge({
  status,
  attempts = 0,
}: {
  status: ConnectionStatus
  attempts?: number
}) {
  const { label, dot, title } = connectionStatusDisplay(status, attempts)

  return (
    <span
      className="connection-status flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub"
      role="status"
      aria-live="polite"
      title={title}
    >
      <span className={clsx('h-2 w-2 rounded-full', dot)} aria-hidden />
      {label}
    </span>
  )
}
