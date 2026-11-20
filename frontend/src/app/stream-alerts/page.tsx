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
import { Code } from '@/components/docs/prose'
import { GuidePage, StepList, howToLd, type GuideStep } from '@/components/guides/GuidePage'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.alerts.metaTitle'),
  description: t('guides.alerts.metaDescription'),
  alternates: { canonical: '/stream-alerts' },
}

// Template syntax, not copy: the editor accepts exactly these names.
const PLACEHOLDERS = '{username} {amount} {count} {message}'

const strong = (text: string) => <strong>{text}</strong>

const STEPS: readonly GuideStep[] = [
  {
    key: 'guides.alerts.stepCreate',
    parts: { kind: { text: t('guides.alerts.stepCreateKind'), render: strong } },
  },
  { key: 'guides.alerts.stepSources' },
  {
    key: 'guides.alerts.stepStyle',
    parts: {
      section: { text: t('guides.alerts.stepStyleSection'), render: strong },
      all: { text: t('guides.alerts.stepStyleAll'), render: strong },
    },
  },
  { key: 'guides.alerts.stepObs' },
  {
    key: 'guides.alerts.stepTest',
    parts: { test: { text: t('guides.alerts.stepTestButton'), render: strong } },
  },
]

const howTo = howToLd(t('guides.alerts.stepsHeading'), t('guides.alerts.intro'), STEPS)

export default function StreamAlertsPage() {
  return (
    <GuidePage
      path="/stream-alerts"
      breadcrumb={t('guides.alerts.breadcrumb')}
      heading={t('guides.alerts.heading')}
      intro={t('guides.alerts.intro')}
      structuredData={[howTo]}
    >
      <section id="events">
        <h2>{t('guides.alerts.eventsHeading')}</h2>
        <ul>
          <li>{t('guides.alerts.eventsTwitch')}</li>
          <li>{t('guides.alerts.eventsYoutube')}</li>
          <li>{t('guides.alerts.eventsTiktok')}</li>
        </ul>
        <p>{t('guides.alerts.eventsBody')}</p>
      </section>

      <section id="setup">
        <h2>{t('guides.alerts.stepsHeading')}</h2>
        <StepList steps={STEPS} />
      </section>

      <section id="customize">
        <h2>{t('guides.alerts.customHeading')}</h2>
        <p>
          {interpolateElements(t('guides.alerts.customBody'), {
            placeholders: <Code>{PLACEHOLDERS}</Code>,
          })}
        </p>
        <p>{t('guides.alerts.codeBody')}</p>
        <p>
          {interpolateElements(t('guides.alerts.commandsBody'), {
            commands: (
              <Link href="/multistream-chat-bot">{t('guides.alerts.commandsLinkText')}</Link>
            ),
          })}
        </p>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>{t('guides.alerts.limitKick')}</li>
          <li>
            {interpolateElements(t('guides.alerts.limitUploads'), {
              premium: <Link href="/upgrade">{t('guides.shared.premiumLinkText')}</Link>,
            })}
          </li>
          <li>
            {interpolateElements(t('guides.alerts.limitTiktok'), {
              guide: (
                <Link href="/tiktok-live-chat-overlay">
                  {t('guides.shared.tiktokGuideLinkText')}
                </Link>
              ),
            })}
          </li>
        </ul>
      </section>
    </GuidePage>
  )
}
