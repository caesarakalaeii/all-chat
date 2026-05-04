import React from 'react';
import { PLATFORM_HEX } from './platform-colors';
import type { Platform } from './types';

/**
 * Mirrors `frontend/src/components/ui/badge.tsx` `<PlatformBadge>`. Real one is:
 *   - inline pill with `bg-badge-bg` (oklch low-alpha white)
 *   - rounded-full, `font-mono`, `whitespace-nowrap`
 *   - 1.5×1.5 platform-coloured glow dot before the text
 *   - text content: `platform.toUpperCase()` in the platform colour
 *
 * NOT a circular monogram. Earlier marketing builds used a "TW"/"YT"
 * coin — that was incorrect.
 */
export const PlatformBadge: React.FC<{ platform: Platform; size?: 'sm' | 'default' }> = ({
  platform,
  size = 'default',
}) => {
  const color = PLATFORM_HEX[platform];
  const isSystem = platform === 'system';
  const padding = size === 'sm' ? '2px 8px' : '3px 10px';
  const fontSize = size === 'sm' ? 10 : 12;
  const dotSize = 6;

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        whiteSpace: 'nowrap',
        borderRadius: 'var(--radius-full)',
        background: 'var(--color-badge-bg)',
        color,
        fontFamily: 'var(--font-mono)',
        padding,
        fontSize,
        letterSpacing: '0.04em',
        flexShrink: 0,
      }}
    >
      <span
        style={{
          display: 'inline-block',
          width: dotSize,
          height: dotSize,
          marginRight: 6,
          borderRadius: '50%',
          background: isSystem ? 'var(--color-text-dim)' : color,
          boxShadow: isSystem ? 'none' : `0 0 8px ${color}`,
          flexShrink: 0,
        }}
      />
      {platform.toUpperCase()}
    </span>
  );
};
