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

import type { Metadata } from 'next'
import Link from 'next/link'
import { notFound } from 'next/navigation'
import { PublicPageShell, SITE_URL } from '@/components/PublicPageShell'
import { POSTS, findPost, formatPostDate } from '@/content/blog'
import { getTranslations } from '@/lib/i18n'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

interface PostPageProps {
  params: Promise<{ slug: string }>
}

// Every post is known at build time; an unknown slug is a 404, not a render.
export const dynamicParams = false

export function generateStaticParams() {
  return POSTS.map(({ meta }) => ({ slug: meta.slug }))
}

export async function generateMetadata({ params }: PostPageProps): Promise<Metadata> {
  const post = findPost((await params).slug)
  if (!post) return {}
  const { meta } = post
  return {
    title: meta.title,
    description: meta.description,
    alternates: { canonical: `/blog/${meta.slug}` },
    authors: [{ name: meta.author }],
    openGraph: {
      type: 'article',
      title: meta.title,
      description: meta.description,
      publishedTime: meta.date,
      authors: [meta.author],
    },
  }
}

export default async function BlogPostPage({ params }: PostPageProps) {
  const post = findPost((await params).slug)
  if (!post) notFound()
  const { meta, Body } = post
  const path = `/blog/${meta.slug}`
  const articleLd = {
    '@context': 'https://schema.org',
    '@type': 'BlogPosting',
    headline: meta.title,
    description: meta.description,
    datePublished: meta.date,
    author: { '@type': 'Person', name: meta.author },
    mainEntityOfPage: `${SITE_URL}${path}`,
  }

  return (
    <PublicPageShell
      crumbs={[
        { name: t('blog.breadcrumb'), path: '/blog' },
        { name: meta.title, path },
      ]}
      structuredData={[articleLd]}
    >
      <article>
        <header className="mb-8 space-y-2">
          <p className="mono-label text-xs">{t('blog.eyebrow')}</p>
          <h1 className="text-3xl">{meta.title}</h1>
          <p className="text-dim text-xs">
            {t('blog.byline', { author: meta.author })}
            <span aria-hidden="true"> · </span>
            <time dateTime={meta.date}>
              {t('blog.publishedOn', { date: formatPostDate(meta.date) })}
            </time>
          </p>
        </header>

        <div className="legal-prose space-y-10 leading-relaxed">
          <Body />
        </div>
      </article>

      <nav className="mt-12 border-t border-white/20 pt-6">
        <Link href="/blog" className="text-sm underline underline-offset-2">
          {t('blog.backToIndex')}
        </Link>
      </nav>
    </PublicPageShell>
  )
}
