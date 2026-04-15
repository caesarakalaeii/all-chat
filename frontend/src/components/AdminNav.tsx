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
import { usePathname } from 'next/navigation'
import { InfinityLogo } from '@/components/InfinityLogo'

const ADMIN_LINKS = [
  { href: '/admin', label: 'Dashboard', exact: true },
  { href: '/admin/users', label: 'Users' },
  { href: '/admin/overlays', label: 'Overlays' },
  { href: '/admin/sources', label: 'Sources' },
  { href: '/admin/viewers', label: 'Viewers' },
  { href: '/admin/cosmetics', label: 'Cosmetics' },
  { href: '/admin/features', label: 'Features' },
  { href: '/admin/maintenance', label: 'Maintenance' },
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
    return exact ? pathname === href : (pathname?.startsWith(href) ?? false)
  }

  return (
    <nav className="sticky top-0 z-50 flex h-[60px] items-center border-b border-border bg-nav-bg px-8 backdrop-blur-[20px]">
      <Link
        href="/dashboard"
        className="mr-6 flex items-center gap-2.5 rounded-sm focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <InfinityLogo size={28} />
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
      </Link>
      <span className="mr-6 text-sm text-text-sub">Admin</span>
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
