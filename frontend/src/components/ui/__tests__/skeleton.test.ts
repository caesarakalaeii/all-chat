import { describe, it, expect } from 'vitest';

describe('skeleton component exports', () => {
  it('Skeleton is exported as a function', async () => {
    const mod = await import('../skeleton');
    expect(typeof mod.Skeleton).toBe('function');
  });

  it('Skeleton function exists and is callable', async () => {
    const { Skeleton } = await import('../skeleton');
    // Just verify it's a valid function with no issues
    expect(Skeleton).toBeDefined();
    expect(typeof Skeleton).toBe('function');
  });
});
