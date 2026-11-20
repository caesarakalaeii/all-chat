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
import { GuidePage } from '@/components/guides/GuidePage'
import {
  COLUMNS,
  LAST_CHECKED,
  ROWS,
  SOURCES,
  VS_TOOLS,
  type SourceId,
} from '@/components/guides/compareData'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.compare.metaTitle'),
  description: t('guides.compare.metaDescription'),
  alternates: { canonical: '/compare' },
}

const ISSUES_URL = 'https://github.com/caesarakalaeii/all-chat/issues'

function SourceRefs({ ids }: { ids: readonly SourceId[] }) {
  if (ids.length === 0) return null
  return (
    <sup className="whitespace-nowrap">
      {ids.map((id) => {
        const number = SOURCES.findIndex((source) => source.id === id) + 1
        return (
          <a
            key={id}
            href={`#source-${number}`}
            aria-label={t('guides.compare.sourceRefLabel', { number })}
            className="ml-1"
          >
            [{number}]
          </a>
        )
      })}
    </sup>
  )
}

const lastChecked = <time dateTime={LAST_CHECKED}>{LAST_CHECKED}</time>

export default function ComparePage() {
  return (
    <GuidePage
      path="/compare"
      breadcrumb={t('guides.compare.breadcrumb')}
      heading={t('guides.compare.heading')}
      intro={t('guides.compare.intro')}
    >
      <section id="table">
        <p>{interpolateElements(t('guides.compare.lastChecked'), { date: lastChecked })}</p>
        {/* Wider than the panel on purpose: on a phone the table scrolls inside
            this box instead of crushing eleven columns into the viewport. */}
        <div className="my-4 overflow-x-auto border-2 border-lanes-fg-strong">
          <table className="w-full min-w-4xl text-left text-sm">
            <caption className="text-sub px-3 py-2 text-left text-xs">
              {interpolateElements(t('guides.compare.caption'), { date: lastChecked })}
            </caption>
            <thead>
              <tr>
                <th scope="col">{t('guides.compare.colTool')}</th>
                {COLUMNS.map((column) => (
                  <th key={column.id} scope="col" className="align-bottom">
                    {t(column.labelKey)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr key={row.nameKey} className="align-top">
                  <th scope="row">{t(row.nameKey)}</th>
                  {COLUMNS.map((column) => {
                    const { textKey, sources } = row.cells[column.id]
                    return (
                      <td key={column.id} className="text-sub">
                        {t(textKey)}
                        <SourceRefs ids={sources} />
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section id="better-pick">
        <h2>{t('guides.compare.betterHeading')}</h2>
        <p>{t('guides.compare.betterIntro')}</p>
        <ul>
          <li>{t('guides.compare.betterAllInOne')}</li>
          <li>{t('guides.compare.betterTts')}</li>
          <li>{t('guides.compare.betterPlatforms')}</li>
          <li>{t('guides.compare.betterRestream')}</li>
        </ul>
      </section>

      <section id="one-on-one">
        <h2>{t('guides.vs.listHeading')}</h2>
        <ul>
          {VS_TOOLS.map(({ id, slug }) => (
            <li key={id}>
              <Link href={`/compare/${slug}`}>{t(`guides.vs.${id}Heading`)}</Link>
            </li>
          ))}
        </ul>
      </section>

      <section id="all-chat">
        <h2>{t('guides.compare.fitHeading')}</h2>
        <p>{t('guides.compare.fitBody')}</p>
        <p>
          {interpolateElements(t('guides.compare.corrections'), {
            issue: (
              <a href={ISSUES_URL} target="_blank" rel="noopener noreferrer">
                {t('guides.compare.correctionsLinkText')}
              </a>
            ),
          })}
        </p>
      </section>

      <section id="sources">
        <h2>{t('guides.compare.sourcesHeading')}</h2>
        <ol className="list-decimal space-y-1 pl-6 text-sm">
          {SOURCES.map(({ id, url, labelKey }, index) => (
            <li key={id} id={`source-${index + 1}`}>
              {t(labelKey)}:
              <a href={url} target="_blank" rel="noopener noreferrer" className="ml-1 break-all">
                {url}
              </a>
            </li>
          ))}
        </ol>
      </section>
    </GuidePage>
  )
}
