/**
 * Adapted from frontend/src/components/InfinityLogo.tsx for Remotion.
 *
 * Original is a client component that uses requestAnimationFrame +
 * getTotalLength() to drive the dasharray. Remotion is frame-driven and
 * renders in a headless bundler where SVG measurements are unreliable, so
 * here we use the SVG `pathLength="100"` attribute to normalize path length
 * to a virtual 100 units, then compute dashoffset deterministically from
 * `useCurrentFrame()`.
 *
 * Visual parity with the live site:
 *   - Same chat-bubble path, same fill/stroke alphas
 *   - Same infinity (figure-8) curve at the same offset within the bubble
 *   - 4 colour segments cycling: Twitch purple → YouTube red → Kick green → TikTok cyan
 *   - 6-second loop (matches LOOP_MS = 6000 from the original)
 *   - SEG_FRAC = 0.55 (matches original)
 */

import React from 'react';
import { useCurrentFrame, useVideoConfig } from 'remotion';

const PLATFORM_STROKES = ['#9146FF', '#FF0000', '#53FC18', '#69C9D0'] as const;
const VIRTUAL_LENGTH = 100;
const SEG_FRAC = 0.55;
const SEG = VIRTUAL_LENGTH * SEG_FRAC;
const PIECE = SEG / PLATFORM_STROKES.length;
const LOOP_SECONDS = 6;

const INF_PATH = 'M6 10c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8';
const BUBBLE_PATH =
  'M4.84836 2.771C7.18302 2.42773 9.57113 2.25 12.0003 2.25C14.4292 2.25 16.8171 2.4277 19.1516 2.77091C21.1299 3.06177 22.5 4.79445 22.5 6.74056V12.7595C22.5 14.7056 21.1299 16.4382 19.1516 16.7291C17.2123 17.0142 15.2361 17.1851 13.2302 17.2348C13.1266 17.2374 13.0318 17.2788 12.9638 17.3468L8.78033 21.5303C8.56583 21.7448 8.24324 21.809 7.96299 21.6929C7.68273 21.5768 7.5 21.3033 7.5 21V17.045C6.60901 16.9634 5.72491 16.8579 4.84836 16.729C2.87004 16.4381 1.5 14.7054 1.5 12.7593V6.74064C1.5 4.79455 2.87004 3.06188 4.84836 2.771Z';

export const InfinityLogo: React.FC<{ size?: number; strokeWidthScale?: number }> = ({
  size = 36,
  strokeWidthScale = 1,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const loopFrames = LOOP_SECONDS * fps;
  const head = ((frame % loopFrames) / loopFrames) * VIRTUAL_LENGTH;

  const innerW = size * 0.67;
  const innerH = size * 0.39;
  /* The original uses transform: translateY(-10%) on the inner SVG; mirror
     that by nudging the inner top up by ~4% of the parent size. */
  const innerOffsetX = (size - innerW) / 2;
  const innerOffsetY = (size - innerH) / 2 - size * 0.04;

  const baseStroke = 2.5 * strokeWidthScale;

  return (
    <div
      style={{
        position: 'relative',
        width: size,
        height: size,
        display: 'inline-block',
        flexShrink: 0,
      }}
      aria-hidden="true"
    >
      <svg
        width={size}
        height={size}
        viewBox="0 0 24 24"
        fill="none"
        style={{ position: 'absolute', inset: 0 }}
      >
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d={BUBBLE_PATH}
          fill="rgba(255,255,255,0.07)"
          stroke="rgba(255,255,255,0.10)"
          strokeWidth="0.5"
        />
      </svg>
      <svg
        width={innerW}
        height={innerH}
        viewBox="0 0 24 14"
        fill="none"
        style={{
          position: 'absolute',
          left: innerOffsetX,
          top: innerOffsetY,
          filter: 'drop-shadow(0 0 3px rgba(0,0,0,0.9))',
        }}
      >
        <path
          d={INF_PATH}
          stroke="rgba(255,255,255,0.08)"
          strokeWidth={baseStroke}
          strokeLinecap="round"
          pathLength={VIRTUAL_LENGTH}
        />
        {PLATFORM_STROKES.map((color, i) => {
          const colorOffset = i * PIECE;
          const t = (((head - colorOffset) % VIRTUAL_LENGTH) + VIRTUAL_LENGTH) % VIRTUAL_LENGTH;
          return (
            <React.Fragment key={`${color}-${i}`}>
              <path
                d={INF_PATH}
                stroke={color}
                strokeWidth={baseStroke}
                strokeLinecap="round"
                pathLength={VIRTUAL_LENGTH}
                strokeDasharray={`${PIECE} ${VIRTUAL_LENGTH * 2}`}
                strokeDashoffset={-t}
              />
              <path
                d={INF_PATH}
                stroke={color}
                strokeWidth={baseStroke}
                strokeLinecap="round"
                pathLength={VIRTUAL_LENGTH}
                strokeDasharray={`${PIECE} ${VIRTUAL_LENGTH * 2}`}
                strokeDashoffset={-(t - VIRTUAL_LENGTH)}
              />
            </React.Fragment>
          );
        })}
      </svg>
    </div>
  );
};
