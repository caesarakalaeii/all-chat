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
import { AppNav } from '@/components/AppNav'
import { LegalThemeToggle } from '@/components/legal/LegalThemeToggle'

interface LegalLayoutProps {
  title: string
  lastUpdated: string
  children: React.ReactNode
}

export default function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps) {
  return (
    <div id="legal-wrapper" className="min-h-screen bg-bg transition-colors duration-300">
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 flex items-start justify-between">
            <div className="space-y-2">
              <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
                All-Chat Legal
              </p>
              <h1 className="text-3xl font-bold text-text">{title}</h1>
              <p className="text-sm text-text-dim">Last updated: {lastUpdated}</p>
            </div>
            <LegalThemeToggle />
          </div>

          <div className="legal-prose space-y-10 leading-relaxed text-text-sub">{children}</div>

          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>&copy; {new Date().getFullYear()} All-Chat</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/" className="transition-colors hover:text-text">
                Home
              </Link>
              <Link href="/legal/privacy" className="transition-colors hover:text-text">
                Privacy Policy
              </Link>
              <Link href="/legal/terms" className="transition-colors hover:text-text">
                Terms of Service
              </Link>
              <Link href="/legal/impressum" className="transition-colors hover:text-text">
                Impressum
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
