'use client'

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
 * Umami Analytics
 *
 * Privacy-friendly, cookieless web analytics, self-hosted at analytics.allch.at.
 * The Website ID and script host are pinned here directly — they aren't secrets
 * (the ID is sent in every tracking request), and hardcoding them avoids the
 * Next.js build-time-inlining gotcha that comes with NEXT_PUBLIC_* env vars in
 * client components (a runtime/manifest env would never reach the browser).
 *
 * The tracker is only rendered in production builds, so local dev (`next dev`,
 * NODE_ENV=development) ships without analytics.
 *
 * Public overlay views (/overlay/...) are OBS browser sources, not real
 * visitors, so we skip them to keep the stats meaningful.
 *
 * URLs are sanitised before they leave the browser (see `lib/umami-sanitize`):
 * path UUIDs collapse to `:id`, token-bearing query params are dropped, and the
 * hash is excluded entirely (`data-exclude-hash`) — so high-cardinality overlay
 * IDs and OAuth tokens (e.g. `/auth/callback#access_token=...`) never reach
 * analytics. The sanitiser is installed before the tracker renders, so even a
 * fresh OAuth-callback page load is covered on its very first page view.
 */

import Script from 'next/script'
import { usePathname } from 'next/navigation'
import { umamiBeforeSend } from '@/lib/umami-sanitize'

const WEBSITE_ID = 'c7a2e7ad-be45-4de3-954f-f15fd8e7dc97'
const SRC = 'https://analytics.allch.at/script.js'

// Install the URL sanitiser at module-eval time — this runs during the client
// bundle bootstrap, before the tracker's `afterInteractive` script can fire its
// first page view. Umami resolves `data-before-send` as `window[name]` at send
// time, so registering it up front guarantees even that first view is sanitised.
if (typeof window !== 'undefined') {
  window.__umamiBeforeSend = umamiBeforeSend
}

export default function Analytics() {
  const pathname = usePathname()

  // Local dev / non-production builds → no-op, so dev traffic stays out of stats.
  if (process.env.NODE_ENV !== 'production') return null

  // Don't count OBS browser-source loads of public overlays as page views.
  if (pathname === '/overlay' || pathname.startsWith('/overlay/')) return null

  return (
    <Script
      src={SRC}
      data-website-id={WEBSITE_ID}
      data-before-send="__umamiBeforeSend"
      data-exclude-hash="true"
      strategy="afterInteractive"
      defer
    />
  )
}
