import React from 'react';
import { AbsoluteFill, Easing, interpolate, spring, useCurrentFrame, useVideoConfig } from 'remotion';
import { ChatMessageRow } from '../primitives/ChatMessageRow';
import { PlatformBadge } from '../primitives/PlatformBadge';
import { PLATFORM_HEX, PLATFORM_LABEL } from '../primitives/platform-colors';
import { MOCK_MESSAGES } from '../data/mock-messages';
import type { Platform } from '../primitives/types';

/**
 * Mirrors `/overlays/[id]` (frontend/src/app/overlays/[id]/page.tsx).
 *
 * NEW in v7: a visible mouse cursor that walks through 3 edits, so viewers
 * actually see *what* is being clicked — without the cursor, settings change
 * "magically" and the scene reads as confusing.
 *
 * Click sequence (240-frame scene):
 *   - Frame 35  → username-colour swatch       → username colour purple → cyan
 *   - Frame 115 → "Badges" toggle              → badges ON → OFF
 *   - Frame 185 → bubble-background swatch     → preview tint cyan → pink
 *
 * Each click fires a "Saved" pulse in the breadcrumb chip (matches the
 * real product's auto-save behaviour) and a brief ripple from the cursor.
 */

const SOURCES: Platform[] = ['twitch', 'youtube', 'kick', 'tiktok', 'discord'];

interface Waypoint {
  frame: number;
  x: number;
  y: number;
}

/* Coordinates are in 1920×1080 canvas space.
 * - Left config column starts ~x=56, sections from ~y=88
 * - Username Color swatch sits roughly at (130, 360)
 * - Bubble Background swatch at (130, 435)
 * - Badges toggle (right-side switch) at ~(720, 525)
 * - Theme Pastel pill at ~(200, 130)
 *
 * Tuned visually — adjust if the editor layout shifts. */
const CURSOR_PATH: Waypoint[] = [
  { frame: 0, x: 1700, y: 80 },
  { frame: 30, x: 130, y: 360 } /* arrive at username colour swatch */,
  { frame: 90, x: 130, y: 360 } /* hold while preview updates */,
  { frame: 115, x: 720, y: 525 } /* glide to badges toggle */,
  { frame: 160, x: 720, y: 525 } /* hold */,
  { frame: 185, x: 130, y: 435 } /* glide to bubble background swatch */,
  { frame: 240, x: 130, y: 435 } /* settle */,
];

const CLICK_FRAMES = [35, 115, 185];

function cursorAt(frame: number): { x: number; y: number } {
  if (frame <= CURSOR_PATH[0]!.frame) return { x: CURSOR_PATH[0]!.x, y: CURSOR_PATH[0]!.y };
  for (let i = 1; i < CURSOR_PATH.length; i++) {
    const a = CURSOR_PATH[i - 1]!;
    const b = CURSOR_PATH[i]!;
    if (frame <= b.frame) {
      const t = a.frame === b.frame ? 1 : (frame - a.frame) / (b.frame - a.frame);
      const eased = Easing.bezier(0.4, 0, 0.2, 1)(t);
      return {
        x: a.x + (b.x - a.x) * eased,
        y: a.y + (b.y - a.y) * eased,
      };
    }
  }
  const last = CURSOR_PATH[CURSOR_PATH.length - 1]!;
  return { x: last.x, y: last.y };
}

export const OverlayEditor: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const enter = spring({
    frame,
    fps,
    config: { damping: 16, stiffness: 100 },
    durationInFrames: 30,
  });

  /* Username colour swaps at click 1 and click 3 (3 distinct values). */
  const accent =
    frame < CLICK_FRAMES[0]! ? '#a37bff' : frame < CLICK_FRAMES[2]! ? '#69c9d0' : '#ff79c6';

  /* Badges toggle flips off after click 2. */
  const badgesOn = frame < CLICK_FRAMES[1]!;

  /* "Saved" pill blinks each time a click fires. */
  const savePulse = CLICK_FRAMES.reduce((acc, kf) => {
    const dt = frame - kf;
    if (dt < 0 || dt > 22) return acc;
    return Math.max(acc, 1 - dt / 22);
  }, 0);

  const cursor = cursorAt(frame);

  /* Click ripple — at each click, a ring expands from cursor and fades. */
  const clickRipple = CLICK_FRAMES.reduce(
    (acc, kf) => {
      const dt = frame - kf;
      if (dt < 0 || dt > 18) return acc;
      const t = dt / 18;
      const x = (() => {
        for (let i = 1; i < CURSOR_PATH.length; i++) {
          if (kf <= CURSOR_PATH[i]!.frame) return CURSOR_PATH[i]!.x;
        }
        return CURSOR_PATH[CURSOR_PATH.length - 1]!.x;
      })();
      const y = (() => {
        for (let i = 1; i < CURSOR_PATH.length; i++) {
          if (kf <= CURSOR_PATH[i]!.frame) return CURSOR_PATH[i]!.y;
        }
        return CURSOR_PATH[CURSOR_PATH.length - 1]!.y;
      })();
      if (1 - t > acc.opacity) {
        return { x, y, opacity: 1 - t, scale: 0.4 + t * 1.6 };
      }
      return acc;
    },
    { x: 0, y: 0, opacity: 0, scale: 0 }
  );

  /* Cursor itself dips on click frames. */
  const cursorScale = (() => {
    const c = CLICK_FRAMES.find((kf) => Math.abs(frame - kf) < 4);
    if (c == null) return 1;
    return 1 - 0.18 * (1 - Math.abs(frame - c) / 4);
  })();

  const previewMessages = MOCK_MESSAGES.slice(0, 4);

  return (
    <AbsoluteFill className="bg-grid">
      <AbsoluteFill
        style={{
          padding: '40px 56px 56px',
          flexDirection: 'column',
          gap: 18,
          opacity: enter,
        }}
      >
        {/* Breadcrumb */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 12,
            color: 'var(--color-text-dim)',
            fontSize: 'var(--text-sm)',
            fontFamily: 'var(--font-mono)',
            letterSpacing: '0.04em',
          }}
        >
          <span style={{ textTransform: 'uppercase', letterSpacing: '0.12em' }}>Overlays</span>
          <span>›</span>
          <span style={{ color: 'var(--color-text)' }}>Main Stream</span>
          <span style={{ flex: 1, height: 1, background: 'oklch(from var(--color-text) l c h / 0.06)' }} />
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 8,
              padding: '4px 10px',
              borderRadius: 'var(--radius-full)',
              background: `color-mix(in oklch, ${PLATFORM_HEX.kick} ${10 + savePulse * 25}%, transparent)`,
              color: PLATFORM_HEX.kick,
              opacity: 0.5 + savePulse * 0.5,
            }}
          >
            <span
              style={{
                width: 6,
                height: 6,
                borderRadius: '50%',
                background: PLATFORM_HEX.kick,
                boxShadow: `0 0 ${4 + savePulse * 10}px ${PLATFORM_HEX.kick}`,
              }}
            />
            Saved
          </span>
        </div>

        {/* Split view */}
        <div style={{ display: 'flex', gap: 24, flex: 1, minHeight: 0 }}>
          {/* LEFT — config */}
          <div
            style={{
              width: '40%',
              flexShrink: 0,
              display: 'flex',
              flexDirection: 'column',
              gap: 10,
              overflow: 'hidden',
              transform: `translateX(${(1 - enter) * -32}px)`,
            }}
          >
            <Section title="Theme" open>
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {['Cyberpunk 2077', 'Pastel Kawaii', 'Modern Dark', 'Noita Minimal'].map(
                  (t, i) => {
                    const active = i === 0;
                    return (
                      <span
                        key={t}
                        style={{
                          padding: '6px 12px',
                          borderRadius: 'var(--radius-md)',
                          fontSize: 'var(--text-sm)',
                          border: active
                            ? `1px solid ${accent}`
                            : '1px solid oklch(from var(--color-text) l c h / 0.08)',
                          background: active
                            ? `color-mix(in oklch, ${accent} 14%, transparent)`
                            : 'oklch(from var(--color-bg) l c h / 0.5)',
                          color: active ? 'var(--color-text)' : 'var(--color-text-sub)',
                        }}
                      >
                        {t}
                      </span>
                    );
                  }
                )}
              </div>
            </Section>

            <Section title="Appearance" open>
              <FieldRow label="Body font">
                <FontPills active="Barlow" />
              </FieldRow>
              <FieldRow label="Username color">
                <ColorPickerControl hex={accent} />
              </FieldRow>
              <FieldRow label="Bubble background">
                <ColorPickerControl hex="#0d0d12" opacity={0.85} />
              </FieldRow>
              <FieldRow label="Visibility">
                <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <Toggle label="Avatars" on accent={accent} />
                  <Toggle label="Badges" on={badgesOn} accent={accent} />
                  <Toggle label="Timestamps" on={false} accent={accent} />
                </div>
              </FieldRow>
            </Section>

            <Section title="Sources">
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                {SOURCES.map((s) => (
                  <div
                    key={s}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 10,
                      padding: '8px 10px',
                      borderRadius: 'var(--radius-md)',
                      background: 'oklch(from var(--color-bg) l c h / 0.45)',
                      border: '1px solid oklch(from var(--color-text) l c h / 0.05)',
                    }}
                  >
                    <PlatformBadge platform={s} size="sm" />
                    <span
                      style={{
                        flex: 1,
                        fontSize: 'var(--text-base)',
                        color: 'var(--color-text)',
                      }}
                    >
                      {PLATFORM_LABEL[s]}
                    </span>
                    <span
                      style={{
                        fontSize: 'var(--text-sm)',
                        color: PLATFORM_HEX[s],
                        fontFamily: 'var(--font-mono)',
                        letterSpacing: '0.04em',
                      }}
                    >
                      Connected
                    </span>
                  </div>
                ))}
              </div>
            </Section>

            <Section title="Behavior" />
            <Section title="Expert" />
          </div>

          {/* RIGHT — live preview */}
          <div
            style={{
              flex: 1,
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
              gap: 10,
              transform: `translateX(${(1 - enter) * 40}px)`,
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <span
                style={{
                  fontSize: 'var(--text-sm)',
                  color: 'var(--color-text-dim)',
                  fontFamily: 'var(--font-mono)',
                  letterSpacing: '0.12em',
                  textTransform: 'uppercase',
                }}
              >
                Live preview
              </span>
              <span
                style={{
                  padding: '4px 10px',
                  borderRadius: 'var(--radius-full)',
                  background: 'oklch(from var(--color-text) l c h / 0.06)',
                  fontSize: 'var(--text-sm)',
                  color: 'var(--color-text-sub)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                /overlay/main/preview/embed
              </span>
            </div>

            <div
              style={{
                flex: 1,
                borderRadius: 'var(--radius-xl)',
                border: `1px solid color-mix(in oklch, ${accent} 28%, transparent)`,
                background:
                  'radial-gradient(circle at 25% 20%, oklch(from var(--color-surface) l c h / 0.6), transparent 60%), oklch(from var(--color-bg) l c h / 0.85)',
                padding: 22,
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'flex-end',
                boxShadow: `inset 0 0 80px color-mix(in oklch, ${accent} 10%, transparent)`,
              }}
            >
              {previewMessages.map((m, i) => {
                const visible = frame >= 18 + i * 22;
                if (!visible) return null;
                const localFrame = frame - (18 + i * 22);
                const slide = interpolate(localFrame, [0, 14], [0, 1], {
                  extrapolateLeft: 'clamp',
                  extrapolateRight: 'clamp',
                });
                const tinted = {
                  ...m,
                  user: { ...m.user, color: accent },
                };
                return (
                  <ChatMessageRow
                    key={m.id}
                    message={tinted}
                    enter={slide}
                    fallbackColor={accent}
                  />
                );
              })}
            </div>
          </div>
        </div>
      </AbsoluteFill>

      {/* Cursor + click ripples — drawn last so they sit on top */}
      <CursorOverlay
        x={cursor.x}
        y={cursor.y}
        scale={cursorScale}
        ripple={clickRipple}
      />
    </AbsoluteFill>
  );
};

/* ----- Cursor overlay -------------------------------------------------- */

const CursorOverlay: React.FC<{
  x: number;
  y: number;
  scale: number;
  ripple: { x: number; y: number; opacity: number; scale: number };
}> = ({ x, y, scale, ripple }) => (
  <AbsoluteFill style={{ pointerEvents: 'none' }}>
    {ripple.opacity > 0 && (
      <div
        style={{
          position: 'absolute',
          left: ripple.x - 30,
          top: ripple.y - 30,
          width: 60,
          height: 60,
          borderRadius: '50%',
          border: '2px solid #fff',
          opacity: ripple.opacity * 0.7,
          transform: `scale(${ripple.scale})`,
        }}
      />
    )}
    <svg
      width={32}
      height={32}
      viewBox="0 0 24 24"
      style={{
        position: 'absolute',
        left: x,
        top: y,
        transform: `scale(${scale})`,
        transformOrigin: 'top left',
        filter: 'drop-shadow(0 2px 4px rgba(0,0,0,0.6))',
      }}
      aria-hidden="true"
    >
      <path
        d="M3 2 L3 19 L7.5 15.5 L10.5 21.5 L13 20.5 L10 14.5 L15.5 13.5 Z"
        fill="#ffffff"
        stroke="#000000"
        strokeWidth={1.4}
        strokeLinejoin="round"
      />
    </svg>
  </AbsoluteFill>
);

/* ----- Section / FieldRow / controls (unchanged from v6) ---------------- */

const Section: React.FC<{
  title: string;
  open?: boolean;
  children?: React.ReactNode;
}> = ({ title, open = false, children }) => (
  <div
    style={{
      borderRadius: 'var(--radius-lg)',
      background: 'oklch(from var(--color-surface) l c h / 0.85)',
      border: '1px solid oklch(from var(--color-text) l c h / 0.05)',
      overflow: 'hidden',
    }}
  >
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '12px 16px',
        fontSize: 'var(--text-base)',
        fontWeight: 600,
        color: 'var(--color-text)',
      }}
    >
      <Chevron open={open} />
      {title}
    </div>
    {open && children && (
      <div
        style={{
          padding: '4px 16px 16px',
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
          borderTop: '1px solid oklch(from var(--color-text) l c h / 0.04)',
        }}
      >
        {children}
      </div>
    )}
  </div>
);

const Chevron: React.FC<{ open: boolean }> = ({ open }) => (
  <svg
    width={14}
    height={14}
    viewBox="0 0 24 24"
    fill="none"
    style={{
      color: 'var(--color-text-sub)',
      transform: open ? 'rotate(0)' : 'rotate(-90deg)',
    }}
    aria-hidden="true"
  >
    <path
      d="M6 9l6 6 6-6"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

const FieldRow: React.FC<{ label: string; children: React.ReactNode }> = ({
  label,
  children,
}) => (
  <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
    <span
      style={{
        fontSize: 'var(--text-sm)',
        color: 'var(--color-text-sub)',
        fontFamily: 'var(--font-mono)',
        letterSpacing: '0.06em',
        textTransform: 'uppercase',
      }}
    >
      {label}
    </span>
    {children}
  </div>
);

const ColorPickerControl: React.FC<{ hex: string; opacity?: number }> = ({
  hex,
  opacity = 1,
}) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
    <span
      style={{
        width: 32,
        height: 32,
        borderRadius: 'var(--radius-md)',
        background: hex,
        border: '1px solid oklch(from var(--color-text) l c h / 0.12)',
        boxShadow: `0 0 14px color-mix(in oklch, ${hex} 35%, transparent)`,
        flexShrink: 0,
      }}
    />
    <span
      style={{
        flex: 1,
        padding: '7px 12px',
        borderRadius: 'var(--radius-md)',
        background: 'oklch(from var(--color-bg) l c h / 0.6)',
        border: '1px solid oklch(from var(--color-text) l c h / 0.08)',
        fontFamily: 'var(--font-mono)',
        fontSize: 'var(--text-sm)',
        color: 'var(--color-text)',
        textTransform: 'uppercase',
      }}
    >
      {hex}
    </span>
    {opacity < 1 && (
      <span
        style={{
          padding: '7px 10px',
          borderRadius: 'var(--radius-md)',
          background: 'oklch(from var(--color-bg) l c h / 0.6)',
          border: '1px solid oklch(from var(--color-text) l c h / 0.08)',
          fontFamily: 'var(--font-mono)',
          fontSize: 'var(--text-sm)',
          color: 'var(--color-text-sub)',
        }}
      >
        {Math.round(opacity * 100)}%
      </span>
    )}
  </div>
);

const FontPills: React.FC<{ active: string }> = ({ active }) => (
  <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
    {['Barlow', 'Inter', 'DM Mono', 'Rajdhani', 'Nunito'].map((font) => {
      const isActive = font === active;
      return (
        <span
          key={font}
          style={{
            padding: '6px 12px',
            borderRadius: 'var(--radius-md)',
            border: isActive
              ? '1px solid color-mix(in oklch, var(--color-text) 30%, transparent)'
              : '1px solid oklch(from var(--color-text) l c h / 0.08)',
            background: isActive
              ? 'oklch(from var(--color-text) l c h / 0.1)'
              : 'transparent',
            color: isActive ? 'var(--color-text)' : 'var(--color-text-sub)',
            fontFamily: font === 'DM Mono' ? 'var(--font-mono)' : 'var(--font-sans)',
            fontSize: 'var(--text-sm)',
          }}
        >
          {font}
        </span>
      );
    })}
  </div>
);

const Toggle: React.FC<{ label: string; on: boolean; accent: string }> = ({
  label,
  on,
  accent,
}) => (
  <div
    style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '4px 4px',
      fontSize: 'var(--text-base)',
      color: 'var(--color-text)',
    }}
  >
    <span>{label}</span>
    <span
      style={{
        width: 38,
        height: 22,
        borderRadius: 'var(--radius-full)',
        background: on
          ? `color-mix(in oklch, ${accent} 60%, transparent)`
          : 'oklch(from var(--color-text-dim) l c h / 0.3)',
        border: '1px solid oklch(from var(--color-text) l c h / 0.08)',
        position: 'relative',
        flexShrink: 0,
      }}
    >
      <span
        style={{
          position: 'absolute',
          top: 2,
          left: on ? 18 : 2,
          width: 16,
          height: 16,
          borderRadius: '50%',
          background: '#fff',
          boxShadow: '0 1px 4px oklch(0 0 0 / 0.4)',
        }}
      />
    </span>
  </div>
);
