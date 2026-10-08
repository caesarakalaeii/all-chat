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
  title: t('guides.obsOverlay.metaTitle'),
  description: t('guides.obsOverlay.metaDescription'),
  alternates: { canonical: '/obs-chat-overlay' },
}

// A URL shape, not copy: a translated URL is one nobody can paste.
const OVERLAY_URL_EXAMPLE = 'https://allch.at/overlay/<overlay-id>'

const strong = (text: string) => <strong>{text}</strong>

const STEPS: readonly GuideStep[] = [
  {
    key: 'guides.obsOverlay.stepSignIn',
    parts: {
      home: {
        text: t('guides.obsOverlay.stepSignInLinkText'),
        render: (text) => <Link href="/">{text}</Link>,
      },
    },
  },
  { key: 'guides.obsOverlay.stepCreate' },
  {
    key: 'guides.obsOverlay.stepCopy',
    parts: { url: { text: OVERLAY_URL_EXAMPLE, render: (text) => <Code>{text}</Code> } },
  },
  {
    key: 'guides.obsOverlay.stepAdd',
    parts: {
      plus: { text: t('guides.obsOverlay.stepAddPlus'), render: strong },
      browser: { text: t('guides.obsOverlay.stepAddBrowser'), render: strong },
    },
  },
  { key: 'guides.obsOverlay.stepSize' },
  {
    key: 'guides.obsOverlay.stepShutdown',
    parts: {
      shutdown: {
        text: t('guides.obsOverlay.stepShutdownEmphasis'),
        render: (text) => <a href="#shutdown-source">{text}</a>,
      },
    },
  },
  { key: 'guides.obsOverlay.stepDone' },
]

const howTo = howToLd(t('guides.obsOverlay.stepsHeading'), t('guides.obsOverlay.intro'), STEPS)

export default function ObsChatOverlayPage() {
  return (
    <GuidePage
      path="/obs-chat-overlay"
      breadcrumb={t('guides.obsOverlay.breadcrumb')}
      heading={t('guides.obsOverlay.heading')}
      intro={t('guides.obsOverlay.intro')}
      structuredData={[howTo]}
    >
      <section id="setup">
        <h2>{t('guides.obsOverlay.stepsHeading')}</h2>
        <StepList steps={STEPS} />
      </section>

      <section id="transparent-background">
        <h2>{t('guides.obsOverlay.backgroundHeading')}</h2>
        <p>{t('guides.obsOverlay.backgroundBody')}</p>
      </section>

      <section id="shutdown-source">
        <h2>{t('guides.obsOverlay.shutdownHeading')}</h2>
        <p>
          {interpolateElements(t('guides.obsOverlay.shutdownBody'), {
            shutdown: strong(t('guides.obsOverlay.stepShutdownEmphasis')),
          })}
        </p>
        <p>{t('guides.obsOverlay.shutdownTradeoff')}</p>
        <p>
          {interpolateElements(t('guides.obsOverlay.shutdown247'), {
            section: (
              <Link href="/docs#24-7-irl">{t('guides.obsOverlay.shutdown247LinkText')}</Link>
            ),
          })}
        </p>
      </section>

      <section id="themes">
        <h2>{t('guides.obsOverlay.themesHeading')}</h2>
        <p>
          {interpolateElements(t('guides.obsOverlay.themesBody'), {
            themes: <Link href="/docs#themes">{t('guides.obsOverlay.themesLinkText')}</Link>,
            css: <Link href="/docs#custom-css">{t('guides.obsOverlay.cssLinkText')}</Link>,
          })}
        </p>
        <p>{t('guides.obsOverlay.platformBadges')}</p>
      </section>

      <section id="emotes">
        <h2>{t('guides.obsOverlay.emotesHeading')}</h2>
        <p>{t('guides.obsOverlay.emotesBody')}</p>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>{t('guides.obsOverlay.limitYoutube')}</li>
          <li>{t('guides.obsOverlay.limitTiktokReadOnly')}</li>
          <li>{t('guides.obsOverlay.limitRetention')}</li>
          <li>
            {interpolateElements(t('guides.obsOverlay.limitTiktok'), {
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
