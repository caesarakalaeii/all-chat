import { describe, it, expect } from 'vitest';
import { getUsernameSpanProps } from '@/lib/utils/usernameSpan';

describe('Overlay gradient render', () => {
  it('renders flat color when name_gradient absent', () => {
    const props = getUsernameSpanProps({ color: '#ff0000' });
    expect(props.className).toBe('font-semibold text-sm');
    expect(props.style).toEqual({ color: '#ff0000' });
    expect(props.style).not.toHaveProperty('backgroundImage');
  });

  it('applies bg-clip-text text-transparent when name_gradient present', () => {
    const props = getUsernameSpanProps({
      color: '#ff0000',
      name_gradient: { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 },
    });
    expect(props.className).toContain('bg-clip-text');
    expect(props.className).toContain('text-transparent');
  });

  it('backgroundImage contains linear-gradient when name_gradient present', () => {
    const props = getUsernameSpanProps({
      color: '#ff0000',
      name_gradient: { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 },
    });
    expect(props.style).toHaveProperty('backgroundImage');
    expect((props.style as { backgroundImage: string }).backgroundImage).toContain('linear-gradient');
  });
});
