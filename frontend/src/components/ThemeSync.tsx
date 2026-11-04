'use client'

/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

import { useEffect } from 'react'
import { usePathname } from 'next/navigation'
import { syncTheme } from '@/lib/theme'

/**
 * Keeps the site theme right across client-side navigation (ADR-0087): the
 * <head> script only runs on a full page load, so a link from a themed page to
 * an OBS overlay route, or back, has to move data-theme here.
 */
export function ThemeSync() {
  const pathname = usePathname()
  useEffect(() => {
    syncTheme(pathname)
  }, [pathname])
  return null
}
