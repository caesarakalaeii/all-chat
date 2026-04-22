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

import { NextResponse } from 'next/server'

// Allow only the gstatic path prefix Google uses for font binaries. This keeps
// the proxy narrow — it cannot be abused to fetch arbitrary gstatic content.
const ALLOWED_PREFIX = 's/'

const ALLOWED_EXTENSIONS: ReadonlySet<string> = new Set([
  'woff2',
  'woff',
  'ttf',
  'otf',
])

const CONTENT_TYPES: Record<string, string> = {
  woff2: 'font/woff2',
  woff: 'font/woff',
  ttf: 'font/ttf',
  otf: 'font/otf',
}

interface RouteParams {
  params: Promise<{ path: string[] }>
}

export async function GET(_request: Request, { params }: RouteParams): Promise<NextResponse> {
  const { path } = await params
  const joined = path.map(encodeURIComponent).join('/')

  if (!joined.startsWith(ALLOWED_PREFIX)) {
    return new NextResponse('not found', { status: 404 })
  }

  const ext = joined.split('.').pop()?.toLowerCase() ?? ''
  if (!ALLOWED_EXTENSIONS.has(ext)) {
    return new NextResponse('not found', { status: 404 })
  }

  const upstream = `https://fonts.gstatic.com/${joined}`
  let body: ArrayBuffer
  try {
    const res = await fetch(upstream, {
      // Font binaries are content-addressed by Google; cache aggressively.
      next: { revalidate: 60 * 60 * 24 * 365 },
    })
    if (!res.ok) {
      return new NextResponse(`upstream ${res.status}`, { status: 502 })
    }
    body = await res.arrayBuffer()
  } catch {
    return new NextResponse('upstream fetch failed', { status: 502 })
  }

  return new NextResponse(body, {
    status: 200,
    headers: {
      'Content-Type': CONTENT_TYPES[ext] ?? 'application/octet-stream',
      'Cache-Control': 'public, max-age=31536000, immutable',
      'X-Content-Type-Options': 'nosniff',
    },
  })
}
