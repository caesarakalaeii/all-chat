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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { GET, dynamic } from '../route'

const KEY = 'test-klipy-key'

// The session check is a call to the gateway's GET /api/v1/auth/me.
const me = (status: number, body: Record<string, unknown> = {}) =>
  vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status }))

const req = (headers: Record<string, string> = {}) =>
  new Request('http://frontend/klipy-config', { headers })

beforeEach(() => {
  vi.stubEnv('KLIPY_API_KEY', KEY)
  vi.stubEnv('API_GATEWAY_URL', 'http://gw:8080')
})
afterEach(() => {
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

async function expectDenied(res: Response, status: number) {
  expect(res.status).toBe(status)
  const text = await res.text()
  expect(text).not.toContain(KEY)
  expect(JSON.parse(text)).toEqual({ apiKey: '' })
}

describe('GET /klipy-config', () => {
  it('is per-request and never shared by a cache', async () => {
    vi.stubGlobal('fetch', me(200, { is_premium: true }))
    const res = await GET(req({ cookie: 'access_token=jwt' }))
    expect(dynamic).toBe('force-dynamic')
    expect(res.headers.get('cache-control')).toBe('private, no-store')
    expect(res.headers.get('vary')).toBe('Cookie, Authorization')
  })

  it('401 for an anonymous caller, without asking the gateway', async () => {
    const fetchMock = me(200, { is_premium: true })
    vi.stubGlobal('fetch', fetchMock)
    const res = await GET(req())
    await expectDenied(res, 401)
    expect(res.headers.get('cache-control')).toBe('private, no-store')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('401 when the gateway rejects the token', async () => {
    vi.stubGlobal('fetch', me(401, { error: 'Unauthorized' }))
    await expectDenied(await GET(req({ cookie: 'access_token=bad' })), 401)
  })

  it('401 when the session check itself fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('ECONNREFUSED')))
    await expectDenied(await GET(req({ cookie: 'access_token=jwt' })), 401)
    vi.stubGlobal('fetch', me(500))
    await expectDenied(await GET(req({ cookie: 'access_token=jwt' })), 401)
  })

  it('403 for a signed-in user without premium or beta access', async () => {
    vi.stubGlobal('fetch', me(200, { id: 'u1', is_premium: false, is_beta_tester: false }))
    await expectDenied(await GET(req({ cookie: 'access_token=jwt' })), 403)
  })

  it('returns the key to a premium user, forwarding only their credentials', async () => {
    const fetchMock = me(200, { is_premium: true })
    vi.stubGlobal('fetch', fetchMock)
    const res = await GET(req({ cookie: 'access_token=jwt', 'x-other': 'no' }))
    expect(res.status).toBe(200)
    expect(await res.json()).toEqual({ apiKey: KEY })
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('http://gw:8080/api/v1/auth/me')
    expect(init.headers).toEqual({ cookie: 'access_token=jwt' })
  })

  it('returns the key to a beta tester, and accepts a bearer token', async () => {
    const fetchMock = me(200, { is_premium: false, is_beta_tester: true })
    vi.stubGlobal('fetch', fetchMock)
    const res = await GET(req({ authorization: 'Bearer jwt' }))
    expect(await res.json()).toEqual({ apiKey: KEY })
    expect(fetchMock.mock.calls[0][1].headers).toEqual({ authorization: 'Bearer jwt' })
  })

  it('returns an empty key when unset, so search degrades to unavailable', async () => {
    vi.stubEnv('KLIPY_API_KEY', '')
    vi.stubGlobal('fetch', me(200, { is_premium: true }))
    const res = await GET(req({ cookie: 'access_token=jwt' }))
    expect(res.status).toBe(200)
    expect(await res.json()).toEqual({ apiKey: '' })
  })
})
