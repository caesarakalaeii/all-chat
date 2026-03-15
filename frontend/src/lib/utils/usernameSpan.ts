import type { CSSProperties } from 'react';
import { NameGradient } from '@/lib/types/message';
import { buildGradientCSS } from '@/lib/utils/gradient';

/**
 * User info subset needed to compute username span rendering props.
 */
export interface UsernameSpanUser {
  color?: string;
  name_gradient?: NameGradient;
}

/**
 * Returns the className and style props for a username span element.
 *
 * When name_gradient is present, returns bg-clip-text text-transparent with
 * a backgroundImage CSS property (pure CSS gradient, no JavaScript animation).
 * When name_gradient is absent, falls back to inline color style.
 */
export function getUsernameSpanProps(user: UsernameSpanUser): {
  className: string;
  style: CSSProperties;
} {
  if (user.name_gradient) {
    return {
      className: 'font-semibold text-sm bg-clip-text text-transparent',
      style: { backgroundImage: buildGradientCSS(user.name_gradient) },
    };
  }
  return {
    className: 'font-semibold text-sm',
    style: { color: user.color || '#FFFFFF' },
  };
}
