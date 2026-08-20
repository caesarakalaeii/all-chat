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
 * Settings → Paired devices page tests (ADR-0049).
 *
 * The load-bearing one is the mirror image of the api-tokens page's: there
 * `allchat_pat_…` is shown exactly once and must never be persisted; here a
 * device secret must never appear AT ALL. A device token goes from auth-service's
 * exchange endpoint to the plugin over the loopback redirect and is never sent to
 * a browser, so any sighting of `allchat_dev_` on this page means a plaintext
 * started travelling a path it was designed not to travel.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import DevicesPage from '@/app/settings/devices/page'
import type { PairedDevice } from '@/lib/api/devices'

// vi.hoisted, because vi.mock's factory is lifted above the imports.
const api = vi.hoisted(() => ({
  listDevices: vi.fn(),
  revokeDevice: vi.fn(),
}))

vi.mock('@/lib/api/devices', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/devices')>('@/lib/api/devices')
  return { ...actual, ...api }
})

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/settings/devices',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

function device(over: Partial<PairedDevice> = {}): PairedDevice {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    name: 'Stream Deck (studio PC)',
    overlay_id: '22222222-2222-2222-2222-222222222222',
    overlay_name: 'Main overlay',
    scopes: ['engagement:write'],
    created_at: '2026-01-05T10:00:00Z',
    last_used_at: '2026-03-01T10:00:00Z',
    expires_at: '2026-06-01T10:00:00Z',
    revoked_at: null,
    ...over,
  }
}

describe('Settings → Paired devices page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listDevices.mockResolvedValue([])
    api.revokeDevice.mockResolvedValue(undefined)
    // A live device: expires_at must be in the future for the Revoke control to
    // render, so anchor "now" rather than depending on the wall clock.
    vi.setSystemTime(new Date('2026-04-01T00:00:00Z'))
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('never renders a device secret, because the browser is never sent one', async () => {
    // The whole reason ADR-0049 preferred loopback over a pasted token: a secret
    // that is never rendered cannot be read aloud, screenshotted or leaked on
    // camera. Unlike the PAT page there is not even a create response here that
    // could carry one, so this assertion covers the whole surface.
    api.listDevices.mockResolvedValue([device()])
    render(<DevicesPage />)
    await screen.findByText('Stream Deck (studio PC)')

    expect(document.body.textContent).not.toContain('allchat_dev_')
    expect(document.body.textContent).not.toContain('token_hash')
    // No copy affordance, because there is nothing to copy.
    expect(screen.queryByRole('button', { name: /copy/i })).toBeNull()
  })

  it('shows the bound overlay, scopes, last used and expiry for each device', async () => {
    api.listDevices.mockResolvedValue([device()])
    render(<DevicesPage />)

    await screen.findByText('Stream Deck (studio PC)')
    // The overlay binding is the property a PAT structurally cannot have, so the
    // list has to make it visible — otherwise a streamer cannot tell which of two
    // decks drives which overlay.
    expect(screen.getByText('Main overlay')).toBeInTheDocument()
    expect(screen.getByText('engagement:write')).toBeInTheDocument()
    expect(document.body.textContent).toContain('Last used')
    expect(document.body.textContent).toContain('Active until')
  })

  it('explains that linking starts in the plugin when there are no devices', async () => {
    render(<DevicesPage />)
    await screen.findByText('No paired devices yet')
    // There is no "create" button here on purpose: a device cannot be minted from
    // the browser, so the empty state has to say where linking actually begins.
    expect(screen.queryByRole('button', { name: /create/i })).toBeNull()
    expect(document.body.textContent).toContain('Link with All-Chat')
  })

  it('revokes a device after the confirmation dialog', async () => {
    api.listDevices.mockResolvedValue([device()])
    render(<DevicesPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Revoke Stream Deck (studio PC)' }))
    // Two-step: the row button only arms the alert dialog.
    expect(api.revokeDevice).not.toHaveBeenCalled()

    fireEvent.click(await screen.findByRole('button', { name: 'Revoke device' }))
    await waitFor(() => {
      expect(api.revokeDevice).toHaveBeenCalledWith('11111111-1111-1111-1111-111111111111')
    })
    await waitFor(() => {
      expect(screen.queryByText('Stream Deck (studio PC)')).toBeNull()
    })
  })

  it('offers no revoke control for a device that is already revoked or lapsed', async () => {
    api.listDevices.mockResolvedValue([
      device({ id: 'a', name: 'Revoked deck', revoked_at: '2026-02-01T10:00:00Z' }),
      device({ id: 'b', name: 'Lapsed deck', expires_at: '2026-02-01T10:00:00Z' }),
    ])
    render(<DevicesPage />)

    await screen.findByText('Revoked deck')
    expect(screen.queryByRole('button', { name: 'Revoke Revoked deck' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Revoke Lapsed deck' })).toBeNull()
    // Both are kept in the list so a streamer can see what lapsed and when.
    expect(document.body.textContent).toContain('Revoked')
    expect(document.body.textContent).toContain('Expired')
  })

  it('points at personal access tokens for the case linking cannot reach', async () => {
    // ADR-0051 is not deprecated by ADR-0049: a loopback redirect cannot reach a
    // headless capture box or a second PC, which is exactly what a pasted token is
    // for. The page has to say so or a streamer on that setup thinks the feature
    // is broken.
    render(<DevicesPage />)
    await screen.findByText('No paired devices yet')
    expect(screen.getByRole('link', { name: 'personal access token' })).toHaveAttribute(
      'href',
      '/settings/api-tokens'
    )
  })

  it('surfaces a load failure as an announced error', async () => {
    api.listDevices.mockRejectedValue(new Error('boom'))
    render(<DevicesPage />)
    const alert = await screen.findByRole('alert')
    expect(alert.textContent).toContain('Could not load your paired devices')
  })
})
