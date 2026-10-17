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

import { afterEach, describe, expect, it, vi } from 'vitest'

import { searchKlipyGifs } from '../klipy'

afterEach(() => vi.unstubAllGlobals())

const item = (id: number, title: string) => ({
  id,
  slug: `s${id}`,
  title,
  file: {
    hd: { gif: { url: `https://static.klipy.com/${id}/hd.gif` } },
    sm: { gif: { url: `https://static.klipy.com/${id}/sm.gif` } },
  },
})

describe('searchKlipyGifs', () => {
  it('queries the documented search endpoint with the key in the path and keeps result order', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ result: true, data: { data: [item(2, 'b'), item(1, 'a')] } }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const gifs = await searchKlipyGifs('party cat', { apiKey: 'k3y' })

    const url = new URL(fetchMock.mock.calls[0][0])
    expect(url.origin + url.pathname).toBe('https://api.klipy.com/api/v1/k3y/gifs/search')
    expect(url.searchParams.get('q')).toBe('party cat')
    expect(gifs).toEqual([
      {
        id: '2',
        title: 'b',
        previewUrl: 'https://static.klipy.com/2/sm.gif',
        url: 'https://static.klipy.com/2/hd.gif',
      },
      {
        id: '1',
        title: 'a',
        previewUrl: 'https://static.klipy.com/1/sm.gif',
        url: 'https://static.klipy.com/1/hd.gif',
      },
    ])
  })

  it('is unavailable without a key, on HTTP errors and on result:false', async () => {
    await expect(searchKlipyGifs('x', { apiKey: '' })).rejects.toThrow()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 404, json: async () => ({}) })
    )
    await expect(searchKlipyGifs('x', { apiKey: 'k' })).rejects.toThrow()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ result: false }) })
    )
    await expect(searchKlipyGifs('x', { apiKey: 'k' })).rejects.toThrow()
  })
})

describe('klipyApiKey', () => {
  // Fresh module per test: the key is memoised at module scope.
  const load = async () => {
    vi.resetModules()
    return (await import('../klipy')).klipyApiKey
  }

  it('fetches the runtime key from /klipy-config and remembers it', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ apiKey: 'rt' }) })
    vi.stubGlobal('fetch', fetchMock)
    const klipyApiKey = await load()
    expect(await klipyApiKey()).toBe('rt')
    expect(await klipyApiKey()).toBe('rt')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toBe('/klipy-config')
    // The route only answers a signed-in, entitled session: send the cookie.
    expect(fetchMock.mock.calls[0][1]).toEqual({ credentials: 'same-origin' })
  })

  it('retries after a failed config fetch', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')))
    const klipyApiKey = await load()
    await expect(klipyApiKey()).rejects.toThrow()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ apiKey: 'k2' }) })
    )
    expect(await klipyApiKey()).toBe('k2')
  })
})
