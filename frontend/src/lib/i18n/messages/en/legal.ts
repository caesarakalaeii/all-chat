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
 * Privacy policy, terms and the Impressum's not-configured fallback.
 *
 * The Impressum body itself is mounted at runtime from IMPRESSUM_FILE_PATH and is
 * not in this repo, so only the fallback copy lives here.
 */

export const legal = {
  // The GDPR disclosures. Reword nothing here without legal review: what
  // All-Chat stores, what it does not track and which third parties see an IP
  // address are statements of fact about the service, not marketing copy.
  cookieBanner: {
    regionLabel: 'Cookie consent',
    iconLabel: 'Cookie',
    title: 'Privacy & Data Storage',
    storageBody:
      'All-Chat uses {storage} to save your authentication tokens and keep you logged in. This is essential for the service to function properly.',
    storageEmphasis: 'browser local storage',
    noTrackingBody:
      '{emphasis} We do not share your data with third parties for advertising purposes.',
    noTrackingEmphasis:
      'We use no tracking cookies, and our usage analytics are cookieless and store no personal data.',
    detailsSummary: 'What data do we store locally?',
    // Each row keeps its bolded label inside the sentence as {label}: the text
    // after the hyphen continues the sentence the label starts, so handing a
    // translator the two halves separately would leave them unable to reorder.
    tokensRow:
      '{label} - Required to keep you logged in and authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)',
    tokensLabel: 'Authentication tokens',
    preferencesRow: '{label} - Your overlay configurations and settings',
    preferencesLabel: 'User preferences',
    noTrackingRow: "{label} - We don't track your browsing behavior",
    noTrackingLabel: 'No tracking cookies',
    noAdsRow: "{label} - We don't serve ads or share data with advertisers",
    noAdsLabel: 'No advertising cookies',
    analyticsRow:
      '{label} - We measure aggregate usage with self-hosted Umami. It sets no cookies, stores no personal identifier, and does not track public overlays',
    analyticsLabel: 'Cookieless analytics',
    fontsNote:
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.',
    fontsLabel: 'Fonts:',
    thirdPartyNote:
      '{label} Dashboard pages may load fallback avatars from {avatars} and themes from the {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.',
    thirdPartyLabel: 'Third-party resources:',
    thirdPartyAvatars: 'UI Avatars',
    thirdPartyGithub: 'GitHub API',
    agreement:
      'By using All-Chat, you agree to this essential data storage. For more details, please read our {privacy} and {terms}.',
    privacyPolicy: 'Privacy Policy',
    termsOfService: 'Terms of Service',
    acknowledge: 'I Understand',
    learnMore: 'Learn More',
    footer: 'Your data is stored locally in your browser and transmitted securely via HTTPS',
  },
  // Chrome on all three legal routes.
  layout: {
    eyebrow: 'All-Chat Legal',
    lastUpdated: 'Last updated: {date}',
    // One string: a &copy; entity, the year and the product name were three
    // separate JSX runs.
    copyright: '© {year} All-Chat',
    homeLink: 'Home',
    privacyLink: 'Privacy Policy',
    termsLink: 'Terms of Service',
    impressumLink: 'Impressum',
  },
  // The terms of service. One key per heading, paragraph and list item: these
  // are contractual statements, and a reworded term is a different term.
  terms: {
    title: 'Terms of Service (Nutzungsbedingungen)',
    lastUpdated: 'July 30, 2026',
    acceptanceHeading: '1. Acceptance of Terms',
    // The <Link> to the privacy policy sits mid-sentence, so the sentence is
    // whole with the link text as a param. Same shape throughout this section.
    acceptanceBody:
      'By accessing or using All-Chat you agree to these Terms of Service and our {privacy}. If you disagree with any part, you should discontinue use immediately.',
    privacyLinkText: 'Privacy Policy',
    descriptionHeading: '2. Description of Service',
    descriptionBody:
      'All-Chat aggregates real-time chat from Twitch, YouTube, Kick, TikTok, and Discord into a single overlay so you can display cross-platform conversation on your stream. You can customize overlays, connect sources, and broadcast them via OBS or browser sources.',
    accountsHeading: '3. Accounts & Authentication',
    accountsIntro:
      'You are responsible for all activity that happens under your account. You agree to:',
    accountsAccurate: 'Provide accurate registration details',
    accountsSecurity: 'Maintain the security of your credentials and OAuth grants',
    accountsNotify: 'Notify us at {email} if you suspect compromise',
    accountsComply:
      'Comply with the terms of Twitch, YouTube, TikTok, Kick, Discord, and any other connected platform',
    supportEmail: 'all.chat.support@gmail.com',
    acceptableUseHeading: '4. Acceptable Use',
    acceptableUseIntro: 'You agree not to misuse the Service, including but not limited to:',
    acceptableUseLaws: 'Breaking local, national, or international laws',
    acceptableUseIp: 'Infringing intellectual property or privacy rights of others',
    acceptableUseMalware: 'Uploading malware, spam, or malicious code',
    acceptableUseBypass: 'Attempting to bypass authentication, rate limits, or security controls',
    acceptableUseHarassment: 'Harassing or abusing other users',
    acceptableUsePartner: 'Using All-Chat in a way that violates partner platform policies',
    thirdPartyHeading: '5. Third-Party Integrations',
    thirdPartyIntro:
      'All-Chat relies on third-party APIs. Their availability, scopes, and rate limits may change.',
    thirdPartyComply: "You must comply with each platform's terms of service",
    thirdPartyOutages: 'We are not accountable for outages or policy shifts by those platforms',
    thirdPartyQuotas: 'Platform-specific quotas can impact overlay functionality',
    youtubeBinding:
      'YouTube Integration: By using All-Chat to connect to YouTube, you agree to be bound by the {youtubeTerms}.',
    youtubeTermsLinkText: 'YouTube Terms of Service',
    privacyHeading: '6. Privacy',
    // The no-break space before 5.6 was an &nbsp; entity and the dash an
    // &ndash;. Both are part of the rendered text, not markup.
    privacyBody:
      'Your use of All-Chat is also governed by our {privacy}, which explains what we collect, how it is used, and your rights under the DSGVO. For transparency: All-Chat measures aggregate usage with {analytics} (Umami) that set nothing on your device and store no personal identifier – see Section\u00A05.6 of the Privacy Policy.',
    privacyAnalyticsEmphasis: 'self-hosted, cookieless analytics',
    licenseHeading: '7. Open Source License',
    // The link text carries the "(AGPL-3.0)" suffix that the sentence's own
    // {license} run does not, because the <a> in the source wraps both.
    licenseBody:
      'All-Chat is released under the {license}. That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.',
    licenseLinkText: 'GNU Affero General Public License v3.0 (AGPL-3.0)',
    licenseRepository: 'The canonical repository is available on {github}.',
    githubLinkText: 'GitHub',
    availabilityHeading: '8. Availability & Support',
    availabilityIntro: 'We aim for high uptime but do not guarantee:',
    availabilityUptime: 'Uninterrupted access or zero bugs',
    availabilityCompat: 'Compatibility with every browser or streamer setup',
    availabilityFixes: 'Immediate fixes or feature requests',
    availabilitySupport:
      'Support for the hosted service at {host} is best-effort. Community/self-hosted deployments must rely on their own maintainers or the open source community for assistance.',
    hostedDomain: 'allch.at',
    liabilityHeading: '9. Limitation of Liability',
    liabilityGross:
      'We are liable without limitation for damages caused intentionally or by gross negligence, for injury to life, body, or health, and under the German Product Liability Act (Produkthaftungsgesetz).',
    liabilitySlight:
      'In cases of slight negligence, we are liable only for the breach of essential contractual obligations (Kardinalpflichten): obligations whose fulfilment makes the proper performance of the contract possible in the first place and on whose fulfilment you may regularly rely. In such cases, our liability is limited to the damage that is foreseeable and typical for this type of service. Any further liability is excluded.',
    liabilityAgents:
      'These limitations also apply in favour of our legal representatives and vicarious agents (Erfüllungsgehilfen).',
    indemnityHeading: '10. Indemnification',
    indemnityBody:
      'You agree to indemnify All-Chat against third-party claims, including the reasonable costs of legal defence, arising from your culpable violation of these Terms, applicable law, or the rights of others. This does not apply to the extent you are not responsible for the violation.',
    premiumHeading: '11. Premium Subscriptions & Right of Withdrawal (Widerrufsrecht)',
    patreonLinkText: 'Patreon',
    premiumBody:
      "The core All-Chat service is free of charge. Premium features are unlocked through a paid membership on {patreon}: you subscribe to our campaign on patreon.com and then connect your Patreon account to All-Chat. The subscription contract, billing, cancellation, and any statutory right of withdrawal for the paid membership are handled by Patreon under Patreon's own terms. All-Chat itself does not charge you and does not process payments.",
    premiumCancellation:
      'You may stop using All-Chat and delete your account at any time via the Settings page. Upon deletion, your personal data is removed as described in our Privacy Policy. Note that deleting your All-Chat account does not cancel a Patreon membership; cancel it directly on Patreon.',
    changesHeading: '12. Changes to Terms',
    changesBody:
      'We may update these Terms over time. Material changes will be announced in the dashboard, and the new version will be posted here with an updated effective date.',
    terminationHeading: '13. Termination',
    terminationBody:
      'We reserve the right to suspend or terminate your access for any breach of these Terms or abusive behavior. You may stop using the Service at any time by deleting your account in the Settings page.',
    governingLawHeading: '14. Governing Law & Jurisdiction',
    governingLawBody:
      'These Terms are governed by the laws of the Federal Republic of Germany, excluding the UN Convention on Contracts for the International Sale of Goods (CISG). If you are a consumer, the mandatory consumer protection provisions of the country in which you habitually reside remain unaffected. If you are a merchant (Kaufmann), a legal entity under public law, or a special fund under public law, the exclusive place of jurisdiction is the domicile of the operator.',
    legalNoticeHeading: '15. Legal Notice',
    legalNoticeBody:
      "The operator's identity and contact details are available in the {impressum}.",
    impressumLinkText: 'Impressum',
    contactHeading: '16. Contact',
    contactBody:
      'Questions? Reach us at {email}. Hosted community forks should contact their own administrators.',
  },
  // Only the fallback is migratable. The Impressum body is mounted at runtime
  // from IMPRESSUM_FILE_PATH and is not in this repo.
  impressum: {
    title: 'Impressum',
    notConfigured: 'The Impressum for this instance has not been configured yet.',
    // One sentence with both paths as params: it was five JSX runs around two
    // <code> elements.
    operatorHint:
      'If you are the operator: mount a ConfigMap containing your Impressum HTML to {path} or set the {variable} environment variable. See the deployment documentation for details.',
  },
} as const
