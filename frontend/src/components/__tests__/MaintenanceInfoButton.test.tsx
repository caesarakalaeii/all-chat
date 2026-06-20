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
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { MaintenanceInfoButton } from '@/components/MaintenanceInfoButton'
import { maintenanceApi } from '@/lib/api/maintenance'
import type { MaintenanceWindow } from '@/lib/types/maintenance'

vi.mock('@/lib/api/maintenance', () => ({
  maintenanceApi: { upcoming: vi.fn() },
}))

const upcoming = vi.mocked(maintenanceApi.upcoming)

function makeWindow(overrides: Partial<MaintenanceWindow> = {}): MaintenanceWindow {
  return {
    id: 'mw1',
    title: 'Twitch IRC migration',
    description: 'Re-add your Twitch source to migrate.',
    starts_at: new Date(Date.now() - 3_600_000).toISOString(),
    ends_at: new Date(Date.now() + 3_600_000).toISOString(),
    created_by: 'admin',
    created_at: new Date().toISOString(),
    ...overrides,
  }
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('MaintenanceInfoButton', () => {
  it('renders nothing when there are no maintenance windows', async () => {
    upcoming.mockResolvedValue([])
    const { container } = render(<MaintenanceInfoButton />)
    await waitFor(() => expect(upcoming).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })

  it('reveals the announcement in a popover only after the info icon is clicked', async () => {
    upcoming.mockResolvedValue([makeWindow()])
    render(<MaintenanceInfoButton />)

    const btn = await screen.findByRole('button', { name: /announcement/i })
    // The detail is hidden until the icon is clicked.
    expect(screen.queryByText('Twitch IRC migration')).not.toBeInTheDocument()

    fireEvent.click(btn)
    expect(screen.getByText('Twitch IRC migration')).toBeInTheDocument()
    expect(screen.getByText('Re-add your Twitch source to migrate.')).toBeInTheDocument()
    // An in-progress window is labelled as such.
    expect(screen.getByText(/in progress/i)).toBeInTheDocument()
  })

  it('stays silent when the maintenance fetch fails', async () => {
    upcoming.mockRejectedValue(new Error('network'))
    const { container } = render(<MaintenanceInfoButton />)
    await waitFor(() => expect(upcoming).toHaveBeenCalled())
    expect(container).toBeEmptyDOMElement()
  })
})
