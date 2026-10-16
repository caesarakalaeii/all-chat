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

import { useEffect } from 'react'

/**
 * Fetches a public JSON read once per url and hands it to seed. A null url
 * fetches nothing: the overlay render route knows its kind only once the
 * public config arrives, and a chat overlay must not pay for a goal or list
 * read on every OBS load.
 */
export function usePublicSeed<T>(url: string | null, seed: (value: T) => void) {
  useEffect(() => {
    if (!url) return
    let cancelled = false
    fetch(url)
      .then((res) => (res.ok ? (res.json() as Promise<T>) : null))
      .then((value) => {
        if (!cancelled && value) seed(value)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [url, seed])
}
