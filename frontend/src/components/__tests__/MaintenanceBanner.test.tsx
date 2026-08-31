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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MaintenanceBanner } from '@/components/MaintenanceBanner'
import type { MaintenanceWindow } from '@/lib/types/maintenance'

// vi.hoisted, because vi.mock's factory is lifted above the imports and would
// otherwise reference this before initialization.
const api = vi.hoisted(() => ({ upcoming: vi.fn() }))

vi.mock('@/lib/api/maintenance', () => ({ maintenanceApi: api }))

const NOW = new Date('2026-03-04T12:00:00Z')

// An independent oracle for the date text: the same options the banner asks for,
// pinned to English, so the assertions hold whatever time zone CI runs in and
// still fail if the component changes locale or options.
const DATE_OPTIONS: Intl.DateTimeFormatOptions = {
  month: 'short',
  day: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
}
function expectedDate(iso: string): string {
  return new Intl.DateTimeFormat('en', DATE_OPTIONS).format(new Date(iso))
}

function maintenanceWindow(over: Partial<MaintenanceWindow> = {}): MaintenanceWindow {
  return {
    id: 'aaaaaaaa-1111-1111-1111-111111111111',
    title: 'Database upgrade',
    description: '',
    starts_at: '2026-03-04T11:00:00Z',
    ends_at: '2026-03-04T13:00:00Z',
    created_by: 'admin',
    created_at: '2026-03-01T00:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.setSystemTime(NOW)
  api.upcoming.mockReset()
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('MaintenanceBanner', () => {
  it('renders the in-progress copy with the expected completion time', async () => {
    const mw = maintenanceWindow()
    api.upcoming.mockResolvedValue([mw])

    render(<MaintenanceBanner />)

    const status = await waitFor(() => screen.getByRole('status'))
    expect(status).toHaveTextContent(
      `Maintenance in progress: ${mw.title} — Expected completion: ${expectedDate(mw.ends_at)}`
    )
  })

  it('renders the scheduled copy as a start-to-end range', async () => {
    const mw = maintenanceWindow({
      starts_at: '2026-03-05T09:00:00Z',
      ends_at: '2026-03-05T11:00:00Z',
      title: 'Region failover',
    })
    api.upcoming.mockResolvedValue([mw])

    render(<MaintenanceBanner />)

    const status = await waitFor(() => screen.getByRole('status'))
    expect(status).toHaveTextContent(
      `Scheduled maintenance: ${mw.title} — ${expectedDate(mw.starts_at)} to ${expectedDate(
        mw.ends_at
      )}`
    )
  })

  it('labels the dismiss button with the window title', async () => {
    api.upcoming.mockResolvedValue([maintenanceWindow({ title: 'Region failover' })])

    render(<MaintenanceBanner />)

    expect(
      await waitFor(() =>
        screen.getByRole('button', { name: 'Dismiss maintenance banner: Region failover' })
      )
    ).toBeInTheDocument()
  })
})
