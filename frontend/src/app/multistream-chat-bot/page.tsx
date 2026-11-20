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
  title: t('guides.chatBot.metaTitle'),
  description: t('guides.chatBot.metaDescription'),
  alternates: { canonical: '/multistream-chat-bot' },
}

const strong = (text: string) => <strong>{text}</strong>

const STEPS: readonly GuideStep[] = [
  {
    key: 'guides.chatBot.stepOpen',
    parts: { commands: { text: t('guides.chatBot.stepOpenNav'), render: strong } },
  },
  {
    key: 'guides.chatBot.stepNew',
    parts: { create: { text: t('guides.chatBot.stepNewButton'), render: strong } },
  },
  { key: 'guides.chatBot.stepReply' },
  { key: 'guides.chatBot.stepSave' },
]

const howTo = howToLd(t('guides.chatBot.stepsHeading'), t('guides.chatBot.intro'), STEPS)

const premiumLink = <Link href="/upgrade">{t('guides.shared.premiumLinkText')}</Link>

export default function MultistreamChatBotPage() {
  return (
    <GuidePage
      path="/multistream-chat-bot"
      breadcrumb={t('guides.chatBot.breadcrumb')}
      heading={t('guides.chatBot.heading')}
      intro={t('guides.chatBot.intro')}
      structuredData={[howTo]}
    >
      <section id="kinds">
        <h2>{t('guides.chatBot.kindsHeading')}</h2>
        <ul>
          <li>{t('guides.chatBot.kindCommand')}</li>
          <li>{t('guides.chatBot.kindKeyword')}</li>
          <li>{t('guides.chatBot.kindTimer')}</li>
        </ul>
      </section>

      <section id="setup">
        <h2>{t('guides.chatBot.stepsHeading')}</h2>
        <StepList steps={STEPS} />
      </section>

      <section id="replies">
        <h2>{t('guides.chatBot.repliesHeading')}</h2>
        <p>{t('guides.chatBot.repliesBody')}</p>
        <p>
          {interpolateElements(t('guides.chatBot.actionsBody'), {
            alerts: <Link href="/stream-alerts">{t('guides.chatBot.actionsAlertsLinkText')}</Link>,
          })}
        </p>
        <p>{t('guides.chatBot.accessBody')}</p>
      </section>

      <section id="sender">
        <h2>{t('guides.chatBot.senderHeading')}</h2>
        <p>{interpolateElements(t('guides.chatBot.senderBody'), { premium: premiumLink })}</p>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>{interpolateElements(t('guides.chatBot.limitYoutube'), { premium: premiumLink })}</li>
          <li>{t('guides.chatBot.limitReadOnly')}</li>
        </ul>
      </section>
    </GuidePage>
  )
}
