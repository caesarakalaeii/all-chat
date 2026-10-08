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
import { GuidePage, StepList, howToLd, type GuideStep } from '@/components/guides/GuidePage'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.tiktok.metaTitle'),
  description: t('guides.tiktok.metaDescription'),
  alternates: { canonical: '/tiktok-live-chat-overlay' },
}

// On main, not beta: these are the links a public reader should land on.
const ADR_0052_URL =
  'https://github.com/caesarakalaeii/all-chat/blob/main/docs/adr/0052-retiring-euler-stream-for-tiktok-signing.md'
const TIKTOK_SIGNER_URL =
  'https://github.com/caesarakalaeii/all-chat/tree/main/services/tiktok-signer'

const STEPS: readonly GuideStep[] = [
  { key: 'guides.tiktok.stepOpen' },
  {
    key: 'guides.tiktok.stepConnect',
    parts: {
      connect: {
        text: t('guides.tiktok.stepConnectEmphasis'),
        render: (text) => <strong>{text}</strong>,
      },
    },
  },
  { key: 'guides.tiktok.stepLive' },
  {
    key: 'guides.tiktok.stepObs',
    parts: {
      guide: {
        text: t('guides.tiktok.stepObsLinkText'),
        render: (text) => <Link href="/obs-chat-overlay">{text}</Link>,
      },
    },
  },
]

const howTo = howToLd(t('guides.tiktok.stepsHeading'), t('guides.tiktok.intro'), STEPS)

export default function TiktokLiveChatOverlayPage() {
  return (
    <GuidePage
      path="/tiktok-live-chat-overlay"
      breadcrumb={t('guides.tiktok.breadcrumb')}
      heading={t('guides.tiktok.heading')}
      intro={t('guides.tiktok.intro')}
      structuredData={[howTo]}
    >
      <section id="setup">
        <h2>{t('guides.tiktok.stepsHeading')}</h2>
        <StepList steps={STEPS} />
      </section>

      <section id="no-official-api">
        <h2>{t('guides.tiktok.apiHeading')}</h2>
        <p>{t('guides.tiktok.apiBody')}</p>
      </section>

      <section id="approach">
        <h2>{t('guides.tiktok.approachHeading')}</h2>
        <ul>
          <li>
            {interpolateElements(t('guides.tiktok.approachSigner'), {
              signer: (
                <a href={TIKTOK_SIGNER_URL} target="_blank" rel="noopener noreferrer">
                  {t('guides.tiktok.approachSignerLinkText')}
                </a>
              ),
              adr: (
                <a href={ADR_0052_URL} target="_blank" rel="noopener noreferrer">
                  {t('guides.tiktok.approachAdrLinkText')}
                </a>
              ),
            })}
          </li>
          <li>{t('guides.tiktok.approachSessions')}</li>
          <li>
            {interpolateElements(t('guides.tiktok.approachFallback'), {
              premium: <Link href="/upgrade">{t('guides.shared.premiumLinkText')}</Link>,
            })}
          </li>
        </ul>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>{t('guides.tiktok.limitInterruptions')}</li>
          <li>{t('guides.tiktok.limitReadOnly')}</li>
        </ul>
        <p>
          {interpolateElements(t('guides.tiktok.verticalNote'), {
            guide: <Link href="/multistream-chat">{t('guides.tiktok.verticalNoteLinkText')}</Link>,
          })}
        </p>
      </section>
    </GuidePage>
  )
}
