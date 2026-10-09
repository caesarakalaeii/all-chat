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

import type { MetadataRoute } from 'next'
import { headers } from 'next/headers'

export default async function robots(): Promise<MetadataRoute.Robots> {
  // The beta deployment shares this codebase; it must never be indexed.
  // Disallowing everything keeps crawlers off the host entirely.
  const host = (await headers()).get('host') ?? ''
  if (host === 'beta.allch.at') {
    return {
      rules: [{ userAgent: '*', disallow: '/' }],
    }
  }

  return {
    rules: [
      {
        userAgent: '*',
        allow: ['/', '/legal/'],
        disallow: [
          '/dashboard/',
          '/admin/',
          '/auth/',
          '/settings/',
          '/overlays/',
          '/overlay/',
          '/chat/',
          '/api/',
          // Build-hashed font/image assets. Their filenames change every deploy,
          // so Google keeps re-crawling stale URLs and reporting 404s. Blocking
          // only media/ (not the JS/CSS chunks Googlebot needs to render) stops
          // the noise without harming rendering.
          '/_next/static/media/',
        ],
      },
    ],
    sitemap: 'https://allch.at/sitemap.xml',
  }
}
