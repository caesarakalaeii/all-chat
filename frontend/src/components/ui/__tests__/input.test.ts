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

describe('input component exports', () => {
  it('Input and inputVariants are exported', async () => {
    const mod = await import('../input')
    expect(typeof mod.Input).toBe('function')
    expect(typeof mod.inputVariants).toBe('function')
  })

  it('inputVariants default returns expected base classes', async () => {
    const { inputVariants } = await import('../input')
    const result = inputVariants({})
    expect(result).toContain('w-full')
    expect(result).toContain('rounded-lg')
    expect(result).toContain('border')
    expect(result).toContain('border-border')
    expect(result).toContain('bg-surface')
    expect(result).toContain('text-sm')
    expect(result).toContain('text-text')
    expect(result).toContain('transition-all')
  })

  it('inputVariants default size includes h-9', async () => {
    const { inputVariants } = await import('../input')
    const result = inputVariants({ size: 'default' })
    expect(result).toContain('h-9')
  })

  it('inputVariants sm size includes h-7 text-xs', async () => {
    const { inputVariants } = await import('../input')
    const result = inputVariants({ size: 'sm' })
    expect(result).toContain('h-7')
    expect(result).toContain('text-xs')
  })

  it('inputVariants includes focus-visible ring matching Button pattern', async () => {
    const { inputVariants } = await import('../input')
    const result = inputVariants({})
    expect(result).toContain('focus-visible:border-ring')
    expect(result).toContain('focus-visible:ring-3')
    expect(result).toContain('focus-visible:ring-ring/50')
  })
})
