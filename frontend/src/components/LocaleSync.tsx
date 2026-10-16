'use client'

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

import { useEffect } from 'react'
import { restoreStoredLocale } from '@/lib/i18n'

/**
 * Applies the UI language this browser stored (ADR-0066), once, after
 * hydration. Doing it in an effect rather than at module load keeps the first
 * client render identical to the statically rendered HTML.
 */
export function LocaleSync() {
  useEffect(() => {
    void restoreStoredLocale()
  }, [])
  return null
}
