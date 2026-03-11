import { describe, it, expect } from 'vitest';

describe('input component exports', () => {
  it('Input and inputVariants are exported', async () => {
    const mod = await import('../input');
    expect(typeof mod.Input).toBe('function');
    expect(typeof mod.inputVariants).toBe('function');
  });

  it('inputVariants default returns expected base classes', async () => {
    const { inputVariants } = await import('../input');
    const result = inputVariants({});
    expect(result).toContain('w-full');
    expect(result).toContain('rounded-lg');
    expect(result).toContain('border');
    expect(result).toContain('border-border');
    expect(result).toContain('bg-surface');
    expect(result).toContain('text-sm');
    expect(result).toContain('text-text');
    expect(result).toContain('transition-all');
  });

  it('inputVariants default size includes h-9', async () => {
    const { inputVariants } = await import('../input');
    const result = inputVariants({ size: 'default' });
    expect(result).toContain('h-9');
  });

  it('inputVariants sm size includes h-7 text-xs', async () => {
    const { inputVariants } = await import('../input');
    const result = inputVariants({ size: 'sm' });
    expect(result).toContain('h-7');
    expect(result).toContain('text-xs');
  });

  it('inputVariants includes focus-visible ring matching Button pattern', async () => {
    const { inputVariants } = await import('../input');
    const result = inputVariants({});
    expect(result).toContain('focus-visible:border-ring');
    expect(result).toContain('focus-visible:ring-3');
    expect(result).toContain('focus-visible:ring-ring/50');
  });
});
