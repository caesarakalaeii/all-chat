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
import { notFound } from 'next/navigation'
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

// One page per tool in VS_TOOLS and nothing else: an unknown slug is a 404,
// not a page rendered on demand.
export const dynamicParams = false

export function generateStaticParams() {
  return VS_TOOLS.map(({ slug }) => ({ tool: slug }))
}

interface Props {
  params: Promise<{ tool: string }>
}

const findTool = (slug: string) => VS_TOOLS.find((tool) => tool.slug === slug)

export async function generateMetadata({ params }: Props) {
  const tool = findTool((await params).tool)
  if (!tool) return {}
  return {
    title: t(`guides.vs.${tool.id}MetaTitle`),
    description: t(`guides.vs.${tool.id}MetaDescription`),
    alternates: { canonical: `/compare/${tool.slug}` },
  }
}

const lastChecked = <time dateTime={LAST_CHECKED}>{LAST_CHECKED}</time>

/**
 * All-Chat next to one other tool. Every cell is the /compare cell for that
 * tool, read from the same data, so the two pages cannot disagree; only the
 * prose around the table is written per tool.
 */
export default async function VsPage({ params }: Props) {
  const tool = findTool((await params).tool)
  if (!tool) notFound()

  const allChat = ROWS.find((row) => row.id === 'allChat')
  const other = ROWS.find((row) => row.id === tool.id)
  if (!allChat || !other) notFound()

  const name = t(`guides.vs.${tool.id}Name`)
  const rows = [allChat, other]

  // Footnotes are numbered per page, in the order of the full source list.
  const cited = new Set<SourceId>(
    rows.flatMap((row) => COLUMNS.flatMap((column) => row.cells[column.id].sources))
  )
  const sources = SOURCES.filter((source) => cited.has(source.id))
  const numberOf = (id: SourceId) => sources.findIndex((source) => source.id === id) + 1

  return (
    <GuidePage
      path={`/compare/${tool.slug}`}
      parent={{ name: t('guides.compare.breadcrumb'), path: '/compare' }}
      breadcrumb={name}
      heading={t(`guides.vs.${tool.id}Heading`)}
      intro={t(`guides.vs.${tool.id}Intro`)}
    >
      <section id="table">
        <p>{interpolateElements(t('guides.compare.lastChecked'), { date: lastChecked })}</p>
        <div className="my-4 overflow-x-auto border-2 border-lanes-fg-strong">
          <table className="w-full text-left text-sm">
            <caption className="text-sub px-3 py-2 text-left text-xs">
              {interpolateElements(t('guides.vs.caption', { tool: name }), {
                date: lastChecked,
              })}
            </caption>
            <thead>
              <tr>
                <th scope="col">{t('guides.vs.colFeature')}</th>
                {rows.map((row) => (
                  <th key={row.id} scope="col">
                    {t(row.nameKey)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {COLUMNS.map((column) => (
                <tr key={column.id} className="align-top">
                  <th scope="row">{t(column.labelKey)}</th>
                  {rows.map((row) => {
                    const { textKey, sources: ids } = row.cells[column.id]
                    return (
                      <td key={row.id} className="text-sub">
                        {t(textKey)}
                        {ids.length > 0 && (
                          <sup className="whitespace-nowrap">
                            {ids.map((id) => (
                              <a
                                key={id}
                                href={`#source-${numberOf(id)}`}
                                aria-label={t('guides.compare.sourceRefLabel', {
                                  number: numberOf(id),
                                })}
                                className="ml-1"
                              >
                                [{numberOf(id)}]
                              </a>
                            ))}
                          </sup>
                        )}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section id="pick-other">
        <h2>{t('guides.vs.pickThemHeading', { tool: name })}</h2>
        <p>{t(`guides.vs.${tool.id}PickThem`)}</p>
      </section>

      <section id="pick-all-chat">
        <h2>{t('guides.vs.pickUsHeading')}</h2>
        <p>{t(`guides.vs.${tool.id}PickUs`)}</p>
        <p>
          {interpolateElements(t('guides.vs.fullComparison'), {
            compare: <Link href="/compare">{t('guides.vs.fullComparisonLinkText')}</Link>,
          })}
        </p>
      </section>

      <section id="sources">
        <h2>{t('guides.compare.sourcesHeading')}</h2>
        <ol className="list-decimal space-y-1 pl-6 text-sm">
          {sources.map(({ id, url, labelKey }, index) => (
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
