import React from 'react';
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion';
import { InfinityLogo } from '../primitives/InfinityLogo';

/**
 * Brand intro. Mirrors the landing-page hero block:
 *
 *   <InfinityLogo size={36} /> all-chat
 *   One overlay. Every platform.
 *   See every message from Twitch, YouTube, Kick, TikTok, and Discord …
 *
 * Source: frontend/src/app/page.tsx:292–303.
 *
 * Wordmark + headline spring in. Tagline subhead fades in afterwards. Timing
 * is proportional to scene duration so this works in both the 4s hero slot
 * and the 3s social slot.
 */
export const LogoIntro: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps, durationInFrames, height } = useVideoConfig();

  const wordmarkSpring = spring({
    frame,
    fps,
    config: { damping: 14, stiffness: 110, mass: 0.8 },
    durationInFrames: 40,
  });
  const wordmarkOpacity = interpolate(frame, [0, 20], [0, 1], { extrapolateRight: 'clamp' });

  const headlineFade = 12;
  const headlineStart = Math.min(durationInFrames * 0.25, 30);
  const headlineEnd = Math.min(headlineStart + headlineFade, durationInFrames * 0.45);
  const headlineOpacity = interpolate(frame, [headlineStart, headlineEnd], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  const subStart = Math.min(durationInFrames * 0.5, 60);
  const subEnd = Math.min(subStart + headlineFade, durationInFrames * 0.7);
  const subOpacity = interpolate(frame, [subStart, subEnd], [0, 1], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  const { width } = useVideoConfig();
  const isVertical = height > width;
  const logoSize = isVertical
    ? Math.min(width * 0.22, 260)
    : Math.min(height * 0.18, 220);
  const wordmarkSize = isVertical
    ? Math.min(width * 0.18, 200)
    : Math.min(height * 0.12, 144);

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
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 22,
            opacity: wordmarkOpacity,
            transform: `translateY(${(1 - wordmarkSpring) * 30}px)`,
          }}
        >
          <InfinityLogo size={logoSize} />
          <span
            style={{
              fontSize: wordmarkSize,
              fontWeight: 800,
              letterSpacing: '-0.04em',
              color: 'var(--color-text)',
              lineHeight: 1,
            }}
          >
            all-chat
          </span>
        </div>

        <h1
          style={{
            margin: 0,
            fontSize: isVertical
              ? Math.min(width * 0.075, 84)
              : Math.min(height * 0.06, 72),
            fontWeight: 800,
            letterSpacing: '-0.03em',
            color: 'var(--color-text)',
            opacity: headlineOpacity,
            maxWidth: isVertical ? 980 : 1100,
            lineHeight: 1.05,
          }}
        >
          One overlay. Every platform.
        </h1>

        <p
          style={{
            margin: 0,
            fontSize: isVertical
              ? Math.min(width * 0.034, 38)
              : Math.min(height * 0.025, 28),
            color: 'var(--color-text-sub)',
            opacity: subOpacity,
            letterSpacing: '0.005em',
            maxWidth: isVertical ? 920 : 900,
            lineHeight: 1.4,
          }}
        >
          {isVertical
            ? 'Twitch · YouTube · Kick · TikTok · Discord. No bots, no setup — just a URL in OBS.'
            : 'Twitch, YouTube, Kick, TikTok, Discord — no bots, no setup, just a URL in OBS.'}
        </p>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};
