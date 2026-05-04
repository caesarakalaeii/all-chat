import { loadFont as loadBarlow } from '@remotion/google-fonts/Barlow';
import { loadFont as loadDMMono } from '@remotion/google-fonts/DMMono';

/**
 * Loaded once at module import. Remotion blocks rendering until each font's
 * waitUntilDone() resolves, so all frames see the loaded glyphs deterministically.
 *
 * Mirrors the typography choice in frontend/src/app/globals.css:
 *   --font-sans: var(--font-barlow);
 *   --font-mono: var(--font-dm-mono);
 */
const barlow = loadBarlow('normal', {
  weights: ['400', '500', '600', '700', '800'],
  subsets: ['latin'],
});

const dmMono = loadDMMono('normal', {
  weights: ['400', '500'],
  subsets: ['latin'],
});

export const FONT_SANS = barlow.fontFamily;
export const FONT_MONO = dmMono.fontFamily;

export const waitForFonts = async (): Promise<void> => {
  await Promise.all([barlow.waitUntilDone(), dmMono.waitUntilDone()]);
};
