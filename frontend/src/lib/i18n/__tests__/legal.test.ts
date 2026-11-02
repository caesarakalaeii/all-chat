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
      '{label} - Required to keep you logged in and authenticate with the streaming platforms you connect'
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
      '{label} - We measure aggregate usage with self-hosted Umami. It sets no cookies and stores no personal identifier, and public overlays are only counted in aggregate'
    )
    expect(t('legal.cookieBanner.analyticsLabel')).toBe('Cookieless analytics')
  })

  it('keeps the fonts and third-party notes whole', () => {
    expect(t('legal.cookieBanner.fontsNote')).toBe(
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.'
    )
    expect(t('legal.cookieBanner.fontsLabel')).toBe('Fonts:')
    // Three emphasised runs in one sentence, one of them a link, so the sentence
    // stays whole and each run is its own placeholder.
    expect(t('legal.cookieBanner.thirdPartyNote')).toBe(
      '{label} Pages that show chat load avatars, badges and emotes from the streaming platforms and emote providers, and the credit-roll theme marketplace loads themes from the {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.'
    )
    expect(t('legal.cookieBanner.thirdPartyLabel')).toBe('Third-party resources:')
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
    expect(t('metadata.impressum.title')).toBe('Impressum')
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

// The privacy policy and the terms pin every string, one expectation per key, in
// catalog order. Section numbers, U+00A0 article references and the
// {placeholders} each render site fills are all part of the pinned text, so a
// reworded disclosure, a renumbered section or a dropped placeholder fails here.
// Composed renders (placeholders filled the way the page fills them) are
// pinned in the 'rendered sentences' block at the end.

describe('privacy policy copy', () => {
  it('keeps every privacy policy string whole', () => {
    expect(t('legal.privacy.title')).toBe('Privacy Policy (Datenschutzerklärung)')
    expect(t('legal.privacy.lastUpdated')).toBe('November 3, 2026')
    expect(t('legal.privacy.controllerHeading')).toBe(
      '1. Controller (Verantwortlicher, Art.\u00A04 Nr.\u00A07 DSGVO)'
    )
    expect(t('legal.privacy.collectHeading')).toBe('2. Information We Collect')
    expect(t('legal.privacy.useHeading')).toBe('3. How We Use Your Information')
    expect(t('legal.privacy.storageHeading')).toBe('4. Data Storage & Security')
    expect(t('legal.privacy.sharingHeading')).toBe('5. Data Sharing & Third Parties')
    expect(t('legal.privacy.retentionHeading')).toBe('6. Data Retention')
    expect(t('legal.privacy.rightsHeading')).toBe('7. Your Rights (Art.\u00A015–21 DSGVO)')
    expect(t('legal.privacy.cookiesHeading')).toBe('8. Cookies, Browser Storage & Analytics')
    expect(t('legal.privacy.childrenHeading')).toBe("9. Children's Privacy")
    expect(t('legal.privacy.transfersHeading')).toBe('10. International Data Transfers')
    expect(t('legal.privacy.updatesHeading')).toBe('11. Updates')
    expect(t('legal.privacy.openSourceHeading')).toBe('12. Transparency & Open Source')
    expect(t('legal.privacy.platformNotesHeading')).toBe('13. Platform-Specific Notes')
    expect(t('legal.privacy.contactHeading')).toBe('14. Contact')
    expect(t('legal.privacy.accountSubheading')).toBe('2.1 Your Account & Connected Platforms')
    expect(t('legal.privacy.chatSubheading')).toBe('2.2 Chat & Event Data')
    expect(t('legal.privacy.featuresSubheading')).toBe('2.3 Overlays & Features')
    expect(t('legal.privacy.viewerSubheading')).toBe('2.4 Viewer Accounts & Participation')
    expect(t('legal.privacy.extensionSubheading')).toBe('2.5 Browser Extension')
    expect(t('legal.privacy.logDataSubheading')).toBe('2.6 Usage & Log Data')
    expect(t('legal.privacy.patreonSubheading')).toBe('2.7 Premium Membership (Patreon)')
    expect(t('legal.privacy.supportSubheading')).toBe('2.8 Support')
    expect(t('legal.privacy.storageLocationsSubheading')).toBe('4.1 Storage Locations')
    expect(t('legal.privacy.safeguardsSubheading')).toBe('4.2 Safeguards')
    expect(t('legal.privacy.platformApisSubheading')).toBe(
      '5.1 Streaming Platforms & Chat Services'
    )
    expect(t('legal.privacy.browserResourcesSubheading')).toBe(
      '5.2 Content Your Browser Loads from Third Parties'
    )
    expect(t('legal.privacy.processorsSubheading')).toBe('5.3 Service Providers')
    expect(t('legal.privacy.streamerRequestsSubheading')).toBe(
      '5.4 Requests Configured by Streamers'
    )
    expect(t('legal.privacy.fontsSubheading')).toBe('5.5 Fonts (Self-Hosted)')
    expect(t('legal.privacy.youtubeNoticeSubheading')).toBe('5.6 YouTube-Specific Notice')
    expect(t('legal.privacy.noSalesSubheading')).toBe('5.7 No Data Sales')
    expect(t('legal.privacy.analyticsSubheading')).toBe(
      '5.8 Usage Analytics (Self-Hosted, Cookieless)'
    )
    expect(t('legal.privacy.backupsSubheading')).toBe('5.9 Backup Storage')
    expect(t('legal.privacy.twitchSubheading')).toBe('Twitch')
    expect(t('legal.privacy.youtubeSubheading')).toBe('YouTube')
    expect(t('legal.privacy.tiktokSubheading')).toBe('TikTok')
    expect(t('legal.privacy.kickSubheading')).toBe('Kick')
    expect(t('legal.privacy.discordSubheading')).toBe('Discord')
    expect(t('legal.privacy.owncastSubheading')).toBe('Owncast (beta)')
    expect(t('legal.privacy.publicChatSubheading')).toBe(
      'GoodGame, Picarto & Altstream (beta, early access)'
    )
    expect(t('legal.privacy.metaSubheading')).toBe('Facebook & Instagram (beta)')
    expect(t('legal.privacy.tldr')).toBe(
      '{label} All-Chat only collects what we need to sign you in, connect your streaming platforms and run your overlays, alerts and chat tools. Tokens are encrypted, chat messages are not stored in our database, and we never sell your data. Section\u00A06 lists how long we keep each kind of data. We use cookieless, self-hosted analytics (no tracking cookies; see Section\u00A05.8).'
    )
    expect(t('legal.privacy.tldrLabel')).toBe('TL;DR:')
    expect(t('legal.privacy.openSourceCallout')).toBe(
      '{label} All-Chat is licensed under AGPL-3.0. Review the entire codebase—including how we store and process your data—on {github}.'
    )
    expect(t('legal.privacy.openSourceCalloutLabel')).toBe('Open Source Transparency:')
    expect(t('legal.privacy.githubLinkText')).toBe('GitHub')
    expect(t('legal.privacy.accountLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO – performance of a contract (providing the service you signed up for).'
    )
    expect(t('legal.privacy.chatLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO for you as the streamer. For the viewers whose messages and events we process, Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in showing chat and events that were posted publicly on the platform, and in preventing abuse.'
    )
    expect(t('legal.privacy.featuresLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO. The ambassador showcase rests on your consent (Art.\u00A06(1)(a) DSGVO), which you can withdraw at any time.'
    )
    expect(t('legal.privacy.viewerLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO for the features you use. Linking several platform identities rests on your consent (Art.\u00A06(1)(a) DSGVO); you can unlink platforms at any time from the viewer settings. Recording participation by chat command rests on Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in running the poll, prediction or points game you joined.'
    )
    expect(t('legal.privacy.extensionLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO – providing the extension features you installed it for.'
    )
    expect(t('legal.privacy.logDataLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in maintaining service security and availability.'
    )
    expect(t('legal.privacy.supportLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in answering support requests; for requests about your account, Art.\u00A06(1)(b) DSGVO.'
    )
    expect(t('legal.privacy.fontsLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in delivering the visual appearance of the overlay. The font files themselves are licensed under the SIL Open Font License 1.1 or Apache 2.0.'
    )
    expect(t('legal.privacy.browserResourcesLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in showing chat the way it appears on the platforms. Fallback avatars (shown when a platform avatar is unavailable) and the sample chat on our homepage are served by All-Chat itself and involve no external request.'
    )
    expect(t('legal.privacy.patreonLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO – providing the premium features you subscribed to. For the data you provide on patreon.com itself, Patreon, Inc. (US) is an independent controller; see the {patreonPolicy}.'
    )
    expect(t('legal.privacy.patreonPolicyLinkText')).toBe('Patreon Privacy Policy')
    expect(t('legal.privacy.controllerBody')).toBe(
      'The person responsible for data processing on this website is listed in the {impressum}. For privacy-related inquiries contact {email}.'
    )
    expect(t('legal.privacy.impressumLinkText')).toBe('Impressum')
    expect(t('legal.privacy.supportEmail')).toBe('all.chat.support@gmail.com')
    expect(t('legal.privacy.accountIntro')).toBe(
      'You sign in to All-Chat with Twitch, YouTube (Google) or Kick. For your account and for every platform you connect as a chat source we store:'
    )
    expect(t('legal.privacy.accountIdentifiers')).toBe(
      'Platform identifiers (user ID, username, display name) and the profile image the platform provides'
    )
    expect(t('legal.privacy.accountTokens')).toBe(
      'Encrypted OAuth access and refresh tokens with their scopes and expiry dates, for each platform you authorise (Twitch, YouTube, Kick, and in beta Facebook and Instagram)'
    )
    expect(t('legal.privacy.accountNameOnly')).toBe(
      'For platforms you add by name or address only (TikTok, Owncast, GoodGame, Picarto, Altstream), just the channel name or server address you enter; for Discord, the server ID and name and the channels you pick for the All-Chat bot'
    )
    expect(t('legal.privacy.accountStatus')).toBe(
      'Your account status: premium, beta tester or ambassador status, admin role, and any ban with its reason'
    )
    expect(t('legal.privacy.accountTermsAcceptance')).toBe(
      'Whether and when you accepted the current Terms of Service and this Privacy Policy'
    )
    expect(t('legal.privacy.accountNoEmail')).toBe(
      'We do not receive an email address from any chat platform.'
    )
    expect(t('legal.privacy.chatIntro')).toBe(
      'While an overlay or chat view is connected, we receive and process:'
    )
    expect(t('legal.privacy.chatMessages')).toBe(
      'Chat messages from the channels you connected, including emotes, GIFs and attachments'
    )
    expect(t('legal.privacy.chatAuthor')).toBe(
      'Per-message author details (platform user ID, username, display name, color, avatar, badges and roles)'
    )
    expect(t('legal.privacy.chatEvents')).toBe(
      'Channel events: follows, subscriptions and gifted subscriptions, bits and cheers, Super Chats and Super Stickers, memberships, raids, channel point redemptions, TikTok gifts, likes, shares and follows, and your most recent YouTube subscribers'
    )
    expect(t('legal.privacy.chatModeration')).toBe(
      'Moderation events (deleted messages, timeouts, bans) and, if you turn on the moderation log, Twitch AutoMod holds and moderator actions, which are shown only to you'
    )
    expect(t('legal.privacy.chatRetention')).toBe(
      'Chat messages are {emphasis}. They pass through memory and short-lived queues: a five-minute replay buffer so a reconnecting overlay can catch up, a capped processing queue that is continuously overwritten, and a seven-day queue for messages that failed processing so we can fix the fault.'
    )
    expect(t('legal.privacy.chatRetentionEmphasis')).toBe('not written to our database')
    expect(t('legal.privacy.chatDerived')).toBe(
      'Some features keep data derived from events: the alert log (sender name, event type and amount) for 30 days, list-overlay leaderboards for a rolling 30 days, and credit-roll session totals for 48 hours.'
    )
    expect(t('legal.privacy.featuresIntro')).toBe(
      'To run your overlays and tools we store what you configure:'
    )
    expect(t('legal.privacy.featuresOverlays')).toBe(
      'Overlay names, IDs, custom CSS, themes, connected sources and filters (blocked words and user rules, which can contain usernames)'
    )
    expect(t('legal.privacy.featuresAlerts')).toBe(
      'Alert, goal, list and credit-roll settings, including any custom HTML, CSS and JavaScript and the image and sound addresses you choose'
    )
    expect(t('legal.privacy.featuresMedia')).toBe(
      'Media you upload (images, audio and video up to 10\u00A0MiB each). Uploaded files are stored on our own servers and can be opened by anyone who has their unguessable address, so that they play in OBS'
    )
    expect(t('legal.privacy.featuresTts')).toBe(
      'Text-to-speech settings, including your own ElevenLabs API key, stored encrypted'
    )
    expect(t('legal.privacy.featuresCommands')).toBe(
      'Chat commands, keyword triggers, timers and bot settings, and for a custom bot account its login, display name and encrypted tokens'
    )
    expect(t('legal.privacy.featuresAccessTokens')).toBe(
      'Personal access tokens and paired devices: their name, scopes and last use. The tokens themselves are stored only as SHA-256 hashes'
    )
    expect(t('legal.privacy.featuresModerators')).toBe(
      'Moderators you invite to an overlay (display name, invitation label and the platform account they are expected to use), and the encrypted platform credentials a moderator connects'
    )
    expect(t('legal.privacy.featuresSharing')).toBe(
      'Share requests you send to or receive from other streamers'
    )
    expect(t('legal.privacy.featuresTranslations')).toBe(
      'Translations you contribute, with your user ID and any review notes'
    )
    expect(t('legal.privacy.featuresShowcase')).toBe(
      'If you are an ambassador and opt in to the public showcase: your username, display name, profile image, platform and tagline, shown publicly on allch.at'
    )
    expect(t('legal.privacy.viewerAccountBody')).toBe(
      'Viewers can sign in with Twitch, YouTube or Kick through the browser extension, the participation page or the viewer settings. We store the platform user ID, username, display name, avatar and encrypted tokens (Twitch: no extra permissions; YouTube: read-only access and basic profile; Kick: basic profile), and the name color, avatar frame, flair and viewer premium status you have.'
    )
    expect(t('legal.privacy.viewerIdentityBody')).toBe(
      'If you choose to link several platform accounts (e.g.\u00A0Twitch + YouTube), we create a unified viewer profile that associates your platform identities. This is {emphasis}.'
    )
    expect(t('legal.privacy.viewerIdentityEmphasis')).toBe(
      'opt-in only and requires your explicit action'
    )
    expect(t('legal.privacy.viewerEngagementBody')).toBe(
      "When you vote in a poll, enter a prediction or earn points in a streamer's channel, including by typing a chat command without ever signing in, we create a viewer record for your platform account (platform user ID and username) and store your votes, predictions and points for that streamer's overlay. Points exist only within that streamer's overlay."
    )
    expect(t('legal.privacy.viewerSentMessages')).toBe(
      'Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and {emphasis}.'
    )
    expect(t('legal.privacy.viewerSentMessagesEmphasis')).toBe(
      'automatically deleted after one hour'
    )
    expect(t('legal.privacy.extensionBody')).toBe(
      "The All-Chat browser extension runs on twitch.tv, youtube.com and kick.com. On each channel page you open it asks All-Chat whether that streamer uses All-Chat, which sends us the channel name together with your IP address, even when you are not signed in. Signed in, it shows the streamer's merged chat and lets you vote, predict and use points. When you chat from the extension, your message goes from your browser straight to the platform using the session you are already signed in with; your platform cookies are never sent to All-Chat. The extension contains no analytics or telemetry."
    )
    expect(t('legal.privacy.logDataIntro')).toBe(
      'For observability and abuse prevention we record:'
    )
    expect(t('legal.privacy.logDataIp')).toBe(
      'Your IP address, anonymised before it is logged (IPv4: last octet zeroed; IPv6: shortened to the first 48 bits), your browser user agent and basic request info'
    )
    expect(t('legal.privacy.logDataRateLimit')).toBe(
      'For rate limiting, your full IP address as a counter key for one minute; it is not logged'
    )
    expect(t('legal.privacy.logDataMetrics')).toBe(
      'API usage metrics, cache hits, and connection status'
    )
    expect(t('legal.privacy.logDataTraces')).toBe('Error traces and diagnostic logs')
    expect(t('legal.privacy.patreonIntro')).toBe(
      'Premium features are unlocked through a paid membership on Patreon. If you connect your Patreon account to All-Chat, we store:'
    )
    expect(t('legal.privacy.patreonUserId')).toBe('Your Patreon user ID')
    expect(t('legal.privacy.patreonTokens')).toBe(
      'Encrypted OAuth access and refresh tokens for the Patreon API'
    )
    expect(t('legal.privacy.patreonMembership')).toBe(
      'Your membership state for our campaign (status, tier, pledge amount, last and next charge date)'
    )
    expect(t('legal.privacy.patreonNoPaymentData')).toBe(
      'We do {not} receive or store your payment details; payments are processed entirely by Patreon. Patreon also sends us your email address and full name when you connect and with membership updates; we do not store them. Patreon notifies us via webhooks when your membership changes, and a periodic reconciliation keeps the state current. Disconnecting Patreon in the Settings page deletes the stored tokens and revokes subscription-derived premium.'
    )
    expect(t('legal.privacy.notEmphasis')).toBe('not')
    expect(t('legal.privacy.supportEmailBody')).toBe(
      'If you email us, we use your message and email address to answer you. Our support mailbox is hosted by Google (Gmail).'
    )
    expect(t('legal.privacy.supportDiscordBody')).toBe(
      "In our Discord server, the support bot reads messages that mention it or are posted in its threads, together with the usernames and thread history it needs for context, and answers using a language model run by an external AI inference provider acting as our processor. The bot keeps short notes to remember recurring problems and can turn a support request into a public issue on our GitHub repository, so do not post anything in a support thread that you do not want to be public. Discord processes your messages under Discord's own privacy policy."
    )
    expect(t('legal.privacy.useIntro')).toBe('Everything we store directly supports the service:')
    expect(t('legal.privacy.useAuthenticate')).toBe(
      'Authenticate against the platforms you authorize'
    )
    expect(t('legal.privacy.useFetch')).toBe(
      'Receive live chat and events and normalize them into a single feed'
    )
    expect(t('legal.privacy.useRender')).toBe(
      'Render overlays, alerts, goals, lists and credit rolls, look up emotes, badges, avatars and pronouns, and apply your filters'
    )
    expect(t('legal.privacy.useAct')).toBe(
      'Carry out what you ask for: send chat messages, moderate, create clips, run chat commands, timers, polls, predictions and points, and read messages aloud'
    )
    expect(t('legal.privacy.usePremium')).toBe('Unlock premium features based on your membership')
    expect(t('legal.privacy.useMonitor')).toBe('Monitor service reliability and debug incidents')
    expect(t('legal.privacy.useAbuse')).toBe('Detect and prevent abuse (rate-limiting, bans)')
    expect(t('legal.privacy.storagePostgres')).toBe(
      'PostgreSQL for account data, overlays, configuration and encrypted tokens'
    )
    expect(t('legal.privacy.storageRedis')).toBe(
      'Redis for sessions, message fan-out, short-lived buffers, leaderboards, caches and rate limiting'
    )
    expect(t('legal.privacy.storageMedia')).toBe('Object storage for the media you upload')
    expect(t('legal.privacy.storageLocation')).toBe(
      'All of the above runs on our own infrastructure on servers located in {country} (hosting provider: Hetzner Online GmbH; data centres in Falkenstein and Nuremberg, Germany, and Helsinki, Finland)'
    )
    expect(t('legal.privacy.storageCountry')).toBe('Germany and Finland')
    expect(t('legal.privacy.storageBackups')).toBe(
      'Backups of the database and of uploaded alert media are kept outside that infrastructure, in Hetzner Object Storage (Falkenstein, Germany), and are transferred there over encrypted connections (see Sections\u00A05.9 and 6)'
    )
    expect(t('legal.privacy.storageCloudflare')).toBe(
      'All traffic to allch.at passes through Cloudflare, Inc. (US), which provides DNS, TLS and protection against attacks as our processor. Cloudflare therefore sees your IP address and the requests you make, including overlay and chat traffic.'
    )
    expect(t('legal.privacy.safeguardsEncryption')).toBe(
      'OAuth tokens and API keys encrypted with AES-GCM before they reach the database; access tokens you create are stored only as hashes'
    )
    expect(t('legal.privacy.safeguardsHttps')).toBe(
      'HTTPS for all external traffic; internal services isolated via Kubernetes network policies'
    )
    expect(t('legal.privacy.safeguardsAccess')).toBe(
      'Role-scoped infrastructure access and audit logging. Administrators can look up accounts and overlays and can temporarily act as a user to resolve support issues; every such session is logged'
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
    expect(t('legal.privacy.platformApisIntro')).toBe(
      'To deliver the product we connect to the platforms you add. They receive your channel identifiers and, when we act on your behalf, your tokens and the messages or moderation actions you send:'
    )
    expect(t('legal.privacy.platformApisTwitch')).toBe(
      'Twitch (Twitch Interactive, Inc., US): EventSub, Helix and chat APIs'
    )
    expect(t('legal.privacy.platformApisYoutube')).toBe(
      'YouTube (Google LLC, US): public live chat endpoints, the YouTube Data API and the live chat API'
    )
    expect(t('legal.privacy.platformApisKick')).toBe(
      "Kick (Kick Streaming Pty Ltd, Australia): Kick's public and official APIs and its realtime service"
    )
    expect(t('legal.privacy.platformApisTiktok')).toBe(
      'TikTok: public TikTok LIVE endpoints (TikTok offers no official chat API). When TikTok cannot find a handle from our servers, we repeat only that lookup, with the public handle, through a relay we run on Google Cloud in the US'
    )
    expect(t('legal.privacy.platformApisDiscord')).toBe(
      'Discord (Discord Inc., US): the Gateway and REST APIs, and webhooks that relay chat from your other platforms into the Discord channel you choose'
    )
    expect(t('legal.privacy.platformApisBeta')).toBe(
      'In beta or early access: Owncast (the server you enter), GoodGame, Picarto, Altstream, and Facebook and Instagram (Meta Platforms, Inc., US)'
    )
    expect(t('legal.privacy.platformApisEmotes')).toBe(
      '7TV, BetterTTV and FrankerFaceZ for emote sets (only channel IDs are sent)'
    )
    expect(t('legal.privacy.platformApisPronouns')).toBe(
      'The alejo.io pronoun service: we send the Twitch username of chatters to show their pronouns, and cache the answer for 24 hours'
    )
    expect(t('legal.privacy.platformApisScopes')).toBe(
      "Every integration remains subject to the platform's own policies and the scopes you approve."
    )
    expect(t('legal.privacy.browserResourcesIntro')).toBe(
      'Pages that show chat (the overlay in OBS, the editor preview, viewer pages and the browser extension) load images directly from the platforms and emote providers. Each request transmits your {emphasis} and browser user agent to that provider:'
    )
    expect(t('legal.privacy.ipAddressEmphasis')).toBe('IP address')
    expect(t('legal.privacy.browserResourcesPlatforms')).toBe(
      'Avatars, badges, emotes and attachments from Twitch, YouTube, Kick, TikTok and Discord, GIFs in Twitch chat (Giphy), and in beta Picarto and Altstream'
    )
    expect(t('legal.privacy.browserResourcesEmotes')).toBe(
      'Emote images from 7TV, BetterTTV and FrankerFaceZ'
    )
    expect(t('legal.privacy.browserResourcesKlipy')).toBe(
      '{label} (api.klipy.com) – GIF search in the alerts editor sends your search terms; the GIF you pick is then loaded from KLIPY wherever the alert plays'
    )
    expect(t('legal.privacy.klipyLabel')).toBe('KLIPY')
    expect(t('legal.privacy.browserResourcesGithub')).toBe(
      '{label} (api.github.com) – loads community themes from our public repository in the credit-roll theme marketplace'
    )
    expect(t('legal.privacy.githubApiLabel')).toBe('GitHub API')
    expect(t('legal.privacy.browserResourcesClips')).toBe(
      "{label} – the credit roll requests clips from Twitch and plays them from Twitch's servers; if that fails, it embeds Twitch's clip player, which may set Twitch cookies"
    )
    expect(t('legal.privacy.clipsLabel')).toBe('Twitch clips')
    expect(t('legal.privacy.browserResourcesAlertMedia')).toBe(
      '{label} – images and sounds load from the addresses the streamer configures'
    )
    expect(t('legal.privacy.alertMediaLabel')).toBe('Alert media')
    expect(t('legal.privacy.processorsIntro')).toBe(
      'Besides our hosting provider and Cloudflare (Section\u00A04.1), these services receive data only when you use the feature that needs them:'
    )
    expect(t('legal.privacy.processorsElevenlabs')).toBe(
      "{label} – if you turn on ElevenLabs text-to-speech with your own API key, the text of each message read aloud, optionally preceded by the sender's display name, is sent from our servers to ElevenLabs under your ElevenLabs account"
    )
    expect(t('legal.privacy.elevenlabsLabel')).toBe('ElevenLabs (US)')
    expect(t('legal.privacy.processorsPatreon')).toBe(
      '{label} – your membership, see Section\u00A02.7'
    )
    expect(t('legal.privacy.patreonLabel')).toBe('Patreon (US)')
    expect(t('legal.privacy.processorsGoogleCloud')).toBe(
      '{label} – the TikTok handle lookup relay (Section\u00A05.1) and our support mailbox (Section\u00A02.8)'
    )
    expect(t('legal.privacy.googleLabel')).toBe('Google')
    expect(t('legal.privacy.processorsAi')).toBe(
      '{label} – messages addressed to the support bot in our Discord server (Section\u00A02.8)'
    )
    expect(t('legal.privacy.aiLabel')).toBe('AI inference provider')
    expect(t('legal.privacy.streamerRequestsBody')).toBe(
      "A streamer's chat commands can fetch an address the streamer chooses. That request comes from our servers and can include the name of the viewer who used the command and the text they wrote; who receives it depends only on the streamer's configuration. Likewise, custom code in an alert runs in a sandbox in the browser that shows the alert and can contact any address the streamer's code names."
    )
    expect(t('legal.privacy.fontsBody')).toBe(
      'Typography assets originally distributed by Google Fonts are {selfHosted}. The fonts used by the All-Chat interface are bundled at build time via Next.js, and fonts selectable for overlay customization are served through a server-side proxy at {proxyPath}. Your browser only connects to the All-Chat origin; {noTransmission} when fonts are loaded. This aligns with the Landgericht München I ruling on Google Fonts (20 January 2022, Az. 3 O 17493/20).'
    )
    expect(t('legal.privacy.fontsSelfHostedEmphasis')).toBe('self-hosted on our infrastructure')
    expect(t('legal.privacy.fontsNoTransmissionEmphasis')).toBe(
      'no IP address, user agent, or request metadata is transmitted to Google'
    )
    expect(t('legal.privacy.youtubeNoticeBody')).toBe(
      "Your use of All-Chat's YouTube integration is also governed by the {googlePolicy}. This applies only to the YouTube API integration (data flows described in Section\u00A05.1); it does {not} apply to fonts, which are self-hosted as described in Section\u00A05.5."
    )
    expect(t('legal.privacy.googlePolicyLinkText')).toBe('Google Privacy Policy')
    expect(t('legal.privacy.noSalesBody')).toBe(
      'We never sell or rent your data. We may disclose information when required by law or to respond to legitimate security incidents.'
    )
    expect(t('legal.privacy.analyticsBody')).toBe(
      'To understand how the site is used and where to improve it, we run {umami}, an open-source analytics tool that we {selfHost}. It is {cookieless}: it sets no cookies, creates no persistent identifier, and performs no cross-site or cross-device tracking. The data is processed on our own infrastructure and is {notShared} (unlike Google Analytics or similar services).'
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
    expect(t('legal.privacy.analyticsIpNote')).toBe(
      'Your {ipNotStored}: it is used only momentarily to derive the country and to generate a daily, salted hash for counting unique visits, after which it is discarded. Public overlay pages (the pages OBS loads as a browser source) are only counted as an aggregate overlay view, with the overlay ID removed from the address. Analytics run only on allch.at, not on beta.allch.at. You can block the analytics script with any browser content blocker without affecting the site.'
    )
    expect(t('legal.privacy.analyticsIpNotStoredEmphasis')).toBe('IP address is not stored')
    expect(t('legal.privacy.analyticsConsentNote')).toBe(
      'Because the tracker stores no information on, and reads none from, your device, it does not require consent under §\u00A025 TDDDG; the processing of the resulting data rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in measuring and improving the service.'
    )
    expect(t('legal.privacy.backupsBody')).toBe(
      'So that the service can be restored after data loss, we keep backups of the database (including account and overlay data, encrypted OAuth tokens and the chat messages and logs retained under Section\u00A06) and of uploaded alert media with Hetzner Online GmbH (Germany), acting as a processor under Art.\u00A028 DSGVO. The provider only stores the backups; we read them only to restore the service. The processing rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in not losing your data.'
    )
    expect(t('legal.privacy.retentionAccount')).toBe(
      '{label} kept until you delete them or your account'
    )
    expect(t('legal.privacy.retentionAccountLabel')).toBe('Account, overlay and feature settings:')
    expect(t('legal.privacy.retentionMedia')).toBe(
      '{label} deleted with the item they belong to or with your account'
    )
    expect(t('legal.privacy.retentionMediaLabel')).toBe('Uploaded media:')
    expect(t('legal.privacy.retentionTokens')).toBe(
      '{label} deleted when you disconnect a platform or delete your account; credentials that can no longer be used are deleted 7 days after they expire'
    )
    expect(t('legal.privacy.retentionTokensLabel')).toBe('OAuth tokens:')
    expect(t('legal.privacy.retentionSentMessages')).toBe(
      '{label} automatically deleted after 1 hour'
    )
    expect(t('legal.privacy.retentionSentMessagesLabel')).toBe(
      'Chat messages sent through All-Chat:'
    )
    expect(t('legal.privacy.retentionDisplayedMessages')).toBe(
      '{label} not stored in our database; up to 5 minutes in the replay buffer, up to 7 days if processing failed'
    )
    expect(t('legal.privacy.retentionDisplayedMessagesLabel')).toBe(
      'Chat messages displayed in overlays:'
    )
    expect(t('legal.privacy.retentionAlerts')).toBe('{label} deleted after 30 days')
    expect(t('legal.privacy.retentionAlertsLabel')).toBe('Alert log:')
    expect(t('legal.privacy.retentionLeaderboards')).toBe(
      '{label} a rolling 30 days for list overlays, 48 hours for credit rolls'
    )
    expect(t('legal.privacy.retentionLeaderboardsLabel')).toBe('Leaderboards:')
    expect(t('legal.privacy.retentionEngagement')).toBe(
      '{label} kept until the streamer deletes the overlay or you delete your viewer data'
    )
    expect(t('legal.privacy.retentionEngagementLabel')).toBe('Votes, predictions and points:')
    expect(t('legal.privacy.retentionModeration')).toBe('{label} deleted after 90 days')
    expect(t('legal.privacy.retentionModerationLabel')).toBe('Moderation action log:')
    expect(t('legal.privacy.retentionLogs')).toBe(
      '{label} logs and traces for 7 days, metrics for 15 days'
    )
    expect(t('legal.privacy.retentionLogsLabel')).toBe('Usage logs:')
    expect(t('legal.privacy.retentionCaches')).toBe('{label} 24 hours')
    expect(t('legal.privacy.retentionCachesLabel')).toBe('Avatar and pronoun caches:')
    expect(t('legal.privacy.retentionQuotaLogs')).toBe('{label} deleted after 30 days')
    expect(t('legal.privacy.retentionQuotaLogsLabel')).toBe('YouTube quota audit logs:')
    expect(t('legal.privacy.retentionBackups')).toBe(
      '{label} kept for 30 days, after which the storage provider erases them within a further 30 days. Data that you delete, or that expires under the periods above, can therefore remain in backups for up to 60 days; it is not used for anything other than restoring the service'
    )
    expect(t('legal.privacy.retentionBackupsLabel')).toBe('Backups:')
    expect(t('legal.privacy.rightsIntro')).toBe('You can exercise the following at any time:')
    expect(t('legal.privacy.rightsAccess')).toBe(
      '{label} Request a copy of the data we store about you'
    )
    expect(t('legal.privacy.rightsAccessLabel')).toBe('Access (Art.\u00A015):')
    expect(t('legal.privacy.rightsRectification')).toBe('{label} Correct inaccurate information')
    expect(t('legal.privacy.rightsRectificationLabel')).toBe('Rectification (Art.\u00A016):')
    expect(t('legal.privacy.rightsErasure')).toBe(
      '{label} Delete your account and all associated data from the Settings page, delete your viewer data from the viewer settings, or contact us'
    )
    expect(t('legal.privacy.rightsErasureLabel')).toBe('Erasure (Art.\u00A017):')
    expect(t('legal.privacy.rightsRestriction')).toBe(
      '{label} Request restriction of processing in certain circumstances'
    )
    expect(t('legal.privacy.rightsRestrictionLabel')).toBe('Restriction (Art.\u00A018):')
    expect(t('legal.privacy.rightsPortability')).toBe(
      '{label} Download your data in a machine-readable format (JSON) from the Settings page or the viewer settings'
    )
    expect(t('legal.privacy.rightsPortabilityLabel')).toBe('Data portability (Art.\u00A020):')
    expect(t('legal.privacy.rightsObjection')).toBe(
      '{label} Object to processing based on legitimate interest'
    )
    expect(t('legal.privacy.rightsObjectionLabel')).toBe('Objection (Art.\u00A021):')
    expect(t('legal.privacy.rightsWithdraw')).toBe(
      '{label} Disconnect platforms, unlink viewer identities or leave the ambassador showcase at any time'
    )
    expect(t('legal.privacy.rightsWithdrawLabel')).toBe('Withdraw consent:')
    expect(t('legal.privacy.rightsContact')).toBe(
      'Contact us at {email} or use the Settings page. You also have the right to lodge a complaint with your supervisory authority (Aufsichtsbehörde).'
    )
    expect(t('legal.privacy.rightsChatters')).toBe(
      'If you wrote in a channel that uses All-Chat but have never used All-Chat yourself, you can exercise these rights by contacting us with your platform and username.'
    )
    expect(t('legal.privacy.noProfiling')).toBe(
      'We do not use automated decision-making or profiling within the meaning of Art.\u00A022 DSGVO.'
    )
    expect(t('legal.privacy.youtubeRevoke')).toBe(
      "For YouTube Data: You can revoke All-Chat's access to your YouTube data via the {googleSettings}."
    )
    expect(t('legal.privacy.googleSettingsPageLinkText')).toBe('Google security settings page')
    expect(t('legal.privacy.cookiesIntro')).toBe(
      'All-Chat uses {emphasis} (not cookies) for essential functionality:'
    )
    expect(t('legal.privacy.localStorageEmphasis')).toBe('browser localStorage')
    expect(t('legal.privacy.cookiesTokens')).toBe(
      'Authentication tokens (JWT) to keep you logged in'
    )
    expect(t('legal.privacy.cookiesPreferences')).toBe(
      'User preferences, cached theme lists and last-visited state'
    )
    expect(t('legal.privacy.cookiesNoAdvertising')).toBe(
      "We do not use advertising cookies or cross-site tracking. We do use {emphasis} (self-hosted Umami) – it sets nothing on your device and stores no personal identifier; see Section\u00A05.8 for the full description. Fonts are self-hosted (Section\u00A05.5) and do not set cookies. The only third-party cookies that can occur come from Twitch's clip player, which the credit roll embeds as a fallback (Section\u00A05.2)."
    )
    expect(t('legal.privacy.cookielessAnalyticsEmphasis')).toBe(
      'privacy-friendly, cookieless usage analytics'
    )
    expect(t('legal.privacy.childrenBody')).toBe(
      'All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data processing in Germany). If we discover data belonging to a minor we will delete it immediately.'
    )
    expect(t('legal.privacy.transfersBody')).toBe(
      'All-Chat itself is hosted on servers located in Germany and Finland, both within the EU (see Section\u00A04.1). Some recipients are outside the EU: Cloudflare (US), Twitch and Amazon (US), Google, YouTube and Google Cloud (US), TikTok (various), Kick (Australia), Discord (US), Meta (US, beta), Patreon (US), GitHub (US), ElevenLabs (US, only with your own API key), and the emote, GIF and pronoun services named in Section\u00A05.'
    )
    expect(t('legal.privacy.transfersLegalBasis')).toBe(
      "Where a provider is certified under the EU–US Data Privacy Framework, the transfer rests on the EU Commission's adequacy decision (Art.\u00A045 DSGVO). Otherwise, the transfer is necessary to perform the service you requested (Art.\u00A049(1)(b) DSGVO). Fonts are self-hosted on our infrastructure and therefore do not involve any third-country transfer when loaded."
    )
    expect(t('legal.privacy.updatesBody')).toBe(
      "We'll post updates to this page when the policy changes and include a new {lastUpdatedLabel} date. Significant changes will be announced inside the dashboard."
    )
    expect(t('legal.privacy.updatesLastUpdatedEmphasis')).toBe('Last Updated')
    expect(t('legal.privacy.openSourceAuditable')).toBe(
      'All source code is publicly auditable under AGPL-3.0'
    )
    expect(t('legal.privacy.openSourceNoTracking')).toBe(
      'No hidden tracking — our analytics are cookieless and documented (Section\u00A05.8), and the whole stack is verifiable on GitHub'
    )
    expect(t('legal.privacy.openSourceScrutiny')).toBe(
      'Privacy practices are open to community scrutiny'
    )
    expect(t('legal.privacy.openSourceRepository')).toBe('Source: {repository}')
    expect(t('legal.privacy.repositoryLinkText')).toBe('github.com/caesarakalaeii/all-chat')
    expect(t('legal.privacy.twitchNote')).toBe(
      'Through EventSub we receive chat, channel events (follows, subscriptions, bits, raids, channel point redemptions) and, if you enable them, polls, predictions and moderation events. Optional permissions you grant let All-Chat send chat messages, moderate, create clips and post as a bot. We do not access payment data or channel analytics.'
    )
    expect(t('legal.privacy.youtubeNote')).toBe(
      "We read live chat through YouTube's public live chat endpoints or, in the premium official-API mode, the YouTube live chat API. With your permission we read your most recent subscribers for alerts and your subscriber count for goals, and optional permissions let All-Chat send messages and moderate. We do not access video content or channel analytics. API usage is subject to YouTube's quota limits. You can revoke access via {googleSettings} or the All-Chat Settings page. Disconnecting deletes your stored OAuth tokens."
    )
    expect(t('legal.privacy.googleSettingsLinkText')).toBe('Google security settings')
    expect(t('legal.privacy.tiktokNote')).toBe(
      'TikTok offers no official chat API, so you only enter your TikTok handle and no TikTok login is involved. We read the public LIVE chat and events (gifts, likes, follows, shares) without accessing your videos, followers or other content. Remove the source to stop.'
    )
    expect(t('legal.privacy.kickNote')).toBe(
      "We use Kick's OAuth for sign-in and sources and connect to Kick's realtime service to receive chat. Optional permissions let All-Chat send messages and moderate. We do not access channel analytics or payment data."
    )
    expect(t('legal.privacy.discordNote')).toBe(
      'When you add the All-Chat bot to a server, we store the server ID and name and read messages, including content and attachments, in the channels you configure. Chat from your other platforms can be relayed into a Discord channel through a webhook. We do not read direct messages or channels you did not configure.'
    )
    expect(t('legal.privacy.owncastNote')).toBe(
      "You enter the address of an Owncast server. We connect to it anonymously, read its public configuration and register an anonymous chat user there to receive chat. The server's operator, not All-Chat, controls the data on that server."
    )
    expect(t('legal.privacy.publicChatNote')).toBe(
      'We read public chat anonymously using the channel name you enter. No login with these platforms is involved, and we never post or moderate there.'
    )
    expect(t('legal.privacy.metaNote')).toBe(
      'You connect a Facebook Page or an Instagram professional account through Meta. We poll live video comments and, on Facebook, can delete comments and block users on your instruction; Instagram is read-only. Both stay inactive until Meta has approved All-Chat in its app review.'
    )
    expect(t('legal.privacy.contactEmailRow')).toBe('Email: {email}')
    expect(t('legal.privacy.contactHostedNote')).toBe(
      'This contact is for users of the official hosted service at {host}. Self-hosted installations should contact their own administrator.'
    )
    expect(t('legal.privacy.hostedDomain')).toBe('allch.at')
  })
})

describe('terms of service copy', () => {
  it('keeps every terms of service string whole', () => {
    expect(t('legal.terms.title')).toBe('Terms of Service (Nutzungsbedingungen)')
    expect(t('legal.terms.lastUpdated')).toBe('November 2, 2026')
    expect(t('legal.terms.acceptanceHeading')).toBe('1. Acceptance of Terms')
    expect(t('legal.terms.acceptanceBody')).toBe(
      'By accessing or using All-Chat you agree to these Terms of Service and our {privacy}. If you disagree with any part, you should discontinue use immediately.'
    )
    expect(t('legal.terms.privacyLinkText')).toBe('Privacy Policy')
    expect(t('legal.terms.descriptionHeading')).toBe('2. Description of Service')
    expect(t('legal.terms.descriptionBody')).toBe(
      'All-Chat combines live chat and events from Twitch, YouTube, Kick, TikTok and Discord, and in beta or early access from Owncast, GoodGame, Picarto, Altstream, Facebook and Instagram, into overlays you show on your stream: chat, alerts, goals, lists and credit rolls. It also lets you send chat messages, moderate across platforms, run chat commands, bots, timers, polls, predictions and a points game, have chat read aloud, and connect the browser extension, desktop plugins and API tokens. You broadcast overlays via OBS or any browser source.'
    )
    expect(t('legal.terms.accountsHeading')).toBe('3. Accounts & Authentication')
    expect(t('legal.terms.accountsIntro')).toBe(
      'You are responsible for all activity that happens under your account. You agree to:'
    )
    expect(t('legal.terms.accountsAccurate')).toBe('Provide accurate registration details')
    expect(t('legal.terms.accountsSecurity')).toBe(
      'Maintain the security of your credentials and OAuth grants'
    )
    expect(t('legal.terms.accountsAccessTokens')).toBe(
      'Keep personal access tokens and paired-device links secret; anything done with them counts as done by you'
    )
    expect(t('legal.terms.accountsModerators')).toBe(
      'Invite moderators with care; they act on your overlays within the permissions you grant them'
    )
    expect(t('legal.terms.accountsNotify')).toBe('Notify us at {email} if you suspect compromise')
    expect(t('legal.terms.accountsComply')).toBe(
      'Comply with the terms of every platform you connect'
    )
    expect(t('legal.terms.supportEmail')).toBe('all.chat.support@gmail.com')
    expect(t('legal.terms.acceptableUseHeading')).toBe('4. Acceptable Use')
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
    expect(t('legal.terms.acceptableUseAutomation')).toBe(
      "Using custom code, chat command URL fetches or bots to attack, overload or scrape other services, or to collect viewers' data"
    )
    expect(t('legal.terms.acceptableUseSpam')).toBe(
      'Flooding chats with bots, timers or automated messages'
    )
    expect(t('legal.terms.contentHeading')).toBe('5. Your Content')
    expect(t('legal.terms.contentOwnership')).toBe(
      'You keep all rights to what you create or upload, such as custom code and CSS, media, chat commands and translations. You grant us the non-exclusive right to store, process and display it as far as needed to run the Service for you.'
    )
    expect(t('legal.terms.contentResponsibility')).toBe(
      'You are responsible for your content and for what your custom code, chat commands, bots and alert media do. Uploaded media can be opened by anyone who has its address. We may remove content that breaks these Terms or the law.'
    )
    expect(t('legal.terms.contentTranslations')).toBe(
      'Translations you contribute are published as part of All-Chat under the AGPL-3.0. By submitting them you confirm that you may license them on those terms.'
    )
    expect(t('legal.terms.thirdPartyHeading')).toBe('6. Third-Party Integrations')
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
    expect(t('legal.terms.thirdPartyUnofficial')).toBe(
      "Some integrations (TikTok, YouTube's public chat endpoints, Picarto, Twitch clip playback) use unofficial interfaces that the platform can change or block at any time"
    )
    expect(t('legal.terms.thirdPartyOwnAccounts')).toBe(
      'Services you connect with your own account or API key, such as ElevenLabs or Patreon, are governed by their own terms'
    )
    expect(t('legal.terms.youtubeBinding')).toBe(
      'YouTube Integration: By using All-Chat to connect to YouTube, you agree to be bound by the {youtubeTerms}.'
    )
    expect(t('legal.terms.youtubeTermsLinkText')).toBe('YouTube Terms of Service')
    expect(t('legal.terms.betaHeading')).toBe('7. Beta & Early Access')
    expect(t('legal.terms.betaBody')).toBe(
      'Features and platforms offered on beta.allch.at or as early access are previews. They can change, break or be removed without notice. beta.allch.at uses the same account and data as allch.at.'
    )
    expect(t('legal.terms.pointsHeading')).toBe('8. Points, Polls & Predictions')
    expect(t('legal.terms.pointsBody')).toBe(
      "Points are a virtual score a streamer awards within their own overlay. They have no monetary value and cannot be bought, sold, transferred to another streamer's overlay or paid out. Predictions are a game played with these points only. Streamers are responsible for running polls, predictions and points in line with the platforms' rules and the law that applies to them."
    )
    expect(t('legal.terms.privacyHeading')).toBe('9. Privacy')
    expect(t('legal.terms.privacyBody')).toBe(
      'Your use of All-Chat is also governed by our {privacy}, which explains what we collect, how it is used, and your rights under the DSGVO. For transparency: All-Chat measures aggregate usage with {analytics} (Umami) that set nothing on your device and store no personal identifier – see Section\u00A05.8 of the Privacy Policy.'
    )
    expect(t('legal.terms.privacyAnalyticsEmphasis')).toBe('self-hosted, cookieless analytics')
    expect(t('legal.terms.licenseHeading')).toBe('10. Open Source License')
    expect(t('legal.terms.licenseBody')).toBe(
      'All-Chat is released under the {license}. That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.'
    )
    expect(t('legal.terms.licenseLinkText')).toBe(
      'GNU Affero General Public License v3.0 (AGPL-3.0)'
    )
    expect(t('legal.terms.licenseRepository')).toBe(
      'The canonical repository is available on {github}.'
    )
    expect(t('legal.terms.githubLinkText')).toBe('GitHub')
    expect(t('legal.terms.availabilityHeading')).toBe('11. Availability & Support')
    expect(t('legal.terms.availabilityIntro')).toBe('We aim for high uptime but do not guarantee:')
    expect(t('legal.terms.availabilityUptime')).toBe('Uninterrupted access or zero bugs')
    expect(t('legal.terms.availabilityCompat')).toBe(
      'Compatibility with every browser or streamer setup'
    )
    expect(t('legal.terms.availabilityFixes')).toBe('Immediate fixes or feature requests')
    expect(t('legal.terms.availabilitySupport')).toBe(
      'Support for the hosted service at {host} is best-effort. Community/self-hosted deployments must rely on their own maintainers or the open source community for assistance.'
    )
    expect(t('legal.terms.availabilityStatus')).toBe(
      'The current service status is published at {status}.'
    )
    expect(t('legal.terms.statusHost')).toBe('status.allch.at')
    expect(t('legal.terms.hostedDomain')).toBe('allch.at')
    expect(t('legal.terms.liabilityHeading')).toBe('12. Limitation of Liability')
    expect(t('legal.terms.liabilityGross')).toBe(
      'We are liable without limitation for damages caused intentionally or by gross negligence, for injury to life, body, or health, and under the German Product Liability Act (Produkthaftungsgesetz).'
    )
    expect(t('legal.terms.liabilitySlight')).toBe(
      'In cases of slight negligence, we are liable only for the breach of essential contractual obligations (Kardinalpflichten): obligations whose fulfilment makes the proper performance of the contract possible in the first place and on whose fulfilment you may regularly rely. In such cases, our liability is limited to the damage that is foreseeable and typical for this type of service. Any further liability is excluded.'
    )
    expect(t('legal.terms.liabilityAgents')).toBe(
      'These limitations also apply in favour of our legal representatives and vicarious agents (Erfüllungsgehilfen).'
    )
    expect(t('legal.terms.indemnityHeading')).toBe('13. Indemnification')
    expect(t('legal.terms.indemnityBody')).toBe(
      'You agree to indemnify All-Chat against third-party claims, including the reasonable costs of legal defence, arising from your culpable violation of these Terms, applicable law, or the rights of others. This does not apply to the extent you are not responsible for the violation.'
    )
    expect(t('legal.terms.premiumHeading')).toBe(
      '14. Premium Subscriptions & Right of Withdrawal (Widerrufsrecht)'
    )
    expect(t('legal.terms.patreonLinkText')).toBe('Patreon')
    expect(t('legal.terms.premiumBody')).toBe(
      "The core All-Chat service is free of charge. Premium features for streamers, and the cosmetic viewer premium, are unlocked through a paid membership on {patreon}: you subscribe to our campaign on patreon.com and then connect your Patreon account to All-Chat. The subscription contract, billing, cancellation, and any statutory right of withdrawal for the paid membership are handled by Patreon under Patreon's own terms. All-Chat itself does not charge you and does not process payments."
    )
    expect(t('legal.terms.premiumGrants')).toBe(
      'We may grant premium access free of charge, for example to beta testers and ambassadors, and may end such grants at any time.'
    )
    expect(t('legal.terms.premiumCancellation')).toBe(
      'You may stop using All-Chat and delete your account at any time via the Settings page. Upon deletion, your personal data is removed as described in our Privacy Policy. Note that deleting your All-Chat account does not cancel a Patreon membership; cancel it directly on Patreon.'
    )
    expect(t('legal.terms.changesHeading')).toBe('15. Changes to Terms')
    expect(t('legal.terms.changesBody')).toBe(
      'We may update these Terms over time. Material changes will be announced in the dashboard, and the new version will be posted here with an updated effective date.'
    )
    expect(t('legal.terms.terminationHeading')).toBe('16. Termination')
    expect(t('legal.terms.terminationBody')).toBe(
      'We reserve the right to suspend or terminate your access for any breach of these Terms or abusive behavior, and to remove content and connections linked to a suspended account. You may stop using the Service at any time by deleting your account in the Settings page.'
    )
    expect(t('legal.terms.governingLawHeading')).toBe('17. Governing Law & Jurisdiction')
    expect(t('legal.terms.governingLawBody')).toBe(
      'These Terms are governed by the laws of the Federal Republic of Germany, excluding the UN Convention on Contracts for the International Sale of Goods (CISG). If you are a consumer, the mandatory consumer protection provisions of the country in which you habitually reside remain unaffected. If you are a merchant (Kaufmann), a legal entity under public law, or a special fund under public law, the exclusive place of jurisdiction is the domicile of the operator.'
    )
    expect(t('legal.terms.legalNoticeHeading')).toBe('18. Legal Notice')
    expect(t('legal.terms.legalNoticeBody')).toBe(
      "The operator's identity and contact details are available in the {impressum}."
    )
    expect(t('legal.terms.impressumLinkText')).toBe('Impressum')
    expect(t('legal.terms.contactHeading')).toBe('19. Contact')
    expect(t('legal.terms.contactBody')).toBe(
      'Questions? Reach us at {email}. Hosted community forks should contact their own administrators.'
    )
  })
})

describe('legal rendered sentences', () => {
  it('keeps the page metadata', () => {
    expect(t('metadata.privacy.title')).toBe('Privacy Policy')
    expect(t('metadata.privacy.description')).toBe(
      'Learn how All-Chat collects, processes, and protects your information.'
    )
    expect(t('metadata.terms.title')).toBe('Terms of Service')
    expect(t('metadata.terms.description')).toBe(
      'Understand the rules and responsibilities for using All-Chat.'
    )
  })

  it('composes the sentences the way the pages fill them', () => {
    expect(
      t('legal.privacy.retentionTokens', { label: t('legal.privacy.retentionTokensLabel') })
    ).toBe(
      'OAuth tokens: deleted when you disconnect a platform or delete your account; credentials that can no longer be used are deleted 7 days after they expire'
    )
    expect(t('legal.terms.licenseBody', { license: t('legal.terms.licenseLinkText') })).toBe(
      'All-Chat is released under the GNU Affero General Public License v3.0 (AGPL-3.0). That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.'
    )
    expect(t('legal.terms.availabilityStatus', { status: t('legal.terms.statusHost') })).toBe(
      'The current service status is published at status.allch.at.'
    )
  })
})
