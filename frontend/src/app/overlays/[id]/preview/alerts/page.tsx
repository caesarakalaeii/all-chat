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

'use client'

/**
 * The editor's preview pane for an alerts overlay (issue #954): the same
 * alert box the OBS overlay renders, fed from the same WebSocket, so a test
 * alert sent from the editor plays here exactly as it will on stream —
 * custom-code sandbox included. Passive: an open editor must not count as
 * overlay demand (it would keep upstream discovery running).
 */

import { use } from 'react'

import { AlertPlayer, useAlertQueue } from '@/components/overlays/AlertBox'
import { useOverlayStream } from '@/hooks/useOverlayStream'

export default function AlertsPreviewPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const queue = useAlertQueue()
  useOverlayStream(id, { onAlert: queue.enqueue, passive: true })
  return <AlertPlayer alert={queue.current} onDone={queue.finish} />
}
