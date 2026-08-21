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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// A stable router: `useRouter()` returning a fresh object every render would make the
// page's fetch effect re-run and defeat the call counts below.
const push = vi.fn()
const router = { push }
vi.mock('next/navigation', () => ({ useRouter: () => router }))
vi.mock('@/lib/api/maintenance', () => ({
  maintenanceApi: { list: vi.fn(), create: vi.fn(), remove: vi.fn() },
}))
vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

const authState: { user: { id: string; is_admin: boolean } | null } = { user: null }
vi.mock('@/lib/stores/auth-store', () => ({ useAuthStore: () => authState }))

import { maintenanceApi } from '@/lib/api/maintenance'
import { toastManager } from '@/lib/toast'
import AdminMaintenancePage from '@/app/admin/maintenance/page'
import type { MaintenanceWindow } from '@/lib/types/maintenance'

const mockList = vi.mocked(maintenanceApi.list)
const mockRemove = vi.mocked(maintenanceApi.remove)
const mockToast = vi.mocked(toastManager.add)

const window_: MaintenanceWindow = {
  id: 'mw-1',
  title: 'Database upgrade',
  description: 'Postgres 17',
  starts_at: '2099-01-01T00:00:00Z',
  ends_at: '2099-01-01T02:00:00Z',
} as MaintenanceWindow

beforeEach(() => {
  push.mockReset()
  mockList.mockReset()
  mockRemove.mockReset()
  mockToast.mockReset()
  authState.user = { id: 'admin-1', is_admin: true }
})

afterEach(() => cleanup())

describe('AdminMaintenancePage access guard', () => {
  it('redirects a non-admin to the dashboard without listing windows', async () => {
    authState.user = { id: 'user-1', is_admin: false }
    render(<AdminMaintenancePage />)
    await waitFor(() => expect(push).toHaveBeenCalledWith('/dashboard'))
    expect(mockList).not.toHaveBeenCalled()
  })

  it('redirects a signed-out visitor to the dashboard without listing windows', async () => {
    authState.user = null
    render(<AdminMaintenancePage />)
    await waitFor(() => expect(push).toHaveBeenCalledWith('/dashboard'))
    expect(mockList).not.toHaveBeenCalled()
  })
})

describe('AdminMaintenancePage window list', () => {
  it('lists the fetched windows for an admin, fetching exactly once', async () => {
    mockList.mockResolvedValue([window_])
    render(<AdminMaintenancePage />)
    await waitFor(() => expect(screen.getByText('Database upgrade')).toBeInTheDocument())
    expect(screen.getByText('Scheduled Windows (1)')).toBeInTheDocument()
    expect(push).not.toHaveBeenCalled()
    expect(mockList).toHaveBeenCalledTimes(1)
  })

  it('shows the empty state once the fetch returns nothing', async () => {
    mockList.mockResolvedValue([])
    render(<AdminMaintenancePage />)
    await waitFor(() =>
      expect(screen.getByText('No maintenance windows scheduled')).toBeInTheDocument()
    )
  })

  it('leaves the loading skeleton and reports a failed fetch', async () => {
    mockList.mockRejectedValue(new Error('boom'))
    render(<AdminMaintenancePage />)
    await waitFor(() =>
      expect(mockToast).toHaveBeenCalledWith({
        title: 'Failed to load maintenance windows',
        type: 'error',
      })
    )
    await waitFor(() =>
      expect(screen.getByText('No maintenance windows scheduled')).toBeInTheDocument()
    )
  })

  it('refetches the list after deleting a window', async () => {
    mockList.mockResolvedValueOnce([window_]).mockResolvedValueOnce([])
    mockRemove.mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    render(<AdminMaintenancePage />)
    await waitFor(() => expect(screen.getByText('Database upgrade')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Delete Database upgrade' }))

    await waitFor(() =>
      expect(screen.getByText('No maintenance windows scheduled')).toBeInTheDocument()
    )
    expect(mockRemove).toHaveBeenCalledWith('mw-1')
    expect(mockList).toHaveBeenCalledTimes(2)
  })
})
