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
 * All-Chat has its own Website ID; the analytics backend keeps its data isolated.
 *
 * The tracker is loaded only when NEXT_PUBLIC_UMAMI_WEBSITE_ID is set (baked in
 * at build time, see Dockerfile + the build-and-push workflow), so local dev
 * and any build without the variable ship without analytics.
 *
 * Public overlay views (/overlay/...) are OBS browser sources, not real
 * visitors, so we skip them to keep the stats meaningful.
 */

import Script from 'next/script'
import { usePathname } from 'next/navigation'

const DEFAULT_SRC = 'https://analytics.allch.at/script.js'

export default function Analytics() {
  const pathname = usePathname()

  const websiteId = process.env.NEXT_PUBLIC_UMAMI_WEBSITE_ID
  const src = process.env.NEXT_PUBLIC_UMAMI_SRC || DEFAULT_SRC

  // No website configured (local dev / builds without the variable) → no-op.
  if (!websiteId) return null

  // Don't count OBS browser-source loads of public overlays as page views.
  if (pathname === '/overlay' || pathname.startsWith('/overlay/')) return null

  return (
    <Script
      src={src}
      data-website-id={websiteId}
      strategy="afterInteractive"
      defer
    />
  )
}
