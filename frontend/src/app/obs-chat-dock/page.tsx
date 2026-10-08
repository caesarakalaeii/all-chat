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
  title: t('guides.obsDock.metaTitle'),
  description: t('guides.obsDock.metaDescription'),
  alternates: { canonical: '/obs-chat-dock' },
}

// A URL shape, not copy: a translated URL is one nobody can paste.
const DOCK_URL_EXAMPLE = 'https://allch.at/overlay/<overlay-id>/view?dock=1'

const STEPS: readonly GuideStep[] = [
  {
    key: 'guides.obsDock.stepMenu',
    parts: {
      menu: { text: t('guides.obsDock.stepMenuPath'), render: (text) => <strong>{text}</strong> },
    },
  },
  { key: 'guides.obsDock.stepPaste' },
  { key: 'guides.obsDock.stepSignIn' },
  { key: 'guides.obsDock.stepPlace' },
]

const howTo = howToLd(t('guides.obsDock.stepsHeading'), t('guides.obsDock.intro'), STEPS)

const premiumLink = <Link href="/upgrade">{t('guides.shared.premiumLinkText')}</Link>

export default function ObsChatDockPage() {
  return (
    <GuidePage
      path="/obs-chat-dock"
      breadcrumb={t('guides.obsDock.breadcrumb')}
      heading={t('guides.obsDock.heading')}
      intro={t('guides.obsDock.intro')}
      structuredData={[howTo]}
    >
      <section id="monitor-url">
        <h2>{t('guides.obsDock.urlHeading')}</h2>
        <p>
          {interpolateElements(t('guides.obsDock.urlBody'), {
            url: <Code>{DOCK_URL_EXAMPLE}</Code>,
          })}
        </p>
      </section>

      <section id="setup">
        <h2>{t('guides.obsDock.stepsHeading')}</h2>
        <StepList steps={STEPS} />
        <p>{t('guides.obsDock.streamlabsNote')}</p>
      </section>

      <section id="features">
        <h2>{t('guides.obsDock.featuresHeading')}</h2>
        <ul>
          <li>{t('guides.obsDock.featureRead')}</li>
          <li>{t('guides.obsDock.featureSend')}</li>
          <li>
            {interpolateElements(t('guides.obsDock.featureModerate'), { premium: premiumLink })}
          </li>
          <li>{t('guides.obsDock.featureEngage')}</li>
          <li>{interpolateElements(t('guides.obsDock.featureMods'), { premium: premiumLink })}</li>
        </ul>
        <p>
          {interpolateElements(t('guides.obsDock.moreDocs'), {
            monitor: <Link href="/docs#monitor">{t('guides.obsDock.moreDocsMonitor')}</Link>,
            moderation: (
              <Link href="/docs#moderation">{t('guides.obsDock.moreDocsModeration')}</Link>
            ),
            engagement: (
              <Link href="/docs#engagement">{t('guides.obsDock.moreDocsEngagement')}</Link>
            ),
          })}
        </p>
      </section>

      <section id="same-feed">
        <h2>{t('guides.obsDock.sameFeedHeading')}</h2>
        <p>{t('guides.obsDock.sameFeedBody')}</p>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>
            {interpolateElements(t('guides.obsDock.limitPermissions'), {
              enable: <strong>{t('guides.obsDock.limitPermissionsEmphasis')}</strong>,
            })}
          </li>
          <li>
            {interpolateElements(t('guides.obsDock.limitTiktok'), {
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
