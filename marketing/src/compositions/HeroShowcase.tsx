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
import { DashboardPreview } from '../scenes/DashboardPreview';
import { MultiPlatformChat } from '../scenes/MultiPlatformChat';
import { OverlayEditor } from '../scenes/OverlayEditor';
import { CustomizationFlash } from '../scenes/CustomizationFlash';
import { Outro } from '../scenes/Outro';

/**
 * 35s @ 30fps = 1050 frames.
 *
 * v7: chat scene shortened (active reveals every 14 frames), editor extended
 * to fit the cursor-walkthrough sequence, themes crossfade smoothly.
 *
 * | Frames      | Scene              | Purpose                               |
 * |-------------|--------------------|---------------------------------------|
 * | 0    – 119  | LogoIntro          | Brand recall                          |
 * | 120  – 209  | HomepageStats      | Platform scale (real /api/v1/stats)   |
 * | 210  – 359  | DashboardPreview   | "It's a real product with overlays"   |
 * | 360  – 569  | MultiPlatformChat  | Active 5-platform chat (210 frames)   |
 * | 570  – 809  | OverlayEditor      | Cursor walks through 3 edits          |
 * | 810  – 959  | CustomizationFlash | Three real themes, crossfaded         |
 * | 960  – 1049 | Outro              | CTA                                   |
 */

const SCENES: { id: string; component: React.FC; from: number; duration: number }[] = [
  { id: 'logo', component: LogoIntro, from: 0, duration: 120 },
  { id: 'stats', component: HomepageStats, from: 120, duration: 90 },
  { id: 'dashboard', component: DashboardPreview, from: 210, duration: 150 },
  { id: 'chat', component: MultiPlatformChat, from: 360, duration: 210 },
  { id: 'editor', component: OverlayEditor, from: 570, duration: 240 },
  { id: 'custom', component: CustomizationFlash, from: 810, duration: 150 },
  { id: 'outro', component: Outro, from: 960, duration: 90 },
];

const BED_BASE_VOLUME = 0.6;
const BED_FADE_OUT_FRAMES = 30;

export const HeroShowcase: React.FC = () => {
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
