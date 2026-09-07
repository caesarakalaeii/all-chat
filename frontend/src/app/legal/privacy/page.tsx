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
import { getTranslations, type MessageKey } from '@/lib/i18n'
import { emphasise, interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not the hook: this route is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('metadata.privacy.title'),
  description: t('metadata.privacy.description'),
  alternates: { canonical: '/legal/privacy' },
}

const listClasses = 'list-disc pl-6 space-y-1 text-text-sub'
const linkClasses = 'text-twitch underline decoration-twitch/30 underline-offset-4'
const noteClasses = 'text-sm text-text-dim'

const SUPPORT_MAILTO = 'mailto:all.chat.support@gmail.com'
const REPOSITORY_URL = 'https://github.com/caesarakalaeii/all-chat'
const GOOGLE_CONNECTIONS_URL = 'https://myaccount.google.com/connections?filters=3,4&hl=en'

// Not copy: the server route the font proxy is actually mounted at. Translating
// it would name a path that does not exist, so the disclosure would describe a
// request the browser never makes.
const FONT_PROXY_PATH = '/font-proxy/*'

/** Bolds a run inside a sentence. Every emphasis on this page renders this way. */
function Emphasis({ children }: { children: React.ReactNode }) {
  return <strong className="text-text">{children}</strong>
}

/** A row whose bolded label opens the sentence the rest of the row continues. */
function LabelledRow({ sentence, label }: { sentence: string; label: string }) {
  return (
    <li>
      {emphasise(sentence, label, (run) => (
        <Emphasis>{run}</Emphasis>
      ))}
    </li>
  )
}

export default function PrivacyPolicyPage() {
  const supportEmail = t('legal.privacy.supportEmail')
  const notEmphasis = t('legal.privacy.notEmphasis')

  const supportLink = (
    <a href={SUPPORT_MAILTO} className={linkClasses}>
      {supportEmail}
    </a>
  )
  const notElement = <Emphasis>{notEmphasis}</Emphasis>

  // Six retention rows and seven rights rows share one shape: a bolded label
  // that opens the sentence the rest of the row continues.
  const labelledRow = (sentenceKey: MessageKey, labelKey: MessageKey) => {
    const label = t(labelKey)
    return <LabelledRow sentence={t(sentenceKey, { label })} label={label} />
  }

  return (
    <LegalLayout title={t('legal.privacy.title')} lastUpdated={t('legal.privacy.lastUpdated')}>
      <div className="space-y-4">
        <div className="rounded-xl border border-twitch/20 bg-twitch/5 p-5 text-text-sub">
          {emphasise(
            t('legal.privacy.tldr', { label: t('legal.privacy.tldrLabel') }),
            t('legal.privacy.tldrLabel'),
            (run) => (
              <Emphasis>{run}</Emphasis>
            )
          )}
        </div>
        <div className="rounded-xl border border-tiktok/20 bg-tiktok/5 p-5 text-text-sub">
          {interpolateElements(
            t('legal.privacy.openSourceCallout', {
              label: t('legal.privacy.openSourceCalloutLabel'),
            }),
            {
              github: (
                <a
                  href={REPOSITORY_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-semibold text-tiktok underline decoration-tiktok/30 underline-offset-4"
                >
                  {t('legal.privacy.githubLinkText')}
                </a>
              ),
            }
          )}
        </div>
      </div>

      {/* --- Verantwortlicher --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.controllerHeading')}</h2>
        <p>
          {interpolateElements(t('legal.privacy.controllerBody'), {
            impressum: (
              <Link href="/legal/impressum" className={linkClasses}>
                {t('legal.privacy.impressumLinkText')}
              </Link>
            ),
            email: supportLink,
          })}
        </p>
      </section>

      {/* --- Information We Collect --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.collectHeading')}</h2>

        <div>
          <h3 className="text-lg font-semibold text-text">{t('legal.privacy.authSubheading')}</h3>
          <p>{t('legal.privacy.authIntro')}</p>
          <ul className={listClasses}>
            <li>{t('legal.privacy.authIdentifiers')}</li>
            <li>{t('legal.privacy.authProfileImages')}</li>
            <li>{t('legal.privacy.authTokens')}</li>
            <li>{t('legal.privacy.authScopes')}</li>
          </ul>
          <p className={noteClasses}>{t('legal.privacy.authLegalBasis')}</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">{t('legal.privacy.chatSubheading')}</h3>
          <p>{t('legal.privacy.chatIntro')}</p>
          <ul className={listClasses}>
            <li>{t('legal.privacy.chatMessages')}</li>
            <li>{t('legal.privacy.chatMetadata')}</li>
            <li>{t('legal.privacy.chatAuthor')}</li>
          </ul>
          <p className={noteClasses}>
            {emphasise(
              t('legal.privacy.chatRetention', {
                emphasis: t('legal.privacy.chatRetentionEmphasis'),
              }),
              t('legal.privacy.chatRetentionEmphasis'),
              (run) => (
                <Emphasis>{run}</Emphasis>
              )
            )}
          </p>
          <p className={noteClasses}>{t('legal.privacy.chatLegalBasis')}</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">
            {t('legal.privacy.overlaySubheading')}
          </h3>
          <p>{t('legal.privacy.overlayIntro')}</p>
          <ul className={listClasses}>
            <li>{t('legal.privacy.overlayNames')}</li>
            <li>{t('legal.privacy.overlaySources')}</li>
            <li>{t('legal.privacy.overlayFilters')}</li>
          </ul>
          <p className={noteClasses}>{t('legal.privacy.overlayLegalBasis')}</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">
            {t('legal.privacy.viewerIdentitySubheading')}
          </h3>
          <p>
            {emphasise(
              t('legal.privacy.viewerIdentityBody', {
                emphasis: t('legal.privacy.viewerIdentityEmphasis'),
              }),
              t('legal.privacy.viewerIdentityEmphasis'),
              (run) => (
                <Emphasis>{run}</Emphasis>
              )
            )}
          </p>
          <p className={noteClasses}>{t('legal.privacy.viewerIdentityLegalBasis')}</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">
            {t('legal.privacy.logDataSubheading')}
          </h3>
          <p>{t('legal.privacy.logDataIntro')}</p>
          <ul className={listClasses}>
            <li>{t('legal.privacy.logDataIp')}</li>
            <li>{t('legal.privacy.logDataMetrics')}</li>
            <li>{t('legal.privacy.logDataTraces')}</li>
          </ul>
          <p className={noteClasses}>{t('legal.privacy.logDataLegalBasis')}</p>
        </div>

        <div>
          <h3 className="text-lg font-semibold text-text">
            {t('legal.privacy.patreonSubheading')}
          </h3>
          <p>{t('legal.privacy.patreonIntro')}</p>
          <ul className={listClasses}>
            <li>{t('legal.privacy.patreonUserId')}</li>
            <li>{t('legal.privacy.patreonTokens')}</li>
            <li>{t('legal.privacy.patreonMembership')}</li>
          </ul>
          <p className={noteClasses}>
            {interpolateElements(t('legal.privacy.patreonNoPaymentData'), { not: notElement })}
          </p>
          <p className={noteClasses}>
            {interpolateElements(t('legal.privacy.patreonLegalBasis'), {
              patreonPolicy: (
                <a
                  href="https://www.patreon.com/policy/legal"
                  target="_blank"
                  rel="noopener noreferrer"
                  className={linkClasses}
                >
                  {t('legal.privacy.patreonPolicyLinkText')}
                </a>
              ),
            })}
          </p>
        </div>
      </section>

      {/* --- How We Use --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.useHeading')}</h2>
        <p>{t('legal.privacy.useIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.privacy.useAuthenticate')}</li>
          <li>{t('legal.privacy.useFetch')}</li>
          <li>{t('legal.privacy.useRender')}</li>
          <li>{t('legal.privacy.useMonitor')}</li>
          <li>{t('legal.privacy.useAbuse')}</li>
        </ul>
      </section>

      {/* --- Storage & Security --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.storageHeading')}</h2>
        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.storageLocationsSubheading')}
        </h3>
        <ul className={listClasses}>
          <li>{t('legal.privacy.storagePostgres')}</li>
          <li>{t('legal.privacy.storageRedis')}</li>
          <li>
            {emphasise(
              t('legal.privacy.storageLocation', { country: t('legal.privacy.storageCountry') }),
              t('legal.privacy.storageCountry'),
              (run) => (
                <Emphasis>{run}</Emphasis>
              )
            )}
          </li>
        </ul>
        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.safeguardsSubheading')}
        </h3>
        <ul className={listClasses}>
          <li>{t('legal.privacy.safeguardsEncryption')}</li>
          <li>{t('legal.privacy.safeguardsHttps')}</li>
          <li>{t('legal.privacy.safeguardsAccess')}</li>
          <li>{t('legal.privacy.safeguardsPatching')}</li>
          <li>{t('legal.privacy.safeguardsHeaders')}</li>
        </ul>
        <p className={noteClasses}>{t('legal.privacy.safeguardsCaveat')}</p>
      </section>

      {/* --- Third Parties --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.sharingHeading')}</h2>

        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.platformApisSubheading')}
        </h3>
        <p>{t('legal.privacy.platformApisIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.privacy.platformApisTwitch')}</li>
          <li>{t('legal.privacy.platformApisYoutube')}</li>
          <li>{t('legal.privacy.platformApisTiktokKick')}</li>
          <li>{t('legal.privacy.platformApisDiscord')}</li>
          <li>{t('legal.privacy.platformApisEmotes')}</li>
        </ul>
        <p>{t('legal.privacy.platformApisScopes')}</p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.fontsSubheading')}</h3>
        <p>
          {interpolateElements(t('legal.privacy.fontsBody'), {
            selfHosted: <Emphasis>{t('legal.privacy.fontsSelfHostedEmphasis')}</Emphasis>,
            proxyPath: (
              <code className="rounded bg-surface-2 px-1 py-0.5 text-xs">{FONT_PROXY_PATH}</code>
            ),
            noTransmission: <Emphasis>{t('legal.privacy.fontsNoTransmissionEmphasis')}</Emphasis>,
          })}
        </p>
        <p className={noteClasses}>{t('legal.privacy.fontsLegalBasis')}</p>

        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.frontendResourcesSubheading')}
        </h3>
        <p>
          {emphasise(
            t('legal.privacy.frontendResourcesIntro', {
              emphasis: t('legal.privacy.ipAddressEmphasis'),
            }),
            t('legal.privacy.ipAddressEmphasis'),
            (run) => (
              <Emphasis>{run}</Emphasis>
            )
          )}
        </p>
        <ul className={listClasses}>
          {labelledRow('legal.privacy.frontendResourcesGithub', 'legal.privacy.githubApiLabel')}
        </ul>
        <p className={noteClasses}>{t('legal.privacy.frontendResourcesLegalBasis')}</p>

        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.youtubeNoticeSubheading')}
        </h3>
        <p className="font-semibold text-text">
          {interpolateElements(t('legal.privacy.youtubeNoticeBody'), {
            googlePolicy: (
              <a
                href="https://policies.google.com/privacy"
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.privacy.googlePolicyLinkText')}
              </a>
            ),
            not: notElement,
          })}
        </p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.noSalesSubheading')}</h3>
        <p className={noteClasses}>{t('legal.privacy.noSalesBody')}</p>

        <h3 className="text-lg font-semibold text-text">
          {t('legal.privacy.analyticsSubheading')}
        </h3>
        <p>
          {interpolateElements(t('legal.privacy.analyticsBody'), {
            umami: (
              <a
                href="https://umami.is"
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.privacy.umamiLinkText')}
              </a>
            ),
            selfHost: <Emphasis>{t('legal.privacy.analyticsSelfHostEmphasis')}</Emphasis>,
            cookieless: <Emphasis>{t('legal.privacy.analyticsCookielessEmphasis')}</Emphasis>,
            notShared: <Emphasis>{t('legal.privacy.analyticsNotSharedEmphasis')}</Emphasis>,
          })}
        </p>
        <p>{t('legal.privacy.analyticsRecordsIntro')}</p>
        <ul className={listClasses}>
          <li>{t('legal.privacy.analyticsRecordsPage')}</li>
          <li>{t('legal.privacy.analyticsRecordsBrowser')}</li>
          <li>{t('legal.privacy.analyticsRecordsCountry')}</li>
        </ul>
        <p className={noteClasses}>
          {interpolateElements(t('legal.privacy.analyticsIpNote'), {
            ipNotStored: <Emphasis>{t('legal.privacy.analyticsIpNotStoredEmphasis')}</Emphasis>,
            not: notElement,
          })}
        </p>
        <p className={noteClasses}>{t('legal.privacy.analyticsConsentNote')}</p>
      </section>

      {/* --- Data Retention --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.retentionHeading')}</h2>
        <ul className={listClasses}>
          {labelledRow('legal.privacy.retentionAccount', 'legal.privacy.retentionAccountLabel')}
          {labelledRow('legal.privacy.retentionTokens', 'legal.privacy.retentionTokensLabel')}
          {labelledRow(
            'legal.privacy.retentionSentMessages',
            'legal.privacy.retentionSentMessagesLabel'
          )}
          {labelledRow(
            'legal.privacy.retentionDisplayedMessages',
            'legal.privacy.retentionDisplayedMessagesLabel'
          )}
          {labelledRow('legal.privacy.retentionLogs', 'legal.privacy.retentionLogsLabel')}
          {labelledRow('legal.privacy.retentionQuotaLogs', 'legal.privacy.retentionQuotaLogsLabel')}
        </ul>
      </section>

      {/* --- Your Rights --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.rightsHeading')}</h2>
        <p>{t('legal.privacy.rightsIntro')}</p>
        <ul className={listClasses}>
          {labelledRow('legal.privacy.rightsAccess', 'legal.privacy.rightsAccessLabel')}
          {labelledRow(
            'legal.privacy.rightsRectification',
            'legal.privacy.rightsRectificationLabel'
          )}
          {labelledRow('legal.privacy.rightsErasure', 'legal.privacy.rightsErasureLabel')}
          {labelledRow('legal.privacy.rightsRestriction', 'legal.privacy.rightsRestrictionLabel')}
          {labelledRow('legal.privacy.rightsPortability', 'legal.privacy.rightsPortabilityLabel')}
          {labelledRow('legal.privacy.rightsObjection', 'legal.privacy.rightsObjectionLabel')}
          {labelledRow('legal.privacy.rightsWithdraw', 'legal.privacy.rightsWithdrawLabel')}
        </ul>
        <p>
          {emphasise(
            t('legal.privacy.rightsContact', { email: supportEmail }),
            supportEmail,
            () => supportLink
          )}
        </p>
        <p className={noteClasses}>{t('legal.privacy.noProfiling')}</p>
        <p className="font-semibold text-text">
          {interpolateElements(t('legal.privacy.youtubeRevoke'), {
            googleSettings: (
              <a
                href={GOOGLE_CONNECTIONS_URL}
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.privacy.googleSettingsPageLinkText')}
              </a>
            ),
          })}
        </p>
      </section>

      {/* --- Cookies & Browser Storage --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.cookiesHeading')}</h2>
        <p>
          {emphasise(
            t('legal.privacy.cookiesIntro', { emphasis: t('legal.privacy.localStorageEmphasis') }),
            t('legal.privacy.localStorageEmphasis'),
            (run) => (
              <Emphasis>{run}</Emphasis>
            )
          )}
        </p>
        <ul className={listClasses}>
          <li>{t('legal.privacy.cookiesTokens')}</li>
          <li>{t('legal.privacy.cookiesPreferences')}</li>
        </ul>
        <p>
          {emphasise(
            t('legal.privacy.cookiesNoAdvertising', {
              emphasis: t('legal.privacy.cookielessAnalyticsEmphasis'),
            }),
            t('legal.privacy.cookielessAnalyticsEmphasis'),
            (run) => (
              <Emphasis>{run}</Emphasis>
            )
          )}
        </p>
      </section>

      {/* --- Children --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.childrenHeading')}</h2>
        <p>{t('legal.privacy.childrenBody')}</p>
      </section>

      {/* --- International Transfers --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.transfersHeading')}</h2>
        <p>{t('legal.privacy.transfersBody')}</p>
        <p className={noteClasses}>{t('legal.privacy.transfersLegalBasis')}</p>
      </section>

      {/* --- Updates --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.updatesHeading')}</h2>
        <p>
          {interpolateElements(t('legal.privacy.updatesBody'), {
            lastUpdatedLabel: <em>{t('legal.privacy.updatesLastUpdatedEmphasis')}</em>,
          })}
        </p>
      </section>

      {/* --- Open Source --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.openSourceHeading')}</h2>
        <ul className={listClasses}>
          <li>{t('legal.privacy.openSourceAuditable')}</li>
          <li>{t('legal.privacy.openSourceNoTracking')}</li>
          <li>{t('legal.privacy.openSourceScrutiny')}</li>
        </ul>
        <p>
          {interpolateElements(t('legal.privacy.openSourceRepository'), {
            repository: (
              <Link href={REPOSITORY_URL} target="_blank" rel="noreferrer" className={linkClasses}>
                {t('legal.privacy.repositoryLinkText')}
              </Link>
            ),
          })}
        </p>
      </section>

      {/* --- Platform-Specific --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">
          {t('legal.privacy.platformNotesHeading')}
        </h2>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.twitchSubheading')}</h3>
        <p>{t('legal.privacy.twitchNote')}</p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.youtubeSubheading')}</h3>
        <p>
          {interpolateElements(t('legal.privacy.youtubeNote'), {
            googleSettings: (
              <a
                href={GOOGLE_CONNECTIONS_URL}
                target="_blank"
                rel="noopener noreferrer"
                className={linkClasses}
              >
                {t('legal.privacy.googleSettingsLinkText')}
              </a>
            ),
          })}
        </p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.tiktokSubheading')}</h3>
        <p>{t('legal.privacy.tiktokNote')}</p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.kickSubheading')}</h3>
        <p>{t('legal.privacy.kickNote')}</p>

        <h3 className="text-lg font-semibold text-text">{t('legal.privacy.discordSubheading')}</h3>
        <p>{t('legal.privacy.discordNote')}</p>
      </section>

      {/* --- Contact --- */}
      <section className="space-y-4">
        <h2 className="text-2xl font-semibold text-text">{t('legal.privacy.contactHeading')}</h2>
        <p>
          {emphasise(
            t('legal.privacy.contactEmailRow', { email: supportEmail }),
            supportEmail,
            () => supportLink
          )}
        </p>
        <p className={noteClasses}>
          {emphasise(
            t('legal.privacy.contactHostedNote', { host: t('legal.privacy.hostedDomain') }),
            t('legal.privacy.hostedDomain'),
            (run) => (
              <Emphasis>{run}</Emphasis>
            )
          )}
        </p>
      </section>
    </LegalLayout>
  )
}
