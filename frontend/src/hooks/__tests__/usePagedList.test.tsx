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
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { usePagedList } from '../usePagedList'

interface Row {
  id: number
}

// A fake admin list endpoint: `total` rows with ids 0..total-1, paged by the
// limit/offset query parameters, answered under `rows`.
function serve(total: number) {
  return vi.fn(async (input: string) => {
    const url = new URL(input, 'http://localhost')
    const limit = Number(url.searchParams.get('limit'))
    const offset = Number(url.searchParams.get('offset'))
    const rows = Array.from({ length: Math.max(0, Math.min(limit, total - offset)) }, (_, i) => ({
      id: offset + i,
    }))
    return new Response(JSON.stringify({ rows, total, extra: url.searchParams.get('q') }))
  })
}

let fetchMock: ReturnType<typeof serve>

beforeEach(() => {
  fetchMock = serve(5)
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => vi.unstubAllGlobals())

const ids = (rows: Row[]) => rows.map((r) => r.id)

describe('usePagedList', () => {
  it('loads the first page, then appends the next on loadMore until the total is reached', async () => {
    const { result } = renderHook(() => usePagedList<Row>('/api/rows?q=a', 'rows', 2))
    expect(result.current.loading).toBe(true)

    await waitFor(() => expect(ids(result.current.items)).toEqual([0, 1]))
    expect(fetchMock).toHaveBeenLastCalledWith('/api/rows?q=a&limit=2&offset=0', expect.anything())
    expect(result.current.loading).toBe(false)
    expect(result.current.total).toBe(5)
    expect(result.current.response?.extra).toBe('a')
    expect(result.current.hasMore).toBe(true)

    act(() => result.current.loadMore())
    await waitFor(() => expect(ids(result.current.items)).toEqual([0, 1, 2, 3]))
    act(() => result.current.loadMore())
    await waitFor(() => expect(ids(result.current.items)).toEqual([0, 1, 2, 3, 4]))
    expect(result.current.hasMore).toBe(false)

    act(() => result.current.loadMore())
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('ignores a second loadMore while one is in flight', async () => {
    const { result } = renderHook(() => usePagedList<Row>('/api/rows', 'rows', 2))
    await waitFor(() => expect(result.current.items).toHaveLength(2))

    act(() => {
      result.current.loadMore()
      result.current.loadMore()
    })
    expect(result.current.loadingMore).toBe(true)
    await waitFor(() => expect(ids(result.current.items)).toEqual([0, 1, 2, 3]))
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenLastCalledWith('/api/rows?limit=2&offset=2', expect.anything())
  })

  it('starts over from the first page when the url changes', async () => {
    const { result, rerender } = renderHook(({ url }) => usePagedList<Row>(url, 'rows', 2), {
      initialProps: { url: '/api/rows?q=a' },
    })
    await waitFor(() => expect(result.current.items).toHaveLength(2))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.items).toHaveLength(4))

    rerender({ url: '/api/rows?q=b' })
    expect(result.current.loading).toBe(true)
    expect(result.current.hasMore).toBe(false)
    await waitFor(() => expect(result.current.response?.extra).toBe('b'))
    expect(ids(result.current.items)).toEqual([0, 1])
    expect(fetchMock).toHaveBeenLastCalledWith('/api/rows?q=b&limit=2&offset=0', expect.anything())
  })

  it('drops a response for a url that is no longer current', async () => {
    let releaseFirst: (r: Response) => void = () => {}
    fetchMock.mockImplementationOnce(() => new Promise((resolve) => (releaseFirst = resolve)))
    const { result, rerender } = renderHook(({ url }) => usePagedList<Row>(url, 'rows', 2), {
      initialProps: { url: '/api/rows?q=old' },
    })
    rerender({ url: '/api/rows?q=new' })
    await waitFor(() => expect(result.current.response?.extra).toBe('new'))

    await act(async () =>
      releaseFirst(new Response(JSON.stringify({ rows: [{ id: 99 }], total: 1, extra: 'old' })))
    )
    expect(result.current.response?.extra).toBe('new')
    expect(ids(result.current.items)).toEqual([0, 1])
  })

  it('reload replaces the loaded pages with a fresh first page', async () => {
    const { result } = renderHook(() => usePagedList<Row>('/api/rows', 'rows', 2))
    await waitFor(() => expect(result.current.items).toHaveLength(2))
    act(() => result.current.loadMore())
    await waitFor(() => expect(result.current.items).toHaveLength(4))

    fetchMock.mockImplementation(serve(1))
    act(() => result.current.reload())
    await waitFor(() => expect(ids(result.current.items)).toEqual([0]))
    expect(result.current.total).toBe(1)
  })

  it('reports a failed page as an error and recovers on retry', async () => {
    fetchMock.mockImplementationOnce(async () => new Response('nope', { status: 500 }))
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const { result } = renderHook(() => usePagedList<Row>('/api/rows', 'rows', 2))

    await waitFor(() => expect(result.current.error).toBe(true))
    expect(result.current.loading).toBe(false)

    act(() => result.current.reload())
    await waitFor(() => expect(result.current.items).toHaveLength(2))
    expect(result.current.error).toBe(false)
  })

  it('does nothing without a url', () => {
    const { result } = renderHook(() => usePagedList<Row>(null, 'rows'))
    expect(result.current.loading).toBe(false)
    expect(result.current.items).toEqual([])
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
