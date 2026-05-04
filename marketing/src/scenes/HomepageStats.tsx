import React from 'react';
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion';
import { PlatformBadge } from '../primitives/PlatformBadge';
import { PLATFORM_HEX } from '../primitives/platform-colors';
import type { Platform } from '../primitives/types';

/**
 * Mirrors the landing-page hero stat block (frontend/src/app/page.tsx:305-328):
 * 4 magnetic-glow cards in a row, each showing
 *   <PlatformBadge> + formatted count + "messages delivered this week"
 *
 * Real-world feel: the production /api/v1/stats endpoint returns these counts
 * weekly. Snapshotted on 2026-05-04 for this video. Update when re-rendering
 * for a new release if the spread has materially changed.
 *
 * High-octane pacing: cards stagger in (3 frame delay), numbers ramp from 0
 * via easeOutQuint over ~50 frames (~1.7s @ 30fps), then a small landing
 * bounce. Scene is intentionally short (90 frames / 3s).
 */

interface Stat {
  platform: Platform;
  /** Snapshot from https://allch.at/api/v1/stats — refresh before each release. */
  target: number;
}

const STATS: Stat[] = [
  { platform: 'twitch', target: 95_744 },
  { platform: 'youtube', target: 3_698 },
  { platform: 'kick', target: 37_154 },
  { platform: 'tiktok', target: 1_220 },
];

/** Mirrors `formatCount` in frontend/src/app/page.tsx:138-142. */
function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return Math.round(n).toLocaleString();
}

const CARD_STAGGER = 3;
const COUNT_START_OFFSET = 14; /* counters start ramping after the cards land */
const COUNT_DURATION = 50;

export const HomepageStats: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps, durationInFrames, width, height } = useVideoConfig();
  const isVertical = height > width;

  const captionEnter = interpolate(frame, [0, 12], [0, 1], { extrapolateRight: 'clamp' });
  const fadeOut = interpolate(
    frame,
    [durationInFrames - 10, durationInFrames],
    [1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
  );

  const numberSize = isVertical
    ? Math.min(width * 0.16, 200)
    : Math.min(height * 0.13, 152);

  return (
    <AbsoluteFill className="bg-grid">
      <AbsoluteFill
        style={{
          alignItems: 'center',
          justifyContent: 'center',
          padding: isVertical ? '0 48px' : '0 64px',
          flexDirection: 'column',
          gap: 36,
          opacity: fadeOut,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 14,
            opacity: captionEnter,
            transform: `translateY(${(1 - captionEnter) * -10}px)`,
          }}
        >
          <span
            style={{
              fontSize: 'var(--text-sm)',
              textTransform: 'uppercase',
              letterSpacing: '0.22em',
              color: 'var(--color-text-dim)',
              fontFamily: 'var(--font-mono)',
            }}
          >
            allch.at · live
          </span>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: '50%',
              background: PLATFORM_HEX.kick,
              boxShadow: `0 0 12px ${PLATFORM_HEX.kick}`,
            }}
          />
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: isVertical ? 'repeat(2, 1fr)' : 'repeat(4, 1fr)',
            gap: isVertical ? 18 : 24,
            width: '100%',
            maxWidth: isVertical ? '100%' : 1680,
          }}
        >
          {STATS.map((s, i) => {
            const cardEnter = spring({
              frame: frame - i * CARD_STAGGER,
              fps,
              config: { damping: 14, stiffness: 130 },
              durationInFrames: 24,
            });

            const countFrame = Math.max(0, frame - COUNT_START_OFFSET - i * 2);
            const t = Math.min(1, countFrame / COUNT_DURATION);
            const eased = 1 - Math.pow(1 - t, 5); /* easeOutQuint */
            const value = s.target * eased;

            const overshoot = countFrame >= COUNT_DURATION
              ? spring({
                  frame: countFrame - COUNT_DURATION,
                  fps,
                  config: { damping: 8, stiffness: 220, mass: 0.6 },
                  durationInFrames: 16,
                })
              : 0;

            return (
              <StatCard
                key={s.platform}
                platform={s.platform}
                value={value}
                rampProgress={t}
                cardEnter={cardEnter}
                landBounce={overshoot}
                numberSize={numberSize}
              />
            );
          })}
        </div>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};

interface StatCardProps {
  platform: Platform;
  value: number;
  rampProgress: number; /* 0..1 — used to drive glow intensity during ramp */
  cardEnter: number;
  landBounce: number;
  numberSize: number;
}

const StatCard: React.FC<StatCardProps> = ({
  platform,
  value,
  rampProgress,
  cardEnter,
  landBounce,
  numberSize,
}) => {
  const color = PLATFORM_HEX[platform];
  const formatted = formatCount(value);
  /* Glow ramps with the counter, then settles. Slight overshoot on land. */
  const glowIntensity = Math.min(1, rampProgress + landBounce * 0.3);

  return (
    <div
      style={{
        position: 'relative',
        padding: '36px 32px',
        borderRadius: 'var(--radius-xl)',
        background: 'oklch(from var(--color-surface) l c h / 0.92)',
        border: `1px solid color-mix(in oklch, ${color} ${
          12 + glowIntensity * 18
        }%, transparent)`,
        overflow: 'hidden',
        opacity: cardEnter,
        transform: `translateY(${(1 - cardEnter) * 28}px) scale(${
          1 + landBounce * 0.025
        })`,
        boxShadow: `0 8px 30px oklch(0 0 0 / 0.5), 0 0 ${
          30 + glowIntensity * 50
        }px color-mix(in oklch, ${color} ${
          14 + glowIntensity * 22
        }%, transparent)`,
      }}
    >
      {/* Glow blob — replaces the magnetic pointer-tracking glow on the live site */}
      <div
        style={{
          position: 'absolute',
          width: '120%',
          height: '160%',
          left: '-10%',
          top: '-30%',
          background: `radial-gradient(circle at 50% 35%, color-mix(in oklch, ${color} ${
            18 + glowIntensity * 22
          }%, transparent) 0%, transparent 60%)`,
          pointerEvents: 'none',
          filter: 'blur(8px)',
        }}
      />

      <div
        style={{
          position: 'relative',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'flex-start',
          gap: 14,
        }}
      >
        <PlatformBadge platform={platform} size="default" />
        <span
          style={{
            fontSize: numberSize,
            fontWeight: 800,
            color,
            letterSpacing: '-0.05em',
            lineHeight: 1,
            fontVariantNumeric: 'tabular-nums',
            textShadow: `0 0 ${
              16 + landBounce * 24
            }px color-mix(in oklch, ${color} ${
              25 + landBounce * 30
            }%, transparent)`,
          }}
        >
          {formatted}
        </span>
        <span
          style={{
            fontSize: 'var(--text-sm)',
            color: 'var(--color-text-sub)',
            textTransform: 'uppercase',
            letterSpacing: '0.08em',
            fontFamily: 'var(--font-mono)',
          }}
        >
          messages delivered this week
        </span>
      </div>
    </div>
  );
};
