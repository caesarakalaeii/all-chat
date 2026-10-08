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
 * The public search-intent guides: /obs-chat-overlay, /obs-chat-dock,
 * /tiktok-live-chat-overlay, /multistream-chat and /compare.
 *
 * Each page keeps its own metaTitle/metaDescription here rather than in
 * `metadata`, so a guide and its search snippet are edited together.
 *
 * These pages are read by search engines and language models as statements of
 * fact. Claim nothing the product does not do today: custom alerts are in
 * development, and the comparison cells about other tools must each stay
 * backed by the source the /compare page lists for them.
 */

export const guides = {
  shared: {
    breadcrumbLabel: 'Breadcrumb',
    breadcrumbHome: 'Home',
    eyebrow: 'Guide',
    ctaHeading: 'Set it up',
    ctaBody:
      'All-Chat is free. Sign in with Twitch, YouTube or Kick, create an overlay, and paste its URL into OBS.',
    ctaSignIn: 'Sign in',
    ctaDocs: 'Read the docs',
    relatedHeading: 'More guides',
    limitsHeading: 'Limits',
    premiumLinkText: 'Premium',
    tiktokGuideLinkText: 'TikTok guide',
    linkObsOverlay: 'OBS chat overlay',
    linkObsDock: 'OBS chat dock',
    linkTiktok: 'TikTok LIVE chat in OBS',
    linkMultistream: 'Multistream chat',
    linkCompare: 'Comparison with other chat tools',
  },
  obsOverlay: {
    metaTitle: 'OBS Chat Overlay for Twitch, YouTube, Kick and TikTok',
    metaDescription:
      'One OBS Browser Source that shows Twitch, YouTube, Kick, TikTok and Discord chat together. The URL, the size, the transparent background and the source settings, step by step.',
    breadcrumb: 'OBS chat overlay',
    heading: 'OBS chat overlay for Twitch, YouTube, Kick, TikTok and Discord',
    intro:
      'All-Chat merges chat from every platform you stream on into one feed and serves it as a web page. OBS shows that page as a Browser Source, so there is nothing to install.',
    stepsHeading: 'Add the overlay to OBS',
    stepSignIn: 'Sign in at {home} with your Twitch, YouTube or Kick account.',
    stepSignInLinkText: 'allch.at',
    stepCreate:
      'In the dashboard, create an overlay and connect the platforms you stream on. Each platform becomes a chat source on that overlay.',
    stepCopy: 'Copy the overlay URL. It has the form {url}.',
    stepAdd:
      'In OBS, click {plus} under Sources, choose {browser}, and paste the URL into the URL field.',
    stepAddPlus: '+',
    stepAddBrowser: 'Browser',
    stepSize:
      'Set the width and height to the area chat should fill, for example 400 × 800. Do not size it to your full canvas.',
    stepShutdown: 'Decide whether to check {shutdown}. The section below explains the trade-off.',
    stepShutdownEmphasis: 'Shutdown source when not visible',
    stepDone: 'Click OK. Chat appears as soon as the overlay connects.',
    backgroundHeading: 'Transparent background',
    backgroundBody:
      'The overlay page has a transparent background, so only the messages show over your scene. You do not need a chroma key or extra CSS in OBS for this.',
    shutdownHeading: 'Shutdown source when not visible',
    shutdownBody:
      'All-Chat reads chat from your platforms only while an overlay is connected. With {shutdown} checked, OBS closes the page whenever the source is not in the active scene. After a short grace period All-Chat stops reading your chat, and it starts again when the source is visible.',
    shutdownTradeoff:
      'Check it if you want chat capture to stop while the overlay is hidden. Leave it unchecked if you switch scenes often and want the feed already running when you come back.',
    shutdown247:
      'Running OBS around the clock for an IRL or 24/7 stream? Read the {section} section of the docs first. It covers a passive overlay URL that keeps YouTube discovery from parking while you are offline.',
    shutdown247LinkText: '24/7 and IRL',
    themesHeading: 'Themes and custom CSS',
    themesBody:
      'Pick one of the {themes} in the overlay editor. Font, spacing, colors and avatars are adjustable in the Appearance panel without code. For full control, write your own styles with {css}.',
    themesLinkText: '16 built-in themes',
    cssLinkText: 'custom CSS',
    platformBadges:
      'Each message can show a badge for the platform it came from. Turn it on in the overlay editor, or style each platform yourself with custom CSS.',
    emotesHeading: 'Emotes',
    emotesBody:
      '7TV, BTTV and FFZ emotes render in the overlay next to native Twitch and YouTube emotes. There is nothing to set up for them.',
    limitYoutube:
      'YouTube chat works for public streams that are live. Unlisted, private and scheduled streams are not picked up.',
    limitTiktokReadOnly:
      'You cannot reply to or moderate TikTok chat from All-Chat. TikTok provides no API for either.',
    limitRetention:
      'Chat messages are deleted from All-Chat after about an hour. The overlay is a live feed, not an archive.',
    limitTiktok:
      'TikTok is the platform most likely to drop out for a short time. The {guide} explains why.',
  },
  obsDock: {
    metaTitle: 'OBS Chat Dock for Twitch, YouTube, Kick and TikTok',
    metaDescription:
      'Dock merged Twitch, YouTube, Kick, TikTok and Discord chat inside OBS with View > Docks > Custom Browser Docks. Read, reply, moderate and run polls from one panel.',
    breadcrumb: 'OBS chat dock',
    heading: 'OBS chat dock for Twitch, YouTube, Kick, TikTok and Discord',
    intro:
      'Every All-Chat overlay comes with a chat monitor: the same messages, laid out for you to read instead of for viewers to watch. OBS can hold the monitor as a dock next to Scenes and Sources, so chat stays in the window you stream from.',
    urlHeading: 'The dock URL',
    urlBody:
      'The dock link for an overlay is {url}: the monitor laid out for a narrow panel. In the overlay editor, click Copy dock URL to copy it. Without ?dock=1 the same address opens the full-width monitor in a browser tab.',
    stepsHeading: 'Add the monitor as an OBS dock',
    stepMenu: 'In OBS, open {menu}.',
    stepMenuPath: 'View > Docks > Custom Browser Docks',
    stepPaste:
      'Enter a name for the dock, paste the dock URL into the URL field, then click Apply and close the dialog.',
    stepSignIn:
      'Sign in inside the dock. OBS docks use their own browser profile, so a sign-in in your normal browser does not carry over. You only do this once.',
    stepPlace:
      'Drag the panel where you want it. A narrow column beside Scenes and Sources works well.',
    streamlabsNote:
      'Streamlabs Desktop has custom browser docks too, and the dock URL works there the same way.',
    featuresHeading: 'What you can do from the dock',
    featureRead: 'Read every platform in one list, with an activity feed beside it.',
    featureSend:
      'Send messages as yourself to Twitch, YouTube or Kick. TikTok and Discord have no send API, so they are read-only here.',
    featureModerate:
      'Delete, time out, ban and unban on the platform the message came from: all four on Twitch, Kick and Discord, time out and ban on YouTube. TikTok has no moderation API. Moderating from All-Chat is a {premium} feature.',
    featureEngage:
      'Start and close polls, and start, lock and pay out predictions. They run across every connected platform, and viewers join from chat.',
    featureMods:
      'Hand the monitor to your moderators. They act with their own platform accounts and do not need Premium themselves. Adding moderators needs {premium} on your account.',
    sameFeedHeading: 'Same feed as the overlay',
    sameFeedBody:
      'The dock and the OBS overlay show the same feed. Display settings in the monitor, such as avatars, timestamps and platform icons, change only your view, never what viewers see on stream.',
    limitPermissions:
      'The first time you send or moderate, the monitor asks you to grant extra permissions with {enable}. For Discord that means inviting the bot again.',
    limitPermissionsEmphasis: 'Enable moderation & chat sending',
    limitTiktok:
      'TikTok chat in the dock is read-only, and TikTok is the platform most likely to drop out for a short time. The {guide} has the details.',
    moreDocs: 'The {monitor}, {moderation} and {engagement} sections of the docs go further.',
    moreDocsMonitor: 'chat monitor',
    moreDocsModeration: 'moderation',
    moreDocsEngagement: 'polls and predictions',
  },
  tiktok: {
    metaTitle: 'TikTok LIVE Chat Overlay for OBS',
    metaDescription:
      'Show TikTok LIVE chat in OBS next to Twitch, YouTube, Kick and Discord chat. How All-Chat reads TikTok without an official API, and where the limits are.',
    breadcrumb: 'TikTok LIVE chat overlay',
    heading: 'TikTok LIVE chat overlay for OBS',
    intro:
      'All-Chat shows TikTok LIVE chat in the same OBS overlay as your Twitch, YouTube, Kick and Discord chat. TikTok makes this harder than any other platform, so this page also explains how it works and where it can break.',
    stepsHeading: 'Add TikTok to an overlay',
    stepOpen: 'Open your overlay in the All-Chat dashboard.',
    stepConnect:
      'Click {connect} and enter the TikTok username of the creator. There is no TikTok login step.',
    stepConnectEmphasis: 'Connect TikTok',
    stepLive:
      'Go live on TikTok. Chat appears in the overlay once All-Chat sees that the account is live.',
    stepObs: 'If the overlay is not in OBS yet, follow the {guide}.',
    stepObsLinkText: 'OBS chat overlay guide',
    apiHeading: 'No tool has an official TikTok LIVE chat API',
    apiBody:
      "TikTok has no public API for LIVE chat. Every tool that shows TikTok LIVE chat, All-Chat included, reads it the way TikTok's own web player does.",
    approachHeading: 'What All-Chat does about it',
    approachSigner:
      'All-Chat maintains its own {signer} instead of depending only on a third-party sign server. The decision and its history are in {adr}.',
    approachSignerLinkText: 'open-source signing service',
    approachAdrLinkText: 'ADR-0052',
    approachSessions:
      'On 2026-09-09 TikTok began requiring browser-grade sessions for live chat data. All-Chat tracked the change the same week; the updates are recorded in the same ADR.',
    approachFallback:
      'Rooms on {premium} accounts get a second delivery path when TikTok refuses the primary connection.',
    limitInterruptions:
      'TikTok is the platform most likely to see short interruptions. When TikTok changes its web player, every tool that reads TikTok LIVE chat has to adapt, All-Chat included. Twitch, YouTube, Kick and Discord chat in the same overlay keep running while that happens.',
    limitReadOnly:
      'You cannot reply to or moderate TikTok chat from All-Chat. TikTok provides no API for either.',
    verticalNote:
      'Streaming vertically to TikTok and horizontally elsewhere? The {guide} covers a second overlay for the vertical canvas.',
    verticalNoteLinkText: 'multistream chat guide',
  },
  multistream: {
    metaTitle: 'Multistream Chat: Twitch, YouTube, Kick and TikTok in One Feed',
    metaDescription:
      'Merge chat from every platform you multistream to into one OBS overlay and one moderation panel. Works with SE.Live, Restream, Streamlabs Multistream or several RTMP outputs.',
    breadcrumb: 'Multistream chat',
    heading: 'Multistream chat: one feed for Twitch, YouTube, Kick, TikTok and Discord',
    intro:
      'Stream to four platforms at once and chat arrives in four places. All-Chat merges it, plus Discord, into one feed for your OBS overlay and one panel for you to read and moderate.',
    methodHeading: 'Works with any multistream setup',
    methodBody:
      'All-Chat connects to the chat of each platform directly and never touches your video. How the video gets out does not matter: SE.Live, Restream, Streamlabs Multistream, several RTMP outputs from OBS, or a second PC all work the same way.',
    methodSources:
      'Chat does not depend on sending your stream through a particular service. Connect each platform as a chat source on your overlay and the feed follows.',
    platformsHeading: 'Tell the platforms apart',
    platformsBody:
      'Each message can show a badge for the platform it came from, so you and your viewers see at a glance whether it came from Kick or YouTube. Turn badges on in the overlay editor. With {css} you can give each platform its own color, because every message is tagged with its platform.',
    platformsCssLinkText: 'custom CSS',
    moderationHeading: 'One place to moderate',
    moderationBody:
      'The chat monitor lists every platform together. From it you can delete, time out, ban and unban on Twitch, Kick and Discord, and time out and ban on YouTube, each applied on the platform the message came from. TikTok has no moderation API. Moderating from All-Chat is a {premium} feature, and you can hand the monitor to your moderators.',
    moderationDock: 'Keep the monitor inside OBS with the {guide}.',
    moderationDockLinkText: 'OBS chat dock guide',
    engagementHeading: 'Polls and predictions across platforms',
    engagementBody:
      'All-Chat runs its own polls, predictions and viewer points. Viewers on every connected platform take part from chat, so a poll is not limited to your Twitch audience.',
    overlaysHeading: 'Separate overlays for separate canvases',
    overlaysBody:
      'An account can have several overlays, each with its own chat sources and theme. A common setup is one overlay with every platform for the horizontal canvas, and a narrower second overlay for a vertical canvas, for example one that shows only TikTok chat.',
    limitTiktok:
      'TikTok is the platform most likely to drop out for a short time. The {guide} explains why.',
    limitYoutube:
      'YouTube chat works for public streams that are live. Unlisted, private and scheduled streams are not picked up.',
    compareNote:
      'Choosing between tools? The {compare} lists what Streamlabs, SE.Live, Social Stream Ninja, SleepyChat and Restream Chat offer, with sources.',
    compareNoteLinkText: 'comparison page',
  },
  // Every cell about another product states only what that product's own pages
  // say; the page pairs each one with its source URL. When a page says nothing,
  // the cell is notDocumented, never a guess.
  compare: {
    metaTitle: 'Multistream Chat Tools Compared',
    metaDescription:
      'All-Chat next to Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat and Restream Chat: platforms, install, OBS overlay and dock, emotes, price, open source, alerts and TTS, with a source for every cell.',
    breadcrumb: 'Comparison',
    heading: 'All-Chat compared with other multistream chat tools',
    intro:
      'This table puts All-Chat next to five other tools that merge chat from several platforms. We make All-Chat, so read it with that in mind. Every claim about another tool links to that tool\'s own pages, and where we found no statement we wrote "not documented" instead of guessing.',
    lastChecked: 'Last checked: {date}',
    caption:
      'Multistream chat tools by feature, last checked {date}. Numbered links point to the sources listed below the table.',
    colTool: 'Tool',
    colPlatforms: 'Twitch, YouTube, Kick, TikTok chat',
    colInstall: 'Install required',
    colOverlay: 'OBS overlay',
    colDock: 'OBS dock',
    colEmotes: '7TV, BTTV, FFZ',
    colPrice: 'Price for merged chat',
    colOpenSource: 'Open source',
    colSelfHost: 'Self-hostable',
    colAlerts: 'Alerts',
    colTts: 'TTS',
    notDocumented: 'Not documented',
    yes: 'Yes',
    allThree: 'Yes, all three',
    sourceRefLabel: 'Source {number}',
    toolAllChat: 'All-Chat',
    allChatPlatforms: 'All four, plus Discord',
    allChatInstall: 'No. Paste a URL into a Browser Source',
    allChatDock: 'Yes, the chat monitor',
    allChatPrice: 'Free. Premium extras through Patreon',
    allChatOpenSource: 'Yes, AGPL-3.0',
    allChatSelfHost: 'Yes, with Docker Compose',
    allChatAlerts: 'In development',
    allChatTts: 'Yes. Browser voices free, ElevenLabs on Premium with your own key',
    toolStreamlabs: 'Streamlabs Chat Box and Multichat',
    streamlabsPlatforms:
      'Twitch, YouTube, Kick. TikTok is not in the Chat Box platform list; a help article says TikTok chat opens in a separate browser window',
    streamlabsInstall:
      'No for the Chat Box, which is a Browser Source URL. Multichat comes with Streamlabs Multistream',
    streamlabsOverlay: 'Yes, the Chat Box',
    streamlabsDock: 'Multichat in Streamlabs Desktop. An OBS dock is not documented',
    streamlabsPrice:
      'Chat Box free. Multistream is an Ultra feature; free Dual Output covers one horizontal and one vertical destination',
    streamlabsOpenSource: 'Streamlabs Desktop is GPL-3.0. Widgets: not documented',
    streamlabsAlerts: 'Yes, Alert Box',
    streamlabsTts: 'Yes, tips and bits read aloud in Alert Box',
    toolSeLive: 'StreamElements SE.Live',
    seLivePlatforms:
      'Multi-Chat shows every connected platform, and Twitch, YouTube and Kick can be connected. TikTok chat has its own dock, outside Multi-Chat',
    seLiveInstall: 'Yes, an OBS plugin, Windows only',
    seLiveOverlay:
      'A merged chat overlay is not documented. In February 2025 staff said the chat widget shows only the primary platform',
    seLiveDock: 'Yes, Chat dock and Multi-Chat window',
    seLivePrice: 'Free plugin',
    seLiveAlerts: 'Yes, overlay alerts',
    seLiveTts: 'Yes, through Stream Store redemptions',
    toolSsn: 'Social Stream Ninja',
    ssnPlatforms: 'All four, plus 100+ other sites',
    ssnInstall: 'Yes, a browser extension or desktop app. A Lite web app has fewer features',
    ssnDock: 'Yes, the chat dock page',
    ssnPrice: 'Free. Some optional integrations need paid accounts',
    ssnOpenSource: 'Yes, GPL-3.0',
    ssnAlerts: 'Yes, event alert overlays',
    ssnTts: 'Yes. System voices, plus optional paid providers',
    toolSleepyChat: 'SleepyChat',
    sleepyChatPlatforms: 'All four',
    sleepyChatInstall: 'No. Copy a link from the web dashboard',
    sleepyChatPrice: 'Free tier with usage limits. Premium raises them',
    sleepyChatOpenSource: 'Not documented. The public GitHub repository is an issue tracker',
    sleepyChatAlerts: 'Yes, custom animated alerts',
    sleepyChatTts: 'Yes, one queue across all four platforms',
    toolRestream: 'Restream Chat',
    restreamPlatforms: 'Twitch, YouTube, Kick. TikTok chat is not read',
    restreamInstall: 'No. Runs in the browser; a desktop app is optional',
    restreamOverlay: 'Yes, an embed URL as a Browser Source',
    restreamPrice:
      'Free plan includes chat. Messages arrive only while you stream through Restream',
    restreamTts: 'Text-to-speech alerts in the desktop chat app',
    sourcesHeading: 'Sources',
    sourceAllChatReadme: 'All-Chat README on GitHub',
    sourceStreamlabsChatBox: 'Streamlabs: Chat Box widget',
    sourceStreamlabsMultistream: 'Streamlabs: Multistream page and Multichat FAQ',
    sourceStreamlabsWidgets: 'Streamlabs: widgets page and FAQ',
    sourceStreamlabsTiktok: 'Streamlabs Help: How to Go Live on TikTok from a Desktop',
    sourceStreamlabsDesktopRepo: 'Streamlabs Desktop source code on GitHub',
    sourceStreamlabsTts: 'Streamlabs: How to Add Text-to-Speech for Tips (and Bits)',
    sourceSeLiveGuide: 'StreamElements Help: SE.Live, The Complete Guide',
    sourceSeMultichatOverlay: 'StreamElements community: Multichat Overlay request and staff reply',
    sourceSeAlerts: 'StreamElements Help: Setting Up Twitch Alerts with StreamElements Overlays',
    sourceSeTts: 'StreamElements Help: Stream Store, The Complete Guide',
    sourceSsnReadme: 'Social Stream Ninja README on GitHub',
    sourceSsnFeatures: 'Social Stream Ninja: Features',
    sourceSsnParameters: 'Social Stream Ninja: overlay parameters',
    sourceSleepyChatHome: 'SleepyChat home page',
    sourceSleepyChatObs: 'SleepyChat: OBS browser dock and overlay guide',
    sourceSleepyChatPricing: 'SleepyChat: Pricing',
    sourceSleepyChatRepo: 'SleepyChat repository on GitHub',
    sourceRestreamRead: 'Restream Help: Read incoming chat messages',
    sourceRestreamChat: 'Restream: Chat product page',
    sourceRestreamEncoders: 'Restream Help: How to use Restream Chat with encoders',
    sourceRestreamEmbed: 'Restream Help: Embed chat on your streaming software',
    sourceRestreamFree: 'Restream Help: Can I use Restream for free?',
    betterHeading: 'Where another tool is the better pick',
    betterIntro: 'No tool on this list is best at everything. Pick another one if:',
    betterAlerts:
      'You need on-stream alerts today. All-Chat alerts are still in development, while SleepyChat, Streamlabs and StreamElements ship them now.',
    betterTts:
      'You want text-to-speech tied to tips and alerts. Streamlabs reads tips and bits aloud in its Alert Box, and SleepyChat keeps TTS from all four platforms in one queue next to its alerts.',
    betterPlatforms:
      'You stream on platforms beyond these four. Social Stream Ninja covers more than 100 sites, including Facebook, Instagram, Rumble and Zoom.',
    betterRestream:
      'You already multistream through Restream and want to reply and relay messages across platforms from one window. Restream Chat is built into that workflow.',
    fitHeading: 'Where All-Chat fits',
    fitBody:
      'All-Chat needs no install, covers TikTok next to Twitch, YouTube, Kick and Discord, renders 7TV, BTTV and FFZ emotes, and is open source under AGPL-3.0. Merged chat is free; moderation from the chat monitor and a few extras are Premium.',
    corrections: 'Products change. If a cell is out of date, {issue} and we will fix it.',
    correctionsLinkText: 'open an issue on GitHub',
  },
} as const
