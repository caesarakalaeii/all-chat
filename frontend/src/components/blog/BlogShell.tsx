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
 * Shared shell for /blog and /blog/<slug>, in the guides' visual language.
 * Server component, so the breadcrumb JSON-LD lands in the initial HTML.
 */

import Link from 'next/link'
import type { ReactNode } from 'react'
import { AppNav } from '@/components/AppNav'
import { JsonLd } from '@/components/JsonLd'
import { formatDateTime, getTranslations } from '@/lib/i18n'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { cn } from '@/lib/utils'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const SITE_URL = 'https://allch.at'

// A post date is a calendar day, not an instant: pin UTC so a server in
// another zone cannot render the day before.
const POST_DATE: Intl.DateTimeFormatOptions = { dateStyle: 'long', timeZone: 'UTC' }

export function formatPostDate(date: string): string {
  return formatDateTime(new Date(`${date}T00:00:00Z`), POST_DATE)
}

interface Crumb {
  name: string
  path: string
}

interface BlogShellProps {
  /** Trail after Home; the last entry is the current page. */
  crumbs: readonly Crumb[]
  structuredData?: readonly Record<string, unknown>[]
  children: ReactNode
}

export function BlogShell({ crumbs, structuredData = [], children }: BlogShellProps) {
  const home = t('common.breadcrumb.home')
  const breadcrumbLd = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [{ name: home, path: '' }, ...crumbs].map((crumb, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: crumb.name,
      item: `${SITE_URL}${crumb.path}`,
    })),
  }
  const current = crumbs[crumbs.length - 1]

  return (
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <JsonLd data={breadcrumbLd} />
      {structuredData.map((data) => (
        <JsonLd key={String(data['@type'])} data={data} />
      ))}
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-4xl px-4 py-12">
        <div className="lanes-panel p-8 md:p-12">
          <nav aria-label={t('common.breadcrumb.label')} className="mb-6">
            <ol className="text-dim flex flex-wrap items-center gap-2 text-xs">
              <li className="flex items-center gap-2">
                <Link href="/" className="underline underline-offset-2">
                  {home}
                </Link>
                <span aria-hidden="true">/</span>
              </li>
              {crumbs.slice(0, -1).map((crumb) => (
                <li key={crumb.path} className="flex items-center gap-2">
                  <Link href={crumb.path} className="underline underline-offset-2">
                    {crumb.name}
                  </Link>
                  <span aria-hidden="true">/</span>
                </li>
              ))}
              <li aria-current="page">{current.name}</li>
            </ol>
          </nav>
          {children}
        </div>
      </main>
    </div>
  )
}
