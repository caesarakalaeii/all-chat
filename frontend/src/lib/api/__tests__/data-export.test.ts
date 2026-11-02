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

// The DSGVO data exports (Art. 15/20): each account kind fetches its own file with its own
// credential — the streamer's session cookie, the viewer's bearer token.

import { afterEach, describe, expect, it, vi } from 'vitest'

const get = vi.fn()
vi.mock('../client', () => ({ apiClient: { get: (...a: unknown[]) => get(...a) } }))

import { authApi } from '../auth'
import { viewerApi } from '../viewer'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.clearAllMocks()
})

describe('data export', () => {
  it('fetches the streamer export through the session client', async () => {
    get.mockResolvedValue({ format_version: 2 })
    await expect(authApi.exportData()).resolves.toEqual({ format_version: 2 })
    expect(get).toHaveBeenCalledWith('/api/v1/auth/me/data-export')
  })

  it('fetches the viewer export with the viewer token', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response(JSON.stringify({ format_version: 2 }), { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('window', { location: { origin: 'https://allch.at' } })
    vi.stubGlobal('localStorage', { getItem: () => 'viewer-jwt', removeItem: vi.fn() })

    await expect(viewerApi.exportData()).resolves.toEqual({ format_version: 2 })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('https://allch.at/api/v1/auth/viewer/data-export')
    expect(init.headers.Authorization).toBe('Bearer viewer-jwt')
  })
})
