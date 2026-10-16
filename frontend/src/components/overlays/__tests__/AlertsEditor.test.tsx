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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { EditableAlertProfile } from '@/lib/types/alert'

const getAlertProfiles = vi.fn()
const saveAlertProfiles = vi.fn()
vi.mock('@/lib/api/overlays', () => ({
  overlaysApi: {
    getAlertProfiles: (...a: unknown[]) => getAlertProfiles(...a),
    saveAlertProfiles: (...a: unknown[]) => saveAlertProfiles(...a),
  },
}))
const searchKlipyGifs = vi.fn()
vi.mock('@/lib/klipy', () => ({ searchKlipyGifs: (...a: unknown[]) => searchKlipyGifs(...a) }))
vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

import { AlertsEditor, validateAlertProfile } from '@/components/overlays/AlertsEditor'

const base = (eventType: string): EditableAlertProfile => ({
  event_type: eventType,
  text_template: '{username} just subscribed!',
  sound_url: '',
  image_url: '',
  duration_ms: 5000,
  volume: 100,
  enabled: true,
  sort_after: 0,
  custom_html: '',
  custom_css: '',
  custom_js: '',
})

beforeEach(() => {
  getAlertProfiles.mockResolvedValue([base('subscription'), base('bits')])
  saveAlertProfiles.mockImplementation((_id: string, p: EditableAlertProfile[]) =>
    Promise.resolve(p)
  )
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('validateAlertProfile', () => {
  it('accepts the defaults', () => {
    expect(validateAlertProfile(base('bits'))).toEqual({})
  })

  it('mirrors the server limits', () => {
    expect(validateAlertProfile({ ...base('bits'), text_template: 'x'.repeat(501) })).toEqual({
      text_template: 'errorTemplate',
    })
    expect(validateAlertProfile({ ...base('bits'), image_url: 'javascript:alert(1)' })).toEqual({
      image_url: 'errorUrl',
    })
    expect(validateAlertProfile({ ...base('bits'), sound_url: '/relative.mp3' })).toEqual({
      sound_url: 'errorUrl',
    })
    expect(validateAlertProfile({ ...base('bits'), duration_ms: 0 })).toEqual({
      duration_ms: 'errorDuration',
    })
    expect(validateAlertProfile({ ...base('bits'), duration_ms: 60001 })).toEqual({
      duration_ms: 'errorDuration',
    })
    expect(validateAlertProfile({ ...base('bits'), custom_js: 'x'.repeat(64 * 1024 + 1) })).toEqual(
      {
        custom_code: 'errorCustomCode',
      }
    )
  })
})

describe('AlertsEditor', () => {
  it('saves only the changed profile, custom code included', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    const template = await screen.findByLabelText('Alert text')

    fireEvent.change(template, { target: { value: '{username} is amazing' } })
    fireEvent.change(screen.getByLabelText('JavaScript'), {
      target: { value: 'onAlert(function () {})' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save alerts' }))

    await waitFor(() => expect(saveAlertProfiles).toHaveBeenCalledTimes(1))
    expect(saveAlertProfiles).toHaveBeenCalledWith('ov1', [
      {
        ...base('subscription'),
        text_template: '{username} is amazing',
        custom_js: 'onAlert(function () {})',
      },
    ])
    await waitFor(() => expect(screen.getByRole('status')).toHaveTextContent('Alerts saved'))
  })

  it('blocks a save with an invalid field and says why', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Image or GIF'), {
      target: { value: 'not a url' },
    })
    expect(screen.getByText('Use a full http(s) URL.')).toBeInTheDocument()
    expect(screen.getByLabelText('Image or GIF')).toHaveAttribute('aria-invalid', 'true')

    fireEvent.click(screen.getByRole('button', { name: 'Save alerts' }))
    expect(saveAlertProfiles).not.toHaveBeenCalled()
    expect(screen.getByText('Fix the highlighted fields first.')).toBeInTheDocument()
  })

  it('shows the placeholder help', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    await screen.findByLabelText('Alert text')
    for (const token of ['{username}', '{amount}', '{count}', '{message}']) {
      expect(screen.getByText(token)).toBeInTheDocument()
    }
  })

  it('locks editing and custom code behind the upgrade link without premium', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium={false} />)
    expect(await screen.findByLabelText('Alert text')).toBeDisabled()
    expect(screen.getByText('Custom alert code is a Premium feature.')).toBeInTheDocument()
    expect(screen.queryByLabelText('JavaScript')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save alerts' })).toBeDisabled()
    const links = screen.getAllByRole('link', { name: 'Upgrade to Premium' })
    expect(links.length).toBeGreaterThan(0)
    links.forEach((l) => expect(l).toHaveAttribute('href', '/upgrade'))
  })

  it('picks a KLIPY GIF into the image field', async () => {
    searchKlipyGifs.mockResolvedValue([
      {
        id: '1',
        title: 'party',
        previewUrl: 'https://static.klipy.com/s.gif',
        url: 'https://static.klipy.com/hd.gif',
      },
    ])
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Search GIFs'), { target: { value: 'party' } })
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    fireEvent.click(await screen.findByRole('button', { name: 'Use GIF: party' }))
    expect(screen.getByLabelText('Image or GIF')).toHaveValue('https://static.klipy.com/hd.gif')
    expect(screen.getByText('Powered by KLIPY')).toBeInTheDocument()
  })

  it('degrades to "search unavailable" when KLIPY fails', async () => {
    searchKlipyGifs.mockRejectedValue(new Error('klipy: no api key'))
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Search GIFs'), { target: { value: 'party' } })
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))
    expect(await screen.findByText(/GIF search is unavailable/)).toBeInTheDocument()
  })

  it('offers a retry when the profiles fail to load', async () => {
    getAlertProfiles.mockRejectedValueOnce(new Error('down'))
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.click(await screen.findByRole('button', { name: 'Try again' }))
    expect(await screen.findByLabelText('Alert text')).toBeInTheDocument()
  })
})
