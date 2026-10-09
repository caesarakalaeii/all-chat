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
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.multistream.metaTitle'),
  description: t('guides.multistream.metaDescription'),
  alternates: { canonical: '/multistream-chat' },
}

// Not a procedure, so no HowTo: GuidePage's breadcrumb trail is the only
// structured data here.
export default function MultistreamChatPage() {
  return (
    <GuidePage
      path="/multistream-chat"
      breadcrumb={t('guides.multistream.breadcrumb')}
      heading={t('guides.multistream.heading')}
      intro={t('guides.multistream.intro')}
    >
      <section id="any-setup">
        <h2>{t('guides.multistream.methodHeading')}</h2>
        <p>{t('guides.multistream.methodBody')}</p>
        <p>{t('guides.multistream.methodSources')}</p>
      </section>

      <section id="platforms">
        <h2>{t('guides.multistream.platformsHeading')}</h2>
        <p>
          {interpolateElements(t('guides.multistream.platformsBody'), {
            css: (
              <Link href="/docs#custom-css">{t('guides.multistream.platformsCssLinkText')}</Link>
            ),
          })}
        </p>
      </section>

      <section id="moderation">
        <h2>{t('guides.multistream.moderationHeading')}</h2>
        <p>
          {interpolateElements(t('guides.multistream.moderationBody'), {
            premium: <Link href="/upgrade">{t('guides.shared.premiumLinkText')}</Link>,
          })}
        </p>
        <p>
          {interpolateElements(t('guides.multistream.moderationDock'), {
            guide: (
              <Link href="/obs-chat-dock">{t('guides.multistream.moderationDockLinkText')}</Link>
            ),
          })}
        </p>
      </section>

      <section id="engagement">
        <h2>{t('guides.multistream.engagementHeading')}</h2>
        <p>{t('guides.multistream.engagementBody')}</p>
      </section>

      <section id="overlays">
        <h2>{t('guides.multistream.overlaysHeading')}</h2>
        <p>{t('guides.multistream.overlaysBody')}</p>
      </section>

      <section id="limits">
        <h2>{t('guides.shared.limitsHeading')}</h2>
        <ul>
          <li>
            {interpolateElements(t('guides.multistream.limitTiktok'), {
              guide: (
                <Link href="/tiktok-live-chat-overlay">
                  {t('guides.shared.tiktokGuideLinkText')}
                </Link>
              ),
            })}
          </li>
          <li>{t('guides.multistream.limitYoutube')}</li>
        </ul>
        <p>
          {interpolateElements(t('guides.multistream.compareNote'), {
            compare: <Link href="/compare">{t('guides.multistream.compareNoteLinkText')}</Link>,
          })}
        </p>
      </section>
    </GuidePage>
  )
}
