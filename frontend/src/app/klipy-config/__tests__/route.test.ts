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

import { GET, dynamic } from '../route'

afterEach(() => vi.unstubAllEnvs())

describe('GET /klipy-config', () => {
  it('reads KLIPY_API_KEY at request time, uncached', async () => {
    vi.stubEnv('KLIPY_API_KEY', 'runtime-key')
    const res = GET()
    expect(dynamic).toBe('force-dynamic')
    expect(res.headers.get('cache-control')).toBe('no-store')
    expect(await res.json()).toEqual({ apiKey: 'runtime-key' })
  })

  it('returns an empty key when unset, so search degrades to unavailable', async () => {
    vi.stubEnv('KLIPY_API_KEY', '')
    expect(await GET().json()).toEqual({ apiKey: '' })
  })
})
