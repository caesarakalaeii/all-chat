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
