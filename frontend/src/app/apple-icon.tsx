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
 * apple-icon — the 180×180 apple-touch-icon (iOS home screen).
 *
 * Renders the brand mark — the four-colour infinity on the dark surface — using
 * the same `InfinityLogo` path. The infinity is supplied to Satori as a base64
 * SVG `<img>` so the gradient stroke rasterises reliably (Satori has no default
 * font, so we avoid text here).
 */

import { ImageResponse } from 'next/og'

export const size = { width: 180, height: 180 }
export const contentType = 'image/png'

export default function AppleIcon() {
  const svg =
    "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 14'>" +
    "<defs><linearGradient id='g' x1='2' y1='0' x2='22' y2='0' gradientUnits='userSpaceOnUse'>" +
    "<stop offset='0' stop-color='#9146FF'/><stop offset='0.38' stop-color='#FF0000'/>" +
    "<stop offset='0.7' stop-color='#53FC18'/><stop offset='1' stop-color='#69C9D0'/>" +
    '</linearGradient></defs>' +
    "<path d='M6 10c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8' fill='none' stroke='url(#g)' stroke-width='2.5' stroke-linecap='round'/>" +
    '</svg>'
  const src = `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`

  return new ImageResponse(
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#0f0f13',
      }}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src={src} width={150} height={88} alt="" />
    </div>,
    { ...size }
  )
}
