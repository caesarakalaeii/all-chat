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

import { useSyncExternalStore } from 'react'

// Pride month is June (Date months are 0-based, so 5). `?pride=1` forces
// it on and `?pride=0` forces it off, to preview the look outside June.
const isPrideMonth = () => {
  const override = new URLSearchParams(window.location.search).get('pride')
  if (override === '1' || override === '0') return override === '1'
  return new Date().getMonth() === 5
}

// The month does not change under a mounted page, so there is nothing to
// subscribe to; the store exists for its server snapshot.
const subscribeNever = () => () => {}

/**
 * Whether the pride month look is on, in the visitor's local calendar.
 *
 * Always `false` on the server and during hydration: prerendered markup may
 * have been built in another month, so it stays the regular look and the
 * client re-renders into pride right after, without a hydration mismatch.
 */
export function usePrideMonth(): boolean {
  return useSyncExternalStore(subscribeNever, isPrideMonth, () => false)
}
