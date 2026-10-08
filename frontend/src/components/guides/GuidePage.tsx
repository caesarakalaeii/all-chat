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
 * Shared shell for the public search-intent guides. Server component: the
 * breadcrumb and HowTo JSON-LD land in the initial HTML for crawlers, like the
 * docs page's.
 */

import Link from 'next/link'
import type { ReactNode } from 'react'
import { AppNav } from '@/components/AppNav'
import { JsonLd } from '@/components/JsonLd'
import { getTranslations, type MessageKey, type MessageParams } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { cn } from '@/lib/utils'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

const SITE_URL = 'https://allch.at'

/** The guide routes, in reading order. Each page links the others. */
export const GUIDES = [
  { path: '/obs-chat-overlay', labelKey: 'guides.shared.linkObsOverlay' },
  { path: '/obs-chat-dock', labelKey: 'guides.shared.linkObsDock' },
  { path: '/tiktok-live-chat-overlay', labelKey: 'guides.shared.linkTiktok' },
  { path: '/multistream-chat', labelKey: 'guides.shared.linkMultistream' },
  { path: '/compare', labelKey: 'guides.shared.linkCompare' },
] as const satisfies readonly { path: string; labelKey: MessageKey }[]

export type GuidePath = (typeof GUIDES)[number]['path']

/** One placeholder of a step sentence: the words it resolves to, and how they render. */
interface StepPart {
  text: string
  render: (text: string) => ReactNode
}

export interface GuideStep {
  key: MessageKey
  parts?: Readonly<Record<string, StepPart>>
}

function stepText({ key, parts = {} }: GuideStep): string {
  const params: MessageParams = {}
  for (const [name, part] of Object.entries(parts)) params[name] = part.text
  return t(key, params)
}

/**
 * A numbered procedure. It and howToLd() resolve every placeholder from the
 * same StepPart, so the HowTo structured data cannot drift from the visible
 * steps, which is what search engines require of it.
 */
export function StepList({ steps }: { steps: readonly GuideStep[] }) {
  return (
    <ol className="list-decimal space-y-2 pl-6">
      {steps.map(({ key, parts }) => {
        if (!parts) return <li key={key}>{t(key)}</li>
        const elements: Record<string, ReactNode> = {}
        for (const [name, part] of Object.entries(parts)) elements[name] = part.render(part.text)
        return <li key={key}>{interpolateElements(t(key), elements)}</li>
      })}
    </ol>
  )
}

export function howToLd(name: string, description: string, steps: readonly GuideStep[]) {
  return {
    '@context': 'https://schema.org',
    '@type': 'HowTo',
    name,
    description,
    step: steps.map((step, index) => ({
      '@type': 'HowToStep',
      position: index + 1,
      text: stepText(step),
    })),
  }
}

interface GuidePageProps {
  path: GuidePath
  breadcrumb: string
  heading: string
  intro: string
  /** JSON-LD beyond the breadcrumb trail, such as a HowTo. */
  structuredData?: readonly Record<string, unknown>[]
  children: ReactNode
}

export function GuidePage({
  path,
  breadcrumb,
  heading,
  intro,
  structuredData = [],
  children,
}: GuidePageProps) {
  const home = t('guides.shared.breadcrumbHome')
  const breadcrumbLd = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: home, item: SITE_URL },
      { '@type': 'ListItem', position: 2, name: breadcrumb, item: `${SITE_URL}${path}` },
    ],
  }

  return (
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <JsonLd data={breadcrumbLd} />
      {structuredData.map((data) => (
        <JsonLd key={String(data['@type'])} data={data} />
      ))}
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-4xl px-4 py-12">
        <div className="lanes-panel p-8 md:p-12">
          <nav aria-label={t('guides.shared.breadcrumbLabel')} className="mb-6">
            <ol className="text-dim flex flex-wrap items-center gap-2 text-xs">
              <li className="flex items-center gap-2">
                <Link href="/" className="underline underline-offset-2">
                  {home}
                </Link>
                <span aria-hidden="true">/</span>
              </li>
              <li aria-current="page">{breadcrumb}</li>
            </ol>
          </nav>

          <header className="mb-8 space-y-2">
            <p className="mono-label text-xs">{t('guides.shared.eyebrow')}</p>
            <h1 className="text-3xl">{heading}</h1>
            <p className="text-sub text-sm">{intro}</p>
          </header>

          <div className="legal-prose space-y-10 leading-relaxed">{children}</div>

          <section
            aria-labelledby="guide-cta-heading"
            className="mt-12 space-y-3 border-t border-white/20 pt-6"
          >
            <h2 id="guide-cta-heading" className="text-lg">
              {t('guides.shared.ctaHeading')}
            </h2>
            <p className="text-sub text-sm">{t('guides.shared.ctaBody')}</p>
            <div className="flex flex-col gap-3 sm:flex-row">
              <Link href="/#get-started" className="lanes-btn">
                {t('guides.shared.ctaSignIn')}
              </Link>
              <Link href="/docs" className="lanes-btn ghost">
                {t('guides.shared.ctaDocs')}
              </Link>
            </div>
          </section>

          <nav aria-labelledby="guide-related-heading" className="mt-8">
            <h2
              id="guide-related-heading"
              className="text-dim mb-2 text-xs tracking-[0.15em] uppercase"
            >
              {t('guides.shared.relatedHeading')}
            </h2>
            <ul className="grid gap-1 sm:grid-cols-2">
              {GUIDES.filter((guide) => guide.path !== path).map((guide) => (
                <li key={guide.path}>
                  <Link href={guide.path} className="text-sm underline underline-offset-2">
                    {t(guide.labelKey)}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </main>
    </div>
  )
}
