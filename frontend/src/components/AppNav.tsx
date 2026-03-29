'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store'
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
  const { user, token, logout } = useAuthStore()
  const { viewerToken, viewerLogout } = useViewerAuthStore()

  const isLoggedIn = !!token || !!viewerToken

  function isActive(href: string): boolean {
    return pathname === href || pathname.startsWith(href + '/')
  }

  function handleLogout() {
    if (token) logout()
    if (viewerToken) viewerLogout()
    router.push('/')
  }

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center border-b border-border bg-nav-bg px-8 backdrop-blur-[20px]">
      <Link
        href="/dashboard"
        className="mr-10 flex items-center gap-2.5 rounded-sm focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <InfinityLogo size={28} />
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
      </Link>
      <div className="flex h-full gap-0.5">
        <Link href="/dashboard" className={isActive('/dashboard') ? activeClass : inactiveClass}>
          Dashboard
        </Link>
        <Link href="/settings/viewer" className={isActive('/settings/viewer') ? activeClass : inactiveClass}>
          Flairs
        </Link>
        {user?.is_admin && (
          <Link href="/admin" className={isActive('/admin') ? activeClass : inactiveClass}>
            Admin
          </Link>
        )}
        <Link
          href="/settings"
          className={pathname === '/settings' ? activeClass : inactiveClass}
        >
          Settings
        </Link>
      </div>
      {isLoggedIn && (
        <button
          onClick={handleLogout}
          className="ml-auto text-sm text-text-sub hover:text-text transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm px-3 py-1.5"
        >
          Log out
        </button>
      )}
    </nav>
  )
}
