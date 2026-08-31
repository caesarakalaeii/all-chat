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
  // The privacy policy. The largest disclosure in the frontend: one key per
  // heading, paragraph, list item and legal basis, and nothing reworded.
  privacy: {
    title: 'Privacy Policy (Datenschutzerklärung)',
    lastUpdated: 'July 30, 2026',
    // Every DSGVO article reference carries a U+00A0 from an &nbsp; entity.
    // It is part of the rendered text, so it stays escaped here to be
    // visible to a reviewer.
    controllerHeading: '1. Controller (Verantwortlicher, Art.\u00A04 Nr.\u00A07 DSGVO)',
    collectHeading: '2. Information We Collect',
    useHeading: '3. How We Use Your Information',
    storageHeading: '4. Data Storage & Security',
    sharingHeading: '5. Data Sharing & Third Parties',
    retentionHeading: '6. Data Retention',
    rightsHeading: '7. Your Rights (Art.\u00A015–21 DSGVO)',
    cookiesHeading: '8. Cookies, Browser Storage & Analytics',
    childrenHeading: "9. Children's Privacy",
    transfersHeading: '10. International Data Transfers',
    updatesHeading: '11. Updates',
    openSourceHeading: '12. Transparency & Open Source',
    platformNotesHeading: '13. Platform-Specific Notes',
    contactHeading: '14. Contact',
    authSubheading: '2.1 Authentication Information',
    chatSubheading: '2.2 Chat Data',
    overlaySubheading: '2.3 Overlay Configuration',
    viewerIdentitySubheading: '2.4 Cross-Platform Viewer Identity',
    logDataSubheading: '2.5 Usage & Log Data',
    patreonSubheading: '2.6 Premium Membership (Patreon)',
    storageLocationsSubheading: '4.1 Storage Locations',
    safeguardsSubheading: '4.2 Safeguards',
    platformApisSubheading: '5.1 Streaming Platform APIs',
    fontsSubheading: '5.2 Fonts (Self-Hosted)',
    frontendResourcesSubheading: '5.3 Third-Party Frontend Resources',
    youtubeNoticeSubheading: '5.4 YouTube-Specific Notice',
    noSalesSubheading: '5.5 No Data Sales',
    analyticsSubheading: '5.6 Usage Analytics (Self-Hosted, Cookieless)',
    twitchSubheading: 'Twitch',
    youtubeSubheading: 'YouTube',
    tiktokSubheading: 'TikTok',
    kickSubheading: 'Kick',
    discordSubheading: 'Discord',
    // The two callout boxes at the top. Each opens with a bolded label that
    // begins the sentence, so the label is a {label} param rather than a
    // separate fragment.
    tldr: '{label} All-Chat only collects the information we need to authenticate with your streaming platforms and render chat in your overlays. Tokens are encrypted, chat messages are automatically deleted after one hour, and we never sell your data. We use cookieless, self-hosted analytics (no tracking cookies; see Section\u00A05.6).',
    tldrLabel: 'TL;DR:',
    openSourceCallout:
      '{label} All-Chat is licensed under AGPL-3.0. Review the entire codebase—including how we store and process your data—on {github}.',
    openSourceCalloutLabel: 'Open Source Transparency:',
    githubLinkText: 'GitHub',
    // The statutory ground for each kind of processing. One key each: the
    // article number and the reason it applies are a single statement, and
    // splitting them would let a translator pair the wrong two.
    authLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO – performance of a contract (providing the service you signed up for).',
    chatLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO (service delivery) and Art.\u00A06(1)(f) DSGVO (legitimate interest in abuse prevention).',
    overlayLegalBasis: 'Legal basis: Art.\u00A06(1)(b) DSGVO.',
    viewerIdentityLegalBasis:
      'Legal basis: Art.\u00A06(1)(a) DSGVO – your consent. You can unlink platforms at any time from the viewer settings.',
    logDataLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in maintaining service security and availability.',
    fontsLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in delivering the visual appearance of the overlay. The font files themselves are licensed under the SIL Open Font License 1.1 or Apache 2.0.',
    frontendResourcesLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in providing a functional and visually complete overlay experience. You can avoid loading these by not opening the theme marketplace. Fallback avatars (shown when a platform avatar is unavailable) are generated locally in your browser and involve no external request.',
    patreonLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO – providing the premium features you subscribed to. For the data you provide on patreon.com itself, Patreon, Inc. (US) is an independent controller; see the {patreonPolicy}.',
    patreonPolicyLinkText: 'Patreon Privacy Policy',
    controllerBody:
      'The person responsible for data processing on this website is listed in the {impressum}. For privacy-related inquiries contact {email}.',
    impressumLinkText: 'Impressum',
    supportEmail: 'all.chat.support@gmail.com',
    authIntro:
      'When you connect Twitch, YouTube, TikTok, Kick, or Discord we store the minimum data required to create overlays and reconnect later:',
    authIdentifiers: 'Platform identifiers (user ID, username, display name)',
    authProfileImages: 'Profile images provided by the platform',
    authTokens: 'Encrypted OAuth access and refresh tokens',
    authScopes: 'Token scopes and expiration dates',
    chatIntro: 'For active overlays we temporarily process:',
    chatMessages: 'Messages flowing through connected channels',
    chatMetadata: 'Message metadata (timestamps, emotes, badges, highlights)',
    chatAuthor: 'Per-message author details (display name, color, avatar)',
    chatRetention:
      'Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and {emphasis}. Messages displayed in the overlay are streamed through memory and are not persisted once the overlay session ends.',
    chatRetentionEmphasis: 'automatically deleted after one hour',
    overlayIntro: 'To render and sync overlays we store:',
    overlayNames: 'Overlay names, IDs, and custom CSS or theme settings',
    overlaySources: 'Connected chat sources (platform, channel ID, channel name)',
    overlayFilters: 'Filter settings (blocked words, user-level rules)',
    viewerIdentityBody:
      'If you choose to link multiple platform accounts as a viewer (e.g.\u00A0Twitch + YouTube), we create a unified viewer profile that associates your platform identities. This is {emphasis}.',
    viewerIdentityEmphasis: 'opt-in only and requires your explicit action',
    logDataIntro: 'For observability and abuse prevention we log:',
    logDataIp:
      'Anonymised IP address (last octet zeroed), browser user agent, and basic request info',
    logDataMetrics: 'API usage metrics, cache hits, and connection status',
    logDataTraces: 'Error traces and diagnostic logs (retained up to 90 days)',
    patreonIntro:
      'Premium features are unlocked through a paid membership on Patreon. If you connect your Patreon account to All-Chat, we store:',
    patreonUserId: 'Your Patreon user ID',
    patreonTokens: 'Encrypted OAuth access and refresh tokens for the Patreon API',
    patreonMembership:
      'Your membership state for our campaign (status, tier, pledge amount, renewal date)',
    patreonNoPaymentData:
      'We do {not} receive or store your payment details; payments are processed entirely by Patreon. Patreon notifies us via webhooks when your membership changes, and a periodic reconciliation keeps the state current. Disconnecting Patreon in the Settings page deletes the stored tokens and revokes subscription-derived premium.',
    notEmphasis: 'not',
    useIntro: 'Everything we store directly supports the core overlay experience:',
    useAuthenticate: 'Authenticate against the platforms you authorize',
    useFetch: 'Fetch live chat messages and normalize them into a single feed',
    useRender: 'Render overlays, perform emote lookups, and respect your filters',
    useMonitor: 'Monitor service reliability and debug incidents',
    useAbuse: 'Detect and prevent abuse (rate-limiting, bans)',
    storagePostgres: 'PostgreSQL for account data, overlays, and encrypted OAuth tokens',
    storageRedis: 'Redis for ephemeral sessions, message fan-out, and rate limiting',
    storageLocation:
      'All of the above runs on our own infrastructure on servers located in {country} (hosting provider: Hetzner Online GmbH)',
    storageCountry: 'Germany',
    safeguardsEncryption: 'OAuth tokens encrypted with AES-GCM before touching the database',
    safeguardsHttps:
      'HTTPS at the ingress layer for all external traffic; internal services isolated via Kubernetes network policies',
    safeguardsAccess: 'Role-scoped infrastructure access and audit logging',
    safeguardsPatching: 'Regular dependency upgrades and security patching',
    safeguardsHeaders:
      'Security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy)',
    safeguardsCaveat:
      'No storage system is perfectly secure, but we follow industry best practices to keep your tokens and overlays safe.',
    platformApisIntro: 'We connect to the following services to deliver the core product:',
    platformApisTwitch: 'Twitch EventSub and Helix APIs',
    platformApisYoutube: 'YouTube Live Chat and OAuth APIs',
    platformApisTiktokKick: 'TikTok Live APIs and Kick APIs',
    platformApisDiscord: 'Discord Gateway API',
    platformApisEmotes: '7TV, BTTV, FFZ for emote metadata',
    platformApisScopes:
      "Every integration remains subject to the platform's own policies and scopes you approve.",
    // Three emphasised runs plus a <code> path in one sentence, so the render
    // site uses interpolateElements rather than nested emphasise calls.
    fontsBody:
      'Typography assets originally distributed by Google Fonts are {selfHosted}. The fonts used by the All-Chat interface are bundled at build time via Next.js, and fonts selectable for overlay customization are served through a server-side proxy at {proxyPath}. Your browser only connects to the All-Chat origin; {noTransmission} when fonts are loaded. This aligns with the Landgericht München I ruling on Google Fonts (20 January 2022, Az. 3 O 17493/20).',
    fontsSelfHostedEmphasis: 'self-hosted on our infrastructure',
    fontsNoTransmissionEmphasis:
      'no IP address, user agent, or request metadata is transmitted to Google',
    frontendResourcesIntro:
      'Overlay and dashboard pages may load the following external resources. Each request transmits your {emphasis} and browser user agent to the respective provider:',
    ipAddressEmphasis: 'IP address',
    frontendResourcesGithub:
      '{label} (api.github.com) – fetches community themes from our public repository in the theme marketplace',
    githubApiLabel: 'GitHub API',
    youtubeNoticeBody:
      "Your use of All-Chat's YouTube integration is also governed by the {googlePolicy}. This applies only to the YouTube API integration (data flows described in Section 5.1); it does {not} apply to fonts, which are self-hosted as described in Section 5.2.",
    googlePolicyLinkText: 'Google Privacy Policy',
    noSalesBody:
      'We never sell or rent your data. We may disclose information when required by law or to respond to legitimate security incidents.',
    analyticsBody:
      'To understand how the site is used and where to improve it, we run {umami}, an open-source analytics tool that we {selfHost}. It is {cookieless}: it sets no cookies, creates no persistent identifier, and performs no cross-site or cross-device tracking. The data is processed on our own infrastructure and is {notShared} (unlike Google Analytics or similar services).',
    umamiLinkText: 'Umami',
    analyticsSelfHostEmphasis: 'host ourselves',
    analyticsCookielessEmphasis: 'cookieless',
    analyticsNotSharedEmphasis: 'not shared with any third party',
    analyticsRecordsIntro: 'For each page view it records aggregate, non-identifying information:',
    analyticsRecordsPage: 'The page you visited and the referring URL',
    analyticsRecordsBrowser: 'Browser, operating system, device type, and screen size',
    analyticsRecordsCountry: 'Approximate country, derived from your IP address at request time',
    analyticsIpNote:
      'Your {ipNotStored}: it is used only momentarily to derive the country and to generate a daily, salted hash for counting unique visits, after which it is discarded. We do {not} track public overlay views (the pages OBS loads as a browser source). You can block the analytics script with any browser content blocker without affecting the site.',
    analyticsIpNotStoredEmphasis: 'IP address is not stored',
    analyticsConsentNote:
      'Because the tracker stores no information on, and reads none from, your device, it does not require consent under §\u00A025 TDDDG; the processing of the resulting data rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in measuring and improving the service.',
    // Retention rows: the bolded label opens the row and the rest continues
    // the sentence it starts, per the legal.cookieBanner convention.
    retentionAccount: '{label} kept until you delete your account',
    retentionAccountLabel: 'Account & overlay data:',
    retentionTokens:
      '{label} deleted when you disconnect a platform, when they expire (cleaned up after 7 days), or when you delete your account',
    retentionTokensLabel: 'OAuth tokens:',
    retentionSentMessages: '{label} automatically deleted after 1 hour',
    retentionSentMessagesLabel: 'Chat messages sent through All-Chat:',
    retentionDisplayedMessages: '{label} streamed through memory only; not persisted',
    retentionDisplayedMessagesLabel: 'Chat messages displayed in overlays:',
    retentionLogs: '{label} retained for up to 90 days',
    retentionLogsLabel: 'Usage logs:',
    retentionQuotaLogs: '{label} retained for 30 days',
    retentionQuotaLogsLabel: 'YouTube quota audit logs:',
    // The Art. 15-21 data subject rights. Same {label} shape as the
    // retention rows above.
    rightsIntro: 'You can exercise the following at any time:',
    rightsAccess: '{label} Request a copy of the data we store about you',
    rightsAccessLabel: 'Access (Art.\u00A015):',
    rightsRectification: '{label} Correct inaccurate information',
    rightsRectificationLabel: 'Rectification (Art.\u00A016):',
    rightsErasure:
      '{label} Delete your account and all associated data from the Settings page or by contacting us',
    rightsErasureLabel: 'Erasure (Art.\u00A017):',
    rightsRestriction: '{label} Request restriction of processing in certain circumstances',
    rightsRestrictionLabel: 'Restriction (Art.\u00A018):',
    rightsPortability:
      '{label} Export your data in a machine-readable format via the Settings page',
    rightsPortabilityLabel: 'Data portability (Art.\u00A020):',
    rightsObjection: '{label} Object to processing based on legitimate interest',
    rightsObjectionLabel: 'Objection (Art.\u00A021):',
    rightsWithdraw: '{label} Disconnect platforms or unlink viewer identities at any time',
    rightsWithdrawLabel: 'Withdraw consent:',
    rightsContact:
      'Contact us at {email} or use the Settings page. You also have the right to lodge a complaint with your supervisory authority (Aufsichtsbehörde).',
    noProfiling:
      'We do not use automated decision-making or profiling within the meaning of Art.\u00A022 DSGVO.',
    youtubeRevoke:
      "For YouTube Data: You can revoke All-Chat's access to your YouTube data via the {googleSettings}.",
    googleSettingsPageLinkText: 'Google security settings page',
    cookiesIntro: 'All-Chat uses {emphasis} (not cookies) for essential functionality:',
    localStorageEmphasis: 'browser localStorage',
    cookiesTokens: 'Authentication tokens (JWT) to keep you logged in',
    cookiesPreferences: 'User preferences and last-visited state',
    cookiesNoAdvertising:
      'We do not use advertising cookies or cross-site tracking. We do use {emphasis} (self-hosted Umami) – it sets nothing on your device and stores no personal identifier; see Section\u00A05.6 for the full description. Fonts are self-hosted (Section\u00A05.2) and do not set cookies. The GitHub API (Section\u00A05.3) may cause GitHub to set its own cookies when the theme marketplace is loaded.',
    cookielessAnalyticsEmphasis: 'privacy-friendly, cookieless usage analytics',
    childrenBody:
      'All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data processing in Germany). If we discover data belonging to a minor we will delete it immediately.',
    transfersBody:
      'All-Chat itself is hosted on servers located in Germany (see Section\u00A04.1). However, when you use streaming platform integrations, data is transferred to servers operated by Twitch (Amazon, US), Google/YouTube (US), TikTok (various), and Kick (AU). If you connect a Patreon membership, data is exchanged with Patreon, Inc. (US). The GitHub API (theme marketplace) may also involve transfers to the US.',
    transfersLegalBasis:
      "Where a provider is certified under the EU–US Data Privacy Framework, the transfer rests on the EU Commission's adequacy decision (Art.\u00A045 DSGVO). Otherwise, the transfer is necessary to perform the service you requested (Art.\u00A049(1)(b) DSGVO). Fonts are self-hosted on our infrastructure and therefore do not involve any third-country transfer when loaded.",
    updatesBody:
      "We'll post updates to this page when the policy changes and include a new {lastUpdatedLabel} date. Significant changes will be announced inside the dashboard.",
    updatesLastUpdatedEmphasis: 'Last Updated',
    openSourceAuditable: 'All source code is publicly auditable under AGPL-3.0',
    openSourceNoTracking:
      'No hidden tracking — our analytics are cookieless and documented (Section\u00A05.6), and the whole stack is verifiable on GitHub',
    openSourceScrutiny: 'Privacy practices are open to community scrutiny',
    openSourceRepository: 'Source: {repository}',
    repositoryLinkText: 'github.com/caesarakalaeii/all-chat',
    // Section 13, one paragraph per platform. What All-Chat does NOT access
    // is the load-bearing half of each of these.
    twitchNote:
      'We connect to Twitch EventSub to receive chat messages. We do not access channel analytics, subscriber information, or payment data.',
    youtubeNote:
      "We use the YouTube Live Chat API. We do not access video content, channel analytics, or subscriber data. API usage is subject to YouTube's quota limits. You can revoke access via {googleSettings} or the All-Chat Settings page. Disconnecting deletes your stored OAuth tokens.",
    googleSettingsLinkText: 'Google security settings',
    tiktokNote:
      'We access live stream chat data only. We do not access your videos, followers, or other personal content. Revoke access through TikTok app settings or All-Chat settings.',
    kickNote:
      'We connect via WebSocket to receive live chat. We do not access channel analytics or payment data.',
    discordNote:
      'When you connect a Discord server, we store the guild ID and name. Chat relay uses webhook URLs you configure. We do not access server member lists or DMs.',
    contactEmailRow: 'Email: {email}',
    contactHostedNote:
      'This contact is for users of the official hosted service at {host}. Self-hosted installations should contact their own administrator.',
    hostedDomain: 'allch.at',
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
