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
import { Dialog } from '@/components/ui/dialog'
import { useAuthStore } from '@/lib/stores/auth-store'
import { cn } from '@/lib/utils'

export interface AdminLink {
  href: string
  label: string
  icon: LucideIcon
  exact?: boolean
  // Shown on the dashboard home nav grid (not the rail).
  description?: string
}

// Single source of truth for admin navigation, shared by the sidebar rail and
// the dashboard home grid so the two can never drift apart.
export const ADMIN_LINKS: AdminLink[] = [
  { href: '/admin', label: 'Dashboard', icon: LayoutDashboard, exact: true },
  { href: '/admin/users', label: 'Users', icon: Users, description: 'View and manage users' },
  {
    href: '/admin/overlays',
    label: 'Overlays',
    icon: LayoutGrid,
    description: 'Overlays and their owners',
  },
  { href: '/admin/sources', label: 'Sources', icon: Radio, description: 'Every chat source' },
  { href: '/admin/viewers', label: 'Viewers', icon: Eye, description: 'Viewer sessions and bans' },
  {
    href: '/admin/cosmetics',
    label: 'Cosmetics',
    icon: Sparkles,
    description: 'Avatar frames and flairs',
  },
  { href: '/admin/features', label: 'Features', icon: Flag, description: 'Premium feature gates' },
  {
    href: '/admin/maintenance',
    label: 'Maintenance',
    icon: Wrench,
    description: 'Maintenance mode and ops',
  },
]

function isActive(pathname: string | null, href: string, exact?: boolean): boolean {
  if (!pathname) return false
  return exact ? pathname === href : pathname === href || pathname.startsWith(href + '/')
}

/** Shared list of navigation links, used by both the desktop rail and the mobile drawer. */
function NavLinks({ pathname, onNavigate }: { pathname: string | null; onNavigate?: () => void }) {
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

  // Close the mobile drawer whenever the route changes. Done during render via
  // React's "previous value" pattern rather than an effect, which avoids a
  // synchronous setState in an effect body (react-hooks/set-state-in-effect).
  const [lastPathname, setLastPathname] = useState(pathname)
  if (pathname !== lastPathname) {
    setLastPathname(pathname)
    setOpen(false)
  }

  // Close the drawer when the viewport grows to the desktop breakpoint, so the
  // dialog's focus trap and scroll lock don't linger invisibly behind the rail.
  useEffect(() => {
    const mq = window.matchMedia('(min-width: 1024px)')
    function onChange(e: MediaQueryListEvent) {
      if (e.matches) setOpen(false)
    }
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

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

      {/* Mobile drawer (left slide-in panel on the dialog primitive) */}
      <Dialog.Root open={open} onOpenChange={setOpen}>
        <Dialog.Content
          showCloseButton={false}
          aria-label="Admin menu"
          className="top-0 left-0 flex h-full w-72 max-w-[85%] translate-x-0 translate-y-0 flex-col rounded-none border-0 border-r border-border bg-bg p-0 px-4 py-5 lg:hidden"
        >
          <div className="flex items-center justify-between">
            <SidebarBrand />
            <Dialog.Close
              aria-label="Close admin menu"
              className="rounded-lg p-2 text-text-sub transition-colors hover:bg-surface-2 hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              <X className="size-5" aria-hidden="true" />
            </Dialog.Close>
          </div>
          <div className="mt-8 flex-1 overflow-y-auto">
            <NavLinks pathname={pathname} onNavigate={() => setOpen(false)} />
          </div>
          <SidebarFooter onNavigate={() => setOpen(false)} />
        </Dialog.Content>
      </Dialog.Root>
    </>
  )
}
