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

const safeExternalRedirect = vi.hoisted(() => vi.fn())
vi.mock('@/lib/auth/redirect-allowlist', () => ({ safeExternalRedirect }))

const toastAdd = vi.hoisted(() => vi.fn())
vi.mock('@/lib/toast', () => ({ toastManager: { add: toastAdd } }))

import { DockSignIn } from '../DockSignIn'

// InfinityLogo animates an SVG path; jsdom has no path geometry.
;(SVGElement.prototype as unknown as { getTotalLength: () => number }).getTotalLength = () => 0

beforeEach(() => {
  vi.stubGlobal('sessionStorage', { setItem: vi.fn(), getItem: () => null, removeItem: vi.fn() })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('DockSignIn', () => {
  // The whole point of signing in here rather than elsewhere: the session
  // cookie has to land in the dock's own CEF profile, so the OAuth navigation
  // must happen in THIS window.
  it('navigates this window to the provider URL the backend returns', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn()
        .mockResolvedValue({ ok: true, json: async () => ({ auth_url: 'https://twitch.tv/x' }) })
    )
    render(<DockSignIn />)

    screen.getByRole('button', { name: 'Sign in with Twitch' }).click()

    await waitFor(() => expect(safeExternalRedirect).toHaveBeenCalledWith('https://twitch.tv/x'))
    expect(fetch).toHaveBeenCalledWith('/api/v1/auth/twitch/login')
  })

  it('says so rather than doing nothing when the login request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    render(<DockSignIn />)

    screen.getByRole('button', { name: 'Sign in with Kick' }).click()

    await waitFor(() =>
      expect(toastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Could not start sign-in', type: 'error' })
      )
    )
    expect(safeExternalRedirect).not.toHaveBeenCalled()
  })

  // A 500 whose body happens to parse (an HTML error page does not, but a JSON
  // error envelope does) must not be read for an auth_url that is not there.
  it('says so when the login endpoint answers with an error status', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({ error: 'boom' }) })
    )
    render(<DockSignIn />)

    screen.getByRole('button', { name: 'Sign in with Twitch' }).click()

    await waitFor(() =>
      expect(toastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Could not start sign-in', type: 'error' })
      )
    )
    expect(safeExternalRedirect).not.toHaveBeenCalled()
  })

  // A backend that answers 200 with no auth_url is the same dead button as a
  // failed request, and silently swallowing it is what makes it unreportable.
  it('says so when the response carries no provider URL', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }))
    render(<DockSignIn />)

    screen.getByRole('button', { name: 'Sign in with YouTube' }).click()

    await waitFor(() => expect(toastAdd).toHaveBeenCalled())
    expect(safeExternalRedirect).not.toHaveBeenCalled()
  })
})
