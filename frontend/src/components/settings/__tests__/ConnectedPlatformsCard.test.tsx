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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, waitFor, fireEvent, cleanup, within } from '@testing-library/react'
import { ConnectedPlatformsCard } from '../ConnectedPlatformsCard'
import { apiClient } from '@/lib/api/client'
import { toastManager } from '@/lib/toast'

vi.mock('@/lib/api/client', () => ({
  apiClient: { get: vi.fn(), delete: vi.fn() },
}))
vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

const mockGet = apiClient.get as unknown as ReturnType<typeof vi.fn>
const mockDelete = apiClient.delete as unknown as ReturnType<typeof vi.fn>
const mockToast = toastManager.add as unknown as ReturnType<typeof vi.fn>

const CONNECTIONS = {
  connections: [
    { platform: 'twitch', accounts: ['streamer'], is_login: true },
    { platform: 'youtube', accounts: ['UCstreamer'], is_login: false },
    { platform: 'facebook', accounts: ['Streamer Page'], is_login: false },
  ],
}

describe('ConnectedPlatformsCard', () => {
  beforeEach(() => {
    mockGet.mockReset()
    mockDelete.mockReset()
    mockToast.mockReset()
  })
  afterEach(() => cleanup())

  it('lists every connected platform and offers no disconnect for the sign-in platform', async () => {
    mockGet.mockResolvedValue(CONNECTIONS)
    render(<ConnectedPlatformsCard />)

    expect(await screen.findByText('UCstreamer')).toBeInTheDocument()
    expect(screen.getByText('Streamer Page')).toBeInTheDocument()
    expect(mockGet).toHaveBeenCalledWith('/api/v1/auth/me/connections')

    expect(screen.getByRole('button', { name: 'Disconnect YouTube' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Disconnect Facebook' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Disconnect Twitch' })).not.toBeInTheDocument()
    expect(screen.getByText(/You sign in with Twitch/)).toBeInTheDocument()
  })

  it('confirms, disconnects and drops the platform from the list', async () => {
    mockGet.mockResolvedValue(CONNECTIONS)
    mockDelete.mockResolvedValue(undefined)
    render(<ConnectedPlatformsCard />)

    fireEvent.click(await screen.findByRole('button', { name: 'Disconnect YouTube' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText('Disconnect YouTube?')).toBeInTheDocument()
    expect(within(dialog).getByText(/asks Google to revoke them/)).toBeInTheDocument()
    expect(
      within(dialog).getByText(/cannot sign in to this account with YouTube/)
    ).toBeInTheDocument()
    // Nothing is deleted until the user confirms.
    expect(mockDelete).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Yes, disconnect' }))

    await waitFor(() =>
      expect(mockDelete).toHaveBeenCalledWith('/api/v1/auth/me/connections/youtube')
    )
    await waitFor(() => expect(screen.queryByText('UCstreamer')).not.toBeInTheDocument())
    expect(mockToast).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'YouTube disconnected', type: 'success' })
    )
  })

  it('tells the user a Facebook disconnect removes the sources and cannot be revoked at Meta', async () => {
    mockGet.mockResolvedValue(CONNECTIONS)
    render(<ConnectedPlatformsCard />)

    fireEvent.click(await screen.findByRole('button', { name: 'Disconnect Facebook' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(within(dialog).getByText(/removes your Facebook sources/)).toBeInTheDocument()
    expect(within(dialog).getByText(/Meta offers no way for us to revoke/)).toBeInTheDocument()
  })

  it('keeps the platform listed and reports a failed disconnect', async () => {
    mockGet.mockResolvedValue(CONNECTIONS)
    mockDelete.mockRejectedValue(new Error('boom'))
    render(<ConnectedPlatformsCard />)

    fireEvent.click(await screen.findByRole('button', { name: 'Disconnect YouTube' }))
    fireEvent.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', {
        name: 'Yes, disconnect',
      })
    )

    await waitFor(() =>
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Could not disconnect YouTube', type: 'error' })
      )
    )
    expect(screen.getByText('UCstreamer')).toBeInTheDocument()
  })

  it('says so when the list cannot be loaded', async () => {
    mockGet.mockRejectedValue(new Error('boom'))
    render(<ConnectedPlatformsCard />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not load your connected platforms.'
    )
  })
})
