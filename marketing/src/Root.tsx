import React from 'react';
import { Composition } from 'remotion';
import { HeroShowcase } from './compositions/HeroShowcase';
import { HeroShowcaseSocial } from './compositions/HeroShowcaseSocial';
import './theme/fonts';
import './index.css';

export const RemotionRoot: React.FC = () => {
  return (
    <>
      <Composition
        id="HeroShowcase"
        component={HeroShowcase}
        durationInFrames={1050}
        fps={30}
        width={1920}
        height={1080}
      />
      <Composition
        id="HeroShowcaseSocial"
        component={HeroShowcaseSocial}
        durationInFrames={480}
        fps={30}
        width={1080}
        height={1920}
      />
    </>
  );
};
