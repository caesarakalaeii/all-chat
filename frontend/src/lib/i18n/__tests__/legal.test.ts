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

/**
 * Copy lock for the consent banner and the legal pages. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 *
 * This namespace is the one where byte-identical transcription matters beyond
 * review hygiene: these are the GDPR disclosures, and a reworded disclosure is
 * a different disclosure.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('cookie banner chrome', () => {
  it('keeps the region and icon labels', () => {
    expect(t('legal.cookieBanner.regionLabel')).toBe('Cookie consent')
    expect(t('legal.cookieBanner.iconLabel')).toBe('Cookie')
  })

  it('keeps the heading and the two lead paragraphs whole', () => {
    expect(t('legal.cookieBanner.title')).toBe('Privacy & Data Storage')
    expect(t('legal.cookieBanner.storageBody')).toBe(
      'All-Chat uses {storage} to save your authentication tokens and keep you logged in. This is essential for the service to function properly.'
    )
    expect(t('legal.cookieBanner.storageEmphasis')).toBe('browser local storage')
    expect(t('legal.cookieBanner.noTrackingBody')).toBe(
      '{emphasis} We do not share your data with third parties for advertising purposes.'
    )
    expect(t('legal.cookieBanner.noTrackingEmphasis')).toBe(
      'We use no tracking cookies, and our usage analytics are cookieless and store no personal data.'
    )
  })

  it('keeps the actions and the footer', () => {
    expect(t('legal.cookieBanner.acknowledge')).toBe('I Understand')
    expect(t('legal.cookieBanner.learnMore')).toBe('Learn More')
    expect(t('legal.cookieBanner.footer')).toBe(
      'Your data is stored locally in your browser and transmitted securely via HTTPS'
    )
  })
})

describe('cookie banner disclosure list', () => {
  it('keeps the summary', () => {
    expect(t('legal.cookieBanner.detailsSummary')).toBe('What data do we store locally?')
  })

  it('keeps each stored-data row whole', () => {
    // The bolded label opens each row and the rest of the row continues the
    // sentence, so the row is one string with a {label} placeholder. Splitting
    // at the hyphen would strand a fragment a translator cannot reorder.
    expect(t('legal.cookieBanner.tokensRow')).toBe(
      '{label} - Required to keep you logged in and authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)'
    )
    expect(t('legal.cookieBanner.tokensLabel')).toBe('Authentication tokens')
    expect(t('legal.cookieBanner.preferencesRow')).toBe(
      '{label} - Your overlay configurations and settings'
    )
    expect(t('legal.cookieBanner.preferencesLabel')).toBe('User preferences')
    expect(t('legal.cookieBanner.noTrackingRow')).toBe(
      "{label} - We don't track your browsing behavior"
    )
    expect(t('legal.cookieBanner.noTrackingLabel')).toBe('No tracking cookies')
    expect(t('legal.cookieBanner.noAdsRow')).toBe(
      "{label} - We don't serve ads or share data with advertisers"
    )
    expect(t('legal.cookieBanner.noAdsLabel')).toBe('No advertising cookies')
    expect(t('legal.cookieBanner.analyticsRow')).toBe(
      '{label} - We measure aggregate usage with self-hosted Umami. It sets no cookies, stores no personal identifier, and does not track public overlays'
    )
    expect(t('legal.cookieBanner.analyticsLabel')).toBe('Cookieless analytics')
  })

  it('keeps the fonts and third-party notes whole', () => {
    expect(t('legal.cookieBanner.fontsNote')).toBe(
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.'
    )
    expect(t('legal.cookieBanner.fontsLabel')).toBe('Fonts:')
    // Four emphasised runs in one sentence, one of them a link, so the sentence
    // stays whole and each run is its own placeholder.
    expect(t('legal.cookieBanner.thirdPartyNote')).toBe(
      '{label} Dashboard pages may load fallback avatars from {avatars} and themes from the {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.'
    )
    expect(t('legal.cookieBanner.thirdPartyLabel')).toBe('Third-party resources:')
    expect(t('legal.cookieBanner.thirdPartyAvatars')).toBe('UI Avatars')
    expect(t('legal.cookieBanner.thirdPartyGithub')).toBe('GitHub API')
  })

  it('keeps the agreement paragraph whole', () => {
    expect(t('legal.cookieBanner.agreement')).toBe(
      'By using All-Chat, you agree to this essential data storage. For more details, please read our {privacy} and {terms}.'
    )
  })

  it('keeps the two legal document names', () => {
    // Referenced from the banner in three places and from the landing footer,
    // but the footer owns its own copies under marketing.footer.* because the
    // banner's are prose links inside a sentence, not nav labels.
    expect(t('legal.cookieBanner.privacyPolicy')).toBe('Privacy Policy')
    expect(t('legal.cookieBanner.termsOfService')).toBe('Terms of Service')
  })
})

describe('legal layout chrome copy', () => {
  it('keeps the eyebrow, last-updated line and copyright', () => {
    expect(t('legal.layout.eyebrow')).toBe('All-Chat Legal')
    // One sentence: 'Last updated:' and the date were JSX siblings.
    expect(t('legal.layout.lastUpdated', { date: '13 July 2026' })).toBe(
      'Last updated: 13 July 2026'
    )
    // The © came from a &copy; entity, and the year and the product name sat
    // either side of it as separate runs. One string with the year as a param.
    expect(t('legal.layout.copyright', { year: 2026 })).toBe('\u00A9 2026 All-Chat')
  })

  it('keeps the four footer links', () => {
    expect(t('legal.layout.homeLink')).toBe('Home')
    expect(t('legal.layout.privacyLink')).toBe('Privacy Policy')
    expect(t('legal.layout.termsLink')).toBe('Terms of Service')
    expect(t('legal.layout.impressumLink')).toBe('Impressum')
  })
})

describe('impressum fallback copy', () => {
  it('keeps the page title and metadata', () => {
    expect(t('legal.impressum.title')).toBe('Impressum')
    expect(t('metadata.impressum.title')).toBe('Impressum | All-Chat')
    expect(t('metadata.impressum.description')).toBe(
      'Legal notice (Impressum) as required by \u00A7 5 DDG.'
    )
  })

  it('keeps the not-configured notice with both paths as params', () => {
    // The Impressum body itself is mounted at runtime and is not in this repo;
    // only this fallback is migratable. The operator hint was five JSX runs
    // around two <code> elements, so it becomes one sentence with the file path
    // and the variable name as params.
    expect(t('legal.impressum.notConfigured')).toBe(
      'The Impressum for this instance has not been configured yet.'
    )
    expect(
      t('legal.impressum.operatorHint', {
        path: '/etc/allchat/impressum.html',
        variable: 'IMPRESSUM_FILE_PATH',
      })
    ).toBe(
      'If you are the operator: mount a ConfigMap containing your Impressum HTML to /etc/allchat/impressum.html or set the IMPRESSUM_FILE_PATH environment variable. See the deployment documentation for details.'
    )
  })
})

describe('terms of service headings', () => {
  it('keeps the page title, the effective date and the metadata', () => {
    expect(t('legal.terms.title')).toBe('Terms of Service (Nutzungsbedingungen)')
    expect(t('legal.terms.lastUpdated')).toBe('July 30, 2026')
    expect(t('metadata.terms.title')).toBe('Terms of Service | All-Chat')
    expect(t('metadata.terms.description')).toBe(
      'Understand the rules and responsibilities for using All-Chat.'
    )
  })

  it('keeps all sixteen section headings, ampersands included', () => {
    expect(t('legal.terms.acceptanceHeading')).toBe('1. Acceptance of Terms')
    expect(t('legal.terms.descriptionHeading')).toBe('2. Description of Service')
    expect(t('legal.terms.accountsHeading')).toBe('3. Accounts & Authentication')
    expect(t('legal.terms.acceptableUseHeading')).toBe('4. Acceptable Use')
    expect(t('legal.terms.thirdPartyHeading')).toBe('5. Third-Party Integrations')
    expect(t('legal.terms.privacyHeading')).toBe('6. Privacy')
    expect(t('legal.terms.licenseHeading')).toBe('7. Open Source License')
    expect(t('legal.terms.availabilityHeading')).toBe('8. Availability & Support')
    expect(t('legal.terms.liabilityHeading')).toBe('9. Limitation of Liability')
    expect(t('legal.terms.indemnityHeading')).toBe('10. Indemnification')
    expect(t('legal.terms.premiumHeading')).toBe(
      '11. Premium Subscriptions & Right of Withdrawal (Widerrufsrecht)'
    )
    expect(t('legal.terms.changesHeading')).toBe('12. Changes to Terms')
    expect(t('legal.terms.terminationHeading')).toBe('13. Termination')
    expect(t('legal.terms.governingLawHeading')).toBe('14. Governing Law & Jurisdiction')
    expect(t('legal.terms.legalNoticeHeading')).toBe('15. Legal Notice')
    expect(t('legal.terms.contactHeading')).toBe('16. Contact')
  })
})

describe('terms of service body copy', () => {
  it('keeps each sentence containing a link whole, with the link text as a param', () => {
    // These sentences wrap a <Link> or an <a> around a run in the middle. The
    // catalog keeps the whole sentence with a placeholder and a sibling key for
    // the link text, so a translator can move the link within the sentence.
    expect(t('legal.terms.acceptanceBody', { privacy: 'Privacy Policy' })).toBe(
      'By accessing or using All-Chat you agree to these Terms of Service and our Privacy Policy. If you disagree with any part, you should discontinue use immediately.'
    )
    expect(t('legal.terms.privacyLinkText')).toBe('Privacy Policy')
    expect(t('legal.terms.legalNoticeBody', { impressum: 'Impressum' })).toBe(
      "The operator's identity and contact details are available in the Impressum."
    )
    expect(t('legal.terms.impressumLinkText')).toBe('Impressum')
    expect(t('legal.terms.contactBody', { email: 'all.chat.support@gmail.com' })).toBe(
      'Questions? Reach us at all.chat.support@gmail.com. Hosted community forks should contact their own administrators.'
    )
    expect(t('legal.terms.supportEmail')).toBe('all.chat.support@gmail.com')
  })

  it('keeps section 2 and the acceptable-use list', () => {
    expect(t('legal.terms.descriptionBody')).toBe(
      'All-Chat aggregates real-time chat from Twitch, YouTube, Kick, TikTok, and Discord into a single overlay so you can display cross-platform conversation on your stream. You can customize overlays, connect sources, and broadcast them via OBS or browser sources.'
    )
    expect(t('legal.terms.acceptableUseIntro')).toBe(
      'You agree not to misuse the Service, including but not limited to:'
    )
    expect(t('legal.terms.acceptableUseLaws')).toBe(
      'Breaking local, national, or international laws'
    )
    expect(t('legal.terms.acceptableUseIp')).toBe(
      'Infringing intellectual property or privacy rights of others'
    )
    expect(t('legal.terms.acceptableUseMalware')).toBe('Uploading malware, spam, or malicious code')
    expect(t('legal.terms.acceptableUseBypass')).toBe(
      'Attempting to bypass authentication, rate limits, or security controls'
    )
    expect(t('legal.terms.acceptableUseHarassment')).toBe('Harassing or abusing other users')
    expect(t('legal.terms.acceptableUsePartner')).toBe(
      'Using All-Chat in a way that violates partner platform policies'
    )
  })

  it('keeps the account responsibilities list, including the emailed row', () => {
    expect(t('legal.terms.accountsIntro')).toBe(
      'You are responsible for all activity that happens under your account. You agree to:'
    )
    expect(t('legal.terms.accountsAccurate')).toBe('Provide accurate registration details')
    expect(t('legal.terms.accountsSecurity')).toBe(
      'Maintain the security of your credentials and OAuth grants'
    )
    expect(t('legal.terms.accountsNotify', { email: 'all.chat.support@gmail.com' })).toBe(
      'Notify us at all.chat.support@gmail.com if you suspect compromise'
    )
    expect(t('legal.terms.accountsComply')).toBe(
      'Comply with the terms of Twitch, YouTube, TikTok, Kick, Discord, and any other connected platform'
    )
  })

  it('keeps the third-party section and the YouTube binding notice', () => {
    expect(t('legal.terms.thirdPartyIntro')).toBe(
      'All-Chat relies on third-party APIs. Their availability, scopes, and rate limits may change.'
    )
    expect(t('legal.terms.thirdPartyComply')).toBe(
      "You must comply with each platform's terms of service"
    )
    expect(t('legal.terms.thirdPartyOutages')).toBe(
      'We are not accountable for outages or policy shifts by those platforms'
    )
    expect(t('legal.terms.thirdPartyQuotas')).toBe(
      'Platform-specific quotas can impact overlay functionality'
    )
    expect(t('legal.terms.youtubeBinding', { youtubeTerms: 'YouTube Terms of Service' })).toBe(
      'YouTube Integration: By using All-Chat to connect to YouTube, you agree to be bound by the YouTube Terms of Service.'
    )
    expect(t('legal.terms.youtubeTermsLinkText')).toBe('YouTube Terms of Service')
  })

  it('keeps the privacy section with its link, its emphasis and its nbsp', () => {
    // Three runs in one sentence: a <Link>, a <strong> and a section reference
    // whose space is a U+00A0 no-break space from an &nbsp; entity.
    expect(
      t('legal.terms.privacyBody', {
        privacy: 'Privacy Policy',
        analytics: 'self-hosted, cookieless analytics',
      })
    ).toBe(
      'Your use of All-Chat is also governed by our Privacy Policy, which explains what we collect, how it is used, and your rights under the DSGVO. For transparency: All-Chat measures aggregate usage with self-hosted, cookieless analytics (Umami) that set nothing on your device and store no personal identifier \u2013 see Section\u00A05.6 of the Privacy Policy.'
    )
    expect(t('legal.terms.privacyAnalyticsEmphasis')).toBe('self-hosted, cookieless analytics')
  })

  it('keeps both license paragraphs', () => {
    // Composed with the link text the render site actually passes, so this pins
    // the sentence as rendered rather than a value nothing produces: the <a> in
    // the source wrapped the "(AGPL-3.0)" suffix too.
    expect(t('legal.terms.licenseBody', { license: t('legal.terms.licenseLinkText') })).toBe(
      'All-Chat is released under the GNU Affero General Public License v3.0 (AGPL-3.0). That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.'
    )
    expect(t('legal.terms.licenseLinkText')).toBe(
      'GNU Affero General Public License v3.0 (AGPL-3.0)'
    )
    expect(t('legal.terms.licenseRepository', { github: 'GitHub' })).toBe(
      'The canonical repository is available on GitHub.'
    )
    expect(t('legal.terms.githubLinkText')).toBe('GitHub')
  })

  it('keeps the availability section and its host emphasis', () => {
    expect(t('legal.terms.availabilityIntro')).toBe('We aim for high uptime but do not guarantee:')
    expect(t('legal.terms.availabilityUptime')).toBe('Uninterrupted access or zero bugs')
    expect(t('legal.terms.availabilityCompat')).toBe(
      'Compatibility with every browser or streamer setup'
    )
    expect(t('legal.terms.availabilityFixes')).toBe('Immediate fixes or feature requests')
    expect(t('legal.terms.availabilitySupport', { host: 'allch.at' })).toBe(
      'Support for the hosted service at allch.at is best-effort. Community/self-hosted deployments must rely on their own maintainers or the open source community for assistance.'
    )
    expect(t('legal.terms.hostedDomain')).toBe('allch.at')
  })

  it('keeps all three liability paragraphs, including the German umlaut', () => {
    expect(t('legal.terms.liabilityGross')).toBe(
      'We are liable without limitation for damages caused intentionally or by gross negligence, for injury to life, body, or health, and under the German Product Liability Act (Produkthaftungsgesetz).'
    )
    expect(t('legal.terms.liabilitySlight')).toBe(
      'In cases of slight negligence, we are liable only for the breach of essential contractual obligations (Kardinalpflichten): obligations whose fulfilment makes the proper performance of the contract possible in the first place and on whose fulfilment you may regularly rely. In such cases, our liability is limited to the damage that is foreseeable and typical for this type of service. Any further liability is excluded.'
    )
    // &uuml; in the source: the u-umlaut must survive transcription.
    expect(t('legal.terms.liabilityAgents')).toBe(
      'These limitations also apply in favour of our legal representatives and vicarious agents (Erf\u00FCllungsgehilfen).'
    )
  })

  it('keeps the indemnity, premium, changes, termination and jurisdiction paragraphs', () => {
    expect(t('legal.terms.indemnityBody')).toBe(
      'You agree to indemnify All-Chat against third-party claims, including the reasonable costs of legal defence, arising from your culpable violation of these Terms, applicable law, or the rights of others. This does not apply to the extent you are not responsible for the violation.'
    )
    expect(t('legal.terms.premiumBody', { patreon: 'Patreon' })).toBe(
      "The core All-Chat service is free of charge. Premium features are unlocked through a paid membership on Patreon: you subscribe to our campaign on patreon.com and then connect your Patreon account to All-Chat. The subscription contract, billing, cancellation, and any statutory right of withdrawal for the paid membership are handled by Patreon under Patreon's own terms. All-Chat itself does not charge you and does not process payments."
    )
    expect(t('legal.terms.premiumCancellation')).toBe(
      'You may stop using All-Chat and delete your account at any time via the Settings page. Upon deletion, your personal data is removed as described in our Privacy Policy. Note that deleting your All-Chat account does not cancel a Patreon membership; cancel it directly on Patreon.'
    )
    expect(t('legal.terms.changesBody')).toBe(
      'We may update these Terms over time. Material changes will be announced in the dashboard, and the new version will be posted here with an updated effective date.'
    )
    expect(t('legal.terms.terminationBody')).toBe(
      'We reserve the right to suspend or terminate your access for any breach of these Terms or abusive behavior. You may stop using the Service at any time by deleting your account in the Settings page.'
    )
    expect(t('legal.terms.governingLawBody')).toBe(
      'These Terms are governed by the laws of the Federal Republic of Germany, excluding the UN Convention on Contracts for the International Sale of Goods (CISG). If you are a consumer, the mandatory consumer protection provisions of the country in which you habitually reside remain unaffected. If you are a merchant (Kaufmann), a legal entity under public law, or a special fund under public law, the exclusive place of jurisdiction is the domicile of the operator.'
    )
  })
})

describe('terms of service link text', () => {
  it('keeps the Patreon link text in the terms namespace, not common', () => {
    // common.patreon.heading is a section heading on a different surface. This
    // is a run inside a legal sentence, so it stays with the sentence.
    expect(t('legal.terms.patreonLinkText')).toBe('Patreon')
  })
})
