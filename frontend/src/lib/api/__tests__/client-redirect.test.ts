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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, apiClient } from '@/lib/api/client'

/**
 * The 401 fallout path: when the cookie refresh fails there is no way back, so the
 * client bounces the browser to '/' with a HARD navigation (window.location.href).
 * That full reload is the point — it drops every in-flight request, every store and
 * every cached component tree that was rendered for the now-invalid session. A soft
 * router.push() would leave all of that alive, so these tests pin the hard
 * navigation: switching to a soft one to satisfy a lint rule must fail here rather
 * than silently break auth.
 */

/** window.location is not writable in jsdom; swap it for a plain settable object. */
function stubLocation(): { href: string; origin: string } {
  const location = { href: 'http://localhost:3000/dashboard', origin: 'http://localhost:3000' }
  Object.defineProperty(window, 'location', { value: location, configurable: true, writable: true })
  return location
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

let location: { href: string; origin: string }

beforeEach(() => {
  location = stubLocation()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('apiClient 401 handling', () => {
  it('hard-navigates to the login path when the cookie refresh fails', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/auth/refresh')) return jsonResponse(401, { error: 'expired' })
      return jsonResponse(401, { error: 'Unauthorized' })
    })
    vi.stubGlobal('fetch', fetchMock as unknown as typeof fetch)

    await expect(apiClient.get('/api/v1/overlays')).rejects.toBeInstanceOf(ApiError)

    expect(location.href).toBe('/')
  })

  it('does not navigate for the /auth/me probe, which runs on public pages too', async () => {
    // init() probes /auth/me on every page. Redirecting a logged-out visitor on the
    // landing page to '/' would reload it forever, so this 401 must stay silent.
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/auth/refresh')) return jsonResponse(401, { error: 'expired' })
      return jsonResponse(401, { error: 'Unauthorized' })
    })
    vi.stubGlobal('fetch', fetchMock as unknown as typeof fetch)

    await expect(apiClient.get('/api/v1/auth/me')).rejects.toBeInstanceOf(ApiError)

    expect(location.href).toBe('http://localhost:3000/dashboard')
  })

  it('does not navigate when the refresh succeeds and the retry works', async () => {
    let overlayCalls = 0
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/auth/refresh')) return jsonResponse(200, {})
      overlayCalls++
      return overlayCalls === 1
        ? jsonResponse(401, { error: 'Unauthorized' })
        : jsonResponse(200, { ok: true })
    })
    vi.stubGlobal('fetch', fetchMock as unknown as typeof fetch)

    await expect(apiClient.get('/api/v1/overlays')).resolves.toEqual({ ok: true })

    expect(location.href).toBe('http://localhost:3000/dashboard')
  })

  it('does not navigate on a reauth_required 401, which the caller surfaces inline', async () => {
    // A revoked platform OAuth token is an application-level signal, not a dead
    // session: the All-Chat session is still valid, so bouncing to '/' would be wrong.
    const fetchMock = vi.fn(async () => jsonResponse(401, { error: 'reauth_required' }))
    vi.stubGlobal('fetch', fetchMock as unknown as typeof fetch)

    await expect(apiClient.get('/api/v1/overlays')).rejects.toBeInstanceOf(ApiError)

    expect(location.href).toBe('http://localhost:3000/dashboard')
    // No refresh was attempted either.
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
