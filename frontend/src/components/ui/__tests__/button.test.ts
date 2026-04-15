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

describe('button component gradient variant', () => {
  it('buttonVariants gradient variant includes linear-gradient', async () => {
    const { buttonVariants } = await import('../button')
    const result = buttonVariants({ variant: 'gradient' })
    expect(result).toContain('bg-[linear-gradient(90deg,#9146FF,#69C9D0)]')
  })

  it('buttonVariants gradient variant includes text-white and font-semibold', async () => {
    const { buttonVariants } = await import('../button')
    const result = buttonVariants({ variant: 'gradient' })
    expect(result).toContain('text-white')
    expect(result).toContain('font-semibold')
  })

  it('buttonVariants base classes have no dark: prefixed classes', async () => {
    const { buttonVariants } = await import('../button')
    // Test all known variants
    const variants = [
      'default',
      'outline',
      'secondary',
      'ghost',
      'destructive',
      'link',
      'gradient',
    ] as const
    for (const variant of variants) {
      const result = buttonVariants({ variant })
      const hasDarkClass = result.split(' ').some((cls) => cls.startsWith('dark:'))
      expect(hasDarkClass, `variant "${variant}" should have no dark: classes`).toBe(false)
    }
  })

  it('buttonVariants existing default variant still works', async () => {
    const { buttonVariants } = await import('../button')
    const result = buttonVariants({ variant: 'default' })
    expect(result).toContain('bg-primary')
    expect(result).toContain('text-primary-foreground')
  })
})
