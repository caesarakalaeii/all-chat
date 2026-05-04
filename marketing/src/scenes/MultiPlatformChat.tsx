import React from 'react';
import { AbsoluteFill, interpolate, useCurrentFrame, useVideoConfig } from 'remotion';
import { ChatMessageRow } from '../primitives/ChatMessageRow';
import { MOCK_MESSAGES } from '../data/mock-messages';

/**
 * Renders the default OBS chat overlay as a streamer would actually see it.
 * No platform pills, no left rail — just the chat stream that lands in OBS.
 *
 * Mirrors `frontend/src/app/overlay/[id]/page.tsx:881-1119`:
 *   <div className="min-h-screen p-4 bg-transparent">
 *     <div className="space-y-3">{messages.map(...)}</div>
 *   </div>
 *
 * For video framing: the overlay is rendered into a centred column that
 * approximates the proportion an overlay typically occupies in OBS (about
 * 600-700px wide on a 1920x1080 canvas). The remainder of the frame stays as
 * the marketing-site grid background — sells "this is the product on top of
 * a stream", not a raw screenshot.
 *
 * Auto-flips to fill width when portrait (social composition).
 */

const REVEAL_DURATION = 10;

/* Stable timestamp — counts forward from a fixed base so the video is
 * deterministic across re-renders. */
const TIMESTAMP_BASE_MIN = 25;
function fmtTimestamp(idx: number): string {
  const m = TIMESTAMP_BASE_MIN + idx;
  return `20:${m.toString().padStart(2, '0')}`;
}

export const MultiPlatformChat: React.FC = () => {
  const frame = useCurrentFrame();
  const { width, height } = useVideoConfig();
  const isVertical = height > width;

  const headerOpacity = interpolate(frame, [0, 20], [0, 1], { extrapolateRight: 'clamp' });

  /* Portrait: keep up to 8 messages stacked top-down so the column fills the
     vertical canvas. Each row scales 1.4× for legibility on 1080w. Landscape:
     7 max, bottom-anchored to look like an OBS overlay sitting against the
     bottom of a stream layout. */
  const maxVisible = isVertical ? 8 : 7;
  const visibleMessages = MOCK_MESSAGES.filter((m) => m.revealAt <= frame).slice(-maxVisible);
  const rowScale = isVertical ? 1.4 : 1;

  /* Overlay column width — narrower on landscape to mimic typical OBS sizing,
     full-bleed on portrait. */
  const overlayWidth = isVertical ? '100%' : Math.min(width * 0.46, 880);

  return (
    <AbsoluteFill className="bg-grid">
      <AbsoluteFill
        style={{
          padding: isVertical ? '60px 32px' : '60px 80px',
          flexDirection: 'column',
          gap: 26,
        }}
      >
        {/* Marketing-voice header — small, never overlapping the actual chat */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 14,
            opacity: headerOpacity,
          }}
        >
          <span
            style={{
              fontSize: 'var(--text-sm)',
              textTransform: 'uppercase',
              letterSpacing: '0.18em',
              color: 'var(--color-text-dim)',
              fontFamily: 'var(--font-mono)',
            }}
          >
            OBS Browser Source · /overlay/main
          </span>
          <span
            style={{
              flex: 1,
              height: 1,
              background: 'oklch(from var(--color-text) l c h / 0.08)',
            }}
          />
        </div>

        {/* The actual overlay surface — no chrome, transparent background.
            Mirrors `<div className="min-h-screen p-4 bg-transparent">`. */}
        <div
          style={{
            flex: 1,
            display: 'flex',
            justifyContent: isVertical ? 'stretch' : 'center',
            /* Portrait: anchor TOP so chat fills from top down (no big empty
               area above). Landscape: anchor BOTTOM to match how OBS overlays
               typically sit against the bottom edge of a stream layout. */
            alignItems: isVertical ? 'flex-start' : 'flex-end',
            minHeight: 0,
          }}
        >
          <div
            style={{
              width: overlayWidth,
              padding: 16 /* p-4 */,
              display: 'flex',
              flexDirection: 'column',
              gap: 12 /* space-y-3 */,
            }}
          >
            {visibleMessages.map((m) => {
              const enter = interpolate(
                frame,
                [m.revealAt, m.revealAt + REVEAL_DURATION],
                [0, 1],
                { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
              );
              const idx = MOCK_MESSAGES.indexOf(m);
              return (
                <ChatMessageRow
                  key={m.id}
                  message={m}
                  enter={enter}
                  scale={rowScale}
                  timestampLabel={fmtTimestamp(idx)}
                />
              );
            })}
          </div>
        </div>
      </AbsoluteFill>
    </AbsoluteFill>
  );
};
