/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Overlay Layout
 *
 * Special layout for overlay pages that need transparent backgrounds for OBS.
 * Overrides the default body background to be transparent.
 */

import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'All-Chat Overlay',
  description: 'Chat overlay for OBS Browser Source',
}

export default function OverlayLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      {/* Override body and html background to be transparent for OBS */}
      <style
        dangerouslySetInnerHTML={{
          __html: `
        html, body {
          background: transparent !important;
          background-image: none !important;
        }
      `,
        }}
      />
      {children}
    </>
  )
}
