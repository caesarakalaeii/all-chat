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
      'All-Chat uses {storage} to keep you logged in and to remember your preferences. This is essential for the service to function properly.'
    )
    expect(t('legal.cookieBanner.storageEmphasis')).toBe(
      'two essential cookies and browser storage'
    )
    expect(t('legal.cookieBanner.noTrackingBody')).toBe(
      '{emphasis} We do not share your data with third parties for advertising purposes.'
    )
    expect(t('legal.cookieBanner.noTrackingEmphasis')).toBe(
      'We use no tracking cookies, and our usage analytics are cookieless.'
    )
  })

  it('keeps the actions and the footer', () => {
    expect(t('legal.cookieBanner.acknowledge')).toBe('I Understand')
    expect(t('legal.cookieBanner.learnMore')).toBe('Learn More')
    expect(t('legal.cookieBanner.footer')).toBe(
      'Your overlays, settings and platform tokens are stored on our servers, and traffic between your browser and All-Chat is encrypted with HTTPS'
    )
  })
})

describe('cookie banner disclosure list', () => {
  it('keeps the summary', () => {
    expect(t('legal.cookieBanner.detailsSummary')).toBe('What do we store in your browser?')
  })

  it('keeps each stored-data row whole', () => {
    // The bolded label opens each row and the rest of the row continues the
    // sentence, so the row is one string with a {label} placeholder. Splitting
    // at the hyphen would strand a fragment a translator cannot reorder.
    expect(t('legal.cookieBanner.tokensRow')).toBe(
      '{label} - Two essential cookies keep you logged in as a streamer, and viewer sign-in is kept in local storage. Your streaming platform tokens stay on our servers and never reach your browser'
    )
    expect(t('legal.cookieBanner.tokensLabel')).toBe('Sign-in')
    expect(t('legal.cookieBanner.preferencesRow')).toBe(
      '{label} - Your language, light or dark mode, chat view and editor layout, onboarding progress, dismissed notices and theme marketplace favorites. Your overlays and settings are stored on our servers'
    )
    expect(t('legal.cookieBanner.preferencesLabel')).toBe('User preferences')
    expect(t('legal.cookieBanner.noTrackingRow')).toBe(
      "{label} - We don't follow you across other websites"
    )
    expect(t('legal.cookieBanner.noTrackingLabel')).toBe('No tracking cookies')
    expect(t('legal.cookieBanner.noAdsRow')).toBe(
      "{label} - We don't serve ads or share data with advertisers"
    )
    expect(t('legal.cookieBanner.noAdsLabel')).toBe('No advertising cookies')
    expect(t('legal.cookieBanner.analyticsRow')).toBe(
      '{label} - We measure usage with self-hosted Umami. It sets no cookies and recognises returning visitors only by a pseudonymous hash that changes every month. Public overlay pages are counted too, tagged as overlay views'
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
      '{label} All-Chat only collects what we need to sign you in, connect your streaming platforms and run your overlays, alerts and chat tools. Tokens are encrypted, the chat shown in your overlays is not stored in our database apart from the records listed in Sections\u00A02.2 and 2.4, and we never sell your data. Section\u00A06 lists how long we keep each kind of data. We use cookieless, self-hosted analytics (no tracking cookies; see Section\u00A05.8).'
    )
    expect(t('legal.privacy.tldrLabel')).toBe('TL;DR:')
    expect(t('legal.privacy.openSourceCallout')).toBe(
      '{label} All-Chat is licensed under AGPL-3.0. Review the entire codebase—including how we store and process your data—on {github}.'
    )
    expect(t('legal.privacy.openSourceCalloutLabel')).toBe('Open Source Transparency:')
    expect(t('legal.privacy.githubLinkText')).toBe('GitHub')
    expect(t('legal.privacy.accountLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO – performance of a contract (providing the service you signed up for). Records of bans and of your acceptance of the Terms rest on Art.\u00A06(1)(f) DSGVO – our legitimate interest in preventing abuse and in being able to show what you agreed to.'
    )
    expect(t('legal.privacy.chatLegalBasis')).toBe(
      "Legal basis: Art.\u00A06(1)(b) DSGVO for you as the streamer. For the viewers whose messages and events we process, Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in showing chat and events posted in the streamer's channel to its audience, and in preventing abuse. Most of this chat is public. Some is posted to a narrower audience, such as members-only or unlisted YouTube streams and private Discord channels, and AutoMod holds are messages Twitch has not shown in chat."
    )
    expect(t('legal.privacy.featuresLegalBasis')).toBe(
      'Legal basis: Art.\u00A06(1)(b) DSGVO. The ambassador showcase rests on your consent (Art.\u00A06(1)(a) DSGVO), which you can withdraw at any time.'
    )
    expect(t('legal.privacy.viewerLegalBasis')).toBe(
      "Legal basis: Art.\u00A06(1)(b) DSGVO for the features you use. Linking several platform identities rests on your consent (Art.\u00A06(1)(a) DSGVO); you can unlink a platform at any time from the viewer settings, except the one you are signed in with and your last remaining one, which deleting your viewer data removes. Recording participation by chat command, and the points a streamer's rules award automatically, rest on Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in running the poll, prediction or points game."
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
      'For the platform you sign in with: your user ID, username, display name and profile image. For each platform you connect as a source: its user or channel ID and name'
    )
    expect(t('legal.privacy.accountTokens')).toBe(
      'Encrypted OAuth tokens with their scopes and, where the platform sets one, their expiry date, for each platform you authorise (Twitch, YouTube, Kick, and in beta Facebook and Instagram). Refresh tokens are stored where the platform issues them; Facebook Page tokens have none and do not expire'
    )
    expect(t('legal.privacy.accountNameOnly')).toBe(
      'For platforms you add by name or address only (TikTok, Owncast, GoodGame, Picarto, Altstream), just the channel name or server address you enter; for Discord, the server ID, name and icon and the channels you pick for the All-Chat bot, and, if you link your Discord account, its Discord user ID and username'
    )
    expect(t('legal.privacy.accountStatus')).toBe(
      'Your account status: premium, beta tester or ambassador status, admin role, and any ban with its reason'
    )
    expect(t('legal.privacy.accountTermsAcceptance')).toBe(
      'Whether you have accepted the current Terms of Service and this Privacy Policy, and when you last accepted them in the prompt shown after login. We do not record which version you accepted'
    )
    expect(t('legal.privacy.accountNoEmail')).toBe(
      'We do not store an email address from any chat platform. Kick includes your email address in the profile it returns at sign-in; we discard it.'
    )
    expect(t('legal.privacy.chatIntro')).toBe(
      'While an overlay or chat view is connected, we receive and process:'
    )
    expect(t('legal.privacy.chatMessages')).toBe(
      'Chat messages from the channels you connected, including emotes, GIFs and attachments'
    )
    expect(t('legal.privacy.chatAuthor')).toBe(
      'Per-message author details (platform user ID, username, display name, color, avatar, badges and roles, and where available their pronouns and the All-Chat name color, avatar frame and flair they chose)'
    )
    expect(t('legal.privacy.chatEvents')).toBe(
      'Channel events: on Twitch, follows, subscriptions, resubscriptions and gifted subscriptions, bits, cheers and Power-ups, raids, channel point redemptions and chat notices such as announcements, watch streaks, gift and Prime upgrades, pay-it-forward gifts and charity donations; on YouTube, Super Chats, Super Stickers, memberships, membership milestones and gifted memberships, and your most recent subscribers; on TikTok, gifts, likes, shares, follows, subscriptions, questions, Super Fan joins and boxes, and treasure chests'
    )
    expect(t('legal.privacy.chatModeration')).toBe(
      'Moderation events (deleted messages, timeouts, bans) and, if you turn on the moderation log, Twitch AutoMod holds and moderator actions, which are shown only to you'
    )
    expect(t('legal.privacy.chatRetention')).toBe(
      'Chat messages are {emphasis}. They pass through memory and short-lived queues: a replay buffer with the last 500 messages of each overlay, so a reconnecting overlay can catch up, which is deleted five minutes after its last message; an incoming queue that holds the most recent 100,000 messages across all channels and is continuously overwritten (Redis keeps it in an on-disk log, and at current traffic a message stays in it for less than a day); queues that hold the most recent 10,000 chat commands and Twitch poll and prediction events; and a seven-day queue for messages that failed processing so we can fix the fault.'
    )
    expect(t('legal.privacy.chatRetentionEmphasis')).toBe('not written to our database')
    expect(t('legal.privacy.chatDerived')).toBe(
      "Some features keep data derived from events: the alert log (the sender's platform user ID and name, the event type and amount, and the event's details as the platform sends them, such as a channel point reward with the text the viewer entered) for 30 days; list-overlay leaderboards (platform user IDs, names and scores) until 30 days pass without a new entry; and credit-roll session boards (platform user IDs, names, avatars and event details, including the full message of raids and mystery gifts) until 48 hours pass without a new entry."
    )
    expect(t('legal.privacy.chatStreamHistory')).toBe(
      "We also keep a record of each overlay's stream sessions (start and end, event counts and the total value of paid events) and, for each connected channel, when it was last seen live or offline and its typical streaming days and times."
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
      'Media you upload (images, audio and video up to 10\u00A0MiB each). Uploaded files are stored on our own servers and can be opened by anyone who has their address, so that they play in OBS. The address contains your account ID, a random part that makes it unguessable, and the name of the file you uploaded'
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
    expect(t('legal.privacy.featuresModerationLog')).toBe(
      "A log of the moderation actions you and your moderators take through All-Chat: who acted and for whom, the overlay, platform and channel, the action, the target's platform user ID and message ID, the reason and the outcome"
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
      'Viewers can sign in with Twitch, YouTube or Kick through the browser extension, the participation page or the viewer settings. For each sign-in we keep a session record with the platform user ID, username, display name, avatar and encrypted tokens (Twitch: no extra permissions; YouTube: read-only access and basic profile; Kick: basic profile) and counters of the chat messages you send, for rate limiting; signing out deletes it. We also store the name color, avatar frame, flair and viewer premium status you have. Unlinking a platform deletes its tokens and revokes them at the platform.'
    )
    expect(t('legal.privacy.viewerStreamerLink')).toBe(
      'If you sign in as a viewer with the same platform account that you use as a streamer on All-Chat, we link the viewer session to your streamer account automatically, so your viewer profile carries your premium status.'
    )
    expect(t('legal.privacy.viewerIdentityBody')).toBe(
      'If you choose to link several platform accounts (e.g.\u00A0Twitch + YouTube), we create a unified viewer profile that associates your platform identities. This is {emphasis}.'
    )
    expect(t('legal.privacy.viewerIdentityEmphasis')).toBe(
      'opt-in only and requires your explicit action'
    )
    expect(t('legal.privacy.viewerEngagementBody')).toBe(
      "When you vote in a poll or enter a prediction in a streamer's channel, including by typing a chat command without ever signing in, we create a viewer record for your platform account. It holds only the platform and your platform user ID, not your username. We store your votes and predictions, and your points balance and each points transaction, for that streamer's overlay. If the streamer has turned points on, you earn them under the streamer's rules for watching while signed in to the extension or the participation page (at most one entry per minute), and for subscribing, gifting subscriptions or memberships, cheering bits and sending Super Chats or Super Stickers. Earning points this way creates a viewer record for you too, even if you have never used All-Chat. Points exist only within that streamer's overlay."
    )
    expect(t('legal.privacy.viewerSentMessages')).toBe(
      'Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and {emphasis}.'
    )
    expect(t('legal.privacy.viewerSentMessagesEmphasis')).toBe(
      'automatically deleted after one hour'
    )
    expect(t('legal.privacy.extensionBody')).toBe(
      "The All-Chat browser extension runs on twitch.tv, youtube.com and kick.com. On each channel page you open it asks All-Chat whether that streamer uses All-Chat, which sends us the channel name together with your IP address, even when you are not signed in. It shows the streamer's merged chat without sign-in. Signed in, it also lets you vote, predict and use points, and while you watch it tells All-Chat once a minute that you are watching, so the streamer's points rules can award watch time. When you chat from the extension, your message goes from your browser straight to the platform using the session you are already signed in with; your platform cookies are never sent to All-Chat. The extension contains no analytics or telemetry."
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
      'API usage metrics, cache hits and connection status, some of them broken down by channel ID, YouTube video ID or Discord server ID'
    )
    expect(t('legal.privacy.logDataTraces')).toBe(
      'Traces of a 10% sample of all requests (method, path, host and user agent), error traces and diagnostic logs'
    )
    expect(t('legal.privacy.logDataIdentifiers')).toBe(
      'Where a request or background job concerns a particular account, logs can also contain the All-Chat user or viewer ID, platform usernames and channel names, a Patreon user ID or a Discord user ID'
    )
    expect(t('legal.privacy.patreonIntro')).toBe(
      'Premium features are unlocked through a paid membership on Patreon, or granted by us free of charge, for example to beta testers and ambassadors. If you connect your Patreon account to All-Chat, we store:'
    )
    expect(t('legal.privacy.patreonUserId')).toBe('Your Patreon user ID')
    expect(t('legal.privacy.patreonTokens')).toBe(
      'Encrypted OAuth access and refresh tokens for the Patreon API'
    )
    expect(t('legal.privacy.patreonMembership')).toBe(
      'Your membership state for our campaign (status, tier, pledge amount and next charge date)'
    )
    expect(t('legal.privacy.patreonUnlinked')).toBe(
      "If you support us on Patreon without connecting All-Chat, Patreon's notifications still reach us; we then store only your Patreon user ID and membership state, so the membership can be matched once you connect."
    )
    expect(t('legal.privacy.patreonNoPaymentData')).toBe(
      "We do {not} receive or store your payment details; payments are processed entirely by Patreon. Patreon's membership notifications also contain your email address, name and other profile details; we discard them without storing them. Patreon notifies us via webhooks when your membership changes, and a periodic reconciliation keeps the state current. Disconnecting Patreon in the Settings page deletes the stored tokens, unlinks the membership from your account and ends the premium that came from your subscription."
    )
    expect(t('legal.privacy.notEmphasis')).toBe('not')
    expect(t('legal.privacy.supportEmailBody')).toBe(
      'If you email us, we use your message and email address to answer you. Our support mailbox is hosted by Google (Gmail).'
    )
    expect(t('legal.privacy.supportDiscordBody')).toBe(
      "In our Discord server, the support bot reads messages that mention it, that are posted in its threads or that use its /support command, together with the usernames, the recent messages of the thread and any message you reply to (with its author's username) that it needs for context, and answers using a language model run by an external AI inference provider acting as our processor. To answer, it can look up our service logs, metrics and system status and pass redacted summaries of them to the model; these can concern your account or overlay. The bot also checks every message posted in the server for spam: if an account posts the same message in three or more channels within ten minutes, the bot bans it and deletes about a day of its messages. The bot keeps short notes to remember recurring problems and can turn a support request into a public issue on our GitHub repository, so do not post anything in a support thread that you do not want to be public. Discord processes your messages under Discord's own privacy policy."
    )
    expect(t('legal.privacy.useIntro')).toBe('We use the data we store to:')
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
    expect(t('legal.privacy.useRecords')).toBe(
      'Keep accountability records: your acceptance of the Terms, the moderation action log and the log of administrator act-as-user sessions'
    )
    expect(t('legal.privacy.useAnalytics')).toBe('Measure how the site is used (Section\u00A05.8)')
    expect(t('legal.privacy.useShowcase')).toBe(
      'Show the ambassadors who opted in on the public showcase'
    )
    expect(t('legal.privacy.storagePostgres')).toBe(
      'PostgreSQL for account data, overlays, configuration and encrypted tokens'
    )
    expect(t('legal.privacy.storageRedis')).toBe(
      'Redis for sessions, message fan-out, short-lived buffers, leaderboards, caches and rate limiting'
    )
    expect(t('legal.privacy.storageMedia')).toBe('Object storage for the media you upload')
    expect(t('legal.privacy.storageObservability')).toBe(
      'Our log, trace and metrics stores (Section\u00A02.6) and the analytics database (Section\u00A05.8)'
    )
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
      'HTTPS for all traffic between your browser and All-Chat, and encrypted connections to the platforms wherever they offer them. An Owncast server you add with an http:// address is contacted unencrypted. Internal services are isolated via Kubernetes network policies'
    )
    expect(t('legal.privacy.safeguardsAccess')).toBe(
      'Administrative access to the servers is limited to the operator. In our admin tools, administrators can look up accounts, overlays and viewers with their message counts, ban accounts, grant premium, and temporarily act as a user to resolve support issues; every act-as-user session is logged'
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
      'The alejo.io pronoun service: for overlays with pronoun display turned on (the default; you can switch it off per overlay), we send the Twitch username of each chatter, or the Twitch account a viewer linked, to look up their pronouns, and cache the answer for 24 hours'
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
      'To understand how the site is used and where to improve it, we run {umami}, an open-source analytics tool that we {selfHost}. It is {cookieless}: it sets no cookies and performs no cross-site or cross-device tracking. To count unique visitors it derives a pseudonymous visitor hash that changes every month. The data is stored on our own infrastructure. Like all our traffic, requests to the analytics server pass through Cloudflare (Section\u00A04.1); apart from that, the data is {notShared} (unlike Google Analytics or similar services).'
    )
    expect(t('legal.privacy.umamiLinkText')).toBe('Umami')
    expect(t('legal.privacy.analyticsSelfHostEmphasis')).toBe('host ourselves')
    expect(t('legal.privacy.analyticsCookielessEmphasis')).toBe('cookieless')
    expect(t('legal.privacy.analyticsNotSharedEmphasis')).toBe('not shared with any third party')
    expect(t('legal.privacy.analyticsRecordsIntro')).toBe('For each page view it records:')
    expect(t('legal.privacy.analyticsRecordsPage')).toBe(
      'The page you visited, its title and the referring URL. In both URLs we keep only utm_ campaign parameters from the query and replace IDs with a placeholder'
    )
    expect(t('legal.privacy.analyticsRecordsBrowser')).toBe(
      'Browser, operating system, device type, screen size and browser language'
    )
    expect(t('legal.privacy.analyticsRecordsCountry')).toBe(
      'Approximate country, derived from your IP address at request time'
    )
    expect(t('legal.privacy.analyticsRecordsEvents')).toBe(
      'Some interactions, recorded as named events: signing in (with the platform, or the reason it failed), creating an overlay, adding or configuring a source (with its platform), turning on moderation, copying the OBS address, using the overlay editor, onboarding steps, previewing or applying a theme, turning on text-to-speech or sounds, accepting a share, and clicks on buttons and outbound links (with the destination)'
    )
    expect(t('legal.privacy.analyticsIpNote')).toBe(
      'Your {ipNotStored}: it is used only momentarily to derive the country and, with a salt that changes every month, the visitor hash for counting unique visits, after which it is discarded. Public overlay pages (the pages OBS loads as a browser source) are recorded like other page views, tagged as overlay views and with the overlay ID replaced by a placeholder. Analytics run only on allch.at, not on beta.allch.at. You can block the analytics script with any browser content blocker without affecting the site.'
    )
    expect(t('legal.privacy.analyticsIpNotStoredEmphasis')).toBe('IP address is not stored')
    expect(t('legal.privacy.analyticsConsentNote')).toBe(
      'The tracker stores nothing on your device. To build the record above it reads your screen size, browser language and the page title, and it checks your browser storage for an opt-out flag (umami.disabled); if that flag is set, it sends nothing. We do not ask for consent under §\u00A025 TDDDG; the processing of the resulting data rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in measuring and improving the service.'
    )
    expect(t('legal.privacy.backupsBody')).toBe(
      'So that the service can be restored after data loss, we keep backups of the database (including account and overlay data, encrypted OAuth tokens and the chat messages and logs retained under Section\u00A06) and of uploaded alert media with Hetzner Online GmbH (Germany), acting as a processor under Art.\u00A028 DSGVO. The provider only stores the backups; we read them only to restore the service. The processing rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in not losing your data.'
    )
    expect(t('legal.privacy.retentionAccount')).toBe(
      '{label} kept until you delete them or your account'
    )
    expect(t('legal.privacy.retentionAccountLabel')).toBe('Account, overlay and feature settings:')
    expect(t('legal.privacy.retentionMedia')).toBe(
      '{label} kept until you delete your account; the files then stop being served within 15 minutes. Removing a file from an alert does not delete the upload; contact us to have a single file deleted'
    )
    expect(t('legal.privacy.retentionMediaLabel')).toBe('Uploaded media:')
    expect(t('legal.privacy.retentionTokens')).toBe(
      '{label} deleted when you disconnect the platform or Patreon in the Settings, remove a custom bot account, unlink a platform in the viewer settings, or delete your account or your viewer data; the platform you sign in with stays connected until you delete your account. Deleting your account does not revoke the tokens at the platforms; to withdraw the grant there too, remove All-Chat from the connected apps of your platform account'
    )
    expect(t('legal.privacy.retentionTokensLabel')).toBe('OAuth tokens:')
    expect(t('legal.privacy.retentionSessions')).toBe(
      '{label} the streamer sign-in cookie expires after 24 hours and the refresh token that renews it after 14 days; a viewer session is kept until you sign out or delete your viewer data'
    )
    expect(t('legal.privacy.retentionSessionsLabel')).toBe('Sign-in sessions:')
    expect(t('legal.privacy.retentionAccessTokens')).toBe(
      '{label} unused pairing requests are deleted one hour after they expire; personal access tokens and paired devices are kept until you revoke them or delete your account'
    )
    expect(t('legal.privacy.retentionAccessTokensLabel')).toBe('Access tokens and device pairing:')
    expect(t('legal.privacy.retentionSentMessages')).toBe(
      '{label} automatically deleted after one hour (checked every 15 minutes)'
    )
    expect(t('legal.privacy.retentionSentMessagesLabel')).toBe(
      'Chat messages sent through All-Chat:'
    )
    expect(t('legal.privacy.retentionDisplayedMessages')).toBe(
      '{label} not stored in our database; in the replay buffer until 500 newer messages replace them, and at most five minutes after its last message; in the incoming queue until 100,000 newer messages have arrived (currently less than a day); up to 7 days if processing failed'
    )
    expect(t('legal.privacy.retentionDisplayedMessagesLabel')).toBe(
      'Chat messages displayed in overlays:'
    )
    expect(t('legal.privacy.retentionQueues')).toBe(
      '{label} until 10,000 newer entries have arrived'
    )
    expect(t('legal.privacy.retentionQueuesLabel')).toBe(
      'Chat command and Twitch engagement queues:'
    )
    expect(t('legal.privacy.retentionAlerts')).toBe('{label} deleted after 30 days')
    expect(t('legal.privacy.retentionAlertsLabel')).toBe('Alert log:')
    expect(t('legal.privacy.retentionLeaderboards')).toBe(
      '{label} until 30 days pass without a new entry for list overlays, 48 hours for credit rolls'
    )
    expect(t('legal.privacy.retentionLeaderboardsLabel')).toBe('Leaderboards:')
    expect(t('legal.privacy.retentionStreamHistory')).toBe(
      "{label} an overlay's stream sessions until you delete the overlay; when a connected channel was last live and its typical streaming times, with no deletion period, also after the account is deleted"
    )
    expect(t('legal.privacy.retentionStreamHistoryLabel')).toBe('Stream history:')
    expect(t('legal.privacy.retentionEngagement')).toBe(
      '{label} kept until the streamer deletes the overlay or you delete your viewer data'
    )
    expect(t('legal.privacy.retentionEngagementLabel')).toBe('Votes, predictions and points:')
    expect(t('legal.privacy.retentionViewerRecords')).toBe(
      '{label} kept until you delete your viewer data'
    )
    expect(t('legal.privacy.retentionViewerRecordsLabel')).toBe(
      'Viewer records and linked platform identities:'
    )
    expect(t('legal.privacy.retentionModeration')).toBe(
      '{label} deleted after 90 days. Until then it is kept even if the account involved is deleted, so abuse can still be investigated'
    )
    expect(t('legal.privacy.retentionModerationLabel')).toBe('Moderation action log:')
    expect(t('legal.privacy.retentionAdminLog')).toBe(
      '{label} kept until the account involved is deleted, with no other deletion period'
    )
    expect(t('legal.privacy.retentionAdminLogLabel')).toBe(
      'Log of administrator act-as-user sessions:'
    )
    expect(t('legal.privacy.retentionBans')).toBe(
      '{label} the platform IDs of banned accounts are kept with no deletion period, also after an unban or after the account is deleted, so that a ban cannot be evaded by signing up again'
    )
    expect(t('legal.privacy.retentionBansLabel')).toBe('Bans:')
    expect(t('legal.privacy.retentionLogs')).toBe(
      '{label} logs and traces for 7 days, metrics for 15 days'
    )
    expect(t('legal.privacy.retentionLogsLabel')).toBe('Usage logs:')
    expect(t('legal.privacy.retentionCaches')).toBe(
      '{label} TikTok avatar images for 48 hours after the chatter was last seen; other avatar data, Twitch display names and pronouns for 24 hours'
    )
    expect(t('legal.privacy.retentionCachesLabel')).toBe('Avatar and pronoun caches:')
    expect(t('legal.privacy.retentionQuotaLogs')).toBe('{label} deleted after 30 days')
    expect(t('legal.privacy.retentionQuotaLogsLabel')).toBe('YouTube quota audit logs:')
    expect(t('legal.privacy.retentionBotNotes')).toBe(
      '{label} kept with no deletion period; once there are 500, the least used older note is removed for each new one'
    )
    expect(t('legal.privacy.retentionBotNotesLabel')).toBe('Support bot notes:')
    expect(t('legal.privacy.retentionAnalytics')).toBe('{label} kept with no deletion period')
    expect(t('legal.privacy.retentionAnalyticsLabel')).toBe('Usage analytics:')
    expect(t('legal.privacy.retentionSupportEmail')).toBe(
      '{label} kept in our mailbox with no deletion period'
    )
    expect(t('legal.privacy.retentionSupportEmailLabel')).toBe('Support emails:')
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
      '{label} Delete your account and its data from the Settings page, delete your viewer data from the viewer settings, or contact us. Some records remain after deletion, as Section\u00A06 describes: the moderation action log for up to 90 days, the platform IDs of banned accounts, the live history of your channels, translations you contributed (without your user ID), and backups for up to 60 days'
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
      '{label} Disconnect platforms (except the one you sign in with), unlink viewer identities or leave the ambassador showcase at any time'
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
      'All-Chat uses two essential cookies and {emphasis} for essential functionality:'
    )
    expect(t('legal.privacy.localStorageEmphasis')).toBe('browser storage')
    expect(t('legal.privacy.cookiesTokens')).toBe(
      'Sign-in: two first-party cookies keep you logged in as a streamer. access_token lasts 24 hours; refresh_token lasts 14 days and is sent only to our sign-in endpoints. Both are httpOnly, so scripts on the page cannot read them. Viewer sign-in is kept as a token (JWT) in localStorage. Your streaming platform tokens stay on our servers and are never stored in your browser'
    )
    expect(t('legal.privacy.cookiesPreferences')).toBe(
      'Preferences in localStorage: your language, that you saw the cookie notice, light or dark mode, the theme and layout of chat views, the editor layout, onboarding progress, dismissed notices, theme marketplace favorites and a cache of its themes, and when you last saw each overlay so that a reconnecting chat view can catch up'
    )
    expect(t('legal.privacy.cookiesSession')).toBe(
      'In sessionStorage, until you close the tab: the platform you started signing in with, so that a completed sign-in can be counted in our analytics (Section\u00A05.8)'
    )
    expect(t('legal.privacy.cookiesNoAdvertising')).toBe(
      "We do not use advertising cookies or cross-site tracking. We do use {emphasis} (self-hosted Umami) – it sets nothing on your device and recognises returning visitors only by a pseudonymous hash that changes every month; see Section\u00A05.8 for the full description. Fonts are self-hosted (Section\u00A05.5) and do not set cookies. The only third-party cookies that can occur come from Twitch's clip player, which the credit roll embeds as a fallback (Section\u00A05.2)."
    )
    expect(t('legal.privacy.cookielessAnalyticsEmphasis')).toBe(
      'privacy-friendly, cookieless usage analytics'
    )
    expect(t('legal.privacy.childrenBody')).toBe(
      'All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data processing in Germany). If we discover data belonging to a minor we will delete it from our live systems immediately; copies in backups expire within 60 days (Section\u00A06).'
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
      'Through EventSub we receive chat, channel events (follows, subscriptions, bits, raids, channel point redemptions) and, if you enable them, polls, predictions and moderation events. Optional permissions you grant let All-Chat send chat messages, moderate, create clips and post as a bot. We do not access payment data or channel analytics. Disconnecting Twitch in the Settings deletes your stored tokens and revokes them at Twitch.'
    )
    expect(t('legal.privacy.youtubeNote')).toBe(
      "We read live chat through YouTube's public live chat endpoints or, in the premium official-API mode, the YouTube live chat API. With your permission we read your most recent subscribers for alerts and your subscriber count for goals, and optional permissions let All-Chat send messages and moderate. We do not access video content or channel analytics. API usage is subject to YouTube's quota limits. You can revoke access via {googleSettings}, or disconnect YouTube in the All-Chat Settings, which deletes your stored OAuth tokens and revokes them at Google."
    )
    expect(t('legal.privacy.googleSettingsLinkText')).toBe('Google security settings')
    expect(t('legal.privacy.tiktokNote')).toBe(
      'TikTok offers no official chat API, so you only enter your TikTok handle and no TikTok login is involved. We read the public LIVE chat and events (gifts, likes, follows, shares) without accessing your videos, followers or other content. Remove the source to stop.'
    )
    expect(t('legal.privacy.kickNote')).toBe(
      "We use Kick's OAuth for sign-in and sources and connect to Kick's realtime service to receive chat. Optional permissions let All-Chat send messages and moderate. We do not access channel analytics or payment data. Disconnecting Kick in the Settings deletes your stored tokens and revokes them at Kick."
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
      'You connect a Facebook Page or an Instagram professional account through Meta. We poll live video comments and, on Facebook, can delete comments and block users on your instruction; Instagram is read-only. Both stay inactive until Meta has approved All-Chat in its app review. Disconnecting deletes the stored tokens and the Facebook or Instagram sources; to revoke the grant at Meta itself, remove All-Chat from the apps connected to your Facebook account.'
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
    expect(t('legal.terms.lastUpdated')).toBe('November 3, 2026')
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
      'Your use of All-Chat is also governed by our {privacy}, which explains what we collect, how it is used, and your rights under the DSGVO. For transparency: All-Chat measures usage with {analytics} (Umami) that set nothing on your device and recognise returning visitors only by a pseudonymous hash that changes every month – see Section\u00A05.8 of the Privacy Policy.'
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
      'OAuth tokens: deleted when you disconnect the platform or Patreon in the Settings, remove a custom bot account, unlink a platform in the viewer settings, or delete your account or your viewer data; the platform you sign in with stays connected until you delete your account. Deleting your account does not revoke the tokens at the platforms; to withdraw the grant there too, remove All-Chat from the connected apps of your platform account'
    )
    expect(t('legal.terms.licenseBody', { license: t('legal.terms.licenseLinkText') })).toBe(
      'All-Chat is released under the GNU Affero General Public License v3.0 (AGPL-3.0). That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.'
    )
    expect(t('legal.terms.availabilityStatus', { status: t('legal.terms.statusHost') })).toBe(
      'The current service status is published at status.allch.at.'
    )
  })
})
