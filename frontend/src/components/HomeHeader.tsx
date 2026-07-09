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
 * HomeHeader — sticky landing-page header.
 *
 * This is the returning-user fix: it keeps the primary action reachable at the
 * very top of the page regardless of scroll position or when the client-side
 * auth check resolves. A logged-in visitor gets a one-click "Dashboard" button
 * instead of scrolling past the whole marketing hero; a logged-out visitor gets
 * a "Sign in" jump to the hero CTA.
 *
 * Homepage-scoped on purpose — it lives inside HomeClient, never in the root
 * layout (which also wraps /overlay, /legal and /docs). The chrome mirrors the
 * app's AppNav (sticky, bg-nav-bg, backdrop-blur) so it sits correctly under the
 * normal-flow ImpersonationBanner.
 */

'use client'

import Link from 'next/link'
import { LayoutGrid } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { InfinityLogo } from '@/components/InfinityLogo'
import { dashStyleFor } from '@/lib/dashboard-button-styles'
import { trackEvent } from '@/lib/analytics'
import { cn } from '@/lib/utils'

export function HomeHeader() {
  const { user, loading } = useAuthStore()
  const dashStyle = dashStyleFor(user?.auth_provider)

  return (
    <header className="sticky top-0 z-50 border-b border-border bg-nav-bg backdrop-blur-[20px]">
      <div className="mx-auto flex h-[60px] max-w-5xl items-center justify-between px-4 sm:px-6">
        {/* Logo → home */}
        <Link
          href="/"
          className="flex items-center gap-2.5 rounded-sm focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          aria-label="All-Chat home"
        >
          <InfinityLogo size={26} />
          <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
        </Link>

        {/* Right side — Docs is always present; the action depends on auth state */}
        <nav className="flex items-center gap-4 sm:gap-5">
          <Link
            href="/docs"
            className="rounded-sm text-sm text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            Docs
          </Link>

          {loading ? (
            /* Reserve the action width so the button doesn't pop in once auth
               resolves. Cosmetic only — rendering it server-side costs nothing. */
            <span className="inline-block h-8 w-[104px]" aria-hidden="true" />
          ) : user ? (
            <Link
              href="/dashboard"
              onClick={() => trackEvent('cta_click', { cta: 'dashboard', location: 'nav' })}
              className={cn(
                'inline-flex items-center gap-2 rounded-lg px-4 py-1.5 text-sm font-semibold transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none',
                dashStyle.bg,
                dashStyle.ring,
                dashStyle.text
              )}
            >
              <LayoutGrid className="h-4 w-4" aria-hidden="true" />
              Dashboard
            </Link>
          ) : (
            <a
              href="#get-started"
              onClick={() => trackEvent('cta_click', { cta: 'signin', location: 'nav' })}
              className="rounded-lg border border-border bg-surface px-4 py-1.5 text-sm font-semibold text-text transition-colors hover:border-border-md focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
            >
              Sign in
            </a>
          )}
        </nav>
      </div>
    </header>
  )
}
