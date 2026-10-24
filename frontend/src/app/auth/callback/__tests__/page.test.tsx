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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AuthCallbackPage from '@/app/auth/callback/page'

const nav = vi.hoisted(() => ({ params: new URLSearchParams(), push: vi.fn() }))

vi.mock('next/navigation', () => ({
  useSearchParams: () => nav.params,
  useRouter: () => ({ push: nav.push, replace: vi.fn() }),
}))
vi.mock('@/lib/stores/auth-store', () => ({ useAuthStore: () => ({ setUser: vi.fn() }) }))
vi.mock('@/lib/analytics', () => ({ trackEvent: vi.fn() }))
vi.mock('@/components/InfinityLogo', () => ({ InfinityLogo: () => null }))

beforeEach(() => {
  vi.clearAllMocks()
  globalThis.fetch = vi.fn()
})
afterEach(cleanup)

describe('streamer auth callback errors', () => {
  // auth-service blocks a second account for a channel that is already a source on an
  // existing account and redirects here with error=duplicate_account and no code. The page
  // used to read only `code` and told the streamer "No authentication code received".
  it('explains a blocked duplicate account instead of reporting a missing code', async () => {
    nav.params = new URLSearchParams(
      'error=duplicate_account&existing_username=hedisaurus&platform=kick'
    )
    render(<AuthCallbackPage />)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('hedisaurus')
    expect(alert).toHaveTextContent('Kick')
    expect(alert).not.toHaveTextContent('No authentication code received')
    expect(globalThis.fetch).not.toHaveBeenCalled()
  })

  it('shows the generic failure for any other error the backend reports', async () => {
    nav.params = new URLSearchParams('error=something_else')
    render(<AuthCallbackPage />)

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Authentication failed. Please try again.')
  })

  it('still reports a missing code when there is neither a code nor an error', async () => {
    nav.params = new URLSearchParams('')
    render(<AuthCallbackPage />)

    expect(await screen.findByRole('alert')).toHaveTextContent('No authentication code received')
  })
})
