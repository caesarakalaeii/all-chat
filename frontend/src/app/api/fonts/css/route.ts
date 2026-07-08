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

import { NextResponse, type NextRequest } from 'next/server'

// Allowlist of font families we proxy. Keep in sync with:
// - frontend/src/components/appearance/FontFamilyCombobox.tsx (GOOGLE_FONTS)
// - frontend/src/app/overlay/[id]/page.tsx (GOOGLE_FONT_NAMES)
// - frontend/src/app/overlays/[id]/preview/embed/page.tsx (GOOGLE_FONT_NAMES)
// - frontend/public/obs-badge.html / static/obs-badge.html (Barlow for brand badge)
const ALLOWED_FAMILIES: ReadonlySet<string> = new Set([
  'Barlow',
  'Bebas Neue',
  'Oswald',
  'Rajdhani',
  'Barlow Condensed',
  'Exo 2',
  'Nunito',
  'Poppins',
  'Roboto',
  'Open Sans',
  'Montserrat',
  // Bundled-theme fonts (audit #11): theme CSS @imports these through this
  // same-origin proxy so viewer IPs never reach Google (DSGVO). Keep in sync
  // with src/lib/theme-marketplace/bundled-themes.generated.ts @import families.
  'Inter',
  'Monoton',
  'Orbitron',
  'Press Start 2P',
  'VT323',
  'Share Tech Mono',
  'Source Code Pro',
  'Space Grotesk',
  'Bangers',
  'Caveat',
])

// Stable WOFF2-capable UA so Google returns modern woff2 faces. Without this
// the CSS varies per-client and may include legacy formats.
const UPSTREAM_UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'

function parseFamilyParam(raw: string): string | null {
  // Accept "Name" or "Name:wght@..." — extract the name and validate.
  const name = raw.split(':', 1)[0]
  if (!name) return null
  if (!ALLOWED_FAMILIES.has(name)) return null
  return raw
}

export async function GET(request: NextRequest): Promise<NextResponse> {
  // Support multiple family= params (audit #11): bundled theme @imports combine
  // several families in one request, e.g. css2?family=Rajdhani:...&family=Share+Tech+Mono.
  // Every family is validated against the allowlist before being forwarded.
  const familyRaws = request.nextUrl.searchParams.getAll('family')
  if (familyRaws.length === 0) {
    return new NextResponse('missing family parameter', { status: 400 })
  }

  const validatedFamilies: string[] = []
  for (const raw of familyRaws) {
    const validated = parseFamilyParam(raw)
    if (!validated) {
      return new NextResponse('family not in allowlist', { status: 400 })
    }
    validatedFamilies.push(`family=${encodeURIComponent(validated)}`)
  }

  const upstream = `https://fonts.googleapis.com/css2?${validatedFamilies.join('&')}&display=swap`
  let css: string
  try {
    const res = await fetch(upstream, {
      headers: { 'User-Agent': UPSTREAM_UA },
      // Next.js data cache: 30 days. Google rarely rotates font URLs.
      next: { revalidate: 60 * 60 * 24 * 30 },
    })
    if (!res.ok) {
      return new NextResponse(`upstream ${res.status}`, { status: 502 })
    }
    css = await res.text()
  } catch {
    return new NextResponse('upstream fetch failed', { status: 502 })
  }

  // Rewrite every https://fonts.gstatic.com/... URL to our file proxy so the
  // browser never connects to Google directly.
  const rewritten = css.replace(
    /https:\/\/fonts\.gstatic\.com\//g,
    '/api/fonts/file/',
  )

  return new NextResponse(rewritten, {
    status: 200,
    headers: {
      'Content-Type': 'text/css; charset=utf-8',
      // Browser + shared cache for 30 days; stale-while-revalidate for smooth rollover.
      'Cache-Control': 'public, max-age=2592000, s-maxage=2592000, stale-while-revalidate=86400',
      'X-Content-Type-Options': 'nosniff',
    },
  })
}
