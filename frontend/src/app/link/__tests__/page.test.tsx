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

/**
 * /link — device approve screen tests (ADR-0049).
 *
 * This page is the only human decision point in the linking flow, so the tests
 * are about what the streamer is told rather than about plumbing:
 *
 *   - the device name is labelled as SELF-REPORTED, because it comes from the
 *     plugin and we cannot verify it;
 *   - the overlay binding is presented as a choice and defaults to the only
 *     overlay when there is one;
 *   - granting chat:write surfaces the honest limit of the binding (chat send has
 *     no overlay dimension), rather than implying a stronger guarantee;
 *   - the typed-code path is labelled and announces a bad code.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import LinkDevicePage from '@/app/link/page'
import type { PendingLink } from '@/lib/api/devices'

const api = vi.hoisted(() => ({
  getPendingLink: vi.fn(),
  approveDevice: vi.fn(),
  denyDevice: vi.fn(),
}))
const overlaysMock = vi.hoisted(() => ({ list: vi.fn() }))
const searchParams = vi.hoisted(() => ({ value: new URLSearchParams() }))

vi.mock('@/lib/api/devices', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/devices')>('@/lib/api/devices')
  return { ...actual, ...api }
})
vi.mock('@/lib/api/overlays', () => ({ overlaysApi: overlaysMock }))

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/link',
  useSearchParams: () => searchParams.value,
}))

vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const REQUEST_ID = '33333333-3333-3333-3333-333333333333'
const OVERLAY_ID = '22222222-2222-2222-2222-222222222222'

function pending(over: Partial<PendingLink> = {}): PendingLink {
  return {
    request_id: REQUEST_ID,
    flow: 'loopback',
    device_name_self_reported: 'Stream Deck',
    requested_scopes: ['chat:write', 'engagement:write'],
    expires_at: '2026-04-01T00:10:00Z',
    ...over,
  }
}

function overlay(id = OVERLAY_ID, name = 'Main overlay') {
  return {
    id,
    user_id: 'u1',
    name,
    is_active: true,
    is_public_for_viewers: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

describe('/link — device approve screen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    searchParams.value = new URLSearchParams(`request_id=${REQUEST_ID}`)
    api.getPendingLink.mockResolvedValue(pending())
    api.approveDevice.mockResolvedValue({
      request_id: REQUEST_ID,
      flow: 'loopback',
      device_name: 'Stream Deck',
      overlay_id: OVERLAY_ID,
      scopes: ['chat:write'],
      redirect_to: '/api/v1/auth/device/link/callback?request_id=' + REQUEST_ID + '&code=abc',
    })
    api.denyDevice.mockResolvedValue(undefined)
    overlaysMock.list.mockResolvedValue([overlay()])
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('labels the device name as self-reported', async () => {
    render(<LinkDevicePage />)
    await screen.findByText(/A device wants to control your chat/)
    // The plugin chose this string. Presenting it as verified would be a lie the
    // streamer makes a security decision on.
    expect(document.body.textContent).toContain('self-reported by the plugin')
    expect(document.body.textContent).toContain('Stream Deck')
  })

  it('defaults to the only overlay when the streamer has exactly one', async () => {
    render(<LinkDevicePage />)
    const radio = await screen.findByRole('radio', { name: 'Main overlay' })
    // Making someone pick from a list of one is friction on the step where
    // friction is least affordable — the first thirty seconds of using the thing.
    expect(radio).toBeChecked()
    expect(screen.getByRole('button', { name: 'Approve this device' })).toBeEnabled()
  })

  it('requires an explicit overlay choice when there are several', async () => {
    overlaysMock.list.mockResolvedValue([overlay(), overlay('other-overlay', 'Second overlay')])
    render(<LinkDevicePage />)
    await screen.findByRole('radio', { name: 'Main overlay' })
    // No default: which overlay a device may drive is the decision this screen
    // exists to capture, so it must not be guessed on the streamer's behalf.
    expect(screen.getByRole('radio', { name: 'Main overlay' })).not.toBeChecked()
    expect(screen.getByRole('button', { name: 'Approve this device' })).toBeDisabled()
  })

  it('states the honest limit of the overlay binding for chat send', async () => {
    render(<LinkDevicePage />)
    await screen.findByText(/A device wants to control your chat/)
    // chat:write is requested and on by default, so the caveat must be visible:
    // POST /auth/chat/send has no overlay dimension, so the binding does not
    // narrow it. Claiming otherwise would overstate the guarantee.
    expect(document.body.textContent).toContain('sending chat is not per-overlay')
  })

  it('only offers the scopes the plugin actually requested', async () => {
    api.getPendingLink.mockResolvedValue(pending({ requested_scopes: ['engagement:write'] }))
    render(<LinkDevicePage />)
    await screen.findByText(/A device wants to control your chat/)
    expect(screen.getByRole('switch', { name: 'Run polls and predictions' })).toBeInTheDocument()
    // The plugin's request is the ceiling: the streamer may narrow it, never widen
    // it, and the server refuses an unrequested scope anyway.
    expect(screen.queryByRole('switch', { name: 'Send chat messages' })).toBeNull()
  })

  it('approves with the chosen overlay and scopes', async () => {
    render(<LinkDevicePage />)
    await screen.findByRole('radio', { name: 'Main overlay' })
    fireEvent.click(screen.getByRole('switch', { name: 'Send chat messages' }))
    fireEvent.click(screen.getByRole('button', { name: 'Approve this device' }))

    await waitFor(() => expect(api.approveDevice).toHaveBeenCalled())
    const call = api.approveDevice.mock.calls[0][0]
    expect(call.overlayId).toBe(OVERLAY_ID)
    expect(call.scopes).toEqual(['engagement:write'])
    expect(call.requestId).toBe(REQUEST_ID)
  })

  it('denies without needing an overlay or a scope set', async () => {
    render(<LinkDevicePage />)
    fireEvent.click(await screen.findByRole('button', { name: 'Deny' }))
    await waitFor(() => expect(api.denyDevice).toHaveBeenCalledWith({ requestId: REQUEST_ID }))
    expect(api.approveDevice).not.toHaveBeenCalled()
  })

  it('falls back to a labelled code field with an announced error', async () => {
    // The fallback path: no request_id in the URL because the plugin is on another
    // machine. ADR-0049 says this path will rot unless it is tested deliberately.
    searchParams.value = new URLSearchParams()
    api.getPendingLink.mockRejectedValue(new Error('nope'))
    render(<LinkDevicePage />)

    const input = await screen.findByLabelText('Pairing code')
    fireEvent.change(input, { target: { value: 'abcd-efgh' } })
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }))

    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('That code is not valid')
    // aria-invalid so a screen reader hears the field is the problem, not just
    // that an error exists somewhere on the page.
    expect(input).toHaveAttribute('aria-invalid', 'true')
  })

  it('accepts a typed code in any casing and without the dash', async () => {
    searchParams.value = new URLSearchParams()
    api.getPendingLink.mockRejectedValueOnce(new Error('no request id'))
    api.getPendingLink.mockResolvedValueOnce(pending({ flow: 'code' }))
    render(<LinkDevicePage />)

    const input = await screen.findByLabelText('Pairing code')
    fireEvent.change(input, { target: { value: 'abcdefgh' } })
    fireEvent.click(screen.getByRole('button', { name: 'Continue' }))

    await waitFor(() =>
      expect(api.getPendingLink).toHaveBeenLastCalledWith({ userCode: 'ABCDEFGH' })
    )
  })

  it('never renders a device secret', async () => {
    render(<LinkDevicePage />)
    await screen.findByText(/A device wants to control your chat/)
    expect(document.body.textContent).not.toContain('allchat_dev_')
  })
})
