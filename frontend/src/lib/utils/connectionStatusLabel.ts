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
 * The copy and colour behind the overlay monitor's connection pill.
 *
 * Extracted from `ConnectionBadge` so the wording can be tested: the `unit`
 * vitest project runs `environment: 'node'`, so a React component test cannot
 * run there at all, and a pure function is the only shape this decision can be
 * held to. `viewLayout.ts` is the existing precedent.
 */

/**
 * Live socket state as reported by `useOverlayStream`. Declared here rather
 * than imported from the hook so this module stays free of React and can be
 * exercised in the node-environment unit project; the hook re-exports the same
 * three states.
 */
export type ConnectionStatus = 'connecting' | 'open' | 'reconnecting'

export interface ConnectionStatusDisplay {
  /** Visible pill text. */
  label: string
  /** Tailwind background class for the status dot. */
  dot: string
  /** `title` attribute — carries the retry count when there is one. */
  title: string
}

/**
 * After this many consecutive failed reconnects the link is treated as a real
 * outage (red) rather than a transient blip (amber), so the streamer can tell a
 * momentary hiccup from a connection that is actually down. With the
 * exponential backoff this is roughly ~13s of failing retries.
 *
 * The threshold is deliberately kept: the amber/red distinction is a useful
 * signal. It is the *word* that was wrong — see SUSTAINED_LABEL.
 */
export const OFFLINE_THRESHOLD = 4

/**
 * Copy for the red, past-threshold state.
 *
 * It used to read "Offline", which is false and expensively so. The socket
 * retries forever with exponential backoff, and a redeploy routinely outlasts
 * the ~13s threshold; "Offline" told the streamer the link was dead when it was
 * merely mid-recovery. The predictable response — close the overlay and reopen
 * it — resets the `ws_last_seen` watermark and *guarantees* the message loss the
 * badge was warning about.
 *
 * So the red state says recovery is automatic. The attempt count still rides in
 * the `title`, where a streamer who wants the detail can find it, and the colour
 * still escalates.
 */
const SUSTAINED_LABEL = 'Reconnecting…'

const STATUS_META: Record<ConnectionStatus, { label: string; dot: string }> = {
  open: { label: 'Live', dot: 'bg-kick' },
  connecting: { label: 'Connecting', dot: 'bg-amber-400' },
  reconnecting: { label: 'Reconnecting', dot: 'bg-amber-400' },
}

/**
 * Map the live socket state and consecutive-failure count onto the pill's
 * label, dot colour and tooltip.
 *
 * @param status   current socket state
 * @param attempts consecutive failed reconnects; 0 while connected
 */
export function connectionStatusDisplay(
  status: ConnectionStatus,
  attempts = 0
): ConnectionStatusDisplay {
  const sustained = status === 'reconnecting' && attempts >= OFFLINE_THRESHOLD
  const { label, dot } = sustained
    ? { label: SUSTAINED_LABEL, dot: 'bg-red-500' }
    : STATUS_META[status]

  const title =
    status === 'reconnecting' && attempts > 0
      ? `${label} — ${attempts} failed attempt${attempts === 1 ? '' : 's'}`
      : label

  return { label, dot, title }
}
