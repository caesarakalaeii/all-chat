import React from 'react';
import { AbsoluteFill, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion';
import { InfinityLogo } from '../primitives/InfinityLogo';
import { PlatformBadge } from '../primitives/PlatformBadge';
import { PLATFORM_HEX } from '../primitives/platform-colors';
import { OVERLAYS } from '../data/overlays';
import type { Platform } from '../primitives/types';

const CARD_STAGGER = 7;

/**
 * Mirrors `/dashboard` (frontend/src/app/dashboard/page.tsx). Every visual
 * element pulls from real source:
 *   - AppNav (`AppNav.tsx`): h-[60px], InfinityLogo size=28 + bold lowercase
 *     wordmark, gradient-underlined active link, "Log out" pushed right
 *   - Page header: "Overlays" h1 + Button variant="gradient" "New Overlay"
 *     (twitch→tiktok gradient, h-8 px-2.5 text-sm)
 *   - Card: rounded-xl border border-border bg-surface, p-6, 3px top border
 *     with platform-multi-gradient
 *   - Card body: name (font-semibold, truncate) + Extension badge (puzzle
 *     icon in twitch/15 pill) + trash icon top-right; platform badges row;
 *     "X sources" text + Extension toggle button
 *
 * Extension badge styling matches dashboard/page.tsx:248-253:
 *   className="inline-flex shrink-0 items-center gap-1 rounded border
 *              border-twitch/30 bg-twitch/15 px-1.5 py-0.5 text-[10px]
 *              font-semibold text-twitch"
 */
export const DashboardPreview: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const headerEnter = spring({
    frame,
    fps,
    config: { damping: 16, stiffness: 110 },
    durationInFrames: 28,
  });

  return (
    <AbsoluteFill className="bg-grid">
      {/* AppNav (matches AppNav.tsx) */}
      <div
        style={{
          height: 60,
          display: 'flex',
          alignItems: 'center',
          padding: '0 32px' /* px-8 */,
          borderBottom: '1px solid oklch(from var(--color-text) l c h / 0.06)',
          background: 'var(--color-nav-bg)',
          backdropFilter: 'blur(20px)',
          opacity: headerEnter,
          transform: `translateY(${(1 - headerEnter) * -8}px)`,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginRight: 40 }}>
          <InfinityLogo size={28} />
          <span
            style={{
              fontSize: 16 /* text-base */,
              fontWeight: 800,
              letterSpacing: '-0.02em',
            }}
          >
            all-chat
          </span>
        </div>
        <div style={{ display: 'flex', height: '100%', gap: 2 }}>
          {['Dashboard', 'Flairs', 'Settings'].map((label, i) => {
            const active = i === 0;
            return (
              <span
                key={label}
                style={{
                  position: 'relative',
                  display: 'inline-flex',
                  alignItems: 'center',
                  height: '100%',
                  padding: '0 14px',
                  fontSize: 14 /* text-sm */,
                  color: active ? 'var(--color-text)' : 'var(--color-text-sub)',
                }}
              >
                {label}
                {active && (
                  <span
                    style={{
                      position: 'absolute',
                      left: 14,
                      right: 14,
                      bottom: 0,
                      height: 2,
                      background: `linear-gradient(90deg, ${PLATFORM_HEX.twitch}, ${PLATFORM_HEX.tiktok})`,
                    }}
                  />
                )}
              </span>
            );
          })}
        </div>
        <div style={{ flex: 1 }} />
        <span
          style={{
            fontSize: 14,
            color: 'var(--color-text-sub)',
            padding: '6px 12px',
          }}
        >
          Log out
        </span>
      </div>

      {/* Page content (matches `<main>` in dashboard/page.tsx) */}
      <div
        style={{
          flex: 1,
          padding: '32px 40px 40px',
          display: 'flex',
          flexDirection: 'column',
          gap: 32 /* mb-8 between header and grid */,
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            opacity: headerEnter,
          }}
        >
          <h1
            style={{
              margin: 0,
              fontSize: 24 /* text-2xl */,
              fontWeight: 700,
              color: 'var(--color-text)',
              letterSpacing: '-0.01em',
            }}
          >
            Overlays
          </h1>
          {/* Button variant="gradient" size="default" — h-8 gap-1.5 px-2.5 */}
          <div
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              height: 32,
              padding: '0 10px',
              borderRadius: 8 /* rounded-lg */,
              background: 'linear-gradient(90deg, #9146FF, #69C9D0)',
              color: '#fff',
              fontSize: 14,
              fontWeight: 600,
            }}
          >
            <PlusGlyph size={14} />
            New Overlay
          </div>
        </div>

        {/* 3-column grid (lg:grid-cols-3) */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(3, 1fr)',
            gap: 24 /* gap-6 */,
          }}
        >
          {OVERLAYS.slice(0, 6).map((o, i) => {
            const cardEnter = spring({
              frame: frame - 14 - i * CARD_STAGGER,
              fps,
              config: { damping: 14, stiffness: 110 },
              durationInFrames: 30,
            });
            return (
              <DashboardCard
                key={o.id}
                name={o.name}
                platforms={o.platforms}
                isExtension={o.isExtension}
                enter={cardEnter}
              />
            );
          })}
        </div>
      </div>
    </AbsoluteFill>
  );
};

const DashboardCard: React.FC<{
  name: string;
  platforms: Platform[];
  isExtension: boolean;
  enter: number;
}> = ({ name, platforms, isExtension, enter }) => {
  return (
    <div
      style={{
        /* Card: rounded-xl border border-border bg-surface */
        borderRadius: 12 /* rounded-xl */,
        background: 'var(--color-surface)',
        border: '1px solid var(--color-border)',
        overflow: 'hidden',
        opacity: enter,
        transform: `translateY(${(1 - enter) * 22}px)`,
      }}
    >
      {/* 3px top border with multi-color gradient (mirrors getTopBorderStyle) */}
      <div style={{ height: 3, background: topBorderGradient(platforms) }} />
      <div style={{ padding: 24 /* p-6 */ }}>
        {/* Header: name + Extension badge + trash icon */}
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            marginBottom: 12 /* mb-3 */,
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 8 /* gap-2 */,
              minWidth: 0,
            }}
          >
            <h3
              style={{
                margin: 0,
                fontSize: 16,
                fontWeight: 600,
                color: 'var(--color-text)',
                whiteSpace: 'nowrap',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
              }}
            >
              {name}
            </h3>
            {isExtension && (
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 4,
                  padding: '2px 6px' /* px-1.5 py-0.5 */,
                  borderRadius: 4 /* rounded */,
                  border: `1px solid color-mix(in oklch, ${PLATFORM_HEX.twitch} 30%, transparent)`,
                  background: `color-mix(in oklch, ${PLATFORM_HEX.twitch} 15%, transparent)`,
                  color: PLATFORM_HEX.twitch,
                  fontSize: 10,
                  fontWeight: 600,
                  flexShrink: 0,
                }}
              >
                <PuzzleGlyph size={10} />
                Extension
              </span>
            )}
          </div>
          <TrashGlyph />
        </div>

        {/* Platform badges row: mb-4 flex flex-wrap gap-1.5 */}
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            gap: 6 /* gap-1.5 */,
            marginBottom: 16 /* mb-4 */,
          }}
        >
          {platforms.map((p) => (
            <PlatformBadge key={p} platform={p} size="sm" />
          ))}
        </div>

        {/* Footer: source count + Extension toggle */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <span
            style={{
              fontSize: 12 /* text-xs */,
              color: 'var(--color-text-dim)',
            }}
          >
            {platforms.length} source{platforms.length === 1 ? '' : 's'}
          </span>
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 6,
              fontSize: 13 /* text-[0.8rem] */,
              color: isExtension ? 'var(--color-text-sub)' : 'var(--color-text-sub)',
              padding: '0 10px',
              height: 28 /* h-7 */,
              borderRadius: 12,
            }}
          >
            {!isExtension && <PuzzleGlyph size={12} />}
            {isExtension ? 'Deactivate Extension' : 'Set as Extension Overlay'}
          </span>
        </div>
      </div>
    </div>
  );
};

/* Platform multi-color top border, blends edges to crisp segments. */
function topBorderGradient(platforms: Platform[]): string {
  if (platforms.length === 0) return 'oklch(from var(--color-text) l c h / 0.08)';
  const colors = platforms.map((p) => PLATFORM_HEX[p]);
  if (colors.length === 1) return colors[0]!;
  const segment = 100 / colors.length;
  const blend = 5;
  const stops: string[] = [];
  colors.forEach((color, i) => {
    const start = i * segment;
    const end = (i + 1) * segment;
    if (i === 0) {
      stops.push(`${color} 0%`, `${color} ${end - blend}%`);
    } else if (i === colors.length - 1) {
      stops.push(`${color} ${start + blend}%`, `${color} 100%`);
    } else {
      stops.push(`${color} ${start + blend}%`, `${color} ${end - blend}%`);
    }
  });
  return `linear-gradient(90deg, ${stops.join(', ')})`;
}

/* Lucide-style glyph stand-ins. */

const PlusGlyph: React.FC<{ size: number }> = ({ size }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
  </svg>
);

const PuzzleGlyph: React.FC<{ size: number }> = ({ size }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path
      d="M19 11h-1V8a2 2 0 0 0-2-2h-3V5a2 2 0 1 0-4 0v1H6a2 2 0 0 0-2 2v3H3a2 2 0 1 0 0 4h1v3a2 2 0 0 0 2 2h3v-1a2 2 0 1 1 4 0v1h3a2 2 0 0 0 2-2v-3h1a2 2 0 1 0 0-4z"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinejoin="round"
    />
  </svg>
);

const TrashGlyph: React.FC = () => (
  <svg
    width={16}
    height={16}
    viewBox="0 0 24 24"
    fill="none"
    style={{ color: 'var(--color-text-dim)', flexShrink: 0 }}
    aria-hidden="true"
  >
    <path
      d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2m3 0v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6h14z"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);
