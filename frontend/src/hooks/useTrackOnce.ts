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
import { trackEvent, type AnalyticsEvent, type EventData } from '@/lib/analytics'

/**
 * Fire a single analytics event exactly once: on mount, or — when `enabled` is
 * passed — the first time it becomes true (e.g. `messages.length > 0` for the
 * "first message rendered" aha-moment). The ref guard makes it idempotent across
 * rerenders and survives React StrictMode's dev double-invoke, so it replaces the
 * hand-rolled `useRef(false)` guards these one-shot events would otherwise need.
 */
export function useTrackOnce(event: AnalyticsEvent, data?: EventData, enabled: boolean = true): void {
  const firedRef = useRef(false)
  useEffect(() => {
    if (firedRef.current || !enabled) return
    firedRef.current = true
    trackEvent(event, data)
  }, [enabled, event, data])
}
