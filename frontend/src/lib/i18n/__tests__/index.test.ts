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

import { describe, expect, it } from 'vitest'

import { getTranslations, useTranslations } from '@/lib/i18n'

describe('useTranslations', () => {
  it('returns the same function on every call so it is safe in a dependency array', () => {
    // A component that resolves copy inside useEffect or useCallback must list t
    // in its dependency array. A fresh closure per render would then re-run the
    // effect on every render — an infinite fetch loop, not a stale-copy bug.
    // Regression guard for /settings/viewer, which does exactly that three times.
    expect(useTranslations()).toBe(useTranslations())
  })
})

describe('getTranslations', () => {
  it('returns the same function for the same locale', () => {
    expect(getTranslations('en')).toBe(getTranslations('en'))
  })
})
