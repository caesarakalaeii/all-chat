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

import { ImageResponse } from 'next/og'
import { readFile } from 'node:fs/promises'
import { join } from 'node:path'

export const alt = 'All-Chat — Every chat. One overlay.'

export const size = {
  width: 1200,
  height: 630,
}

export const contentType = 'image/png'

export default async function Image() {
  const barlowFontData = await readFile(
    join(process.cwd(), 'public', 'fonts', 'Barlow-Bold.ttf'),
  )

  return new ImageResponse(
    (
      <div
        style={{
          background: '#0f0f13',
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          fontFamily: 'Barlow, sans-serif',
          padding: '60px',
        }}
      >
        {/* Title */}
        <div
          style={{
            fontSize: 96,
            fontWeight: 800,
            color: '#ffffff',
            letterSpacing: '-2px',
            marginBottom: '16px',
            lineHeight: 1,
          }}
        >
          All-Chat
        </div>

        {/* Subtitle */}
        <div
          style={{
            fontSize: 32,
            fontWeight: 500,
            color: '#a1a1aa',
            marginBottom: '52px',
            letterSpacing: '0.5px',
          }}
        >
          Every chat. One overlay.
        </div>

        {/* Platform badges */}
        <div
          style={{
            display: 'flex',
            flexDirection: 'row',
            gap: '16px',
            marginBottom: '28px',
          }}
        >
          <div
            style={{
              background: '#9146FF',
              color: '#ffffff',
              fontSize: 24,
              fontWeight: 700,
              padding: '10px 28px',
              borderRadius: '9999px',
              letterSpacing: '0.5px',
            }}
          >
            Twitch
          </div>
          <div
            style={{
              background: '#FF0000',
              color: '#ffffff',
              fontSize: 24,
              fontWeight: 700,
              padding: '10px 28px',
              borderRadius: '9999px',
              letterSpacing: '0.5px',
            }}
          >
            YouTube
          </div>
          <div
            style={{
              background: '#53FC18',
              color: '#0f0f13',
              fontSize: 24,
              fontWeight: 700,
              padding: '10px 28px',
              borderRadius: '9999px',
              letterSpacing: '0.5px',
            }}
          >
            Kick
          </div>
          <div
            style={{
              background: '#FE2C55',
              color: '#ffffff',
              fontSize: 24,
              fontWeight: 700,
              padding: '10px 28px',
              borderRadius: '9999px',
              letterSpacing: '0.5px',
            }}
          >
            TikTok
          </div>
        </div>

        {/* Emote providers */}
        <div
          style={{
            fontSize: 20,
            fontWeight: 500,
            color: '#71717a',
            marginBottom: '48px',
            letterSpacing: '0.5px',
          }}
        >
          7TV + BTTV + FFZ Emotes
        </div>

        {/* Bottom tagline */}
        <div
          style={{
            fontSize: 28,
            fontWeight: 600,
            color: '#e4e4e7',
            letterSpacing: '0.5px',
          }}
        >
          One overlay. Every chat. All platforms.
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [
        {
          name: 'Barlow',
          data: barlowFontData,
          style: 'normal',
          weight: 700,
        },
      ],
    },
  )
}
