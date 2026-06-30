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
import { afterEach, describe, expect, it, vi } from 'vitest'
import { startAddSourceReflow } from '@/lib/api/add-source'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('startAddSourceReflow', () => {
  it('returns a redirect when the backend issues an auth_url', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ auth_url: 'https://id.twitch.tv/oauth' }), { status: 200 })
    )
    const result = await startAddSourceReflow('/api/v1/auth/twitch/add-source/o1')
    expect(result).toEqual({ kind: 'redirect', authUrl: 'https://id.twitch.tv/oauth' })
  })

  it('returns "added" on the short-circuit (existing credentials reused)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({ source_added: 'twitch', reused_existing_credentials: true }),
        { status: 200 }
      )
    )
    const result = await startAddSourceReflow('/api/v1/auth/twitch/add-source/o1')
    expect(result).toEqual({ kind: 'added' })
  })

  it('surfaces a server error message instead of failing silently', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ error: 'Failed to add source' }), { status: 500 })
    )
    const result = await startAddSourceReflow('/api/v1/auth/twitch/add-source/o1')
    expect(result).toEqual({ kind: 'error', message: 'Failed to add source' })
  })

  it('treats an unexpected 200 with no auth_url/source_added as an error', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({}), { status: 200 })
    )
    const result = await startAddSourceReflow('/api/v1/auth/twitch/add-source/o1')
    expect(result.kind).toBe('error')
  })

  it('refreshes the expired access cookie and retries once (the H3 fix)', async () => {
    const fetchMock = vi
      .spyOn(globalThis, 'fetch')
      // 1) original request: access cookie expired -> 401
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'token expired' }), { status: 401 })
      )
      // 2) apiClient's cookie refresh succeeds
      .mockResolvedValueOnce(new Response(null, { status: 200 }))
      // 3) retried request now succeeds
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ auth_url: 'https://id.twitch.tv/oauth' }), { status: 200 })
      )

    const result = await startAddSourceReflow('/api/v1/auth/twitch/add-source/o1')

    expect(result).toEqual({ kind: 'redirect', authUrl: 'https://id.twitch.tv/oauth' })
    // original + refresh + retry
    expect(fetchMock).toHaveBeenCalledTimes(3)
    expect(fetchMock.mock.calls[1][0]).toContain('/api/v1/auth/refresh')
  })
})
