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
 * Overlay View Layout
 *
 * The observability view is a solid, readable dashboard — the opposite of the
 * OBS overlay. The parent overlay layout forces a transparent body for OBS;
 * this nested layout renders after it, so its equal-specificity !important rule
 * wins and restores a solid background. The light/dark toggle then sets the body
 * background imperatively per theme.
 */

import type { Metadata } from 'next'

import { OverlayViewGuard } from './OverlayViewGuard'

export const metadata: Metadata = {
  title: 'All-Chat Monitor',
  description: 'Readable chat & activity monitor for streamers',
}

export default function OverlayViewLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <style
        dangerouslySetInnerHTML={{
          __html: `
        html, body {
          background: #07070a !important;
          background-image: none !important;
          min-height: 100vh;
          overflow: hidden;
        }
      `,
        }}
      />
      <OverlayViewGuard>{children}</OverlayViewGuard>
    </>
  )
}
