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

describe('privacy policy headings', () => {
  it('keeps the page title, the effective date and the metadata', () => {
    // &auml; in the source title.
    expect(t('legal.privacy.title')).toBe('Privacy Policy (Datenschutzerkl\u00E4rung)')
    expect(t('legal.privacy.lastUpdated')).toBe('July 30, 2026')
    expect(t('metadata.privacy.title')).toBe('Privacy Policy | All-Chat')
    expect(t('metadata.privacy.description')).toBe(
      'Learn how All-Chat collects, processes, and protects your information.'
    )
  })

  it('keeps all fourteen numbered section headings', () => {
    // Every DSGVO article reference is separated by a U+00A0 from an &nbsp;.
    expect(t('legal.privacy.controllerHeading')).toBe(
      '1. Controller (Verantwortlicher, Art.\u00A04 Nr.\u00A07 DSGVO)'
    )
    expect(t('legal.privacy.collectHeading')).toBe('2. Information We Collect')
    expect(t('legal.privacy.useHeading')).toBe('3. How We Use Your Information')
    expect(t('legal.privacy.storageHeading')).toBe('4. Data Storage & Security')
    expect(t('legal.privacy.sharingHeading')).toBe('5. Data Sharing & Third Parties')
    expect(t('legal.privacy.retentionHeading')).toBe('6. Data Retention')
    expect(t('legal.privacy.rightsHeading')).toBe('7. Your Rights (Art.\u00A015\u201321 DSGVO)')
    expect(t('legal.privacy.cookiesHeading')).toBe('8. Cookies, Browser Storage & Analytics')
    expect(t('legal.privacy.childrenHeading')).toBe("9. Children's Privacy")
    expect(t('legal.privacy.transfersHeading')).toBe('10. International Data Transfers')
    expect(t('legal.privacy.updatesHeading')).toBe('11. Updates')
    expect(t('legal.privacy.openSourceHeading')).toBe('12. Transparency & Open Source')
    expect(t('legal.privacy.platformNotesHeading')).toBe('13. Platform-Specific Notes')
    expect(t('legal.privacy.contactHeading')).toBe('14. Contact')
  })

  it('keeps every subsection heading', () => {
    expect(t('legal.privacy.authSubheading')).toBe('2.1 Authentication Information')
    expect(t('legal.privacy.chatSubheading')).toBe('2.2 Chat Data')
    expect(t('legal.privacy.overlaySubheading')).toBe('2.3 Overlay Configuration')
    expect(t('legal.privacy.viewerIdentitySubheading')).toBe('2.4 Cross-Platform Viewer Identity')
    expect(t('legal.privacy.logDataSubheading')).toBe('2.5 Usage & Log Data')
    expect(t('legal.privacy.patreonSubheading')).toBe('2.6 Premium Membership (Patreon)')
    expect(t('legal.privacy.storageLocationsSubheading')).toBe('4.1 Storage Locations')
    expect(t('legal.privacy.safeguardsSubheading')).toBe('4.2 Safeguards')
    expect(t('legal.privacy.platformApisSubheading')).toBe('5.1 Streaming Platform APIs')
    expect(t('legal.privacy.fontsSubheading')).toBe('5.2 Fonts (Self-Hosted)')
    expect(t('legal.privacy.frontendResourcesSubheading')).toBe(
      '5.3 Third-Party Frontend Resources'
    )
    expect(t('legal.privacy.youtubeNoticeSubheading')).toBe('5.4 YouTube-Specific Notice')
    expect(t('legal.privacy.noSalesSubheading')).toBe('5.5 No Data Sales')
    expect(t('legal.privacy.analyticsSubheading')).toBe(
      '5.6 Usage Analytics (Self-Hosted, Cookieless)'
    )
    expect(t('legal.privacy.twitchSubheading')).toBe('Twitch')
    expect(t('legal.privacy.youtubeSubheading')).toBe('YouTube')
    expect(t('legal.privacy.tiktokSubheading')).toBe('TikTok')
    expect(t('legal.privacy.kickSubheading')).toBe('Kick')
    expect(t('legal.privacy.discordSubheading')).toBe('Discord')
  })
})

describe('privacy policy summary callouts', () => {
  it('keeps the TL;DR box whole', () => {
    expect(t('legal.privacy.tldr', { label: 'TL;DR:' })).toBe(
      'TL;DR: All-Chat only collects the information we need to authenticate with your streaming platforms and render chat in your overlays. Tokens are encrypted, chat messages are automatically deleted after one hour, and we never sell your data. We use cookieless, self-hosted analytics (no tracking cookies; see Section\u00A05.6).'
    )
    expect(t('legal.privacy.tldrLabel')).toBe('TL;DR:')
  })

  it('keeps the open source box, both em dashes included', () => {
    expect(
      t('legal.privacy.openSourceCallout', { label: 'Open Source Transparency:', github: 'GitHub' })
    ).toBe(
      'Open Source Transparency: All-Chat is licensed under AGPL-3.0. Review the entire codebase\u2014including how we store and process your data\u2014on GitHub.'
    )
    expect(t('legal.privacy.openSourceCalloutLabel')).toBe('Open Source Transparency:')
    expect(t('legal.privacy.githubLinkText')).toBe('GitHub')
  })
})

describe('privacy policy legal bases', () => {
  it('keeps each legal-basis line whole, article reference included', () => {
    // These are the statutory grounds for each kind of processing. Each is one
    // key: the article number and the reason it applies are one statement.
    expect(t('legal.privacy.authLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO \u2013 performance of a contract (providing the service you signed up for).'
    )
    expect(t('legal.privacy.chatLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO (service delivery) and Art.\u00A06(1)(f) DSGVO (legitimate interest in abuse prevention).'
    )
    expect(t('legal.privacy.overlayLegalBasis')).toBe('Legal basis: Art.\u00A06(1)(b) DSGVO.')
    expect(t('legal.privacy.viewerIdentityLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(a) DSGVO \u2013 your consent. You can unlink platforms at any time from the viewer settings.'
    )
    expect(t('legal.privacy.logDataLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO \u2013 legitimate interest in maintaining service security and availability.'
    )
    expect(t('legal.privacy.fontsLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO \u2013 legitimate interest in delivering the visual appearance of the overlay. The font files themselves are licensed under the SIL Open Font License 1.1 or Apache 2.0.'
    )
    expect(t('legal.privacy.frontendResourcesLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO \u2013 legitimate interest in providing a functional and visually complete overlay experience. You can avoid loading these by not opening the theme marketplace. Fallback avatars (shown when a platform avatar is unavailable) are generated locally in your browser and involve no external request.'
    )
    expect(t('legal.privacy.patreonLegalBasis', { patreonPolicy: 'Patreon Privacy Policy' })).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO \u2013 providing the premium features you subscribed to. For the data you provide on patreon.com itself, Patreon, Inc. (US) is an independent controller; see the Patreon Privacy Policy.'
    )
    expect(t('legal.privacy.patreonPolicyLinkText')).toBe('Patreon Privacy Policy')
  })
})

describe('privacy policy collection sections', () => {
  it('keeps section 1 with both links', () => {
    expect(
      t('legal.privacy.controllerBody', {
        impressum: 'Impressum',
        email: 'all.chat.support@gmail.com',
      })
    ).toBe(
      'The person responsible for data processing on this website is listed in the Impressum. For privacy-related inquiries contact all.chat.support@gmail.com.'
    )
    expect(t('legal.privacy.impressumLinkText')).toBe('Impressum')
    expect(t('legal.privacy.supportEmail')).toBe('all.chat.support@gmail.com')
  })

  it('keeps the authentication data list', () => {
    expect(t('legal.privacy.authIntro')).toBe(
      'When you connect Twitch, YouTube, TikTok, Kick, or Discord we store the minimum data required to create overlays and reconnect later:'
    )
    expect(t('legal.privacy.authIdentifiers')).toBe(
      'Platform identifiers (user ID, username, display name)'
    )
    expect(t('legal.privacy.authProfileImages')).toBe('Profile images provided by the platform')
    expect(t('legal.privacy.authTokens')).toBe('Encrypted OAuth access and refresh tokens')
    expect(t('legal.privacy.authScopes')).toBe('Token scopes and expiration dates')
  })

  it('keeps the chat data list and its retention sentence', () => {
    expect(t('legal.privacy.chatIntro')).toBe('For active overlays we temporarily process:')
    expect(t('legal.privacy.chatMessages')).toBe('Messages flowing through connected channels')
    expect(t('legal.privacy.chatMetadata')).toBe(
      'Message metadata (timestamps, emotes, badges, highlights)'
    )
    expect(t('legal.privacy.chatAuthor')).toBe(
      'Per-message author details (display name, color, avatar)'
    )
    expect(
      t('legal.privacy.chatRetention', { emphasis: 'automatically deleted after one hour' })
    ).toBe(
      'Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and automatically deleted after one hour. Messages displayed in the overlay are streamed through memory and are not persisted once the overlay session ends.'
    )
    expect(t('legal.privacy.chatRetentionEmphasis')).toBe('automatically deleted after one hour')
  })

  it('keeps the overlay configuration list', () => {
    expect(t('legal.privacy.overlayIntro')).toBe('To render and sync overlays we store:')
    expect(t('legal.privacy.overlayNames')).toBe(
      'Overlay names, IDs, and custom CSS or theme settings'
    )
    expect(t('legal.privacy.overlaySources')).toBe(
      'Connected chat sources (platform, channel ID, channel name)'
    )
    expect(t('legal.privacy.overlayFilters')).toBe(
      'Filter settings (blocked words, user-level rules)'
    )
  })

  it('keeps the viewer identity paragraph and its opt-in emphasis', () => {
    // e.g.&nbsp;Twitch: the no-break space after "e.g." is in the rendered text.
    expect(
      t('legal.privacy.viewerIdentityBody', {
        emphasis: 'opt-in only and requires your explicit action',
      })
    ).toBe(
      'If you choose to link multiple platform accounts as a viewer (e.g.\u00A0Twitch + YouTube), we create a unified viewer profile that associates your platform identities. This is opt-in only and requires your explicit action.'
    )
    expect(t('legal.privacy.viewerIdentityEmphasis')).toBe(
      'opt-in only and requires your explicit action'
    )
  })

  it('keeps the log data list', () => {
    expect(t('legal.privacy.logDataIntro')).toBe('For observability and abuse prevention we log:')
    expect(t('legal.privacy.logDataIp')).toBe(
      'Anonymised IP address (last octet zeroed), browser user agent, and basic request info'
    )
    expect(t('legal.privacy.logDataMetrics')).toBe(
      'API usage metrics, cache hits, and connection status'
    )
    expect(t('legal.privacy.logDataTraces')).toBe(
      'Error traces and diagnostic logs (retained up to 90 days)'
    )
  })

  it('keeps the Patreon section', () => {
    expect(t('legal.privacy.patreonIntro')).toBe(
      'Premium features are unlocked through a paid membership on Patreon. If you connect your Patreon account to All-Chat, we store:'
    )
    expect(t('legal.privacy.patreonUserId')).toBe('Your Patreon user ID')
    expect(t('legal.privacy.patreonTokens')).toBe(
      'Encrypted OAuth access and refresh tokens for the Patreon API'
    )
    expect(t('legal.privacy.patreonMembership')).toBe(
      'Your membership state for our campaign (status, tier, pledge amount, renewal date)'
    )
    expect(t('legal.privacy.patreonNoPaymentData', { not: 'not' })).toBe(
      'We do not receive or store your payment details; payments are processed entirely by Patreon. Patreon notifies us via webhooks when your membership changes, and a periodic reconciliation keeps the state current. Disconnecting Patreon in the Settings page deletes the stored tokens and revokes subscription-derived premium.'
    )
    expect(t('legal.privacy.notEmphasis')).toBe('not')
  })
})

describe('privacy policy usage, storage and sharing', () => {
  it('keeps the how-we-use list', () => {
    expect(t('legal.privacy.useIntro')).toBe(
      'Everything we store directly supports the core overlay experience:'
    )
    expect(t('legal.privacy.useAuthenticate')).toBe(
      'Authenticate against the platforms you authorize'
    )
    expect(t('legal.privacy.useFetch')).toBe(
      'Fetch live chat messages and normalize them into a single feed'
    )
    expect(t('legal.privacy.useRender')).toBe(
      'Render overlays, perform emote lookups, and respect your filters'
    )
    expect(t('legal.privacy.useMonitor')).toBe('Monitor service reliability and debug incidents')
    expect(t('legal.privacy.useAbuse')).toBe('Detect and prevent abuse (rate-limiting, bans)')
  })

  it('keeps both storage lists and the no-system-is-secure note', () => {
    expect(t('legal.privacy.storagePostgres')).toBe(
      'PostgreSQL for account data, overlays, and encrypted OAuth tokens'
    )
    expect(t('legal.privacy.storageRedis')).toBe(
      'Redis for ephemeral sessions, message fan-out, and rate limiting'
    )
    expect(t('legal.privacy.storageLocation', { country: 'Germany' })).toBe(
      'All of the above runs on our own infrastructure on servers located in Germany (hosting provider: Hetzner Online GmbH)'
    )
    expect(t('legal.privacy.storageCountry')).toBe('Germany')
    expect(t('legal.privacy.safeguardsEncryption')).toBe(
      'OAuth tokens encrypted with AES-GCM before touching the database'
    )
    expect(t('legal.privacy.safeguardsHttps')).toBe(
      'HTTPS at the ingress layer for all external traffic; internal services isolated via Kubernetes network policies'
    )
    expect(t('legal.privacy.safeguardsAccess')).toBe(
      'Role-scoped infrastructure access and audit logging'
    )
    expect(t('legal.privacy.safeguardsPatching')).toBe(
      'Regular dependency upgrades and security patching'
    )
    expect(t('legal.privacy.safeguardsHeaders')).toBe(
      'Security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy)'
    )
    expect(t('legal.privacy.safeguardsCaveat')).toBe(
      'No storage system is perfectly secure, but we follow industry best practices to keep your tokens and overlays safe.'
    )
  })

  it('keeps the platform API list and its scopes note', () => {
    expect(t('legal.privacy.platformApisIntro')).toBe(
      'We connect to the following services to deliver the core product:'
    )
    expect(t('legal.privacy.platformApisTwitch')).toBe('Twitch EventSub and Helix APIs')
    expect(t('legal.privacy.platformApisYoutube')).toBe('YouTube Live Chat and OAuth APIs')
    expect(t('legal.privacy.platformApisTiktokKick')).toBe('TikTok Live APIs and Kick APIs')
    expect(t('legal.privacy.platformApisDiscord')).toBe('Discord Gateway API')
    expect(t('legal.privacy.platformApisEmotes')).toBe('7TV, BTTV, FFZ for emote metadata')
    expect(t('legal.privacy.platformApisScopes')).toBe(
      "Every integration remains subject to the platform's own policies and scopes you approve."
    )
  })

  it('keeps the self-hosted fonts paragraph and its Munich ruling reference', () => {
    // Three runs plus a <code> path in one sentence, and &uuml; in Muenchen.
    expect(
      t('legal.privacy.fontsBody', {
        selfHosted: 'self-hosted on our infrastructure',
        proxyPath: '/font-proxy/*',
        noTransmission: 'no IP address, user agent, or request metadata is transmitted to Google',
      })
    ).toBe(
      'Typography assets originally distributed by Google Fonts are self-hosted on our infrastructure. The fonts used by the All-Chat interface are bundled at build time via Next.js, and fonts selectable for overlay customization are served through a server-side proxy at /font-proxy/*. Your browser only connects to the All-Chat origin; no IP address, user agent, or request metadata is transmitted to Google when fonts are loaded. This aligns with the Landgericht M\u00FCnchen I ruling on Google Fonts (20 January 2022, Az. 3 O 17493/20).'
    )
    expect(t('legal.privacy.fontsSelfHostedEmphasis')).toBe('self-hosted on our infrastructure')
    expect(t('legal.privacy.fontsNoTransmissionEmphasis')).toBe(
      'no IP address, user agent, or request metadata is transmitted to Google'
    )
  })

  it('keeps the third-party frontend resource disclosure', () => {
    expect(t('legal.privacy.frontendResourcesIntro', { emphasis: 'IP address' })).toBe(
      'Overlay and dashboard pages may load the following external resources. Each request transmits your IP address and browser user agent to the respective provider:'
    )
    expect(t('legal.privacy.ipAddressEmphasis')).toBe('IP address')
    expect(t('legal.privacy.frontendResourcesGithub', { label: 'GitHub API' })).toBe(
      'GitHub API (api.github.com) \u2013 fetches community themes from our public repository in the theme marketplace'
    )
    expect(t('legal.privacy.githubApiLabel')).toBe('GitHub API')
  })

  it('keeps the YouTube-specific notice and the no-data-sales note', () => {
    expect(
      t('legal.privacy.youtubeNoticeBody', { googlePolicy: 'Google Privacy Policy', not: 'not' })
    ).toBe(
      "Your use of All-Chat's YouTube integration is also governed by the Google Privacy Policy. This applies only to the YouTube API integration (data flows described in Section 5.1); it does not apply to fonts, which are self-hosted as described in Section 5.2."
    )
    expect(t('legal.privacy.googlePolicyLinkText')).toBe('Google Privacy Policy')
    expect(t('legal.privacy.noSalesBody')).toBe(
      'We never sell or rent your data. We may disclose information when required by law or to respond to legitimate security incidents.'
    )
  })

  it('keeps the whole cookieless analytics disclosure', () => {
    expect(
      t('legal.privacy.analyticsBody', {
        umami: 'Umami',
        selfHost: 'host ourselves',
        cookieless: 'cookieless',
        notShared: 'not shared with any third party',
      })
    ).toBe(
      'To understand how the site is used and where to improve it, we run Umami, an open-source analytics tool that we host ourselves. It is cookieless: it sets no cookies, creates no persistent identifier, and performs no cross-site or cross-device tracking. The data is processed on our own infrastructure and is not shared with any third party (unlike Google Analytics or similar services).'
    )
    expect(t('legal.privacy.umamiLinkText')).toBe('Umami')
    expect(t('legal.privacy.analyticsSelfHostEmphasis')).toBe('host ourselves')
    expect(t('legal.privacy.analyticsCookielessEmphasis')).toBe('cookieless')
    expect(t('legal.privacy.analyticsNotSharedEmphasis')).toBe('not shared with any third party')
    expect(t('legal.privacy.analyticsRecordsIntro')).toBe(
      'For each page view it records aggregate, non-identifying information:'
    )
    expect(t('legal.privacy.analyticsRecordsPage')).toBe(
      'The page you visited and the referring URL'
    )
    expect(t('legal.privacy.analyticsRecordsBrowser')).toBe(
      'Browser, operating system, device type, and screen size'
    )
    expect(t('legal.privacy.analyticsRecordsCountry')).toBe(
      'Approximate country, derived from your IP address at request time'
    )
    expect(
      t('legal.privacy.analyticsIpNote', { ipNotStored: 'IP address is not stored', not: 'not' })
    ).toBe(
      'Your IP address is not stored: it is used only momentarily to derive the country and to generate a daily, salted hash for counting unique visits, after which it is discarded. We do not track public overlay views (the pages OBS loads as a browser source). You can block the analytics script with any browser content blocker without affecting the site.'
    )
    expect(t('legal.privacy.analyticsIpNotStoredEmphasis')).toBe('IP address is not stored')
    // &sect; and the &nbsp; after it are both part of the rendered text.
    expect(t('legal.privacy.analyticsConsentNote')).toBe(
      'Because the tracker stores no information on, and reads none from, your device, it does not require consent under \u00A7\u00A025 TDDDG; the processing of the resulting data rests on Art.\u00A06(1)(f) DSGVO \u2013 our legitimate interest in measuring and improving the service.'
    )
  })
})

describe('privacy policy retention, rights and the rest', () => {
  it('keeps each retention row with its bolded label inside the sentence', () => {
    expect(t('legal.privacy.retentionAccount', { label: 'Account & overlay data:' })).toBe(
      'Account & overlay data: kept until you delete your account'
    )
    expect(t('legal.privacy.retentionAccountLabel')).toBe('Account & overlay data:')
    expect(t('legal.privacy.retentionTokens', { label: 'OAuth tokens:' })).toBe(
      'OAuth tokens: deleted when you disconnect a platform, when they expire (cleaned up after 7 days), or when you delete your account'
    )
    expect(t('legal.privacy.retentionTokensLabel')).toBe('OAuth tokens:')
    expect(
      t('legal.privacy.retentionSentMessages', { label: 'Chat messages sent through All-Chat:' })
    ).toBe('Chat messages sent through All-Chat: automatically deleted after 1 hour')
    expect(t('legal.privacy.retentionSentMessagesLabel')).toBe(
      'Chat messages sent through All-Chat:'
    )
    expect(
      t('legal.privacy.retentionDisplayedMessages', {
        label: 'Chat messages displayed in overlays:',
      })
    ).toBe('Chat messages displayed in overlays: streamed through memory only; not persisted')
    expect(t('legal.privacy.retentionDisplayedMessagesLabel')).toBe(
      'Chat messages displayed in overlays:'
    )
    expect(t('legal.privacy.retentionLogs', { label: 'Usage logs:' })).toBe(
      'Usage logs: retained for up to 90 days'
    )
    expect(t('legal.privacy.retentionLogsLabel')).toBe('Usage logs:')
    expect(t('legal.privacy.retentionQuotaLogs', { label: 'YouTube quota audit logs:' })).toBe(
      'YouTube quota audit logs: retained for 30 days'
    )
    expect(t('legal.privacy.retentionQuotaLogsLabel')).toBe('YouTube quota audit logs:')
  })

  it('keeps each data-subject right with its article reference', () => {
    expect(t('legal.privacy.rightsIntro')).toBe('You can exercise the following at any time:')
    expect(t('legal.privacy.rightsAccess', { label: 'Access (Art.\u00A015):' })).toBe(
      'Access (Art.\u00A015): Request a copy of the data we store about you'
    )
    expect(t('legal.privacy.rightsAccessLabel')).toBe('Access (Art.\u00A015):')
    expect(t('legal.privacy.rightsRectification', { label: 'Rectification (Art.\u00A016):' })).toBe(
      'Rectification (Art.\u00A016): Correct inaccurate information'
    )
    expect(t('legal.privacy.rightsRectificationLabel')).toBe('Rectification (Art.\u00A016):')
    expect(t('legal.privacy.rightsErasure', { label: 'Erasure (Art.\u00A017):' })).toBe(
      'Erasure (Art.\u00A017): Delete your account and all associated data from the Settings page or by contacting us'
    )
    expect(t('legal.privacy.rightsErasureLabel')).toBe('Erasure (Art.\u00A017):')
    expect(t('legal.privacy.rightsRestriction', { label: 'Restriction (Art.\u00A018):' })).toBe(
      'Restriction (Art.\u00A018): Request restriction of processing in certain circumstances'
    )
    expect(t('legal.privacy.rightsRestrictionLabel')).toBe('Restriction (Art.\u00A018):')
    expect(
      t('legal.privacy.rightsPortability', { label: 'Data portability (Art.\u00A020):' })
    ).toBe(
      'Data portability (Art.\u00A020): Export your data in a machine-readable format via the Settings page'
    )
    expect(t('legal.privacy.rightsPortabilityLabel')).toBe('Data portability (Art.\u00A020):')
    expect(t('legal.privacy.rightsObjection', { label: 'Objection (Art.\u00A021):' })).toBe(
      'Objection (Art.\u00A021): Object to processing based on legitimate interest'
    )
    expect(t('legal.privacy.rightsObjectionLabel')).toBe('Objection (Art.\u00A021):')
    expect(t('legal.privacy.rightsWithdraw', { label: 'Withdraw consent:' })).toBe(
      'Withdraw consent: Disconnect platforms or unlink viewer identities at any time'
    )
    expect(t('legal.privacy.rightsWithdrawLabel')).toBe('Withdraw consent:')
  })

  it('keeps the rights contact, the no-profiling note and the YouTube revocation', () => {
    // &ouml; in Aufsichtsbehoerde.
    expect(t('legal.privacy.rightsContact', { email: 'all.chat.support@gmail.com' })).toBe(
      'Contact us at all.chat.support@gmail.com or use the Settings page. You also have the right to lodge a complaint with your supervisory authority (Aufsichtsbeh\u00F6rde).'
    )
    expect(t('legal.privacy.noProfiling')).toBe(
      'We do not use automated decision-making or profiling within the meaning of Art.\u00A022 DSGVO.'
    )
    expect(
      t('legal.privacy.youtubeRevoke', { googleSettings: 'Google security settings page' })
    ).toBe(
      "For YouTube Data: You can revoke All-Chat's access to your YouTube data via the Google security settings page."
    )
    expect(t('legal.privacy.googleSettingsPageLinkText')).toBe('Google security settings page')
  })

  it('keeps the browser storage section', () => {
    expect(t('legal.privacy.cookiesIntro', { emphasis: 'browser localStorage' })).toBe(
      'All-Chat uses browser localStorage (not cookies) for essential functionality:'
    )
    expect(t('legal.privacy.localStorageEmphasis')).toBe('browser localStorage')
    expect(t('legal.privacy.cookiesTokens')).toBe(
      'Authentication tokens (JWT) to keep you logged in'
    )
    expect(t('legal.privacy.cookiesPreferences')).toBe('User preferences and last-visited state')
    expect(
      t('legal.privacy.cookiesNoAdvertising', {
        emphasis: 'privacy-friendly, cookieless usage analytics',
      })
    ).toBe(
      'We do not use advertising cookies or cross-site tracking. We do use privacy-friendly, cookieless usage analytics (self-hosted Umami) \u2013 it sets nothing on your device and stores no personal identifier; see Section\u00A05.6 for the full description. Fonts are self-hosted (Section\u00A05.2) and do not set cookies. The GitHub API (Section\u00A05.3) may cause GitHub to set its own cookies when the theme marketplace is loaded.'
    )
    expect(t('legal.privacy.cookielessAnalyticsEmphasis')).toBe(
      'privacy-friendly, cookieless usage analytics'
    )
  })

  it('keeps the children, transfers, updates and open source sections', () => {
    expect(t('legal.privacy.childrenBody')).toBe(
      'All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data processing in Germany). If we discover data belonging to a minor we will delete it immediately.'
    )
    expect(t('legal.privacy.transfersBody')).toBe(
      'All-Chat itself is hosted on servers located in Germany (see Section\u00A04.1). However, when you use streaming platform integrations, data is transferred to servers operated by Twitch (Amazon, US), Google/YouTube (US), TikTok (various), and Kick (AU). If you connect a Patreon membership, data is exchanged with Patreon, Inc. (US). The GitHub API (theme marketplace) may also involve transfers to the US.'
    )
    expect(t('legal.privacy.transfersLegalBasis')).toBe(
      "Where a provider is certified under the EU\u2013US Data Privacy Framework, the transfer rests on the EU Commission's adequacy decision (Art.\u00A045 DSGVO). Otherwise, the transfer is necessary to perform the service you requested (Art.\u00A049(1)(b) DSGVO). Fonts are self-hosted on our infrastructure and therefore do not involve any third-country transfer when loaded."
    )
    expect(t('legal.privacy.updatesBody', { lastUpdatedLabel: 'Last Updated' })).toBe(
      "We'll post updates to this page when the policy changes and include a new Last Updated date. Significant changes will be announced inside the dashboard."
    )
    expect(t('legal.privacy.updatesLastUpdatedEmphasis')).toBe('Last Updated')
    expect(t('legal.privacy.openSourceAuditable')).toBe(
      'All source code is publicly auditable under AGPL-3.0'
    )
    expect(t('legal.privacy.openSourceNoTracking')).toBe(
      'No hidden tracking \u2014 our analytics are cookieless and documented (Section\u00A05.6), and the whole stack is verifiable on GitHub'
    )
    expect(t('legal.privacy.openSourceScrutiny')).toBe(
      'Privacy practices are open to community scrutiny'
    )
    expect(
      t('legal.privacy.openSourceRepository', { repository: 'github.com/caesarakalaeii/all-chat' })
    ).toBe('Source: github.com/caesarakalaeii/all-chat')
    expect(t('legal.privacy.repositoryLinkText')).toBe('github.com/caesarakalaeii/all-chat')
  })

  it('keeps every platform-specific note', () => {
    expect(t('legal.privacy.twitchNote')).toBe(
      'We connect to Twitch EventSub to receive chat messages. We do not access channel analytics, subscriber information, or payment data.'
    )
    expect(t('legal.privacy.youtubeNote', { googleSettings: 'Google security settings' })).toBe(
      "We use the YouTube Live Chat API. We do not access video content, channel analytics, or subscriber data. API usage is subject to YouTube's quota limits. You can revoke access via Google security settings or the All-Chat Settings page. Disconnecting deletes your stored OAuth tokens."
    )
    expect(t('legal.privacy.googleSettingsLinkText')).toBe('Google security settings')
    expect(t('legal.privacy.tiktokNote')).toBe(
      'We access live stream chat data only. We do not access your videos, followers, or other personal content. Revoke access through TikTok app settings or All-Chat settings.'
    )
    expect(t('legal.privacy.kickNote')).toBe(
      'We connect via WebSocket to receive live chat. We do not access channel analytics or payment data.'
    )
    expect(t('legal.privacy.discordNote')).toBe(
      'When you connect a Discord server, we store the guild ID and name. Chat relay uses webhook URLs you configure. We do not access server member lists or DMs.'
    )
  })

  it('keeps the contact section', () => {
    expect(t('legal.privacy.contactEmailRow', { email: 'all.chat.support@gmail.com' })).toBe(
      'Email: all.chat.support@gmail.com'
    )
    expect(t('legal.privacy.contactHostedNote', { host: 'allch.at' })).toBe(
      'This contact is for users of the official hosted service at allch.at. Self-hosted installations should contact their own administrator.'
    )
    expect(t('legal.privacy.hostedDomain')).toBe('allch.at')
  })
})
