import { NameGradient } from '@/lib/types/message';

/**
 * Converts a NameGradient definition into a CSS linear-gradient() string.
 *
 * @example
 * buildGradientCSS({ type: 'linear', colors: ['#ff0000', '#0000ff'], angle: 90 })
 * // => "linear-gradient(90deg, #ff0000, #0000ff)"
 */
export function buildGradientCSS(g: NameGradient): string {
  return `linear-gradient(${g.angle}deg, ${g.colors.join(', ')})`;
}
