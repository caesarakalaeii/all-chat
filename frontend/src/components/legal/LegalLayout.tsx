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
import { getTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'
import { archivoBlack, spaceMono } from '@/lib/fonts'

interface LegalLayoutProps {
  title: string
  lastUpdated: string
  children: React.ReactNode
}

export default function LegalLayout({ title, lastUpdated, children }: LegalLayoutProps) {
  // getTranslations, not the hook: the legal routes are Server Components.
  const t = getTranslations()
  return (
    <div
      id="legal-wrapper"
      className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}
    >
      <AppNav />
      <div className="mx-auto max-w-4xl px-4 py-12">
        <div className="lanes-panel p-8 md:p-12">
          <div className="mb-8 flex items-start justify-between">
            <div className="space-y-2">
              <span className="mono-label">{t('legal.layout.eyebrow')}</span>
              <h1 className="text-3xl">{title}</h1>
              <p className="text-dim text-sm">
                {t('legal.layout.lastUpdated', { date: lastUpdated })}
              </p>
            </div>
            <LegalThemeToggle />
          </div>

          <div className="legal-prose text-sub space-y-10 leading-relaxed">{children}</div>

          <div className="text-dim mt-12 flex flex-col gap-3 border-t border-white/20 pt-6 text-sm sm:flex-row sm:items-center sm:justify-between">
            <span>{t('legal.layout.copyright', { year: new Date().getFullYear() })}</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/">{t('legal.layout.homeLink')}</Link>
              <Link href="/legal/privacy">{t('legal.layout.privacyLink')}</Link>
              <Link href="/legal/terms">{t('legal.layout.termsLink')}</Link>
              <Link href="/legal/impressum">{t('legal.layout.impressumLink')}</Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
