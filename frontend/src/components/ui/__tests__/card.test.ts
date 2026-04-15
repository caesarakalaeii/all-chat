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

describe('card component exports', () => {
  it('Card and cardVariants are exported', async () => {
    const mod = await import('../card')
    expect(typeof mod.Card).toBe('function')
    expect(typeof mod.cardVariants).toBe('function')
  })

  it('cardVariants default returns expected base classes', async () => {
    const { cardVariants } = await import('../card')
    const result = cardVariants({})
    expect(result).toContain('rounded-xl')
    expect(result).toContain('border')
    expect(result).toContain('border-border')
    expect(result).toContain('bg-surface')
    expect(result).toContain('text-text')
    expect(result).toContain('transition-all')
  })

  it('cardVariants interactive=true adds hover:scale-[1.02] and hover:shadow-lg', async () => {
    const { cardVariants } = await import('../card')
    const result = cardVariants({ interactive: true })
    expect(result).toContain('hover:scale-[1.02]')
    expect(result).toContain('hover:shadow-lg')
    expect(result).toContain('cursor-pointer')
  })

  it('cardVariants interactive=false has no hover scale', async () => {
    const { cardVariants } = await import('../card')
    const result = cardVariants({ interactive: false })
    expect(result).not.toContain('hover:scale-[1.02]')
    expect(result).not.toContain('hover:shadow-lg')
  })
})
