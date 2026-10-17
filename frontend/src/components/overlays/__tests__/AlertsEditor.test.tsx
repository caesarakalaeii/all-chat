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
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ALERT_EVENT_TYPES, type EditableAlertProfile } from '@/lib/types/alert'

const getAlertProfiles = vi.fn()
const saveAlertProfiles = vi.fn()
const sendTestAlert = vi.fn()
vi.mock('@/lib/api/overlays', () => ({
  overlaysApi: {
    getAlertProfiles: (...a: unknown[]) => getAlertProfiles(...a),
    saveAlertProfiles: (...a: unknown[]) => saveAlertProfiles(...a),
    sendTestAlert: (...a: unknown[]) => sendTestAlert(...a),
  },
}))
const searchKlipyGifs = vi.fn()
vi.mock('@/lib/klipy', () => ({ searchKlipyGifs: (...a: unknown[]) => searchKlipyGifs(...a) }))
const toast = vi.fn()
vi.mock('@/lib/toast', () => ({ toastManager: { add: (t: unknown) => toast(t) } }))

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

const allEvents = (): EditableAlertProfile => ({ ...base('all'), text_template: '' })

beforeEach(() => {
  getAlertProfiles.mockResolvedValue({
    profiles: [allEvents(), base('subscription'), base('bits')],
    saved: [],
  })
  saveAlertProfiles.mockImplementation((_id: string, p: EditableAlertProfile[]) =>
    Promise.resolve({ profiles: p, saved: p.map((x) => x.event_type) })
  )
  sendTestAlert.mockResolvedValue({ alert_id: 'a1' })
})

/** Switch the event-type picker (a Base UI select) to label. */
async function pickEventType(label: string) {
  fireEvent.click(await screen.findByRole('combobox', { name: 'Event type' }))
  const option = await screen.findByRole('option', { name: new RegExp(`^${label}`) })
  // A bare fireEvent.click does not select a Base UI item in jsdom; a full press does.
  fireEvent.pointerDown(option, { pointerType: 'mouse' })
  fireEvent.mouseDown(option)
  fireEvent.pointerUp(option, { pointerType: 'mouse' })
  fireEvent.mouseUp(option)
  fireEvent.click(option)
}

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
    await pickEventType('Subscription')
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
    await pickEventType('Subscription')
    await screen.findByLabelText('Alert text')
    for (const token of ['{username}', '{amount}', '{count}', '{message}']) {
      expect(screen.getByText(token)).toBeInTheDocument()
    }
  })

  it('locks editing and custom code behind the upgrade link without premium', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium={false} />)
    expect(await screen.findByLabelText('Image or GIF')).toBeDisabled()
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
    expect(await screen.findByLabelText('Image or GIF')).toBeInTheDocument()
  })

  it('opens on the All events style, which has no text of its own', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    expect(await screen.findByLabelText('Event type')).toHaveTextContent('All events')
    expect(screen.getByText(/Applies to every event/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Alert text')).not.toBeInTheDocument()
  })

  it('one GIF on All events is one save, not one per event type', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Image or GIF'), {
      target: { value: 'https://static.klipy.com/hd.gif' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save alerts' }))

    await waitFor(() =>
      expect(saveAlertProfiles).toHaveBeenCalledWith('ov1', [
        { ...allEvents(), image_url: 'https://static.klipy.com/hd.gif' },
      ])
    )
  })

  it('an event type still on the base style shows and keeps the base timing', async () => {
    getAlertProfiles.mockResolvedValue({
      profiles: [{ ...allEvents(), duration_ms: 8000, volume: 40 }, base('subscription')],
      saved: ['all'],
    })
    render(<AlertsEditor overlayId="ov1" isPremium />)
    await pickEventType('Subscription')
    expect(screen.getByText(/Uses the All events style/)).toBeInTheDocument()
    expect(screen.getByLabelText('Duration (seconds)')).toHaveValue(8)

    fireEvent.change(screen.getByLabelText('Alert text'), { target: { value: '{username}!' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save alerts' }))

    await waitFor(() =>
      expect(saveAlertProfiles).toHaveBeenCalledWith('ov1', [
        { ...base('subscription'), text_template: '{username}!', duration_ms: 8000, volume: 40 },
      ])
    )
  })

  it('saves unsaved changes before sending the test, so the test shows them', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Image or GIF'), {
      target: { value: 'https://static.klipy.com/hd.gif' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save and send test alert' }))

    await waitFor(() => expect(sendTestAlert).toHaveBeenCalledWith('ov1', 'subscription'))
    expect(saveAlertProfiles.mock.invocationCallOrder[0]).toBeLessThan(
      sendTestAlert.mock.invocationCallOrder[0]
    )
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(expect.objectContaining({ title: 'Test alert sent' }))
    )
  })

  it('tests the event type being edited', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium />)
    await pickEventType('Bits')
    fireEvent.click(screen.getByRole('button', { name: 'Send test alert' }))
    await waitFor(() => expect(sendTestAlert).toHaveBeenCalledWith('ov1', 'bits'))
    expect(saveAlertProfiles).not.toHaveBeenCalled()
  })

  it('does not send the test when the save fails', async () => {
    saveAlertProfiles.mockRejectedValue(new Error('boom'))
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.change(await screen.findByLabelText('Image or GIF'), {
      target: { value: 'https://static.klipy.com/hd.gif' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save and send test alert' }))
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(expect.objectContaining({ type: 'error' }))
    )
    expect(sendTestAlert).not.toHaveBeenCalled()
  })

  it('sends free test alerts without premium', async () => {
    render(<AlertsEditor overlayId="ov1" isPremium={false} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Send test alert' }))
    await waitFor(() => expect(sendTestAlert).toHaveBeenCalledWith('ov1', 'subscription'))
    expect(saveAlertProfiles).not.toHaveBeenCalled()
  })

  it('reports a failed test send', async () => {
    sendTestAlert.mockRejectedValue(new Error('boom'))
    render(<AlertsEditor overlayId="ov1" isPremium />)
    fireEvent.click(await screen.findByRole('button', { name: 'Send test alert' }))
    await waitFor(() =>
      expect(toast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Could not send the test alert', type: 'error' })
      )
    )
  })

  it('falls back to per-event editing against an overlay-manager without the base style', async () => {
    // overlay-manager deploys from :main only; the beta frontend can meet one
    // that predates ADR-0075 and returns no "all" row and no saved list.
    getAlertProfiles.mockResolvedValue({
      profiles: [base('subscription'), base('bits')],
      saved: [],
    })
    render(<AlertsEditor overlayId="ov1" isPremium />)
    expect(await screen.findByLabelText('Alert text')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Event type' })).toHaveTextContent('Subscription')
    expect(screen.queryByText(/Uses the All events style/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('combobox', { name: 'Event type' }))
    expect(screen.queryByRole('option', { name: /All events/ })).not.toBeInTheDocument()
  })

  it('offers exactly the event types shared/alerts knows', () => {
    // The server owns the list (shared/alerts eventDefs) and 400s an unknown
    // type; this keeps the picker from drifting out of sync with it.
    const goSource = readFileSync(
      path.join(__dirname, '../../../../../shared/alerts/alerts.go'),
      'utf8'
    )
    const goTypes = [...goSource.matchAll(/^\s*\{"([a-z_]+)", "[a-z]+", "/gm)].map((m) => m[1])
    expect(goTypes.length).toBeGreaterThan(0)
    expect([...ALERT_EVENT_TYPES]).toEqual(goTypes)
  })
})
