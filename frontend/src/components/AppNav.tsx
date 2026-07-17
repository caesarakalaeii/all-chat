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

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store'
import { viewerApi } from '@/lib/api/viewer'
import { InfinityLogo } from '@/components/InfinityLogo'

const activeClass =
  'relative text-text flex items-center px-3.5 h-full text-sm ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm ' +
  'after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 ' +
  'after:bg-linear-to-r after:from-twitch after:to-tiktok'

const inactiveClass =
  'text-text-sub hover:text-text transition-colors flex items-center px-3.5 h-full text-sm ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm'

export function AppNav() {
  const pathname = usePathname()
  const router = useRouter()
  const { user, logout } = useAuthStore()
  const { viewerToken, viewerLogout } = useViewerAuthStore()

  const isLoggedIn = !!user || !!viewerToken

  function isActive(href: string): boolean {
    return pathname === href || pathname.startsWith(href + '/')
  }

  function handleLogout() {
    if (user) logout()
    if (viewerToken) {
      // audit #18: hit the backend (blacklists the viewer JWT) while the token is
      // still attached, THEN clear local state. viewerApi.logout() reads the token
      // synchronously when invoked, so the in-flight request keeps its bearer
      // header even though viewerLogout() clears it next. Fire-and-forget — don't
      // block navigation on a network error.
      viewerApi.logout().catch(() => {})
      viewerLogout()
    }
    router.push('/')
  }

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center border-b border-border bg-nav-bg px-3 backdrop-blur-[20px] sm:px-8">
      <Link
        href="/dashboard"
        className="mr-4 flex shrink-0 items-center gap-2.5 rounded-sm focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none sm:mr-10"
      >
        <InfinityLogo size={28} />
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
      </Link>
      {/* Scrolls within itself on narrow viewports so the PAGE never scrolls
          horizontally (WCAG 1.4.10 reflow at 320px). */}
      <div className="flex h-full gap-0.5 overflow-x-auto">
        <Link href="/dashboard" className={isActive('/dashboard') ? activeClass : inactiveClass}>
          Dashboard
        </Link>
        <Link
          href="/settings/viewer"
          className={isActive('/settings/viewer') ? activeClass : inactiveClass}
        >
          Flairs
        </Link>
        {user?.is_admin && (
          <Link href="/admin" className={isActive('/admin') ? activeClass : inactiveClass}>
            Admin
          </Link>
        )}
        <Link href="/settings" className={pathname === '/settings' ? activeClass : inactiveClass}>
          Settings
        </Link>
        <Link href="/docs" className={isActive('/docs') ? activeClass : inactiveClass}>
          Docs
        </Link>
      </div>
      {isLoggedIn && (
        <button
          onClick={handleLogout}
          className="ml-auto rounded-sm px-3 py-1.5 text-sm text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          Log out
        </button>
      )}
    </nav>
  )
}
