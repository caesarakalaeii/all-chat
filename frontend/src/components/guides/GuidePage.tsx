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
 * Shared body for the public search-intent guides, inside the public page
 * shell. Server component: the breadcrumb and HowTo JSON-LD land in the initial
 * HTML for crawlers, like the docs page's.
 */

import Link from 'next/link'
import type { ReactNode } from 'react'
import { PublicPageShell } from '@/components/PublicPageShell'
import { getTranslations, type MessageKey, type MessageParams } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

/**
 * The guide routes, in reading order: /guides lists them, and each guide links
 * the others. The description is the guide's own search snippet, so the
 * table of contents cannot promise something the page does not say.
 */
export const GUIDES = [
  {
    path: '/obs-chat-overlay',
    labelKey: 'guides.shared.linkObsOverlay',
    descriptionKey: 'guides.obsOverlay.metaDescription',
  },
  {
    path: '/obs-chat-dock',
    labelKey: 'guides.shared.linkObsDock',
    descriptionKey: 'guides.obsDock.metaDescription',
  },
  {
    path: '/tiktok-live-chat-overlay',
    labelKey: 'guides.shared.linkTiktok',
    descriptionKey: 'guides.tiktok.metaDescription',
  },
  {
    path: '/multistream-chat',
    labelKey: 'guides.shared.linkMultistream',
    descriptionKey: 'guides.multistream.metaDescription',
  },
  {
    path: '/compare',
    labelKey: 'guides.shared.linkCompare',
    descriptionKey: 'guides.compare.metaDescription',
  },
] as const satisfies readonly { path: string; labelKey: MessageKey; descriptionKey: MessageKey }[]

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
  return (
    <PublicPageShell
      crumbs={[
        { name: t('guides.index.breadcrumb'), path: '/guides' },
        { name: breadcrumb, path },
      ]}
      structuredData={structuredData}
    >
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
    </PublicPageShell>
  )
}
