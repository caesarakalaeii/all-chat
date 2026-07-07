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

import { useEffect, useRef } from 'react'
import { WebSocketClient } from '../api/websocket'

/**
 * Subscribes to an overlay's poll_update / prediction_update WebSocket frames and
 * invokes `onSignal` when one arrives (issue #523, L-D1). This is a latency
 * accelerator only: callers keep their periodic HTTP poll as the source of truth
 * (it applies the All-Chat-over-native display precedence and self-heals a dropped
 * frame), and use this to refetch immediately instead of waiting for the next tick.
 *
 * A no-op when overlayId is falsy. The socket connects on an anonymous/viewer basis
 * (no token) — the overlay poll/prediction broadcast is public — and is torn down on
 * unmount or overlayId change.
 *
 * `opts.viewerParticipant` (default false) marks the ANONYMOUS participate-page use: it
 * tells the gateway to skip source auto-activation and bounds reconnects, so a viewer tab
 * on an inactive overlay can't drive YouTube quota or reconnect-storm (P2-3). The
 * streamer's OWN OBS poll/prediction display widgets (and the owner monitor) leave it
 * false so they still auto-activate sources — required for Twitch-native round mirroring —
 * and reconnect indefinitely for the whole stream.
 */
export function useEngagementLive(
  overlayId: string,
  onSignal: (kind: 'poll' | 'prediction') => void,
  opts?: { viewerParticipant?: boolean }
) {
  const viewerParticipant = opts?.viewerParticipant ?? false
  // Keep the latest callback in a ref so re-renders don't churn the socket. Updated in
  // an effect (never during render) so the connection effect below can depend only on
  // overlayId and still call the freshest callback.
  const cbRef = useRef(onSignal)
  useEffect(() => {
    cbRef.current = onSignal
  }, [onSignal])

  useEffect(() => {
    if (!overlayId) return
    // Bound reconnects only for the anonymous participate socket: it's a latency
    // accelerator over the page's HTTP poll, so on a persistently-failing handshake it must
    // give up rather than storm the gateway from every viewer tab. OBS display widgets keep
    // unlimited reconnects (they must survive any blip for the whole stream).
    const client = viewerParticipant
      ? new WebSocketClient({ maxReconnectAttempts: 8 })
      : new WebSocketClient()
    const unsubscribe = client.onEngagementUpdate((kind) => cbRef.current(kind))
    // engagementOnly=true: this socket only wants poll/prediction signals — it must not
    // render chat or touch the shared chat replay watermark. viewerParticipant gates the
    // gateway's skip-source-activation separately (see the hook doc). See WebSocketClient.
    client.connect(overlayId, null, true, viewerParticipant)
    return () => {
      unsubscribe()
      client.disconnect()
    }
  }, [overlayId, viewerParticipant])
}
