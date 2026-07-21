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
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { AmbassadorSettingsCard } from '../AmbassadorSettingsCard'
import { apiClient } from '@/lib/api/client'

vi.mock('@/lib/api/client', () => ({
  apiClient: { get: vi.fn(), put: vi.fn() },
}))
vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

const mockGet = apiClient.get as unknown as ReturnType<typeof vi.fn>
const mockPut = apiClient.put as unknown as ReturnType<typeof vi.fn>

describe('AmbassadorSettingsCard', () => {
  beforeEach(() => {
    mockGet.mockReset()
    mockPut.mockReset()
  })
  afterEach(() => cleanup())

  it('loads current consent and shows the admin-curated tagline', async () => {
    mockGet.mockResolvedValue({
      is_ambassador: true,
      tagline: 'Multistreams to 3 platforms',
      sort_order: 5,
      featured_consent: true,
    })

    render(<AmbassadorSettingsCard />)

    const sw = await screen.findByRole('switch', { name: /feature me on the homepage/i })
    await waitFor(() => expect(sw).toHaveAttribute('aria-checked', 'true'))
    expect(screen.getByText(/Multistreams to 3 platforms/)).toBeInTheDocument()
    expect(mockGet).toHaveBeenCalledWith('/api/v1/ambassadors/me/showcase')
  })

  it('PUTs the new consent when toggled on', async () => {
    mockGet.mockResolvedValue({
      is_ambassador: true,
      tagline: null,
      sort_order: 0,
      featured_consent: false,
    })
    mockPut.mockResolvedValue({ featured_consent: true })

    render(<AmbassadorSettingsCard />)

    const sw = await screen.findByRole('switch', { name: /feature me on the homepage/i })
    // Switch starts disabled until the initial GET resolves.
    await waitFor(() => expect(sw).not.toBeDisabled())
    expect(sw).toHaveAttribute('aria-checked', 'false')

    fireEvent.click(sw)

    await waitFor(() =>
      expect(mockPut).toHaveBeenCalledWith('/api/v1/ambassadors/me/showcase', {
        featured_consent: true,
      })
    )
  })

  it('defaults to opted-out and does not crash when the load fails', async () => {
    mockGet.mockRejectedValue(new Error('boom'))

    render(<AmbassadorSettingsCard />)

    const sw = await screen.findByRole('switch', { name: /feature me on the homepage/i })
    await waitFor(() => expect(sw).toHaveAttribute('aria-checked', 'false'))
  })
})
