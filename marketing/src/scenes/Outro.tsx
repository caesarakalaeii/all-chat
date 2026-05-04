import React from 'react';
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion';
import { InfinityLogo } from '../primitives/InfinityLogo';
import { PLATFORM_HEX } from '../primitives/platform-colors';

/**
 * CTA. Three lines, in priority order:
 *   1. The promise. "No install. Hands-off once setup." — top user USP.
 *   2. The URL. allch.at — confirmed in metadataBase (frontend/src/app/layout.tsx:56).
 *   3. The repo. github.com/caesarakalaeii/all-chat — visible in landing footer.
 */
export const Outro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps, durationInFrames, height } = useVideoConfig();

  const cardEnter = spring({
    frame,
    fps,
    config: { damping: 14, stiffness: 110 },
    durationInFrames: 30,
  });
  const fadeOut = interpolate(frame, [durationInFrames - 12, durationInFrames], [1, 0], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  const platforms = [
    PLATFORM_HEX.twitch,
    PLATFORM_HEX.youtube,
    PLATFORM_HEX.kick,
    PLATFORM_HEX.tiktok,
    PLATFORM_HEX.discord,
  ];

  const { width } = useVideoConfig();
  const isVertical = height > width;
  const logoSize = isVertical
    ? Math.min(width * 0.2, 240)
    : Math.min(height * 0.12, 160);
  const headlineSize = isVertical
    ? Math.min(width * 0.062, 70)
    : Math.min(height * 0.05, 56);
  const ctaSize = isVertical
    ? Math.min(width * 0.06, 64)
    : Math.min(height * 0.04, 44);

  return (
    <AbsoluteFill className="bg-grid">
      <AbsoluteFill
        style={{
          alignItems: 'center',
          justifyContent: 'center',
          flexDirection: 'column',
          gap: 28,
          padding: '0 64px',
          textAlign: 'center',
          opacity: cardEnter * fadeOut,
          transform: `translateY(${(1 - cardEnter) * 30}px)`,
        }}
      >
        <InfinityLogo size={logoSize} />

        <div
          style={{
            display: 'flex',
            gap: 14,
          }}
        >
          {platforms.map((c) => (
            <span
              key={c}
              style={{
                width: 12,
                height: 12,
                borderRadius: '50%',
                background: c,
                boxShadow: `0 0 16px ${c}`,
                opacity: 0.85,
              }}
            />
          ))}
        </div>

        <h2
          style={{
            margin: 0,
            fontSize: headlineSize,
            fontWeight: 800,
            color: 'var(--color-text)',
            letterSpacing: '-0.025em',
            maxWidth: 1100,
          }}
        >
          No install. No setup. Just a URL in OBS.
        </h2>

        <a
          style={{
            padding: '16px 36px',
            borderRadius: 'var(--radius-full)',
            border: '1px solid color-mix(in oklch, #a37bff 50%, transparent)',
            background: 'color-mix(in oklch, #a37bff 18%, transparent)',
            color: 'var(--color-text)',
            fontSize: ctaSize,
            fontWeight: 700,
            textDecoration: 'none',
            boxShadow: 'var(--shadow-glow-twitch)',
            letterSpacing: '-0.01em',
          }}
        >
          allch.at
        </a>

        <span
          style={{
            fontSize: 'var(--text-sm)',
            color: 'var(--color-text-dim)',
            fontFamily: 'var(--font-mono)',
            letterSpacing: '0.1em',
          }}
        >
          github.com/caesarakalaeii/all-chat
        </span>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};
