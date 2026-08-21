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
 * Admin Cosmetics Catalog page tests.
 *
 * The page guards on `user.is_admin` and then fetches both catalogues from a
 * single effect. Getting that effect's dependency array right is the whole
 * point of these tests: too few deps is a lint error, but a dep whose identity
 * changes every render is an endless fetch loop. The "exactly once" assertions
 * below fail loudly in the second case, which a lint run cannot detect.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useState } from 'react'
import type { User } from '@/lib/types/auth'
import { apiClient } from '@/lib/api/client'

const push = vi.fn()

// A single router object for the whole module: `useRouter()` returning a fresh
// object on every render would itself churn the effect deps and mask the bug
// these tests exist to catch.
const router = { push, replace: vi.fn(), prefetch: vi.fn() }

vi.mock('next/navigation', () => ({
  useRouter: () => router,
  usePathname: () => '/admin/cosmetics',
  useSearchParams: () => new URLSearchParams(),
}))

vi.mock('@/lib/api/client', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))

// The real store subscribes to zustand; here the current user is swapped per
// test. The returned object is stable per user so re-renders do not fake a
// dependency change.
let currentAuthState: { user: User | null }

vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: () => currentAuthState,
}))

import AdminCosmeticsPage from '../page'

const FRAMES_ENDPOINT = '/api/v1/admin/cosmetics/frames'
const FLAIRS_ENDPOINT = '/api/v1/admin/cosmetics/flairs'

const mockGet = apiClient.get as unknown as ReturnType<typeof vi.fn>

function buildUser(overrides: Partial<User> = {}): User {
  return {
    id: 'u-1',
    username: 'admin',
    display_name: 'Admin',
    is_admin: true,
    is_premium: true,
    is_beta_tester: false,
    is_ambassador: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function callsTo(endpoint: string): number {
  return mockGet.mock.calls.filter((call) => call[0] === endpoint).length
}

/**
 * Renders the page under a parent that can re-render it without changing any
 * prop or store value — the "unrelated re-render" the loop test needs.
 */
function PageWithUnrelatedState() {
  const [ticks, setTicks] = useState(0)
  return (
    <div>
      <button onClick={() => setTicks(ticks + 1)}>re-render {ticks}</button>
      <AdminCosmeticsPage />
    </div>
  )
}

describe('AdminCosmeticsPage', () => {
  beforeEach(() => {
    push.mockReset()
    mockGet.mockReset()
    mockGet.mockResolvedValue({ frames: [], flairs: [] })
    currentAuthState = { user: buildUser() }
  })

  afterEach(() => cleanup())

  it('fetches each catalogue exactly once for an admin', async () => {
    render(<AdminCosmeticsPage />)

    await waitFor(() => expect(callsTo(FRAMES_ENDPOINT)).toBe(1))
    expect(callsTo(FLAIRS_ENDPOINT)).toBe(1)
  })

  it('does not refetch when the page re-renders for an unrelated reason', async () => {
    render(<PageWithUnrelatedState />)

    await waitFor(() => expect(callsTo(FRAMES_ENDPOINT)).toBe(1))

    fireEvent.click(screen.getByRole('button', { name: /re-render/i }))
    await screen.findByRole('button', { name: 're-render 1' })

    expect(callsTo(FRAMES_ENDPOINT)).toBe(1)
    expect(callsTo(FLAIRS_ENDPOINT)).toBe(1)
  })

  it('replaces the loading skeletons with the catalogue once it arrives', async () => {
    mockGet.mockResolvedValue({
      frames: [
        { id: 'f-1', name: 'Gold Ring', image_url: 'https://example.com/g.png', is_premium: true },
      ],
      flairs: [],
    })

    render(<AdminCosmeticsPage />)

    expect(document.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0)

    expect(await screen.findByText('Gold Ring')).toBeInTheDocument()
    expect(document.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(0)
  })

  it('redirects a non-admin to the dashboard without fetching the catalogue', async () => {
    currentAuthState = { user: buildUser({ is_admin: false }) }

    render(<AdminCosmeticsPage />)

    await waitFor(() => expect(push).toHaveBeenCalledWith('/dashboard'))
    expect(mockGet).not.toHaveBeenCalled()
  })
})
