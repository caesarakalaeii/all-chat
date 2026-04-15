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

import { describe, it, expect } from 'vitest'

describe('skeleton component exports', () => {
  it('Skeleton is exported as a function', async () => {
    const mod = await import('../skeleton')
    expect(typeof mod.Skeleton).toBe('function')
  })

  it('Skeleton function exists and is callable', async () => {
    const { Skeleton } = await import('../skeleton')
    // Just verify it's a valid function with no issues
    expect(Skeleton).toBeDefined()
    expect(typeof Skeleton).toBe('function')
  })
})
