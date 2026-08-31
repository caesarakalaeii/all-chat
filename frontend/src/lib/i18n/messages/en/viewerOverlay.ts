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
 * Viewer-facing overlay chrome: labels, buttons, empty states, aria labels.
 *
 * Overlay chat messages and event bodies are viewer-authored content and are
 * never translated, so nothing here describes them.
 */

export const viewerOverlay = {
  engagement: {
    pollHeading: 'Poll',
    predictionHeading: 'Prediction',
    twitchSourceBadge: 'Twitch',

    labelListEntry: '{noun} {index}',
    labelListRemove: 'Remove',
    labelListAdd: 'Add {noun}',

    pollQuestionPlaceholder: 'Question',
    pollQuestionLabel: 'Poll question',
    pollOptionNoun: 'Option',
    pollAllowChange: 'Allow vote changes',
    pollAutoCloseAfter: 'Auto-close after',
    secondsSuffix: 's',
    pollStart: 'Start poll',

    pollVotes: '{total} votes',
    // Appended to pollVotes, so it opens with its own separator.
    pollAutoCloses: ' · auto-closes {time}',
    pollNew: 'New poll',
    pollClose: 'Close poll',
    pollMirroredNote: 'Mirrored from Twitch — viewers vote in the Twitch UI/chat',
    pollParticipateHint:
      'Viewers vote on the {link} or from chat ({voteCommand} or just {shortCommand})',
    participateLink: 'participate page',

    predictionTitlePlaceholder: 'Title (e.g. Will we win this round?)',
    predictionTitleLabel: 'Prediction title',
    predictionOutcomeNoun: 'Outcome',
    predictionAutoLockAfter: 'Auto-lock wagers after',
    predictionStart: 'Start prediction',
    predictionParticipateHint:
      'Viewers wager on the {link} (they can see their balance) — or from chat: {predictCommand}',

    predictionPointsWagered: '{total} points wagered',
    predictionAutoLocks: ' · auto-locks {time}',
    predictionOutcomeTally: '{points} pts · {entrants} entrants',
    winningOutcome: 'Winning outcome',
    winningOutcomeChoice: 'Winning outcome: {label}',
    predictionLock: 'Lock wagers',
    predictionNew: 'New prediction',
    predictionLockedNote: 'Pick the winning outcome, then pay out. Payouts are final.',
    predictionMirroredNote: 'Mirrored from Twitch — runs on Twitch channel points',

    predictionResolve: 'Resolve',
    predictionPayOut: 'Pay out "{label}"',
    predictionPayOutConfirm: 'Pay out "{label}" — final?',
    predictionResolveDisabledTitle: 'Select the winning outcome first',

    predictionCancel: 'Cancel & refund',
    predictionCancelTitle: 'Cancel and refund all wagers',
    predictionCancelConfirm: 'Really refund all wagers?',

    mirrorNote:
      'Mirror native Twitch polls & predictions onto your overlays (read-only). Opt-in; takes effect after the next channel sync (a stream restart or re-adding the source).',
    mirrorEnable: 'Enable Twitch mirroring',
  },

  participate: {
    loading: 'Loading…',
    loginHeading: 'Join the fun',
    loginBlurb: 'Log in with your platform account to vote and wager.',
    loginWith: 'Continue with {platform}',
    noWebLoginNote:
      "Watching on TikTok or Discord? Take part with the on-screen chat commands — web login isn't available for those platforms yet.",

    heading: 'Participate',
    balanceLabel: 'Balance: {balance} {pointsName}',
    balance: '{balance} {pointsName}',
    // Fallback for a streamer who has not renamed their points.
    defaultPointsName: 'Points',

    settledBanner:
      'Your prediction on “{outcome}” settled — you wagered {amount} {pointsName}. Check your balance above.',

    pollNativeNote: 'This poll runs on Twitch — vote in Twitch chat or the Twitch app.',
    pollVoteNativeTitle: 'Vote in Twitch chat',
    working: 'Working…',
    yourVote: '(your vote)',
    pollOptionTally: '{pct}% ({votes})',

    predictionLocked: 'locked',
    predictionNativeNote: 'This prediction runs on Twitch channel points.',
    youHave: 'You have {balance} {pointsName}',
    maxWager: 'Max',
    noPointsYet:
      'You have no {pointsName} yet. Earn them by keeping this page open and by supporting the stream (subs, bits, donations, gifts), then come back to wager.',
    wagerAmountLabel: 'Amount to wager in {pointsName}',
    wagerAmountPlaceholder: 'Amount to wager ({pointsName})',
    // Appended to the outcome label, so it opens with its own separator.
    yourWager: ' · your wager: {amount}',
    outcomeTally: '{points} · {pct}%',
    alreadyWagered: "You've locked in your wager for this round.",
    nothingActive: 'No active poll or prediction right now. Hang tight!',

    wagerNativeTitle: 'Runs on Twitch channel points',
    wagerAlreadyTitle: 'You already wagered this round',
    wagerClosedTitle: 'Betting is closed',

    lockedAnnouncement: 'Prediction locked — betting is closed.',

    // Server reasons for a rejected wager; see repository/predictions.go.
    rejectNotFound: 'This prediction is no longer available.',
    rejectNotActive: 'Betting is closed for this round.',
    rejectBadOutcome: 'That outcome is not valid.',
    rejectAlreadyWagered: 'You already placed a wager this round.',
    rejectInsufficient: 'Not enough {pointsName}. You have {balance}.',
    rejectNative: 'This prediction runs on Twitch channel points.',
    // Worded differently from rejectInsufficient on purpose: this one fires
    // locally, before the request, and both reach viewers today.
    insufficientLocal: 'Not enough {pointsName} — you have {balance}.',

    loginFailed: 'Could not start login. Please try again.',
    voteFailed: 'Vote failed',
    wagerFailed: 'Wager failed',
    wagerNeedsAmount: 'Enter a positive amount to wager.',
  },

  pollWidget: {
    finalBadge: 'Final',
    finalResults: 'Final results',
    // P3-12: a non-unique top vote count is a tie, and every tied option says so.
    winnerPill: 'Winner',
    tiePill: 'Tie',
    optionTally: '{pct}% ({votes})',
    remaining: '{clock} remaining',
  },

  predictionWidget: {
    // The padlock and trophy have no sibling word here, so they are part of what
    // the badge reads rather than decoration beside it.
    stateLocked: '🔒 LOCKED',
    stateResolved: '🏆 RESOLVED',
    stateOpen: 'OPEN',
    winnerPill: 'Winner',
    outcomeTally: '{points} pts · {pct}%',
    pool: '{points} pts wagered · {players} players',
  },

  credits: {
    loading: 'Loading Credits...',
    loadFailed: 'Failed to load credit roll',
    errorHeading: 'Unable to Load Credit Roll',
    errorHint: 'Make sure you have an active streaming session',
    empty: 'No credit roll data available',

    heading: '🎬 Stream Credits',
    subheading: 'Thank you to everyone who supported the stream!',
    session: 'Session: {date}',
    duration: 'Duration: {duration}',

    // English-only pluralisation, so one key per form.
    durationHourOne: '{hours} hour',
    durationHourMany: '{hours} hours',
    durationMinuteOne: '{minutes} minute',
    durationMinuteMany: '{minutes} minutes',
    durationHoursAndMinutes: '{hours} {minutes}',
    durationJustStarted: 'just started',

    topSubscribers: 'Top Subscribers',
    topGifters: 'Top Gifters',
    topCheerers: 'Top Cheerers',
    topChannelPoints: 'Top Channel Points',
    topRaiders: 'Top Raiders',
    topSuperChats: 'Top Super Chats',
    newFollowers: 'New Followers',

    nowPlaying: 'Now Playing',
    clipViews: '{views} views',
    clipCounter: 'Clip {index}/{total}',

    thanks: 'Thank you for watching! ❤️',
    seeYou: 'See you next stream!',
  },

  // The OBS chat overlay renders viewer-authored content — usernames, message
  // bodies, emotes, badges — none of which is translated. This is its only copy.
  chatOverlay: {
    sharedChat: 'Shared Chat',
  },

  activity: {
    heading: 'Activity & Events',
    empty: 'No events yet.',
    modBadge: 'mod',
    // Stands in for the name when a moderation frame carries neither a username
    // nor a resolvable user id.
    someUser: 'a user',

    // One whole sentence per reachable combination. The moderator, the timeout
    // duration and the AutoMod category are each independently optional, and a
    // sentence with an empty hole in it is not something a translator can place.
    deleted: 'Message deleted',
    deletedBy: 'Message deleted by {moderator}',
    cleared: 'Chat cleared',
    clearedBy: 'Chat cleared by {moderator}',
    banned: 'Banned {user}',
    bannedBy: 'Banned {user} by {moderator}',
    timedOut: 'Timed out {user}',
    timedOutFor: 'Timed out {user} for {seconds}s',
    timedOutBy: 'Timed out {user} by {moderator}',
    timedOutForBy: 'Timed out {user} for {seconds}s by {moderator}',
    automodHeld: 'AutoMod held a message from {user}',
    automodHeldCategory: 'AutoMod held a message from {user} ({category})',
    automodResolved: 'AutoMod hold {resolution}',
    automodResolvedBy: 'AutoMod hold {resolution} by {moderator}',
    automodHeldBadge: 'held',
  },

  chatPanel: {
    heading: 'Chat',
    empty: 'No chat messages yet.',
    filteredBy: 'Showing only messages from {user}',
    showAll: 'Show all chat',
    filteredEmpty: 'No messages from {user} yet.',
    filteredCount: '{shown} of {total}',
    sharedBadge: 'shared',
  },

  observability: {
    sources: 'Sources ({count})',
    configuredEvents: 'Configured Events',
    emotes: 'Emotes',
    filters: 'Filters',

    noSources: 'No sources configured.',
    sourceLive: 'live',
    sourceIdle: 'idle',

    eventsUnavailable: 'Event configuration unavailable; events appear here as they arrive.',
    sevenTvSet: '7TV set',
    sevenTvDefault: 'per-source default',

    bannedWords: 'Banned words',
    bannedUsers: 'Banned users',
    minLength: 'Min length',
    hideCommands: 'Hide commands',
    sayHiFilter: 'Say hi filter',
    yes: 'yes',
    no: 'no',
    filtersNote: 'Filters are shown for reference; this view displays all messages.',

    // Worded differently from the overlay editor's event names in
    // overlayEditor.*, so they are their own keys rather than shared.
    eventTwitchSubs: 'Twitch Subs',
    eventTwitchResubs: 'Twitch Resubs',
    eventTwitchGiftSubs: 'Twitch Gift Subs',
    eventTwitchBits: 'Twitch Bits',
    eventTwitchRaids: 'Twitch Raids',
    eventTwitchChannelPoints: 'Channel Points',
    eventTwitchFollows: 'Twitch Follows',
    eventTwitchWatchStreaks: 'Watch Streaks',
    eventYoutubeSuperChat: 'YouTube Super Chat',
    eventYoutubeSuperSticker: 'Super Sticker',
    eventYoutubeMembers: 'YouTube Members',
    eventYoutubeMemberMilestones: 'Member Milestones',
    eventYoutubeMemberGifts: 'Member Gifts',
    eventKickSubs: 'Kick Subs',
    eventKickGifts: 'Kick Gifts',
    eventTiktokLikes: 'TikTok Likes',
    eventTiktokGifts: 'TikTok Gifts',
    eventTiktokFollows: 'TikTok Follows',
    eventTiktokShares: 'TikTok Shares',
    eventTiktokTreasureChests: 'TikTok Coin Chests',
    eventTokenWarnings: 'Token Warnings',
  },

  viewSettings: {
    buttonLabel: 'Display settings',
    buttonText: 'Display',
    heading: 'View settings',
    chatOrderHeading: 'Chat order',
    activitySoundHeading: 'Activity sound',

    showAvatars: 'Avatars',
    showBadges: 'Badges',
    showPronouns: 'Pronouns',
    showTimestamps: 'Timestamps',
    showPlatformGlyph: 'Platform icon',
    showModeration: 'Moderation controls',

    newestFirst: 'Newest messages first',
    newestFirstNote:
      'Puts the newest message at the top of the Chat panel, so you can read chat without looking down. Only affects this browser.',

    activitySoundEnabled: 'Sound on new activity',
    soundPresetLabel: 'Sound',
    volume: 'Volume',
    testSound: 'Test sound',
    activitySoundNote:
      "Plays only here, in this browser, so you notice easy-to-miss activity like channel-point redeems or a TikTok Rose. This is separate from your overlay's on-stream notification sounds.",
  },

  chatSend: {
    formLabel: 'Send a chat message',
    targetGroupLabel: 'Send to',
    sendToPlatform: 'Send to {platform}',
    enableSendingFor: 'Enable sending for {platform}',
    allLabel: 'Send to all platforms',
    allText: 'All',
    messageLabel: 'Chat message',
    placeholderAll: 'Message all platforms\u2026',
    placeholderOne: 'Send a message\u2026',
    send: 'Send',

    enableSending: 'Enable sending',
    reconnect: 'Reconnect',
    enablePlatform: 'Enable {platform}',
    reconnectPlatform: 'Reconnect {platform}',

    sendFailed: 'Could not send. Please try again.',
    // Named and unnamed forms: the error body does not always carry a platform.
    missingScope: "Sending isn't enabled yet.",
    missingScopeFor: "Sending isn't enabled for {platform} yet.",
    reauthRequired: 'Your {platform} login expired. Please reconnect.',
    reauthRequiredGeneric: 'Your platform login expired. Please reconnect.',
    rateLimitedRetry: 'Rate limited \u2014 try again in {seconds}s.',
    rateLimited: 'Rate limited \u2014 please slow down.',
    streamOffline: 'That channel is not live right now.',

    sent: 'Sent \u2713',
    resultOk: '{platform} \u2713',
    resultFailed: '{platform} \u2717',
    resultFailedWhy: '{platform} \u2717 {why}',

    // One word each, naming why a single platform in a send-to-all failed.
    reasonReauthRequired: 'reconnect',
    reasonMissingScope: 'locked',
    reasonStreamOffline: 'offline',
    reasonQuotaExhausted: 'quota',
    reasonSendFailed: 'failed',
  },

  moderationControls: {
    menuLabel: 'Moderate user',
    timeout: 'Timeout',
    ban: 'Ban user',
    unban: 'Unban user',
    deleteMessage: 'Delete message',

    // ADR-0048: each reason names what stands in the way and who can clear it.
    // Sending someone at a fix that is not theirs to make is the failure mode
    // this vocabulary exists to prevent, so none of these collapse together.
    noModerationApi: '{platform} has no moderation API',
    unavailable: 'Moderation is unavailable for this source',
    missingScope: 'Grant moderation permissions to enable mod actions',
    needsDiscordLink: 'Link your Discord account to moderate here',
    ownerChannelUnverified:
      "This streamer's Discord account isn't connected, so nothing can be moderated here",
    botMissingPermission:
      "The All-Chat bot wasn't given this Discord permission \u2014 ask the streamer to re-invite it",
  },

  layoutPicker: {
    groupLabel: 'Panel layout',
    chatLeft: 'Chat left, events right',
    chatRight: 'Chat right, events left',
    chatTop: 'Chat top, events below',
    eventsTop: 'Events top, chat below',
  },

  platformGlyph: {
    groupLabel: 'Platforms: {platforms}',
  },

  monitor: {
    details: 'Details',
    engagement: 'Engagement',
    engagementTitle: 'Run polls and predictions for this overlay',
    rediscoverYouTube: 'Re-discover YouTube',
    rediscoverYouTubeTitle:
      'Force YouTube to re-discover the live stream \u2014 use if chat stopped after a stream crash or restart',
    obsOverlay: 'OBS overlay',

    stillReconnecting:
      'Still reconnecting \u2014 this recovers on its own, and messages sent meanwhile replay when the connection returns. Closing this page is what loses them.',
    replayTruncated:
      'Some earlier messages may be missing \u2014 the disconnection outlasted the replay buffer, so the oldest part of the gap could not be recovered.',

    // Says nothing about the overlay itself: the payload behind it is identical
    // for an overlay that does not exist.
    noRole: "You can view this monitor, but you don't moderate here \u2014 moderation is disabled.",

    // The gate is keyed on the OWNER, so only the owner's form has a call to
    // action \u2014 /upgrade would sell a moderator a plan that is not theirs to buy.
    featureGatedOwner: 'Chat moderation is a premium feature.',
    featureGatedUpgrade: 'Upgrade to moderate from your overlay',
    featureGatedModerator:
      "This streamer's plan doesn't include moderation right now, so your actions are unavailable until they renew it.",

    // The channel name is optional, so two whole sentences rather than one with
    // a fragment appended. {platform} is the raw lowercase wire value, which is
    // what renders today.
    needsConsent: 'Connect your own {platform} account to moderate.',
    needsConsentChannel: 'Connect your own {platform} account to moderate {channel}.',
    connectPlatform: 'Connect {platform}',

    // One banner for the whole overlay: the link is per PERSON, not per server.
    needsDiscordLink:
      'Link your Discord account to moderate Discord here \u2014 All-Chat checks your own server permissions before acting.',
    linkDiscord: 'Link Discord',

    missingScope: 'Grant moderation permissions to enable mod actions for {platform}.',
    missingScopeChannel:
      'Grant moderation permissions to enable mod actions for {platform} ({channel}).',
    missingScopeDiscord:
      'Re-invite the bot with moderation permissions to enable mod actions for {platform}.',
    missingScopeDiscordChannel:
      'Re-invite the bot with moderation permissions to enable mod actions for {platform} ({channel}).',
    reinviteBot: 'Re-invite the bot',
    enableModeration: 'Enable moderation & chat sending',
    comingSoonFor: '(coming soon for {platform})',

    // The scope note is not padding: the consent screen asks for
    // moderator:manage:automod, which on a read-only feature looks like a
    // mistake and gets declined.
    modLogOptIn:
      'Show Twitch moderation actions and AutoMod holds in this activity feed. Twitch requires an AutoMod \u201cmanage\u201d permission to send us held messages at all \u2014 All-Chat only reads them; there are no approve/deny buttons yet.',
    enableModLog: 'Show moderation & AutoMod events',

    // Two role variants: a moderator sent down the streamer's re-consent path
    // half-succeeds and then 404s, so each is told to re-authorize in its own
    // place.
    reauthOwner:
      'Your {platform} moderation permission expired or was never granted \u2014 re-authorize to keep moderating from your overlay.',
    reauthModerator:
      'Your {platform} moderation permission expired or was never granted \u2014 re-authorize to keep moderating here.',
    reconnectPlatform: 'Reconnect {platform}',
    reauthorizeModeration: 'Re-authorize moderation & chat sending',

    // Toasts raised when a consent flow cannot even be started.
    consentStartFailed: 'Could not start moderation setup. Please try again.',
    twitchConsentStartFailed: 'Could not start Twitch consent. Please try again.',
    modConnectUnavailable:
      'Connecting {platform} is not available yet. Ask the streamer to moderate there for now.',
    discordLinkUnavailable:
      'Linking Discord is not available right now. Ask the streamer to moderate there for now.',
    reloginStartFailed: 'Could not start re-login. Please try again.',

    // U+2026 ellipsis.
    rediscoverStarted: 'Re-discovering YouTube stream\u2026',
    rediscoverRateLimited: 'Please wait a moment before retrying',
    rediscoverForbidden: 'Not authorized for this overlay',
    rediscoverFailed: 'Could not trigger re-discovery',

    // Outcomes of a moderation action. The platform is the raw lowercase wire
    // value, which is what renders today.
    reauthNeededToast: '{platform} needs you to re-authorize moderation',
    actionFailed: 'Moderation action failed',
    messageDeleted: 'Message deleted',
    timedOut: 'Timed out {name}',
    banned: 'Banned {name}',
    unbanned: 'Unbanned {name}',
    unbanFailed: 'Unban failed',
    // Stands in for the target's name when the request carries neither a
    // username nor a display name.
    unnamedTarget: 'user',

    // Delegated-moderation failures (ADR-0048), one whole sentence per code.
    connectRequired: 'Connect your own {platform} account to moderate here',
    // Two sentences rather than one: the moderator is told only the cause,
    // because only the streamer can fix it, while the owner is told the remedy.
    ownerChannelUnverifiedModerator:
      "This streamer's {platform} account isn't connected, so nothing can be moderated here",
    ownerChannelUnverifiedOwner:
      "Your {platform} account isn't connected for this channel \u2014 reconnect it to moderate here",
    delegationUnsupported:
      "Moderators can't act on {platform} yet \u2014 ask the streamer to handle this one",
    targetNotActionable:
      "{platform} won't let anyone moderate this person \u2014 they're the channel owner or another moderator",
    // Discord's five. The shared bot performs every write there, so these codes
    // carry the whole explanation and name the person to ask.
    discordLinkRequired: 'Link your Discord account to moderate here',
    modNotInGuild: "You're not in this Discord server \u2014 ask the streamer to invite you",
    modLacksPermission:
      "Your Discord roles don't allow this \u2014 ask the streamer for a role that does",
    modBelowTarget:
      "Discord's role hierarchy blocks this \u2014 your highest role has to sit above theirs",
    botMissingPermission:
      "The All-Chat bot wasn't given this Discord permission \u2014 ask the streamer to re-invite it",
  },

  // Streamer-facing system notices rendered inside an event body. The event's
  // own content is viewer-authored and never translated; these lines are ours.
  eventNotice: {
    tokenExpired: 'OAuth token has expired',
    tokenRefreshFailed: 'Failed to refresh OAuth token',
    tokenExpiredFor: 'OAuth token has expired for {username}',
    tokenRefreshFailedFor: 'Failed to refresh OAuth token for {username}',
    tokenRemedy: '\u2192 Please reconnect your account in Settings \u2192 Connections',

    channelInaccessible: 'Channel {channel} is not accessible',
    channelRemedy: '\u2192 Grant the bot "View Channel" permission in your Discord server settings',

    listenerDeprecated: 'The legacy Twitch chat connection is being retired.',
    listenerRemedy: '\u2192 Re-add your Twitch source to switch to the new EventSub connection',
  },
  // The per-channel connection tooltips on the overlay pages' status strip. Each
  // is one whole string: the ' - ' separator is punctuation a language may
  // change, and neither the order of name and status nor the parenthesis
  // convention is fixed.
  statusIndicator: {
    active: '{platform} - {channel} (Active)',
    inactive: '{platform} - {channel} (Inactive)',
    connected: '{platform} - {channel} (Connected)',
    reconnecting: '{platform} - {channel} - Reconnecting in {seconds}s',
    reconnectingWithError: '{platform} - {channel} - {error} (retry in {seconds}s)',
    quotaExceeded: '{platform} - {channel} - Quota exceeded',
    error: '{platform} - {channel} - Error',
    discoveryPaused: '{platform} - {channel} - Discovery paused (use chat monitor to retry)',
    authRequired: '{platform} - {channel} - Auth Required',
    offline: '{platform} - {channel} - Offline',
    // A backend error_message is not copy; the sentence framing it is.
    withErrorMessage: '{platform} - {channel} - {error}',
    // The seconds abbreviation on the reconnect badge, for the same reason the
    // unit letters in common.duration are copy.
    countdownSeconds: '{seconds}s',
  },
} as const
