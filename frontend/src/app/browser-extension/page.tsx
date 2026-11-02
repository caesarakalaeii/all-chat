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
import { ExtensionInstallButtons } from '@/components/ExtensionInstallButtons'
import { PublicPageShell } from '@/components/PublicPageShell'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('marketing.extensionPage.metaTitle'),
  description: t('marketing.extensionPage.metaDescription'),
  alternates: { canonical: '/browser-extension' },
}

// Viewers are the audience here, so this is not a GuidePage: GuidePage closes
// on the streamer sign-in and files the page under /guides in the trail.
export default function BrowserExtensionPage() {
  return (
    <PublicPageShell
      crumbs={[{ name: t('marketing.extensionPage.breadcrumb'), path: '/browser-extension' }]}
    >
      <header className="mb-8 space-y-2">
        <p className="mono-label text-xs">{t('marketing.extensionPage.eyebrow')}</p>
        <h1 className="text-3xl">{t('marketing.extensionPage.heading')}</h1>
        <p className="text-sub text-sm">{t('marketing.extensionPage.intro')}</p>
      </header>

      <section className="mb-10 space-y-3">
        <ExtensionInstallButtons />
        <p className="text-dim text-xs">{t('marketing.extensionPage.installNote')}</p>
      </section>

      <div className="legal-prose space-y-10 leading-relaxed">
        <section id="features">
          <h2>{t('marketing.extensionPage.featuresHeading')}</h2>
          <ul>
            <li>{t('marketing.extensionPage.featureMerged')}</li>
            <li>{t('marketing.extensionPage.featureSend')}</li>
            <li>{t('marketing.extensionPage.featureEmotes')}</li>
            <li>{t('marketing.extensionPage.featureEngage')}</li>
            <li>{t('marketing.extensionPage.featureNative')}</li>
          </ul>
        </section>

        <section id="which-streams">
          <h2>{t('marketing.extensionPage.whereHeading')}</h2>
          <p>{t('marketing.extensionPage.whereBody')}</p>
          <p>
            {interpolateElements(t('marketing.extensionPage.sourceBody'), {
              github: (
                <a href="https://github.com/caesarakalaeii/all-chat-extension">
                  {t('marketing.extensionPage.sourceLinkText')}
                </a>
              ),
            })}
          </p>
        </section>

        <section id="streamers">
          <h2>{t('marketing.extensionPage.streamerHeading')}</h2>
          <p>
            {interpolateElements(t('marketing.extensionPage.streamerBody'), {
              dashboard: (
                <Link href="/dashboard">{t('marketing.extensionPage.streamerLinkText')}</Link>
              ),
            })}
          </p>
        </section>
      </div>
    </PublicPageShell>
  )
}
