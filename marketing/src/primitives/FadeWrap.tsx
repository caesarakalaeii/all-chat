import React from 'react';
import { AbsoluteFill, interpolate, useCurrentFrame } from 'remotion';

interface FadeWrapProps {
  /**
   * Total duration of the wrapped Sequence in frames. Required because
   * `useVideoConfig().durationInFrames` returns the composition duration,
   * not the surrounding Sequence's.
   */
  duration: number;
  /** Crossfade ramp in frames. Default 10 (~330ms @ 30fps). */
  fadeFrames?: number;
  children: React.ReactNode;
}

export const FadeWrap: React.FC<FadeWrapProps> = ({ duration, fadeFrames = 10, children }) => {
  const frame = useCurrentFrame();
  const opacity = interpolate(
    frame,
    [0, fadeFrames, duration - fadeFrames, duration],
    [0, 1, 1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
  );
  return <AbsoluteFill style={{ opacity }}>{children}</AbsoluteFill>;
};
