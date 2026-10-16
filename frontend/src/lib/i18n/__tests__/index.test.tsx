// @vitest-environment jsdom
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

import { act, cleanup, render, renderHook, screen, waitFor } from '@testing-library/react'
import { useEffect } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  chooseLocale,
  formatNumber,
  getTranslations,
  restoreStoredLocale,
  useTranslations,
} from '@/lib/i18n'
import { deMessages } from '@/lib/i18n/messages/de'
import { enMessages } from '@/lib/i18n/messages/en'

// The active locale is module state, so every test leaves it where it found it.
afterEach(async () => {
  cleanup()
  await chooseLocale('en')
  localStorage.clear()
  document.documentElement.lang = 'en'
})

describe('useTranslations', () => {
  it('returns the same function on every render so it is safe in a dependency array', () => {
    // A component that resolves copy inside useEffect or useCallback must list t
    // in its dependency array. A fresh closure per render would then re-run the
    // effect on every render — an infinite fetch loop, not a stale-copy bug.
    // Regression guard for /settings/viewer, which does exactly that three times.
    const { result, rerender } = renderHook(() => useTranslations())
    const first = result.current
    rerender()
    expect(result.current).toBe(first)
  })

  it('re-renders a subscriber in the chosen language without re-running its effects', async () => {
    // The stored choice lands right after hydration on every page load. An effect
    // listing t must not fire again then: the OAuth callback's is a single-use
    // code exchange.
    const effectRuns = vi.fn()
    function Heading() {
      const t = useTranslations()
      useEffect(() => effectRuns(), [t])
      return <h2>{t('settings.language.heading')}</h2>
    }
    render(<Heading />)
    expect(screen.getByRole('heading').textContent).toBe(enMessages.settings.language.heading)

    await act(() => chooseLocale('de'))

    expect(screen.getByRole('heading').textContent).toBe(deMessages.settings?.language?.heading)
    expect(effectRuns).toHaveBeenCalledTimes(1)
    expect(document.documentElement.lang).toBe('de')
  })
})

describe('getTranslations', () => {
  it('returns the same function for the same locale', () => {
    expect(getTranslations('en')).toBe(getTranslations('en'))
  })

  it('follows a switch without an argument, so module-scope translators are not stuck in English', async () => {
    const t = getTranslations()
    await chooseLocale('de')
    expect(t('settings.language.heading')).toBe(deMessages.settings?.language?.heading)
  })

  it('resolves a key the German catalog leaves out in English, not as the raw key', async () => {
    // legal.* is deliberately untranslated: binding text needs a legal review first.
    await chooseLocale('de')
    expect(getTranslations('de')('legal.privacy.title')).toBe(enMessages.legal.privacy.title)
  })
})

describe('formatters', () => {
  it('format in the active locale', async () => {
    await chooseLocale('de')
    expect(formatNumber(1234567)).toBe('1.234.567')
  })
})

describe('the stored choice', () => {
  it('is remembered by chooseLocale and reapplied by restoreStoredLocale', async () => {
    await chooseLocale('de')
    expect(localStorage.getItem('allchat_locale')).toBe('de')

    await chooseLocale('en')
    localStorage.setItem('allchat_locale', 'de')
    await restoreStoredLocale()
    const { result } = renderHook(() => useTranslations())
    await waitFor(() =>
      expect(result.current('settings.language.heading')).toBe(
        deMessages.settings?.language?.heading
      )
    )
  })

  it('ignores a stored value that is not a supported locale', async () => {
    localStorage.setItem('allchat_locale', 'xx')
    await restoreStoredLocale()
    expect(getTranslations()('settings.language.heading')).toBe(
      enMessages.settings.language.heading
    )
    expect(document.documentElement.lang).toBe('en')
  })
})
