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

import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import { LanguageToggle } from '@/components/home/LanguageToggle'
import { chooseLocale } from '@/lib/i18n'

afterEach(async () => {
  cleanup()
  await act(() => chooseLocale('en'))
  localStorage.clear()
})

describe('LanguageToggle', () => {
  it('switches to German without a gate probe and remembers the choice', async () => {
    render(<LanguageToggle />)
    expect(screen.getByRole('button', { name: 'en, English' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )

    fireEvent.click(screen.getByRole('button', { name: 'de, Deutsch' }))

    // The group label is itself catalog copy, so it proves the re-render too.
    expect(await screen.findByRole('group', { name: 'Sprache' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'de, Deutsch' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(document.documentElement.lang).toBe('de')
    expect(localStorage.getItem('allchat_locale')).toBe('de')
  })
})
