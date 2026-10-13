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
import { PublicPageShell } from '@/components/PublicPageShell'
import { POSTS, formatPostDate } from '@/content/blog'
import { getTranslations } from '@/lib/i18n'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('blog.metaTitle'),
  description: t('blog.metaDescription'),
  alternates: { canonical: '/blog' },
}

export default function BlogIndexPage() {
  return (
    <PublicPageShell crumbs={[{ name: t('blog.breadcrumb'), path: '/blog' }]}>
      <header className="mb-8 space-y-2">
        <p className="mono-label text-xs">{t('blog.eyebrow')}</p>
        <h1 className="text-3xl">{t('blog.heading')}</h1>
        <p className="text-sub text-sm">{t('blog.intro')}</p>
      </header>

      <section aria-labelledby="blog-posts-heading">
        <h2 id="blog-posts-heading" className="sr-only">
          {t('blog.listHeading')}
        </h2>
        <ul className="space-y-8">
          {POSTS.map(({ meta }) => (
            <li key={meta.slug} className="space-y-2 border-t border-white/20 pt-6">
              <p className="text-dim text-xs">
                <time dateTime={meta.date}>{formatPostDate(meta.date)}</time>
              </p>
              <h3 className="text-xl">
                <Link href={`/blog/${meta.slug}`} className="underline underline-offset-4">
                  {meta.title}
                </Link>
              </h3>
              <p className="text-sub text-sm">{meta.description}</p>
            </li>
          ))}
        </ul>
      </section>
    </PublicPageShell>
  )
}
