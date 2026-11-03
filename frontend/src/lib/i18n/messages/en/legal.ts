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
      'All-Chat uses {storage} to keep you logged in and to remember your preferences. This is essential for the service to function properly.',
    storageEmphasis: 'two essential cookies and browser storage',
    noTrackingBody:
      '{emphasis} We do not share your data with third parties for advertising purposes.',
    noTrackingEmphasis: 'We use no tracking cookies, and our usage analytics are cookieless.',
    detailsSummary: 'What do we store in your browser?',
    // Each row keeps its bolded label inside the sentence as {label}: the text
    // after the hyphen continues the sentence the label starts, so handing a
    // translator the two halves separately would leave them unable to reorder.
    tokensRow:
      '{label} - Two essential cookies keep you logged in as a streamer, and viewer sign-in is kept in local storage. Your streaming platform tokens stay on our servers and never reach your browser',
    tokensLabel: 'Sign-in',
    preferencesRow:
      '{label} - Your language, light or dark mode, chat view and editor layout, onboarding progress, dismissed notices and theme marketplace favorites. Your overlays and settings are stored on our servers',
    preferencesLabel: 'User preferences',
    noTrackingRow: "{label} - We don't follow you across other websites",
    noTrackingLabel: 'No tracking cookies',
    noAdsRow: "{label} - We don't serve ads or share data with advertisers",
    noAdsLabel: 'No advertising cookies',
    analyticsRow:
      '{label} - We measure usage with self-hosted Umami. It sets no cookies and recognises returning visitors only by a pseudonymous hash that changes every month. Public overlay pages are counted too, tagged as overlay views',
    analyticsLabel: 'Cookieless analytics',
    fontsNote:
      '{label} All fonts (including ones originally distributed by Google Fonts) are self-hosted on our infrastructure – your IP address is never sent to Google.',
    fontsLabel: 'Fonts:',
    thirdPartyNote:
      '{label} Pages that show chat load avatars, badges and emotes from the streaming platforms and emote providers. The alerts editor searches GIFs on KLIPY, the credit roll plays Twitch clips from Twitch, and the credit-roll theme marketplace loads themes from {github}. These requests transmit your IP address to the respective providers. See our {privacy} for details.',
    thirdPartyLabel: 'Third-party resources:',
    thirdPartyGithub: 'GitHub',
    agreement:
      'By using All-Chat, you agree to this essential data storage. For more details, please read our {privacy} and {terms}.',
    privacyPolicy: 'Privacy Policy',
    termsOfService: 'Terms of Service',
    acknowledge: 'I Understand',
    learnMore: 'Learn More',
    footer:
      'Your overlays, settings and platform tokens are stored on our servers, and traffic between your browser and All-Chat is encrypted with HTTPS',
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
    lastUpdated: 'November 3, 2026',
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
    accountSubheading: '2.1 Your Account & Connected Platforms',
    chatSubheading: '2.2 Chat & Event Data',
    featuresSubheading: '2.3 Overlays & Features',
    viewerSubheading: '2.4 Viewer Accounts & Participation',
    extensionSubheading: '2.5 Browser Extension',
    logDataSubheading: '2.6 Usage & Log Data',
    patreonSubheading: '2.7 Premium Membership (Patreon)',
    supportSubheading: '2.8 Support',
    storageLocationsSubheading: '4.1 Storage Locations',
    safeguardsSubheading: '4.2 Safeguards',
    platformApisSubheading: '5.1 Streaming Platforms & Chat Services',
    browserResourcesSubheading: '5.2 Content Your Browser Loads from Third Parties',
    processorsSubheading: '5.3 Service Providers',
    streamerRequestsSubheading: '5.4 Requests Configured by Streamers',
    fontsSubheading: '5.5 Fonts (Self-Hosted)',
    youtubeNoticeSubheading: '5.6 YouTube-Specific Notice',
    noSalesSubheading: '5.7 No Data Sales',
    analyticsSubheading: '5.8 Usage Analytics (Self-Hosted, Cookieless)',
    backupsSubheading: '5.9 Backup Storage',
    twitchSubheading: 'Twitch',
    youtubeSubheading: 'YouTube',
    tiktokSubheading: 'TikTok',
    kickSubheading: 'Kick',
    discordSubheading: 'Discord',
    owncastSubheading: 'Owncast (beta)',
    publicChatSubheading: 'GoodGame, Picarto & Altstream (beta, early access)',
    metaSubheading: 'Facebook & Instagram (beta)',
    // The two callout boxes at the top. Each opens with a bolded label that
    // begins the sentence, so the label is a {label} param rather than a
    // separate fragment.
    tldr: '{label} All-Chat only collects what we need to sign you in, connect your streaming platforms and run your overlays, alerts and chat tools. Tokens are encrypted, the chat shown in your overlays is not stored in our database apart from the records listed in Sections\u00A02.2 and 2.4, and we never sell your data. Section\u00A06 lists how long we keep each kind of data. We use cookieless, self-hosted analytics (no tracking cookies; see Section\u00A05.8).',
    tldrLabel: 'TL;DR:',
    openSourceCallout:
      '{label} All-Chat is licensed under AGPL-3.0. Review the application code, including how we store and process your data, on {github}.',
    openSourceCalloutLabel: 'Open Source Transparency:',
    githubLinkText: 'GitHub',
    // The statutory ground for each kind of processing. One key each: the
    // article number and the reason it applies are a single statement, and
    // splitting them would let a translator pair the wrong two.
    accountLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO – performance of a contract (providing the service you signed up for). Records of bans and of your acceptance of the Terms rest on Art.\u00A06(1)(f) DSGVO – our legitimate interest in preventing abuse and in being able to show what you agreed to.',
    chatLegalBasis:
      "Legal basis: Art.\u00A06(1)(b) DSGVO for you as the streamer. For the viewers whose messages and events we process, Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in showing chat and events posted in the streamer's channel to its audience, and in preventing abuse. Most of this chat is public. Some is posted to a narrower audience, such as members-only or unlisted YouTube streams and private Discord channels, and AutoMod holds are messages Twitch has not shown in chat.",
    featuresLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO. The ambassador showcase rests on your consent (Art.\u00A06(1)(a) DSGVO), which you can withdraw at any time.',
    viewerLegalBasis:
      "Legal basis: Art.\u00A06(1)(b) DSGVO for the features you use. Linking several platform identities rests on your consent (Art.\u00A06(1)(a) DSGVO); you can unlink a platform at any time from the viewer settings, except the one you are signed in with and your last remaining one, which deleting your viewer data removes. Recording participation by chat command, and the points a streamer's rules award automatically, rest on Art.\u00A06(1)(f) DSGVO – the legitimate interest of the streamer and of All-Chat in running the poll, prediction or points game.",
    extensionLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO – providing the extension features you installed it for.',
    logDataLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in maintaining service security and availability.',
    supportLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in answering support requests; for requests about your account, Art.\u00A06(1)(b) DSGVO.',
    fontsLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in delivering the visual appearance of the overlay. The font files themselves are licensed under the SIL Open Font License 1.1 or Apache 2.0.',
    browserResourcesLegalBasis:
      'Legal basis: Art.\u00A06(1)(f) DSGVO – legitimate interest in showing chat the way it appears on the platforms. Fallback avatars (shown when a platform avatar is unavailable) and the sample chat on our homepage are served by All-Chat itself and involve no external request.',
    patreonLegalBasis:
      'Legal basis: Art.\u00A06(1)(b) DSGVO – providing the premium features you subscribed to. For the data you provide on patreon.com itself, Patreon, Inc. (US) is an independent controller; see the {patreonPolicy}.',
    patreonPolicyLinkText: 'Patreon Privacy Policy',
    controllerBody:
      'The person responsible for data processing on this website is listed in the {impressum}. For privacy-related inquiries contact {email}.',
    impressumLinkText: 'Impressum',
    supportEmail: 'all.chat.support@gmail.com',
    // 2.1
    accountIntro:
      'You sign in to All-Chat with Twitch, YouTube (Google) or Kick. For your account and for every platform you connect as a chat source we store:',
    accountIdentifiers:
      'For the platform you sign in with: your user ID, username, display name and profile image. For each platform you connect as a source: its user or channel ID and name',
    accountTokens:
      'Encrypted OAuth tokens with their scopes and, where the platform sets one, their expiry date, for each platform you authorise (Twitch, YouTube, Kick, and in beta Facebook and Instagram). Refresh tokens are stored where the platform issues them; Facebook Page tokens have none and do not expire',
    accountNameOnly:
      'For platforms you add by name or address only (TikTok, Owncast, GoodGame, Picarto, Altstream), just the channel name or server address you enter; for Discord, the server ID, name and icon and the channels you pick for the All-Chat bot, and, if you link your Discord account, its Discord user ID and username',
    accountStatus:
      'Your account status: premium, beta tester or ambassador status, admin role, and any ban with its reason',
    accountTermsAcceptance:
      'Whether you have accepted the current Terms of Service and this Privacy Policy, and when you last accepted them in the prompt shown after login. We do not record which version you accepted',
    accountNoEmail:
      'We do not store an email address from any chat platform. Kick includes your email address in the profile it returns at sign-in; we discard it.',
    // 2.2
    chatIntro: 'While an overlay or chat view is connected, we receive and process:',
    chatMessages:
      'Chat messages from the channels you connected, including emotes, GIFs and attachments',
    chatAuthor:
      'Per-message author details (platform user ID, username, display name, color, avatar, badges and roles, and where available their pronouns and the All-Chat name color, avatar frame and flair they chose)',
    chatEvents:
      'Channel events: on Twitch, follows, subscriptions, resubscriptions and gifted subscriptions, bits, cheers and Power-ups, raids, channel point redemptions and chat notices such as announcements, watch streaks, gift and Prime upgrades, pay-it-forward gifts and charity donations; on YouTube, Super Chats, Super Stickers, memberships, membership milestones and gifted memberships, and your most recent subscribers; on TikTok, gifts, likes, shares, follows, subscriptions, questions, Super Fan joins and boxes, and treasure chests',
    chatModeration:
      'Moderation events (deleted messages, timeouts, bans) and, if you turn on the moderation log, Twitch AutoMod holds and moderator actions, which are shown only to you',
    chatRetention:
      'Chat messages are {emphasis}. They pass through memory and short-lived queues: a replay buffer with the last 500 messages of each overlay, so a reconnecting overlay can catch up, which is deleted five minutes after its last message; an incoming queue that holds the most recent 100,000 messages across all channels and is continuously overwritten (Redis keeps it in an on-disk log, and at current traffic a message stays in it for less than a day); queues that hold the most recent 10,000 chat commands and Twitch poll and prediction events; and a seven-day queue for messages that failed processing so we can fix the fault.',
    chatRetentionEmphasis: 'not written to our database',
    chatDerived:
      "Some features keep data derived from events: the alert log (the sender's platform user ID and name, the event type and amount, and the event's details as the platform sends them, such as a channel point reward with the text the viewer entered) for 30 days; list-overlay leaderboards (platform user IDs, names and scores) until 30 days pass without a new entry; and credit-roll session boards (platform user IDs, names, avatars and event details, including the full message of raids and mystery gifts) until 48 hours pass without a new entry.",
    chatStreamHistory:
      "We also keep a record of each overlay's stream sessions (start and end, event counts and the total value of paid events) and, for each connected channel, when it was last seen live or offline and its typical streaming days and times.",
    // 2.3
    featuresIntro: 'To run your overlays and tools we store what you configure:',
    featuresOverlays:
      'Overlay names, IDs, custom CSS, themes, connected sources and filters (blocked words and user rules, which can contain usernames)',
    featuresAlerts:
      'Alert, goal, list and credit-roll settings, including any custom HTML, CSS and JavaScript and the image and sound addresses you choose',
    featuresMedia:
      'Media you upload (images, audio and video up to 10\u00A0MiB each). Uploaded files are stored on our own servers and can be opened by anyone who has their address, so that they play in OBS. The address contains your account ID, a random part that makes it unguessable, and the name of the file you uploaded',
    featuresTts: 'Text-to-speech settings, including your own ElevenLabs API key, stored encrypted',
    featuresCommands:
      'Chat commands, keyword triggers, timers and bot settings, and for a custom bot account its login, display name and encrypted tokens',
    featuresAccessTokens:
      'Personal access tokens and paired devices: their name, scopes and last use. The tokens themselves are stored only as SHA-256 hashes',
    featuresModerators:
      'Moderators you invite to an overlay (display name, invitation label and the platform account they are expected to use), and the encrypted platform credentials a moderator connects',
    featuresModerationLog:
      "A log of the moderation actions you and your moderators take through All-Chat: who acted and for whom, the overlay, platform and channel, the action, the target's platform user ID and message ID, the reason and the outcome",
    featuresSharing: 'Share requests you send to or receive from other streamers',
    featuresTranslations: 'Translations you contribute, with your user ID and any review notes',
    featuresShowcase:
      'If you are an ambassador and opt in to the public showcase: your username, display name, profile image, platform and tagline, shown publicly on allch.at',
    // 2.4
    viewerAccountBody:
      'Viewers can sign in with Twitch, YouTube or Kick through the browser extension, the participation page or the viewer settings. For each sign-in we keep a session record with the platform user ID, username, display name, avatar and encrypted tokens (Twitch: no extra permissions; YouTube: read-only access and basic profile; Kick: basic profile) and counters of the chat messages you send, for rate limiting; signing out deletes it. We also store the name color, avatar frame, flair and viewer premium status you have. Unlinking a platform deletes its tokens and revokes them at the platform.',
    viewerStreamerLink:
      'If you sign in as a viewer with the same platform account that you use as a streamer on All-Chat, we link the viewer session to your streamer account automatically, so your viewer profile carries your premium status.',
    viewerIdentityBody:
      'If you choose to link several platform accounts (e.g.\u00A0Twitch + YouTube), we create a unified viewer profile that associates your platform identities. This is {emphasis}.',
    viewerIdentityEmphasis: 'opt-in only and requires your explicit action',
    viewerEngagementBody:
      "When you vote in a poll or enter a prediction in a streamer's channel, including by typing a chat command without ever signing in, we create a viewer record for your platform account. It holds only the platform and your platform user ID, not your username. We store your votes and predictions, and your points balance and each points transaction, for that streamer's overlay. If the streamer has turned points on, you earn them under the streamer's rules for watching while signed in to the extension or the participation page (at most one entry per minute), and for subscribing, gifting subscriptions or memberships, cheering bits and sending Super Chats or Super Stickers. Earning points this way creates a viewer record for you too, even if you have never used All-Chat. Points exist only within that streamer's overlay.",
    viewerSentMessages:
      'Chat messages sent through All-Chat are logged for rate-limiting and abuse detection and {emphasis}.',
    viewerSentMessagesEmphasis: 'automatically deleted after one hour',
    // 2.5
    extensionBody:
      "The All-Chat browser extension runs on twitch.tv, youtube.com and kick.com. On each channel page you open it asks All-Chat whether that streamer uses All-Chat, which sends us the channel name together with your IP address, even when you are not signed in. It shows the streamer's merged chat without sign-in. Signed in, it also lets you vote, predict and use points, and while you watch it tells All-Chat once a minute that you are watching, so the streamer's points rules can award watch time. When you chat from the extension, your message goes from your browser straight to the platform using the session you are already signed in with; your platform cookies are never sent to All-Chat. The extension contains no analytics or telemetry.",
    // 2.6
    logDataIntro: 'For observability and abuse prevention we record:',
    logDataIp:
      'Your IP address, anonymised before it is logged (IPv4: last octet zeroed; IPv6: shortened to the first 48 bits), your browser user agent and basic request info',
    logDataRateLimit:
      'For rate limiting, your full IP address as a counter key for one minute; it is not logged',
    logDataMetrics:
      'API usage metrics, cache hits and connection status, some of them broken down by channel ID, YouTube video ID or Discord server ID',
    logDataTraces:
      'Traces of a 10% sample of all requests (method, path, host and user agent), error traces and diagnostic logs',
    logDataIdentifiers:
      'Where a request or background job concerns a particular account, logs can also contain the All-Chat user or viewer ID, platform usernames and channel names, a Patreon user ID or a Discord user ID',
    // 2.7
    patreonIntro:
      'Premium features are unlocked through a paid membership on Patreon, or granted by us free of charge, for example to beta testers and ambassadors. If you connect your Patreon account to All-Chat, we store:',
    patreonUserId: 'Your Patreon user ID',
    patreonTokens: 'Encrypted OAuth access and refresh tokens for the Patreon API',
    patreonMembership:
      'Your membership state for our campaign (status, tier, pledge amount and next charge date)',
    patreonUnlinked:
      "If you support us on Patreon without connecting All-Chat, Patreon's notifications still reach us; we then store only your Patreon user ID and membership state, so the membership can be matched once you connect.",
    patreonNoPaymentData:
      "We do {not} receive or store your payment details; payments are processed entirely by Patreon. Patreon's membership notifications also contain your email address, name and other profile details; we discard them without storing them. Patreon notifies us via webhooks when your membership changes, and a periodic reconciliation keeps the state current. Disconnecting Patreon in the Settings page deletes the stored tokens, unlinks the membership from your account and ends the premium that came from your subscription.",
    notEmphasis: 'not',
    // 2.8
    supportEmailBody:
      'If you email us, we use your message and email address to answer you. Our support mailbox is hosted by Google (Gmail).',
    supportDiscordBody:
      "In our Discord server, the support bot reads messages that mention it, that are posted in its threads or that use its /support command, together with the usernames, the recent messages of the thread and any message you reply to (with its author's username) that it needs for context, and answers using a language model run by an external AI inference provider acting as our processor. To answer, it can look up our service logs, metrics and system status and pass redacted summaries of them to the model; these can concern your account or overlay. The bot also checks every message posted in the server for spam: if an account posts the same message in three or more channels within ten minutes, the bot bans it and deletes about a day of its messages. The bot keeps short notes to remember recurring problems and can turn a support request into a public issue on our GitHub repository, so do not post anything in a support thread that you do not want to be public. Discord processes your messages under Discord's own privacy policy.",
    // 3
    useIntro: 'We use the data we store to:',
    useAuthenticate: 'Authenticate against the platforms you authorize',
    useFetch: 'Receive live chat and events and normalize them into a single feed',
    useRender:
      'Render overlays, alerts, goals, lists and credit rolls, look up emotes, badges, avatars and pronouns, and apply your filters',
    useAct:
      'Carry out what you ask for: send chat messages, moderate, create clips, run chat commands, timers, polls, predictions and points, and read messages aloud',
    usePremium: 'Unlock premium features based on your membership',
    useMonitor: 'Monitor service reliability and debug incidents',
    useAbuse: 'Detect and prevent abuse (rate-limiting, bans)',
    useRecords:
      'Keep accountability records: your acceptance of the Terms, the moderation action log and the log of administrator act-as-user sessions',
    useAnalytics: 'Measure how the site is used (Section\u00A05.8)',
    useShowcase: 'Show the ambassadors who opted in on the public showcase',
    // 4
    storagePostgres: 'PostgreSQL for account data, overlays, configuration and encrypted tokens',
    storageRedis:
      'Redis for sessions, message fan-out, short-lived buffers, leaderboards, caches and rate limiting',
    storageMedia: 'Object storage for the media you upload',
    storageObservability:
      'Our log, trace and metrics stores (Section\u00A02.6) and the analytics database (Section\u00A05.8)',
    storageLocation:
      'All of the above runs on our own infrastructure on servers located in {country} (hosting provider: Hetzner Online GmbH; data centres in Falkenstein and Nuremberg, Germany, and Helsinki, Finland)',
    storageCountry: 'Germany and Finland',
    storageBackups:
      'Backups of the database and of uploaded alert media are kept outside that infrastructure, in Hetzner Object Storage (Falkenstein, Germany), and are transferred there over encrypted connections (see Sections\u00A05.9 and 6)',
    storageCloudflare:
      'All traffic to allch.at passes through Cloudflare, Inc. (US), which provides DNS, TLS and protection against attacks as our processor. Cloudflare therefore sees your IP address and the requests you make, including overlay and chat traffic.',
    safeguardsEncryption:
      'OAuth tokens and API keys encrypted with AES-GCM before they reach the database; access tokens you create are stored only as hashes',
    safeguardsHttps:
      'HTTPS for all traffic between your browser and All-Chat, and encrypted connections to the platforms wherever they offer them. An Owncast server you add with an http:// address is contacted unencrypted. Internal services are isolated via Kubernetes network policies',
    safeguardsAccess:
      'Administrative access to the servers is limited to the operator. In our admin tools, administrators can look up accounts, overlays and viewers with their message counts, ban accounts, grant premium, and temporarily act as a user to resolve support issues; every act-as-user session is logged',
    safeguardsPatching: 'Regular dependency upgrades and security patching',
    safeguardsHeaders:
      'Security headers (X-Content-Type-Options, X-Frame-Options, Referrer-Policy)',
    safeguardsCaveat:
      'No storage system is perfectly secure, but we follow industry best practices to keep your tokens and overlays safe.',
    // 5.1
    platformApisIntro:
      'To deliver the product we connect to the platforms you add. They receive your channel identifiers and, when we act on your behalf, your tokens and the messages or moderation actions you send:',
    platformApisTwitch: 'Twitch (Twitch Interactive, Inc., US): EventSub, Helix and chat APIs',
    platformApisYoutube:
      'YouTube (Google LLC, US): public live chat endpoints, the YouTube Data API and the live chat API',
    platformApisKick:
      "Kick (Kick Streaming Pty Ltd, Australia): Kick's public and official APIs and its realtime service",
    platformApisTiktok:
      'TikTok: public TikTok LIVE endpoints (TikTok offers no official chat API). When TikTok cannot find a handle from our servers, we repeat only that lookup, with the public handle, through a relay we run on Google Cloud in the US',
    platformApisDiscord:
      'Discord (Discord Inc., US): the Gateway and REST APIs, and webhooks that relay chat from your other platforms into the Discord channel you choose',
    platformApisBeta:
      'In beta or early access: Owncast (the server you enter), GoodGame, Picarto, Altstream, and Facebook and Instagram (Meta Platforms, Inc., US)',
    platformApisEmotes:
      '7TV, BetterTTV and FrankerFaceZ for emote sets. Our servers send them only channel IDs; the browser extension requests emote sets from them directly, which also sends them your IP address',
    platformApisPronouns:
      'The alejo.io pronoun service: for overlays with pronoun display turned on (the default; you can switch it off per overlay), we send the Twitch username of each chatter, or the Twitch account a viewer linked, to look up their pronouns, and cache the answer for 24 hours',
    platformApisScopes:
      "Every integration remains subject to the platform's own policies and the scopes you approve.",
    // 5.2
    browserResourcesIntro:
      'Pages that show chat (the overlay in OBS, the editor preview, viewer pages and the browser extension) load images directly from the platforms and emote providers. Each request transmits your {emphasis} and browser user agent to that provider:',
    ipAddressEmphasis: 'IP address',
    browserResourcesPlatforms:
      'Avatars, badges, emotes and attachments from Twitch, YouTube, Kick, TikTok and Discord, GIFs in Twitch chat (Giphy), images from Picarto (beta) and Altstream (early access), and, in the settings and the editor, the icons of your Discord servers',
    browserResourcesEmotes: 'Emote images from 7TV, BetterTTV and FrankerFaceZ',
    browserResourcesKlipy:
      '{label} (api.klipy.com) – GIF search in the alerts editor sends your search terms; the GIF you pick is then loaded from KLIPY wherever the alert plays',
    klipyLabel: 'KLIPY',
    browserResourcesGithub:
      '{label} (api.github.com and raw.githubusercontent.com) – loads community themes from our public repository in the credit-roll theme marketplace',
    githubApiLabel: 'GitHub API',
    browserResourcesClips:
      "{label} – the credit roll requests clips from Twitch and plays them from Twitch's servers; if that fails, it embeds Twitch's clip player, which may set Twitch cookies",
    clipsLabel: 'Twitch clips',
    browserResourcesAlertMedia:
      '{label} – images and sounds load from the addresses the streamer configures',
    alertMediaLabel: 'Alert media',
    browserResourcesTts:
      "{label} – when an overlay reads chat aloud with your browser's built-in voices, some browsers send the text to their vendor to turn it into speech, for example for the online voices in Chrome or Edge",
    browserTtsLabel: 'Browser text-to-speech',
    // 5.3
    processorsIntro:
      'Besides our hosting provider and Cloudflare (Section\u00A04.1), these services receive data in the cases below:',
    processorsElevenlabs:
      "{label} – if you turn on ElevenLabs text-to-speech with your own API key, the text of each message read aloud, optionally preceded by the sender's display name, is sent from our servers to ElevenLabs under your ElevenLabs account",
    elevenlabsLabel: 'ElevenLabs (US)',
    processorsPatreon: '{label} – your membership, see Section\u00A02.7',
    patreonLabel: 'Patreon (US)',
    processorsGoogleCloud:
      '{label} – the TikTok handle lookup relay (Section\u00A05.1) and our support mailbox (Section\u00A02.8)',
    googleLabel: 'Google',
    processorsAi:
      '{label} – messages addressed to the support bot in our Discord server (Section\u00A02.8)',
    aiLabel: 'AI inference provider',
    processorsDiscord:
      '{label} – our monitoring alerts are posted to our own Discord server; an alert about a failing source can name its YouTube channel ID or TikTok username',
    discordLabel: 'Discord (US)',
    // 5.4
    streamerRequestsBody:
      "A streamer's chat commands can fetch an address the streamer chooses. That request comes from our servers and can include the name of the viewer who used the command and the text they wrote; who receives it depends only on the streamer's configuration. Likewise, custom code in an alert runs in a sandbox in the browser that shows the alert and can contact any address the streamer's code names.",
    // Three emphasised runs plus a <code> path in one sentence, so the render
    // site uses interpolateElements rather than nested emphasise calls.
    fontsBody:
      'Typography assets originally distributed by Google Fonts are {selfHosted}. The fonts used by the All-Chat interface are bundled at build time via Next.js, and fonts selectable for overlay customization are served through a server-side proxy at {proxyPath}. Your browser only connects to the All-Chat origin; {noTransmission} when fonts are loaded. This aligns with the Landgericht München I ruling on Google Fonts (20 January 2022, Az. 3 O 17493/20).',
    fontsSelfHostedEmphasis: 'self-hosted on our infrastructure',
    fontsNoTransmissionEmphasis:
      'no IP address, user agent, or request metadata is transmitted to Google',
    youtubeNoticeBody:
      "Your use of All-Chat's YouTube integration is also governed by the {googlePolicy}. This applies only to the YouTube API integration (data flows described in Section\u00A05.1); it does {not} apply to fonts, which are self-hosted as described in Section\u00A05.5.",
    googlePolicyLinkText: 'Google Privacy Policy',
    noSalesBody:
      'We never sell or rent your data. We may disclose information when required by law or to respond to legitimate security incidents.',
    analyticsBody:
      'To understand how the site is used and where to improve it, we run {umami}, an open-source analytics tool that we {selfHost}. It is {cookieless}: it sets no cookies and performs no cross-site or cross-device tracking. To count unique visitors it derives a pseudonymous visitor hash that changes every month. The data is stored on our own infrastructure. Like all our traffic, requests to the analytics server pass through Cloudflare (Section\u00A04.1); apart from that, the data is {notShared} (unlike Google Analytics or similar services).',
    umamiLinkText: 'Umami',
    analyticsSelfHostEmphasis: 'host ourselves',
    analyticsCookielessEmphasis: 'cookieless',
    analyticsNotSharedEmphasis: 'not shared with any third party',
    analyticsRecordsIntro: 'For each page view it records:',
    analyticsRecordsPage:
      'The page you visited, its title and the referring URL. In both URLs we keep only utm_ campaign parameters from the query and replace IDs with a placeholder',
    analyticsRecordsBrowser:
      'Browser, operating system, device type, screen size and browser language',
    analyticsRecordsCountry: 'Approximate country, derived from your IP address at request time',
    analyticsRecordsEvents:
      'Some interactions, recorded as named events: signing in (with the platform, or the reason it failed), creating an overlay, adding or configuring a source (with its platform), turning on moderation, copying the OBS address, using the overlay editor, onboarding steps, previewing or applying a theme, turning on text-to-speech or sounds, accepting a share, and clicks on buttons and outbound links (with the destination)',
    analyticsIpNote:
      'Your {ipNotStored}: it is used only momentarily to derive the country and, with a salt that changes every month, the visitor hash for counting unique visits, after which it is discarded. Public overlay pages (the pages OBS loads as a browser source) are recorded like other page views, tagged as overlay views and with the overlay ID replaced by a placeholder. Analytics run only on allch.at, not on beta.allch.at. You can block the analytics script with any browser content blocker without affecting the site.',
    analyticsIpNotStoredEmphasis: 'IP address is not stored',
    analyticsConsentNote:
      'The tracker stores nothing on your device. To build the record above it reads your screen size, browser language and the page title, and it checks your browser storage for an opt-out flag (umami.disabled); if that flag is set, it sends nothing. We do not ask for consent under §\u00A025 TDDDG; the processing of the resulting data rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in measuring and improving the service.',
    backupsBody:
      'So that the service can be restored after data loss, we keep backups of the database (including account and overlay data, encrypted OAuth tokens and the chat messages and logs retained under Section\u00A06) and of uploaded alert media with Hetzner Online GmbH (Germany), acting as a processor under Art.\u00A028 DSGVO. The provider only stores the backups; we read them only to restore the service. The processing rests on Art.\u00A06(1)(f) DSGVO – our legitimate interest in not losing your data.',
    // Retention rows: the bolded label opens the row and the rest continues
    // the sentence it starts, per the legal.cookieBanner convention.
    retentionAccount: '{label} kept until you delete them or your account',
    retentionAccountLabel: 'Account, overlay and feature settings:',
    retentionMedia:
      '{label} kept until you delete your account; the files then stop being served within 15 minutes. Removing a file from an alert does not delete the upload; contact us to have a single file deleted',
    retentionMediaLabel: 'Uploaded media:',
    retentionTokens:
      '{label} deleted when you disconnect the platform or Patreon in the Settings, remove a custom bot account, unlink a platform in the viewer settings, or delete your account or your viewer data; the platform you sign in with stays connected until you delete your account. Deleting your account does not revoke the tokens at the platforms; to withdraw the grant there too, remove All-Chat from the connected apps of your platform account',
    retentionTokensLabel: 'OAuth tokens:',
    retentionSessions:
      '{label} the streamer sign-in cookie expires after 24 hours and the refresh token that renews it after 14 days; a viewer session is kept until you sign out or delete your viewer data',
    retentionSessionsLabel: 'Sign-in sessions:',
    retentionAccessTokens:
      '{label} unused pairing requests are deleted one hour after they expire; personal access tokens and paired devices are kept until you revoke them or delete your account',
    retentionAccessTokensLabel: 'Access tokens and device pairing:',
    retentionSentMessages:
      '{label} automatically deleted after one hour (checked every 15 minutes)',
    retentionSentMessagesLabel: 'Chat messages sent through All-Chat:',
    retentionDisplayedMessages:
      '{label} not stored in our database; in the replay buffer until 500 newer messages replace them, and at most five minutes after its last message; in the incoming queue until 100,000 newer messages have arrived (currently less than a day); up to 7 days if processing failed',
    retentionDisplayedMessagesLabel: 'Chat messages displayed in overlays:',
    retentionQueues: '{label} until 10,000 newer entries have arrived',
    retentionQueuesLabel: 'Chat command and Twitch engagement queues:',
    retentionAlerts: '{label} deleted after 30 days',
    retentionAlertsLabel: 'Alert log:',
    retentionLeaderboards:
      '{label} until 30 days pass without a new entry for list overlays, 48 hours for credit rolls',
    retentionLeaderboardsLabel: 'Leaderboards:',
    retentionStreamHistory:
      "{label} an overlay's stream sessions until you delete the overlay; when a connected channel was last live and its typical streaming times, with no deletion period, also after the account is deleted",
    retentionStreamHistoryLabel: 'Stream history:',
    retentionEngagement:
      '{label} kept until the streamer deletes the overlay or you delete your viewer data',
    retentionEngagementLabel: 'Votes, predictions and points:',
    retentionViewerRecords: '{label} kept until you delete your viewer data',
    retentionViewerRecordsLabel: 'Viewer records and linked platform identities:',
    retentionModeration:
      '{label} deleted after 90 days. Until then it is kept even if the account involved is deleted, so abuse can still be investigated',
    retentionModerationLabel: 'Moderation action log:',
    retentionAdminLog:
      '{label} kept until the account involved is deleted, with no other deletion period',
    retentionAdminLogLabel: 'Log of administrator act-as-user sessions:',
    retentionBans:
      '{label} the platform IDs of banned accounts are kept with no deletion period, also after an unban or after the account is deleted, so that a ban cannot be evaded by signing up again',
    retentionBansLabel: 'Bans:',
    retentionLogs: '{label} logs and traces for 7 days, metrics for 15 days',
    retentionLogsLabel: 'Usage logs:',
    retentionCaches:
      '{label} TikTok avatar images for 48 hours after the chatter was last seen; other avatar data, Twitch display names and pronouns for 24 hours',
    retentionCachesLabel: 'Avatar and pronoun caches:',
    retentionQuotaLogs: '{label} deleted after 30 days',
    retentionQuotaLogsLabel: 'YouTube quota audit logs:',
    retentionBotNotes:
      '{label} kept with no deletion period; once there are 500, the least used older note is removed for each new one',
    retentionBotNotesLabel: 'Support bot notes:',
    retentionAnalytics: '{label} kept with no deletion period',
    retentionAnalyticsLabel: 'Usage analytics:',
    retentionSupportEmail: '{label} kept in our mailbox with no deletion period',
    retentionSupportEmailLabel: 'Support emails:',
    retentionBackups:
      '{label} kept for 30 days, after which the storage provider erases them within a further 30 days. Data that you delete, or that expires under the periods above, can therefore remain in backups for up to 60 days; it is not used for anything other than restoring the service',
    retentionBackupsLabel: 'Backups:',
    // The Art. 15-21 data subject rights. Same {label} shape as the
    // retention rows above.
    rightsIntro: 'You can exercise the following at any time:',
    rightsAccess: '{label} Request a copy of the data we store about you',
    rightsAccessLabel: 'Access (Art.\u00A015):',
    rightsRectification: '{label} Correct inaccurate information',
    rightsRectificationLabel: 'Rectification (Art.\u00A016):',
    rightsErasure:
      '{label} Delete your account and its data from the Settings page, delete your viewer data from the viewer settings, or contact us. Some records remain after deletion, as Section\u00A06 describes: the moderation action log for up to 90 days, the platform IDs of banned accounts, the live history of your channels, translations you contributed (without your user ID), and backups for up to 60 days',
    rightsErasureLabel: 'Erasure (Art.\u00A017):',
    rightsRestriction: '{label} Request restriction of processing in certain circumstances',
    rightsRestrictionLabel: 'Restriction (Art.\u00A018):',
    rightsPortability:
      '{label} Download your data in a machine-readable format (JSON) from the Settings page or the viewer settings',
    rightsPortabilityLabel: 'Data portability (Art.\u00A020):',
    rightsObjection: '{label} Object to processing based on legitimate interest',
    rightsObjectionLabel: 'Objection (Art.\u00A021):',
    rightsWithdraw:
      '{label} Disconnect platforms (except the one you sign in with), unlink viewer identities or leave the ambassador showcase at any time',
    rightsWithdrawLabel: 'Withdraw consent:',
    rightsContact:
      'Contact us at {email} or use the Settings page. You also have the right to lodge a complaint with your supervisory authority (Aufsichtsbehörde).',
    rightsChatters:
      'If you wrote in a channel that uses All-Chat but have never used All-Chat yourself, you can exercise these rights by contacting us with your platform and username.',
    noProfiling:
      'We do not use automated decision-making or profiling within the meaning of Art.\u00A022 DSGVO.',
    youtubeRevoke:
      "For YouTube Data: You can revoke All-Chat's access to your YouTube data via the {googleSettings}.",
    googleSettingsPageLinkText: 'Google security settings page',
    cookiesIntro: 'All-Chat uses two essential cookies and {emphasis} for essential functionality:',
    localStorageEmphasis: 'browser storage',
    cookiesTokens:
      'Sign-in: two first-party cookies keep you logged in as a streamer. access_token lasts 24 hours; refresh_token lasts 14 days and is sent only to our sign-in endpoints. Both are httpOnly, so scripts on the page cannot read them. Viewer sign-in is kept as a token (JWT) in localStorage. Your streaming platform tokens stay on our servers and are never stored in your browser',
    cookiesPreferences:
      'Preferences in localStorage: your language, that you saw the cookie notice, light or dark mode, the theme and layout of chat views, the editor layout, onboarding progress, dismissed notices, theme marketplace favorites and a cache of its themes, and when you last saw each overlay so that a reconnecting chat view can catch up',
    cookiesSession:
      'In sessionStorage, until you close the tab: the platform you started signing in with, so that a completed sign-in can be counted in our analytics (Section\u00A05.8)',
    cookiesNoAdvertising:
      "We do not use advertising cookies or cross-site tracking. We do use {emphasis} (self-hosted Umami) – it sets nothing on your device and recognises returning visitors only by a pseudonymous hash that changes every month; see Section\u00A05.8 for the full description. Fonts are self-hosted (Section\u00A05.5) and do not set cookies. The only third-party cookies that can occur come from Twitch's clip player, which the credit roll embeds as a fallback (Section\u00A05.2).",
    cookielessAnalyticsEmphasis: 'privacy-friendly, cookieless usage analytics',
    childrenBody:
      'All-Chat is not intended for children under 16 (the DSGVO minimum age for consent to data processing in Germany). If we discover data belonging to a minor we will delete it from our live systems immediately; copies in backups expire within 60 days (Section\u00A06).',
    transfersBody:
      'All-Chat itself is hosted on servers located in Germany and Finland, both within the EU (see Section\u00A04.1). Some recipients are outside the EU: Cloudflare (US), Twitch Interactive, Inc. (US, an Amazon company), Google, YouTube and Google Cloud (US), TikTok (on servers TikTok chooses, which can be outside the EU), Kick (Australia) and Pusher (UK), whose realtime service Kick uses for chat in one of its two modes, Discord (US), Meta (US, beta), GoodGame (Russia, beta), Patreon (US), GitHub (US), ElevenLabs (US, only with your own API key), and the emote, GIF and pronoun services named in Section\u00A05. Picarto (beta) and Altstream (early access) do not state where they are based, so we treat them as recipients outside the EU. An Owncast server (beta) is wherever its operator runs it.',
    transfersLegalBasis:
      "Where a provider is certified under the EU–US Data Privacy Framework, the transfer rests on the EU Commission's adequacy decision (Art.\u00A045 DSGVO). Otherwise, the transfer is necessary to perform the service you requested (Art.\u00A049(1)(b) DSGVO). Fonts are self-hosted on our infrastructure and therefore do not involve any third-country transfer when loaded.",
    updatesBody:
      "We'll post updates to this page when the policy changes and include a new {lastUpdatedLabel} date. Significant changes will be announced inside the dashboard.",
    updatesLastUpdatedEmphasis: 'Last Updated',
    openSourceAuditable: 'The All-Chat application code is publicly auditable under AGPL-3.0',
    openSourceNoTracking:
      'No hidden tracking: our analytics are cookieless and documented (Section\u00A05.8), and the application code that sends them is verifiable on GitHub. Our server configuration is not public',
    openSourceScrutiny: 'Privacy practices are open to community scrutiny',
    openSourceRepository: 'Source: {repository}',
    repositoryLinkText: 'github.com/caesarakalaeii/all-chat',
    // Section 13, one paragraph per platform. What All-Chat does NOT access
    // is the load-bearing half of each of these.
    twitchNote:
      'Through EventSub we receive chat, channel events (follows, subscriptions, bits and Power-ups, raids, channel point redemptions and chat notices), moderation events (deleted messages, timeouts and bans) and, if you grant the extra permissions, polls, predictions and the moderation log (AutoMod holds and moderator actions). The permissions you grant when adding Twitch as a source include letting the All-Chat bot account read and post in your chat. Optional permissions let All-Chat send chat messages as you, moderate and create clips. We do not access payment data or channel analytics. Disconnecting Twitch in the Settings deletes your stored tokens and revokes them at Twitch. If you sign in with Twitch, you cannot disconnect it.',
    youtubeNote:
      "We read live chat through YouTube's public live chat endpoints or, in the premium official-API mode, the YouTube live chat API. With your permission we read your most recent subscribers for alerts and your subscriber count for goals, and optional permissions let All-Chat send messages and time out or ban chatters. We do not access video content or channel analytics. API usage is subject to YouTube's quota limits. You can revoke access via {googleSettings}, or disconnect YouTube in the All-Chat Settings, which deletes your stored OAuth tokens and revokes them at Google. Your YouTube sources then keep reading public chat, which needs no token, until you remove them. If you sign in with YouTube, you cannot disconnect it.",
    googleSettingsLinkText: 'Google security settings',
    tiktokNote:
      'TikTok offers no official chat API, so you only enter a TikTok handle and no TikTok login is involved. Any public handle can be added; we do not check that it belongs to you. We read the public LIVE chat and events (gifts, likes, follows, shares, subscriptions, questions, Super Fan joins and boxes, and treasure chests) without accessing videos, followers or other content. Removing the source stops it for your overlays; we keep reading that handle while another All-Chat user has it as a source.',
    kickNote:
      "We use Kick's OAuth for sign-in and sources and connect to Kick's realtime service to receive chat. When that service runs in its Pusher mode, our servers use the stored Kick token of the account that linked your channel to subscribe to its chat. Optional permissions let All-Chat send messages and moderate. We do not access channel analytics or payment data. Disconnecting Kick in the Settings deletes your stored tokens and revokes them at Kick; your Kick sources then keep reading public chat until you remove them. If you sign in with Kick, you cannot disconnect it.",
    discordNote:
      'When you add the All-Chat bot to a server, we store the server ID, name and icon and read messages, including content and attachments, in the channels you configure. On your instruction, All-Chat can delete messages there and time out, ban and unban members. Chat from your other platforms can be relayed into a Discord channel through a webhook. If you link your Discord account, we store its Discord user ID and username. We do not read direct messages or channels you did not configure. Removing the server from All-Chat makes the bot leave it.',
    owncastNote:
      "You enter the address of an Owncast server. We connect to it anonymously, read its public configuration and register an anonymous chat user there to receive chat. The server's operator, not All-Chat, controls the data on that server.",
    publicChatNote:
      'We read public chat anonymously using the channel name you enter. No login with these platforms is involved, and we never post or moderate there.',
    metaNote:
      'You connect a Facebook Page or an Instagram professional account through Meta. On Facebook we poll live video comments and can delete comments and block users on your instruction. Until Meta has approved All-Chat in its app review, this works only for accounts that Meta lets test our app. We currently do not read Instagram comments: connecting Instagram only stores the account and its tokens. Disconnecting deletes the stored tokens and the Facebook or Instagram sources; to revoke the grant at Meta itself, remove All-Chat from the apps connected to your Facebook account.',
    contactEmailRow: 'Email: {email}',
    contactHostedNote:
      'This contact is for users of the official hosted service at {host}. Self-hosted installations should contact their own administrator.',
    hostedDomain: 'allch.at',
  },
  // The terms of service. One key per heading, paragraph and list item: these
  // are contractual statements, and a reworded term is a different term.
  terms: {
    title: 'Terms of Service (Nutzungsbedingungen)',
    lastUpdated: 'November 3, 2026',
    acceptanceHeading: '1. Acceptance of Terms',
    // The <Link> to the privacy policy sits mid-sentence, so the sentence is
    // whole with the link text as a param. Same shape throughout this section.
    acceptanceBody:
      'By accessing or using All-Chat you agree to these Terms of Service and our {privacy}. If you disagree with any part, you should discontinue use immediately.',
    privacyLinkText: 'Privacy Policy',
    descriptionHeading: '2. Description of Service',
    descriptionBody:
      'All-Chat combines live chat and events from Twitch, YouTube, Kick, TikTok and Discord, and in beta or early access from Owncast, GoodGame, Picarto, Altstream and Facebook, into overlays you show on your stream: chat, alerts and credit rolls, and on beta.allch.at also goals and lists. It also lets you send chat messages, moderate across platforms, run polls, predictions and a points game, have chat read aloud, and connect the browser extension, desktop plugins and API tokens. On beta.allch.at it also runs chat commands, custom bots and timers. You broadcast overlays via OBS or any browser source.',
    accountsHeading: '3. Accounts & Authentication',
    accountsIntro:
      'You are responsible for all activity that happens under your account. You agree to:',
    accountsAccurate: 'Provide accurate registration details',
    accountsSecurity: 'Maintain the security of your credentials and OAuth grants',
    accountsAccessTokens:
      'Keep personal access tokens and paired-device links secret; anything done with them counts as done by you',
    accountsModerators:
      'Invite moderators with care; they act on your overlays within the permissions you grant them',
    accountsNotify: 'Notify us at {email} if you suspect compromise',
    accountsComply: 'Comply with the terms of every platform you connect',
    supportEmail: 'all.chat.support@gmail.com',
    acceptableUseHeading: '4. Acceptable Use',
    acceptableUseIntro: 'You agree not to misuse the Service, including but not limited to:',
    acceptableUseLaws: 'Breaking local, national, or international laws',
    acceptableUseIp: 'Infringing intellectual property or privacy rights of others',
    acceptableUseMalware: 'Uploading malware, spam, or malicious code',
    acceptableUseBypass: 'Attempting to bypass authentication, rate limits, or security controls',
    acceptableUseHarassment: 'Harassing or abusing other users',
    acceptableUsePartner: 'Using All-Chat in a way that violates partner platform policies',
    acceptableUseAutomation:
      "Using custom code, chat command URL fetches or bots to attack, overload or scrape other services, or to collect viewers' data",
    acceptableUseSpam: 'Flooding chats with bots, timers or automated messages',
    contentHeading: '5. Your Content',
    contentOwnership:
      'You keep all rights to what you create or upload, such as custom code and CSS, media, chat commands and translations. You grant us the non-exclusive right to store, process and display it as far as needed to run the Service for you.',
    contentResponsibility:
      'You are responsible for your content and for what your custom code, chat commands, bots and alert media do. Uploaded media can be opened by anyone who has its address. We may remove content that breaks these Terms or the law.',
    contentTranslations:
      'Translations you contribute that we accept are published as part of All-Chat under the AGPL-3.0. By submitting them you confirm that you may license them on those terms.',
    thirdPartyHeading: '6. Third-Party Integrations',
    thirdPartyIntro:
      'All-Chat relies on third-party APIs. Their availability, scopes, and rate limits may change.',
    thirdPartyComply: "You must comply with each platform's terms of service",
    thirdPartyOutages: 'We are not accountable for outages or policy shifts by those platforms',
    thirdPartyQuotas: 'Platform-specific quotas can impact overlay functionality',
    thirdPartyUnofficial:
      "Some integrations (TikTok, YouTube's public chat endpoints, Kick chat, Picarto, Twitch clip playback) use unofficial interfaces that the platform can change or block at any time",
    thirdPartyOwnAccounts:
      'Services you connect with your own account or API key, such as ElevenLabs or Patreon, are governed by their own terms',
    youtubeBinding:
      'YouTube Integration: By using All-Chat to connect to YouTube, you agree to be bound by the {youtubeTerms}.',
    youtubeTermsLinkText: 'YouTube Terms of Service',
    betaHeading: '7. Beta & Early Access',
    betaBody:
      'Features and platforms offered on beta.allch.at or as early access are previews. They can change, break or be removed without notice. beta.allch.at uses the same account and data as allch.at.',
    pointsHeading: '8. Points, Polls & Predictions',
    pointsBody:
      "Points are a virtual score within a streamer's overlay. Viewers earn them under rules the streamer sets: for watching, and for subscribing, gifting subscriptions, cheering bits or sending Super Chats on the platform. All-Chat does not sell points. They have no monetary value, cannot be transferred to another viewer or to another streamer's overlay, and cannot be paid out. Their only use is predictions, a game played with these points only. Streamers are responsible for running polls, predictions and points in line with the platforms' rules and the law that applies to them.",
    privacyHeading: '9. Privacy',
    // The no-break space before 5.8 was an &nbsp; entity and the dash an
    // &ndash;. Both are part of the rendered text, not markup.
    privacyBody:
      'Your use of All-Chat is also governed by our {privacy}, which explains what we collect, how it is used, and your rights under the DSGVO. For transparency: All-Chat measures usage with {analytics} (Umami) that set nothing on your device and recognise returning visitors only by a pseudonymous hash that changes every month – see Section\u00A05.8 of the Privacy Policy.',
    privacyAnalyticsEmphasis: 'self-hosted, cookieless analytics',
    licenseHeading: '10. Open Source License',
    // The link text carries the "(AGPL-3.0)" suffix that the sentence's own
    // {license} run does not, because the <a> in the source wraps both.
    licenseBody:
      'All-Chat is released under the {license}. That means you may use, study, modify, and distribute the software as long as your derivative works also inherit the AGPL-3.0 terms. If you run a modified version of All-Chat as a hosted service, you must provide the source to your users.',
    licenseLinkText: 'GNU Affero General Public License v3.0 (AGPL-3.0)',
    licenseRepository: 'The canonical repository is available on {github}.',
    githubLinkText: 'GitHub',
    availabilityHeading: '11. Availability & Support',
    availabilityIntro: 'We aim for high uptime but do not guarantee:',
    availabilityUptime: 'Uninterrupted access or zero bugs',
    availabilityCompat: 'Compatibility with every browser or streamer setup',
    availabilityFixes: 'Immediate fixes or feature requests',
    availabilitySupport:
      'Support for the hosted service at {host} is best-effort. Community/self-hosted deployments must rely on their own maintainers or the open source community for assistance.',
    availabilityStatus: 'The current service status is published at {status}.',
    statusHost: 'status.allch.at',
    hostedDomain: 'allch.at',
    liabilityHeading: '12. Limitation of Liability',
    liabilityGross:
      'We are liable without limitation for damages caused intentionally or by gross negligence, for injury to life, body, or health, and under the German Product Liability Act (Produkthaftungsgesetz).',
    liabilitySlight:
      'In cases of slight negligence, we are liable only for the breach of essential contractual obligations (Kardinalpflichten): obligations whose fulfilment makes the proper performance of the contract possible in the first place and on whose fulfilment you may regularly rely. In such cases, our liability is limited to the damage that is foreseeable and typical for this type of service. Any further liability is excluded.',
    liabilityAgents:
      'These limitations also apply in favour of our legal representatives and vicarious agents (Erfüllungsgehilfen).',
    indemnityHeading: '13. Indemnification',
    indemnityBody:
      'You agree to indemnify All-Chat against third-party claims, including the reasonable costs of legal defence, arising from your culpable violation of these Terms, applicable law, or the rights of others. This does not apply to the extent you are not responsible for the violation.',
    premiumHeading: '14. Premium Subscriptions & Right of Withdrawal (Widerrufsrecht)',
    patreonLinkText: 'Patreon',
    premiumBody:
      "The core All-Chat service is free of charge. Premium features for streamers, and the cosmetic viewer premium, are unlocked through a paid membership on {patreon}: you subscribe to our campaign on patreon.com and then connect your Patreon account to All-Chat. The subscription contract, billing, cancellation, and any statutory right of withdrawal for the paid membership are handled by Patreon under Patreon's own terms. All-Chat itself does not charge you and does not process payments.",
    premiumGrants:
      'We may grant premium access free of charge, for example to beta testers and ambassadors, and may end such grants at any time.',
    premiumCancellation:
      'You may stop using All-Chat and delete your account at any time via the Settings page. Upon deletion, your personal data is removed as described in our Privacy Policy. Note that deleting your All-Chat account does not cancel a Patreon membership; cancel it directly on Patreon.',
    changesHeading: '15. Changes to Terms',
    changesBody:
      'We may update these Terms over time. When we make material changes, we post the new version here with an updated date and ask streamers to accept it in a prompt after their next login. Viewers who only use the browser extension, the participation page or the viewer settings are not shown that prompt.',
    terminationHeading: '16. Termination',
    terminationBody:
      'We reserve the right to suspend or terminate your access for any breach of these Terms or abusive behavior, and to remove content and connections linked to a suspended account. You may stop using the Service at any time by deleting your account in the Settings page.',
    governingLawHeading: '17. Governing Law & Jurisdiction',
    governingLawBody:
      'These Terms are governed by the laws of the Federal Republic of Germany, excluding the UN Convention on Contracts for the International Sale of Goods (CISG). If you are a consumer, the mandatory consumer protection provisions of the country in which you habitually reside remain unaffected. If you are a merchant (Kaufmann), a legal entity under public law, or a special fund under public law, the exclusive place of jurisdiction is the domicile of the operator.',
    legalNoticeHeading: '18. Legal Notice',
    legalNoticeBody:
      "The operator's identity and contact details are available in the {impressum}.",
    impressumLinkText: 'Impressum',
    contactHeading: '19. Contact',
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
  // The blocking post-login prompt shown while users.tos_accepted is false:
  // for new accounts on their first login and for everyone after a reset when
  // the terms change, so the copy fits both. Both documents are links the user
  // can open (new tab) before accepting.
  tosAcceptance: {
    title: 'Terms of Service and Privacy Policy',
    body: 'Please read our {terms} and {privacy} and accept them to continue using All-Chat.',
    termsLinkText: 'Terms of Service',
    privacyLinkText: 'Privacy Policy',
    newTabHint: 'Opens in a new tab',
    agreement: 'I have read and accept the Terms of Service and the Privacy Policy.',
    accept: 'Accept',
    accepting: 'Saving…',
    logout: 'Log out',
    error: 'Your acceptance could not be saved. Please try again.',
  },
} as const
