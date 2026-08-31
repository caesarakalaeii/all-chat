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
import LegalLayout from '@/components/legal/LegalLayout'
import { getTranslations } from '@/lib/i18n'
import { emphasise, interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not the hook: this route is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('metadata.terms.title'),
  description: t('metadata.terms.description'),
  alternates: { canonical: '/legal/terms' },
}

const listClasses = 'list-disc pl-6 space-y-1 text-text-sub'
const linkClasses = 'text-twitch underline decoration-twitch/30 underline-offset-4'

const SUPPORT_MAILTO = 'mailto:all.chat.support@gmail.com'

export default function TermsOfServicePage() {
  const supportEmail = t('legal.terms.supportEmail')
  const privacyLinkText = t('legal.terms.privacyLinkText')

  const privacyLink = (
    <Link href="/legal/privacy" className={linkClasses}>
      {privacyLinkText}
    </Link>
  )
  const supportLink = (
    <a href={SUPPORT_MAILTO} className={linkClasses}>
      {supportEmail}
    </a>
  )

  return (
    <LegalLayout title={t('legal.terms.title')} lastUpdated={t('legal.terms.lastUpdated')}>
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.acceptanceHeading')}</h2>
        <p>
          {emphasise(
            t('legal.terms.acceptanceBody', { privacy: privacyLinkText }),
            privacyLinkText,
            () => privacyLink
          )}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.descriptionHeading')}</h2>
        <p>{t('legal.terms.descriptionBody')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.accountsHeading')}</h2>
        <p>{t('legal.terms.accountsIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.terms.accountsAccurate')}</li>
          <li>{t('legal.terms.accountsSecurity')}</li>
          <li>
            {emphasise(
              t('legal.terms.accountsNotify', { email: supportEmail }),
              supportEmail,
              () => supportLink
            )}
          </li>
          <li>{t('legal.terms.accountsComply')}</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          {t('legal.terms.acceptableUseHeading')}
        </h2>
        <p>{t('legal.terms.acceptableUseIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.terms.acceptableUseLaws')}</li>
          <li>{t('legal.terms.acceptableUseIp')}</li>
          <li>{t('legal.terms.acceptableUseMalware')}</li>
          <li>{t('legal.terms.acceptableUseBypass')}</li>
          <li>{t('legal.terms.acceptableUseHarassment')}</li>
          <li>{t('legal.terms.acceptableUsePartner')}</li>
        </ul>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.thirdPartyHeading')}</h2>
        <p>{t('legal.terms.thirdPartyIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.terms.thirdPartyComply')}</li>
          <li>{t('legal.terms.thirdPartyOutages')}</li>
          <li>{t('legal.terms.thirdPartyQuotas')}</li>
        </ul>
        <p className="font-semibold text-text">
          {interpolateElements(t('legal.terms.youtubeBinding'), {
            youtubeTerms: (
              <a
                href="https://www.youtube.com/t/terms"
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.terms.youtubeTermsLinkText')}
              </a>
            ),
          })}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.privacyHeading')}</h2>
        {/* Two runs in one sentence, so interpolateElements rather than two
            nested emphasise calls. */}
        <p>
          {interpolateElements(t('legal.terms.privacyBody'), {
            privacy: privacyLink,
            analytics: (
              <strong className="text-text">{t('legal.terms.privacyAnalyticsEmphasis')}</strong>
            ),
          })}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.licenseHeading')}</h2>
        <p>
          {interpolateElements(t('legal.terms.licenseBody'), {
            license: (
              <a
                href="https://www.gnu.org/licenses/agpl-3.0.html"
                target="_blank"
                rel="noreferrer"
                className={linkClasses}
              >
                {t('legal.terms.licenseLinkText')}
              </a>
            ),
          })}
        </p>
        <p>
          {interpolateElements(t('legal.terms.licenseRepository'), {
            github: (
              <a
                href="https://github.com/caesarakalaeii/all-chat"
                target="_blank"
                rel="noreferrer"
                className={linkClasses}
              >
                {t('legal.terms.githubLinkText')}
              </a>
            ),
          })}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.availabilityHeading')}</h2>
        <p>{t('legal.terms.availabilityIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.terms.availabilityUptime')}</li>
          <li>{t('legal.terms.availabilityCompat')}</li>
          <li>{t('legal.terms.availabilityFixes')}</li>
        </ul>
        <p>
          {emphasise(
            t('legal.terms.availabilitySupport', { host: t('legal.terms.hostedDomain') }),
            t('legal.terms.hostedDomain'),
            (run) => (
              <strong className="text-text">{run}</strong>
            )
          )}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.liabilityHeading')}</h2>
        <p>{t('legal.terms.liabilityGross')}</p>
        <p>{t('legal.terms.liabilitySlight')}</p>
        <p>{t('legal.terms.liabilityAgents')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.indemnityHeading')}</h2>
        <p>{t('legal.terms.indemnityBody')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.premiumHeading')}</h2>
        <p>
          {interpolateElements(t('legal.terms.premiumBody'), {
            patreon: (
              <a
                href="https://www.patreon.com"
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.terms.patreonLinkText')}
              </a>
            ),
          })}
        </p>
        <p>{t('legal.terms.premiumCancellation')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.changesHeading')}</h2>
        <p>{t('legal.terms.changesBody')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.terminationHeading')}</h2>
        <p>{t('legal.terms.terminationBody')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.governingLawHeading')}</h2>
        <p>{t('legal.terms.governingLawBody')}</p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.legalNoticeHeading')}</h2>
        <p>
          {interpolateElements(t('legal.terms.legalNoticeBody'), {
            impressum: (
              <Link href="/legal/impressum" className={linkClasses}>
                {t('legal.terms.impressumLinkText')}
              </Link>
            ),
          })}
        </p>
      </section>

      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.terms.contactHeading')}</h2>
        <p>
          {emphasise(
            t('legal.terms.contactBody', { email: supportEmail }),
            supportEmail,
            () => supportLink
          )}
        </p>
      </section>
    </LegalLayout>
  )
}
