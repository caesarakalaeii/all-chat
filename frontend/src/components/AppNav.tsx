'use client'

import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'

function LogoRing({ 'aria-hidden': ariaHidden }: { 'aria-hidden'?: boolean | 'true' | 'false' }) {
  return <div className="logo-ring" aria-hidden={ariaHidden} />
}

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
  const { user } = useAuthStore()

  function isActive(href: string): boolean {
    return pathname === href || pathname.startsWith(href + '/')
  }

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center px-8 bg-nav-bg backdrop-blur-[20px] border-b border-border">
      <Link
        href="/dashboard"
        className="flex items-center gap-2.5 mr-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm"
      >
        <LogoRing aria-hidden="true" />
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
      </Link>
      <div className="flex h-full gap-0.5">
        <Link
          href="/dashboard"
          className={isActive('/dashboard') ? activeClass : inactiveClass}
        >
          Dashboard
        </Link>
        {user?.is_admin && (
          <Link
            href="/admin"
            className={isActive('/admin') ? activeClass : inactiveClass}
          >
            Admin
          </Link>
        )}
        <Link
          href="/settings"
          className={isActive('/settings') ? activeClass : inactiveClass}
        >
          Settings
        </Link>
      </div>
    </nav>
  )
}
