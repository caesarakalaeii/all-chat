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
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { LanguageSettingsCard } from '@/components/settings/LanguageSettingsCard'
import { ApiError } from '@/lib/api/client'
import { chooseLocale, getTranslations } from '@/lib/i18n'
import { deMessages } from '@/lib/i18n/messages/de'

const api = vi.hoisted(() => ({ get: vi.fn() }))
vi.mock('@/lib/api/client', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/client')>('@/lib/api/client')
  return { ...actual, apiClient: { get: api.get } }
})

const t = getTranslations('en')

afterEach(async () => {
  cleanup()
  await act(() => chooseLocale('en'))
  localStorage.clear()
})

async function openPicker() {
  fireEvent.click(screen.getByRole('combobox', { name: t('settings.language.selectLabel') }))
  return screen.findByRole('option', { name: 'Deutsch' })
}

describe('LanguageSettingsCard', () => {
  it('offers German when the ui_languages gate admits the user, and switches the UI', async () => {
    api.get.mockResolvedValue({ enabled: true })
    render(<LanguageSettingsCard />)
    await waitFor(() => expect(api.get).toHaveBeenCalledWith('/api/v1/ui-languages'))

    const german = await openPicker()
    expect(german).not.toHaveAttribute('aria-disabled', 'true')
    // A bare fireEvent.click does not select a Base UI item in jsdom; a full press does.
    fireEvent.pointerDown(german, { pointerType: 'mouse' })
    fireEvent.mouseDown(german)
    fireEvent.pointerUp(german, { pointerType: 'mouse' })
    fireEvent.mouseUp(german)
    fireEvent.click(german)

    expect(
      await screen.findByRole('heading', { name: deMessages.settings?.language?.heading })
    ).toBeInTheDocument()
    expect(screen.queryByText(t('settings.language.betaNote'))).not.toBeInTheDocument()
  })

  it('explains the beta and keeps German unselectable on a 403', async () => {
    api.get.mockRejectedValue(new ApiError(403, 'forbidden'))
    render(<LanguageSettingsCard />)

    expect(await screen.findByText(t('settings.language.betaNote'))).toBeInTheDocument()
    expect(await openPicker()).toHaveAttribute('aria-disabled', 'true')
  })
})
