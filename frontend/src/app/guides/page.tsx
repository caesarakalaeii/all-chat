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
import { GUIDES } from '@/components/guides/GuidePage'
import { PublicPageShell } from '@/components/PublicPageShell'
import { getTranslations } from '@/lib/i18n'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.index.metaTitle'),
  description: t('guides.index.metaDescription'),
  alternates: { canonical: '/guides' },
}

export default function GuidesIndexPage() {
  return (
    <PublicPageShell crumbs={[{ name: t('guides.index.breadcrumb'), path: '/guides' }]}>
      <header className="mb-8 space-y-2">
        <h1 className="text-3xl">{t('guides.index.heading')}</h1>
        <p className="text-sub text-sm">{t('guides.index.intro')}</p>
      </header>

      <ul className="space-y-8">
        {GUIDES.map((guide) => (
          <li key={guide.path} className="space-y-2 border-t border-white/20 pt-6">
            <h2 className="text-xl">
              <Link href={guide.path} className="underline underline-offset-4">
                {t(guide.labelKey)}
              </Link>
            </h2>
            <p className="text-sub text-sm">{t(guide.descriptionKey)}</p>
          </li>
        ))}
      </ul>
    </PublicPageShell>
  )
}
