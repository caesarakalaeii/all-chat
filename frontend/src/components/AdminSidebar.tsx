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

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import {
  LayoutDashboard,
  Users,
  LayoutGrid,
  Radio,
  Eye,
  Sparkles,
  Flag,
  Wrench,
  Menu,
  X,
  ArrowLeft,
  LogOut,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { InfinityLogo } from '@/components/InfinityLogo'
import { useAuthStore } from '@/lib/stores/auth-store'
import { cn } from '@/lib/utils'

interface AdminLink {
  href: string
  label: string
  icon: LucideIcon
  exact?: boolean
}

const ADMIN_LINKS: AdminLink[] = [
  { href: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true },
  { href: '/admin/users', label: 'Users', icon: Users },
  { href: '/admin/overlays', label: 'Overlays', icon: LayoutGrid },
  { href: '/admin/sources', label: 'Sources', icon: Radio },
  { href: '/admin/viewers', label: 'Viewers', icon: Eye },
  { href: '/admin/cosmetics', label: 'Cosmetics', icon: Sparkles },
  { href: '/admin/features', label: 'Features', icon: Flag },
  { href: '/admin/maintenance', label: 'Maintenance', icon: Wrench },
]

function isActive(pathname: string | null, href: string, exact?: boolean): boolean {
  if (!pathname) return false
  return exact ? pathname === href : pathname === href || pathname.startsWith(href + '/')
}

/** Shared list of navigation links, used by both the desktop rail and the mobile drawer. */
function NavLinks({
  pathname,
  onNavigate,
}: {
  pathname: string | null
  onNavigate?: () => void
}) {
  return (
    <nav className="flex flex-col gap-0.5">
      {ADMIN_LINKS.map((link) => {
        const active = isActive(pathname, link.href, link.exact)
        const Icon = link.icon
        return (
          <Link
            key={link.href}
            href={link.href}
            onClick={onNavigate}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors',
              'focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
              active
                ? 'bg-surface-2 font-medium text-text'
                : 'text-text-sub hover:bg-surface-2/60 hover:text-text'
            )}
          >
            <Icon className="size-4 shrink-0" aria-hidden="true" />
            <span className="truncate">{link.label}</span>
          </Link>
        )
      })}
    </nav>
  )
}

function SidebarFooter({ onNavigate }: { onNavigate?: () => void }) {
  const router = useRouter()
  const { logout } = useAuthStore()

  function handleLogout() {
    onNavigate?.()
    logout()
    router.push('/')
  }

  return (
    <div className="flex flex-col gap-0.5 border-t border-border pt-3">
      <Link
        href="/dashboard"
        onClick={onNavigate}
        className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-text-sub transition-colors hover:bg-surface-2/60 hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <ArrowLeft className="size-4 shrink-0" aria-hidden="true" />
        <span className="truncate">Back to app</span>
      </Link>
      <button
        type="button"
        onClick={handleLogout}
        className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-text-sub transition-colors hover:bg-surface-2/60 hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <LogOut className="size-4 shrink-0" aria-hidden="true" />
        <span className="truncate">Log out</span>
      </button>
    </div>
  )
}

function SidebarBrand() {
  return (
    <Link
      href="/dashboard"
      className="flex items-center gap-2.5 rounded-sm focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
    >
      <InfinityLogo size={28} />
      <span className="flex flex-col leading-tight">
        <span className="text-base font-extrabold tracking-tight text-text">all-chat</span>
        <span className="text-xs text-text-sub">Admin</span>
      </span>
    </Link>
  )
}

export function AdminSidebar() {
  const pathname = usePathname()
  const [open, setOpen] = useState(false)

  // Close the mobile drawer whenever the route changes.
  useEffect(() => {
    setOpen(false)
  }, [pathname])

  // Close on Escape and lock body scroll while the drawer is open.
  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('keydown', onKey)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prevOverflow
    }
  }, [open])

  return (
    <>
      {/* Desktop sidebar (fixed rail) */}
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-60 flex-col border-r border-border bg-nav-bg px-4 py-5 backdrop-blur-[20px] lg:flex">
        <div className="px-1">
          <SidebarBrand />
        </div>
        <div className="mt-8 flex-1 overflow-y-auto">
          <NavLinks pathname={pathname} />
        </div>
        <SidebarFooter />
      </aside>

      {/* Mobile top bar */}
      <header className="sticky top-0 z-40 flex h-14 items-center gap-3 border-b border-border bg-nav-bg px-4 backdrop-blur-[20px] lg:hidden">
        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label="Open admin menu"
          aria-expanded={open}
          className="-ml-2 rounded-lg p-2 text-text-sub transition-colors hover:bg-surface-2 hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          <Menu className="size-5" aria-hidden="true" />
        </button>
        <SidebarBrand />
      </header>

      {/* Mobile drawer */}
      {open && (
        <div className="fixed inset-0 z-50 lg:hidden" role="dialog" aria-modal="true" aria-label="Admin menu">
          <button
            type="button"
            aria-label="Close admin menu"
            onClick={() => setOpen(false)}
            className="absolute inset-0 bg-black/60 backdrop-blur-[2px]"
          />
          <div className="absolute inset-y-0 left-0 flex w-72 max-w-[85%] flex-col border-r border-border bg-bg px-4 py-5 shadow-xl">
            <div className="flex items-center justify-between">
              <SidebarBrand />
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close admin menu"
                className="rounded-lg p-2 text-text-sub transition-colors hover:bg-surface-2 hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                <X className="size-5" aria-hidden="true" />
              </button>
            </div>
            <div className="mt-8 flex-1 overflow-y-auto">
              <NavLinks pathname={pathname} onNavigate={() => setOpen(false)} />
            </div>
            <SidebarFooter onNavigate={() => setOpen(false)} />
          </div>
        </div>
      )}
    </>
  )
}
