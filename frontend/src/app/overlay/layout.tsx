/**
 * Overlay Layout
 *
 * Special layout for overlay pages that need transparent backgrounds for OBS.
 * Overrides the default body background to be transparent.
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'All-Chat Overlay',
  description: 'Chat overlay for OBS Browser Source',
};

export default function OverlayLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <>
      {/* Override body and html background to be transparent for OBS */}
      <style dangerouslySetInnerHTML={{ __html: `
        html, body {
          background: transparent !important;
          background-image: none !important;
        }
      ` }} />
      {children}
    </>
  );
}
