import React from 'react';
import {
  AbsoluteFill,
  Audio,
  Sequence,
  interpolate,
  staticFile,
  useVideoConfig,
} from 'remotion';
import { FadeWrap } from '../primitives/FadeWrap';
import { LogoIntro } from '../scenes/LogoIntro';
import { HomepageStats } from '../scenes/HomepageStats';
import { MultiPlatformChat } from '../scenes/MultiPlatformChat';
import { Outro } from '../scenes/Outro';

/**
 * Vertical 1080×1920 cut for TikTok / Shorts / Reels. 20s @ 30fps = 600 frames.
 *
 * | Frames    | Scene             |
 * | 0   – 89  | LogoIntro         |
 * | 90  – 179 | HomepageStats     |
 * | 180 – 509 | MultiPlatformChat |
 * | 510 – 599 | Outro             |
 *
 * All scenes auto-detect portrait via `useVideoConfig().height > width` and
 * adjust their layouts (HomepageStats 2×2 grid, MultiPlatformChat full-width
 * chat with no left rail, Logo+Outro recentred to use vertical real estate).
 */
const SCENES: { id: string; component: React.FC; from: number; duration: number }[] = [
  { id: 'logo', component: LogoIntro, from: 0, duration: 90 },
  { id: 'stats', component: HomepageStats, from: 90, duration: 90 },
  { id: 'chat', component: MultiPlatformChat, from: 180, duration: 210 },
  { id: 'outro', component: Outro, from: 390, duration: 90 },
];

const BED_BASE_VOLUME = 0.6;
const BED_FADE_OUT_FRAMES = 30;

export const HeroShowcaseSocial: React.FC = () => {
  const { durationInFrames } = useVideoConfig();
  return (
    <AbsoluteFill style={{ background: 'var(--color-bg)' }}>
      <Audio
        src={staticFile('audio/bed.mp3')}
        volume={(f) =>
          interpolate(
            f,
            [0, durationInFrames - BED_FADE_OUT_FRAMES, durationInFrames],
            [BED_BASE_VOLUME, BED_BASE_VOLUME, 0],
            { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
          )
        }
      />
      {SCENES.map(({ id, component: Scene, from, duration }) => (
        <Sequence key={id} from={from} durationInFrames={duration}>
          <FadeWrap duration={duration} fadeFrames={10}>
            <Scene />
          </FadeWrap>
        </Sequence>
      ))}
    </AbsoluteFill>
  );
};
