import React from 'react';
import { AbsoluteFill, interpolate, useCurrentFrame } from 'remotion';
import { MOCK_MESSAGES } from '../data/mock-messages';
import type { ChatMessage } from '../primitives/types';

/**
 * Flash-cuts through three real shipped themes, lifted from
 * `docs/overlay-themes/{cyberpunk,pastel-cute,minimal}-theme.css`. Same chat
 * content rendered three times with each theme's visual treatment — proves
 * "12 Themes, Full CSS Control" is real, not a sales bullet.
 *
 * v7 transition: variants render in PARALLEL, each with a fade window. The
 * fade-out of the outgoing theme overlaps the fade-in of the incoming theme,
 * giving a real crossfade instead of the previous hard cut + black flash.
 *
 * Layout per variant:
 *   variant N visible:  [start, start + DURATION]
 *   variant N+1 visible: [start + DURATION - OVERLAP, ...]
 *
 * With DURATION 60, OVERLAP 18: each theme holds ~42 frames solo, then 18
 * frames crossfading into the next. 3 themes → 60+42+42+18 = 162 frames
 * total but our scene is 150, so we trim slightly: DURATION 56, OVERLAP 14.
 */

const VARIANT_DURATION = 60;
const OVERLAP = 18;

interface ThemeVariant {
  label: string;
  background: string;
  fontFamily: string;
  accent: string;
  renderMessage: (props: { message: ChatMessage; key: React.Key }) => React.ReactNode;
}

/* --- Cyberpunk 2077 -- yellow/black, scan lines, angular notch ---------- */
const Cyberpunk: ThemeVariant = {
  label: 'Theme · Cyberpunk 2077',
  background:
    'linear-gradient(135deg, rgba(0,0,0,0.97) 0%, rgba(20,20,0,0.95) 100%), #000',
  fontFamily: "'Rajdhani', var(--font-sans)",
  accent: '#ffed4e',
  renderMessage: ({ message, key }) => (
    <div
      key={key}
      style={{
        position: 'relative',
        background:
          'linear-gradient(135deg, rgba(0, 0, 0, 0.95) 0%, rgba(20, 20, 0, 0.9) 100%)',
        border: '2px solid #ffed4e',
        borderLeft: '6px solid #ffed4e',
        borderRadius: 0,
        padding: 18,
        marginBottom: 12,
        boxShadow:
          '0 0 20px rgba(255, 237, 78, 0.3), inset 0 1px 0 rgba(255, 237, 78, 0.2), 0 4px 8px rgba(0, 0, 0, 0.8)',
        clipPath:
          'polygon(0 0, 100% 0, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%)',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          position: 'absolute',
          inset: 0,
          background:
            'repeating-linear-gradient(0deg, rgba(255, 237, 78, 0.04) 0px, transparent 1px, transparent 2px, rgba(255, 237, 78, 0.04) 3px)',
          pointerEvents: 'none',
        }}
      />
      <div style={{ position: 'relative', display: 'flex', flexDirection: 'column', gap: 4 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span
            style={{
              fontFamily: "'Rajdhani', sans-serif",
              fontSize: 18,
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
              color: message.user.color ?? '#ffed4e',
              textShadow:
                '2px 0 0 rgba(255, 0, 255, 0.4), -2px 0 0 rgba(0, 255, 255, 0.4), 0 0 10px rgba(255, 237, 78, 0.5)',
            }}
          >
            {message.user.display_name}
          </span>
          <span
            style={{
              fontFamily: "'Share Tech Mono', monospace",
              fontSize: 10,
              fontWeight: 700,
              letterSpacing: '0.2em',
              color: '#000',
              background: '#ffed4e',
              padding: '3px 10px',
              clipPath:
                'polygon(0 0, calc(100% - 6px) 0, 100% 50%, calc(100% - 6px) 100%, 0 100%)',
              boxShadow: '0 0 10px rgba(255, 237, 78, 0.6)',
              textTransform: 'uppercase',
            }}
          >
            {message.platform}
          </span>
        </div>
        <div
          style={{
            fontFamily: "'Rajdhani', sans-serif",
            fontSize: 17,
            color: '#e0e0e0',
            textShadow: '0 0 5px rgba(255, 237, 78, 0.3)',
          }}
        >
          {message.message.text}
        </div>
      </div>
    </div>
  ),
};

/* --- Pastel Kawaii -- pink/lavender gradient, rounded clouds ------------- */
const Pastel: ThemeVariant = {
  label: 'Theme · Pastel Kawaii',
  background:
    'linear-gradient(135deg, rgba(255, 218, 234, 0.4) 0%, rgba(230, 230, 250, 0.4) 100%), oklch(0.18 0.04 320)',
  fontFamily: "'Nunito', var(--font-sans)",
  accent: '#ffb6c1',
  renderMessage: ({ message, key }) => (
    <div
      key={key}
      style={{
        background:
          'linear-gradient(135deg, rgba(255, 218, 234, 0.95) 0%, rgba(230, 230, 250, 0.95) 100%)',
        border: '3px solid rgba(255, 182, 193, 0.6)',
        borderRadius: 24,
        padding: 18,
        marginBottom: 12,
        boxShadow:
          '0 8px 32px rgba(255, 182, 193, 0.3), inset 0 1px 0 rgba(255, 255, 255, 0.8)',
      }}
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <span
            style={{
              fontFamily: "'Nunito', sans-serif",
              fontSize: 17,
              fontWeight: 700,
              color: message.user.color ?? '#7e5b8e',
              textShadow:
                '0 2px 4px rgba(255, 182, 193, 0.3), 0 0 20px rgba(255, 255, 255, 0.8)',
            }}
          >
            {message.user.display_name}
          </span>
          <span
            style={{
              fontFamily: "'Nunito', sans-serif",
              fontSize: 10,
              fontWeight: 700,
              color: '#ffffff',
              background: 'linear-gradient(135deg, #ffc0cb 0%, #ffb6c1 100%)',
              padding: '3px 10px',
              borderRadius: 999,
              border: '2px solid rgba(255, 255, 255, 0.6)',
              boxShadow: '0 4px 12px rgba(255, 182, 193, 0.4)',
              letterSpacing: '0.04em',
              textTransform: 'uppercase',
            }}
          >
            {message.platform}
          </span>
        </div>
        <div
          style={{
            fontFamily: "'Nunito', sans-serif",
            fontSize: 16,
            fontWeight: 600,
            color: '#6b5b8c',
            textShadow: '0 1px 2px rgba(255, 255, 255, 0.8)',
          }}
        >
          {message.message.text}
        </div>
      </div>
    </div>
  ),
};

/* --- Minimal Clean -- inline `username: message`, white-on-outline ------- */
const Minimal: ThemeVariant = {
  label: 'Theme · Minimal Clean',
  background:
    'linear-gradient(135deg, rgba(50, 0, 80, 0.55) 0%, rgba(0, 30, 50, 0.55) 100%), oklch(0.12 0.01 270)',
  fontFamily: "'Roboto', var(--font-sans)",
  accent: '#ffffff',
  renderMessage: ({ message, key }) => {
    const outline = `
      -1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000,
      -2px -2px 0 #000, 2px -2px 0 #000, -2px 2px 0 #000, 2px 2px 0 #000,
      -1px 0 0 #000, 1px 0 0 #000, 0 -1px 0 #000, 0 1px 0 #000`;
    return (
      <div
        key={key}
        style={{
          background: 'transparent',
          border: 'none',
          padding: 0,
          marginBottom: 8,
          fontFamily: "'Roboto', sans-serif",
        }}
      >
        <span
          style={{
            fontSize: 20,
            fontWeight: 700,
            color: message.user.color ?? '#ffffff',
            textShadow: outline,
            marginRight: 4,
          }}
        >
          {message.user.display_name}:
        </span>
        <span
          style={{
            fontSize: 18,
            fontWeight: 400,
            color: '#ffffff',
            textShadow: outline,
          }}
        >
          {' '}
          {message.message.text}
        </span>
      </div>
    );
  },
};

const VARIANTS: ThemeVariant[] = [Cyberpunk, Pastel, Minimal];

export const CustomizationFlash: React.FC = () => {
  const frame = useCurrentFrame();
  const sample = MOCK_MESSAGES.slice(0, 3);

  /* Each variant occupies [start, start + DURATION] with OVERLAP-frame
     crossfades on each side (except the very first/last edges). */
  return (
    <AbsoluteFill style={{ background: 'var(--color-bg)' }}>
      {VARIANTS.map((variant, i) => {
        const start = i * (VARIANT_DURATION - OVERLAP);
        const end = start + VARIANT_DURATION;
        const opacity = interpolate(
          frame,
          [start, start + OVERLAP, end - OVERLAP, end],
          [0, 1, 1, 0],
          { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
        );
        if (opacity <= 0) return null;
        return (
          <AbsoluteFill key={variant.label} style={{ opacity, background: variant.background }}>
            <AbsoluteFill
              style={{
                padding: '70px 90px',
                flexDirection: 'column',
                justifyContent: 'center',
                alignItems: 'center',
                gap: 28,
                fontFamily: variant.fontFamily,
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 14,
                  fontSize: 'var(--text-sm)',
                  textTransform: 'uppercase',
                  letterSpacing: '0.18em',
                  color: variant.accent,
                  fontFamily: 'var(--font-mono)',
                }}
              >
                <span
                  style={{
                    width: 10,
                    height: 10,
                    borderRadius: '50%',
                    background: variant.accent,
                    boxShadow: `0 0 12px ${variant.accent}`,
                  }}
                />
                {variant.label}
              </div>

              <div
                style={{
                  padding: 28,
                  borderRadius: 16,
                  background: 'oklch(from var(--color-bg) l c h / 0.45)',
                  backdropFilter: 'blur(12px)',
                  display: 'flex',
                  flexDirection: 'column',
                  width: 1280,
                  maxWidth: '85%',
                }}
              >
                {sample.map((m) =>
                  variant.renderMessage({ message: m, key: `${i}-${m.id}` })
                )}
              </div>

              {/* High-contrast caption pill — readable across all theme bgs */}
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: 4,
                  padding: '14px 28px',
                  borderRadius: 'var(--radius-full)',
                  background: 'rgba(0, 0, 0, 0.7)',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                  backdropFilter: 'blur(14px)',
                  fontFamily: 'var(--font-sans)',
                  textAlign: 'center',
                }}
              >
                <span
                  style={{
                    fontSize: 'var(--text-xl)',
                    fontWeight: 700,
                    color: '#ffffff',
                    letterSpacing: '-0.01em',
                  }}
                >
                  12 Themes, Full CSS Control
                </span>
                <span
                  style={{
                    fontSize: 'var(--text-base)',
                    color: 'rgba(255, 255, 255, 0.7)',
                    maxWidth: 720,
                  }}
                >
                  From Win98 retro to cyberpunk neon. Pick a built-in theme or write your own CSS.
                </span>
              </div>
            </AbsoluteFill>
          </AbsoluteFill>
        );
      })}
    </AbsoluteFill>
  );
};
