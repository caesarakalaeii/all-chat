'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'

const ADMIN_LINKS = [
  { href: '/admin', label: 'Dashboard', exact: true },
  { href: '/admin/users', label: 'Users' },
  { href: '/admin/overlays', label: 'Overlays' },
  { href: '/admin/sources', label: 'Sources' },
  { href: '/admin/viewers', label: 'Viewers' },
]

const activeClass =
  'relative text-text flex items-center px-3 h-full text-sm ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm ' +
  'after:absolute after:bottom-0 after:left-0 after:right-0 after:h-0.5 ' +
  'after:bg-linear-to-r after:from-twitch after:to-tiktok'

const inactiveClass =
  'text-text-sub hover:text-text transition-colors flex items-center px-3 h-full text-sm ' +
  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm'

export function AdminNav() {
  const pathname = usePathname()

  function isActive(href: string, exact?: boolean) {
    return exact ? pathname === href : pathname.startsWith(href)
  }

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center px-8 bg-nav-bg backdrop-blur-[20px] border-b border-border">
      <Link
        href="/dashboard"
        className="flex items-center gap-2.5 mr-6 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch rounded-sm"
      >
        <div className="logo-ring" aria-hidden="true" />
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
      </Link>
      <span className="text-text-sub text-sm mr-6">Admin</span>
      <div className="flex h-full gap-0.5">
        {ADMIN_LINKS.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            className={isActive(link.href, link.exact) ? activeClass : inactiveClass}
          >
            {link.label}
          </Link>
        ))}
      </div>
    </nav>
  )
}
