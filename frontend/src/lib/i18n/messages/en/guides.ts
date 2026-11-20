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
 * /tiktok-live-chat-overlay, /multistream-chat and /compare, plus their table
 * of contents at /guides.
 *
 * Each page keeps its own metaTitle/metaDescription here rather than in
 * `metadata`, so a guide and its search snippet are edited together.
 *
 * These pages are read by search engines and language models as statements of
 * fact. Claim nothing the product does not do today, and correct a claim
 * in the same change that ships or removes the feature: assistants kept
 * answering that All-Chat has no alerts for weeks after alerts were released. The
 * comparison cells about other tools must each stay
 * backed by the source the /compare page lists for them.
 */

export const guides = {
  shared: {
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
    linkAlerts: 'Stream alerts for OBS',
    linkChatBot: 'Multistream chat bot',
    linkCompare: 'Comparison with other chat tools',
  },
  // The /guides table of contents. Each entry's summary is that guide's own
  // metaDescription (GUIDES in components/guides/GuidePage.tsx).
  index: {
    metaTitle: 'Guides: OBS chat overlay, chat dock, stream alerts, chat bot and multistream chat',
    metaDescription:
      'Step-by-step guides for All-Chat: put merged chat in OBS, dock it inside OBS, show TikTok LIVE chat, merge multistream chat, add stream alerts and a chat bot, and compare chat tools.',
    breadcrumb: 'Guides',
    heading: 'Guides',
    intro:
      'How to get merged Twitch, YouTube, Kick, TikTok and Discord chat into OBS, how to add alerts and a chat bot on top, and how All-Chat compares with other chat tools.',
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
      'YouTube chat works for public live streams on its own. For an unlisted stream, paste its link into the source settings. Members-only chat on your own channel needs premium official-API mode. Scheduled streams are picked up once they go live.',
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
      'Show TikTok LIVE chat in OBS next to Twitch, YouTube, Kick and Discord chat, built to a 99.9% availability target without an official TikTok API.',
    breadcrumb: 'TikTok LIVE chat overlay',
    heading: 'TikTok LIVE chat overlay for OBS',
    intro:
      'All-Chat shows TikTok LIVE chat in the same OBS overlay as your Twitch, YouTube, Kick and Discord chat. TikTok is the hardest platform to support and the least stable of the ones All-Chat connects to, and TikTok chat still runs against the same 99.9% availability target as every other platform. This page explains how.',
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
      'All-Chat signs TikTok connections with its own {signer}; no third-party sign server is involved. The decision and its history are in {adr}.',
    approachSignerLinkText: 'open-source signing service',
    approachAdrLinkText: 'ADR-0052',
    approachSessions:
      'On 2026-09-09 TikTok began requiring browser-grade sessions for live chat data. All-Chat tracked the change the same week; the updates are recorded in the same ADR.',
    approachFallback:
      'Rooms on {premium} accounts get a second delivery path when TikTok refuses the primary connection.',
    approachStory: 'The full story is written up in {post}.',
    limitInterruptions:
      'TikTok is still the least stable platform All-Chat supports. When TikTok changes its web player, every tool that reads TikTok LIVE chat has to adapt, All-Chat included, so short interruptions can happen. Twitch, YouTube, Kick and Discord chat in the same overlay keep running while that happens.',
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
      'YouTube chat works for public live streams on its own. For an unlisted stream, paste its link into the source settings. Members-only chat on your own channel needs premium official-API mode. Scheduled streams are picked up once they go live.',
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
      'All-Chat next to Streamlabs Chat Box, StreamElements SE.Live, Social Stream Ninja, SleepyChat, Restream Chat and Multistream: platforms, install, OBS overlay and dock, emotes, price, open source, alerts and TTS, with a source for every cell.',
    breadcrumb: 'Comparison',
    heading: 'All-Chat compared with other multistream chat tools',
    intro:
      'This table puts All-Chat next to six other tools that merge chat from several platforms. We make All-Chat, so read it with that in mind. Every claim about another tool links to that tool\'s own pages, and where we found no statement we wrote "not documented" instead of guessing.',
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
    allChatAlerts: 'Yes, an alerts overlay, with custom HTML, CSS and JavaScript if you want',
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
    toolMultistream: 'Multistream (multistream.gg)',
    multistreamPlatforms: 'All four, plus Instagram',
    multistreamInstall: 'Yes, a desktop app for Windows or Mac. Linux is not supported',
    multistreamOverlay:
      'Yes, a Browser Source link to the local app. It works only while the app is open on the same PC as OBS',
    multistreamDock: 'Private widgets can be added as OBS docks. A chat dock is not documented',
    multistreamPrice: 'Free in the desktop app. Chat on the website is part of Pro, $10 a month',
    multistreamAlerts: 'Yes, follows, subs, gifts and raids',
    multistreamTts: 'Yes, built-in voices, while the app is running',
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
    sourceMultistreamHome: 'Multistream home page',
    sourceMultistreamDownload: 'Multistream: Download the app',
    sourceMultistreamOverlay: 'Multistream tutorial: Add an overlay to OBS',
    sourceMultistreamChats: 'Multistream guide: See all your stream chats in one window',
    sourceMultistreamPricing: 'Multistream: Pricing',
    sourceMultistreamTts: 'Multistream tutorial: Read chat out loud',
    betterHeading: 'Where another tool is the better pick',
    betterIntro: 'No tool on this list is best at everything. Pick another one if:',
    betterAllInOne:
      'You want the tool that merges your chat to also send your video to every platform. Multistream and Restream do both, and Streamlabs Multistream does on Ultra. All-Chat does not relay video; it adds chat, alerts and a chat bot on top of whatever sends your stream.',
    betterTts:
      'You want text-to-speech tied to tips and alerts. Streamlabs reads tips and bits aloud in its Alert Box, and SleepyChat keeps TTS from all four platforms in one queue next to its alerts.',
    betterPlatforms:
      'You stream on platforms beyond these four. Social Stream Ninja covers more than 100 sites, including Facebook, Instagram, Rumble and Zoom.',
    betterRestream:
      'You already multistream through Restream and want to reply and relay messages across platforms from one window. Restream Chat is built into that workflow.',
    fitHeading: 'Where All-Chat fits',
    fitBody:
      'All-Chat needs no install, covers TikTok next to Twitch, YouTube, Kick and Discord, renders 7TV, BTTV and FFZ emotes, and is open source under AGPL-3.0. Alert overlays, chat commands, keyword triggers and timers are built in, so chat, alerts and a chat bot come from one browser tab with no app left running next to OBS. Merged chat, alerts and commands are free; moderation from the chat monitor and a few extras are Premium.',
    corrections: 'Products change. If a cell is out of date, {issue} and we will fix it.',
    correctionsLinkText: 'open an issue on GitHub',
  },
  // /stream-alerts. The event lists mirror shared/alerts eventDefs: name a
  // platform's event here only once that file has it.
  alerts: {
    metaTitle: 'Stream Alerts for OBS: Twitch, YouTube and TikTok in One Alert Box',
    metaDescription:
      'Free alert box for OBS that covers Twitch, YouTube and TikTok at once: subs, bits, raids, follows, Super Chats, memberships and TikTok gifts, with your own GIF, sound and text or custom HTML, CSS and JavaScript.',
    breadcrumb: 'Stream alerts',
    heading: 'Stream alerts for Twitch, YouTube and TikTok in one alert box',
    intro:
      'An All-Chat alerts overlay is an alert box you add to OBS as a Browser Source. It shows events from every platform it is connected to, so a multistream needs one alert source instead of one per platform. It is free and runs in the browser, with nothing to install.',
    eventsHeading: 'What it shows',
    eventsTwitch:
      'Twitch: subs, resubs, gift subs, bits, raids, follows, channel point redemptions and charity donations.',
    eventsYoutube:
      'YouTube: Super Chats, Super Stickers, new members, member milestones, gifted memberships and new subscribers.',
    eventsTiktok: 'TikTok LIVE: gifts, shares and subscriptions.',
    eventsBody:
      'Alerts pop up one at a time with their text, an image or GIF and a sound, then the source goes empty again.',
    stepsHeading: 'Add an alert box to OBS',
    stepCreate: 'Sign in at allch.at and create an overlay of kind {kind}.',
    stepCreateKind: 'Alerts',
    stepSources:
      'Add a chat source for each platform you stream on. An alerts overlay uses the same sources as a chat overlay.',
    stepStyle:
      'In the {section} section of the overlay editor, set the look of every alert under {all}: an image or GIF, a sound, a duration and a volume.',
    stepStyleSection: 'Alerts',
    stepStyleAll: 'All events (default style)',
    stepObs: 'Copy the overlay URL and add it to OBS as a Browser Source.',
    stepTest: 'Click {test}. The alert plays in the editor preview and in OBS.',
    stepTestButton: 'Send test alert',
    customHeading: 'Make each alert your own',
    customBody:
      'Pick an event type to give it its own text or its own image and sound. The text takes these placeholders: {placeholders}. Fields you leave empty use the default style. Search GIFs in the editor or paste a link.',
    codeBody:
      'For full control, write your own HTML, CSS and JavaScript for an alert. It runs in an isolated sandbox on the overlay.',
    commandsBody:
      'A {commands} can show its own alert on the same overlay: an image, a sound and a text, triggered from any chat.',
    commandsLinkText: 'chat command',
    limitKick:
      'Kick events such as subs and follows are not shown as alerts. A chat command typed in Kick chat can still trigger its own alert.',
    limitUploads:
      'Uploading your own images, GIFs and sounds is a {premium} feature. Searching GIFs and pasting a link are free.',
    limitTiktok:
      'TikTok is the platform most likely to drop out for a short time, and its alerts stop with it. The {guide} has the details.',
  },
  // /multistream-chat-bot (ADR-0079, ADR-0080, ADR-0081).
  chatBot: {
    metaTitle: 'Multistream Chat Bot: Commands and Timers for Twitch, YouTube and Kick',
    metaDescription:
      'Free chat bot for multistreamers: define !commands, keyword triggers and timers once and they answer in Twitch, Kick and YouTube chat. Variables, web lookups, JavaScript, Twitch clips and on-stream alerts.',
    breadcrumb: 'Chat bot',
    heading: 'One chat bot for Twitch, YouTube and Kick: commands, keywords and timers',
    intro:
      'All-Chat has a chat bot built in. You define a command once and it answers in every chat your overlays carry, so a multistream does not need one bot per platform. It runs on All-Chat servers: nothing to install and nothing to leave running.',
    kindsHeading: 'Commands, keywords and timers',
    kindCommand: 'A command fires when a message starts with its trigger, for example !discord.',
    kindKeyword:
      'A keyword fires when its word or phrase appears anywhere in a message, for example clip that.',
    kindTimer:
      'A timer posts a message every X minutes, after every X chat messages, or every X minutes once enough messages came in.',
    stepsHeading: 'Create your first command',
    stepOpen: 'Sign in at allch.at and open {commands} in the top bar.',
    stepOpenNav: 'Commands',
    stepNew: 'Click {create} and enter a trigger such as !discord.',
    stepNewButton: 'New command',
    stepReply: 'Write the reply. It is posted to the chat the message came from.',
    stepSave: 'Save. The command is live in every chat your overlays carry.',
    repliesHeading: 'What a reply can do',
    repliesBody:
      'Replies are templates. Variables insert the chatter, the text after the trigger, a counter, a random number, a pick from a list or the time in a time zone. A urlfetch variable inserts what a web address answers, and a js block runs sandboxed JavaScript.',
    actionsBody:
      'A command can also clip your live Twitch stream, or show its own alert with an image, a sound and a text on your {alerts}.',
    actionsAlertsLinkText: 'alerts overlay',
    accessBody:
      'Each command has a cooldown and can be limited to subscribers, VIPs, moderators or yourself.',
    senderHeading: 'Who sends the replies',
    senderBody:
      'By default the All-Chat Bot answers, on Twitch, Kick and YouTube, without a separate account. You can have replies come from your own account instead. Connecting your own bot account, so replies carry its name, is a {premium} feature.',
    limitYoutube:
      'Replies and timers are free in Twitch and Kick chat. In YouTube chat they are a {premium} feature, because each one spends YouTube API quota.',
    limitReadOnly:
      'TikTok and Discord chats can trigger a command, for example a clip or an alert, but cannot be replied to.',
  },
  // /compare/<tool>: All-Chat next to one tool. The table cells are the
  // /compare cells; only this prose is per tool, and it may state about the
  // other tool only what its /compare cells already source.
  vs: {
    colFeature: 'Feature',
    caption:
      'All-Chat and {tool} by feature, last checked {date}. Numbered links point to the sources listed below.',
    pickThemHeading: 'When {tool} is the better pick',
    pickUsHeading: 'When All-Chat is the better pick',
    fullComparison: 'All the tools side by side are on the {compare}.',
    fullComparisonLinkText: 'comparison page',
    listHeading: 'One-on-one comparisons',
    multistreamName: 'Multistream',
    multistreamMetaTitle:
      'All-Chat vs Multistream (multistream.gg): Chat Overlay, Alerts and Chat Bot',
    multistreamMetaDescription:
      'All-Chat and Multistream side by side: both merge Twitch, YouTube, Kick and TikTok chat and add alerts and a chat bot. One runs in the browser, the other is a desktop app for Windows and Mac. A source for every cell.',
    multistreamHeading: 'All-Chat vs Multistream',
    multistreamIntro:
      'Both tools put Twitch, YouTube, Kick and TikTok chat into one OBS overlay and add alerts, a chat bot and text-to-speech. The difference is where they run. Multistream is a desktop app that also sends your video to the platforms. All-Chat is a hosted, open-source service you add to OBS as a URL. We make All-Chat; every Multistream cell links to its own pages.',
    multistreamPickThem:
      'You want the same app to send your stream to every platform as well. Multistream multistreams from your own computer for free and from its cloud on Pro, and its chat, alerts and chat bot live in the same Windows or Mac app.',
    multistreamPickUs:
      'You do not want another app running next to OBS. An All-Chat overlay is a URL that works on any computer, Linux included, while the Multistream overlay link works only while its app is open on the same PC as OBS. All-Chat also reads Discord, renders 7TV, BTTV and FFZ emotes, takes your own CSS, and is open source and self-hostable.',
    ssnName: 'Social Stream Ninja',
    ssnMetaTitle: 'All-Chat vs Social Stream Ninja: Hosted or Extension-Based Chat Overlay',
    ssnMetaDescription:
      'All-Chat and Social Stream Ninja side by side: both are free and open source and merge Twitch, YouTube, Kick and TikTok chat. One is hosted, the other runs in a browser extension or desktop app.',
    ssnHeading: 'All-Chat vs Social Stream Ninja',
    ssnIntro:
      'Both are free, open-source tools that merge chat from several platforms into an OBS overlay and a dock. Social Stream Ninja captures chat with a browser extension or desktop app on your computer. All-Chat reads the platforms on its servers and hands you a URL. We make All-Chat; every Social Stream Ninja cell links to its own pages.',
    ssnPickThem:
      'You stream on platforms beyond Twitch, YouTube, Kick, TikTok and Discord, or you want everything to run on your own machine. Social Stream Ninja covers more than 100 sites.',
    ssnPickUs:
      'You want nothing to install and nothing that has to stay open. All-Chat needs no extension and no app: the overlay URL works in OBS on any computer. Moderation, polls, alerts, chat commands and timers run from the same account.',
    streamlabsName: 'Streamlabs',
    streamlabsMetaTitle: 'All-Chat vs Streamlabs Chat Box: Multistream Chat With TikTok',
    streamlabsMetaDescription:
      'All-Chat next to the Streamlabs Chat Box and Multichat: platforms, TikTok chat, OBS overlay and dock, emotes, price, alerts and TTS, with a source for every cell.',
    streamlabsHeading: 'All-Chat vs Streamlabs Chat Box and Multichat',
    streamlabsIntro:
      'Streamlabs has two chat tools: the Chat Box widget, a Browser Source for Twitch, YouTube and Kick, and Multichat, which comes with Streamlabs Multistream. All-Chat is a hosted, open-source chat overlay that includes TikTok and Discord. We make All-Chat; every Streamlabs cell links to its own pages.',
    streamlabsPickThem:
      'You already stream with Streamlabs Desktop and want chat, the Alert Box and tips in one account, or you want tips and bits read aloud by text-to-speech.',
    streamlabsPickUs:
      'You stream to TikTok as well. TikTok is not in the Chat Box platform list, while All-Chat merges TikTok LIVE chat into the same overlay as Twitch, YouTube, Kick and Discord. Merged chat from every platform is free in All-Chat, where Streamlabs Multistream is an Ultra feature, and All-Chat is open source and self-hostable.',
    seLiveName: 'StreamElements SE.Live',
    seLiveMetaTitle: 'All-Chat vs StreamElements SE.Live: Merged Chat Overlay for OBS',
    seLiveMetaDescription:
      'All-Chat and StreamElements SE.Live side by side: merged chat overlay, Multi-Chat dock, TikTok, install, price, alerts and TTS, with a source for every cell.',
    seLiveHeading: 'All-Chat vs StreamElements SE.Live',
    seLiveIntro:
      'SE.Live is the StreamElements plugin for OBS on Windows, with a Multi-Chat window for the streamer. All-Chat is a hosted, open-source service that puts the merged chat on stream as a Browser Source. We make All-Chat; every StreamElements cell links to its own pages.',
    seLivePickThem:
      'You already run StreamElements overlays, alerts and a Stream Store on Windows and mainly want to read every chat yourself in an OBS dock.',
    seLivePickUs:
      'You want viewers to see the merged chat on stream. We found no merged chat overlay in the StreamElements documentation, and in February 2025 staff said the chat widget shows only the primary platform. All-Chat also keeps TikTok inside the merged chat instead of in a separate dock, and it needs no plugin and no particular operating system.',
    sleepyChatName: 'SleepyChat',
    sleepyChatMetaTitle: 'All-Chat vs SleepyChat: Multistream Chat Overlays Compared',
    sleepyChatMetaDescription:
      'All-Chat and SleepyChat side by side: both merge Twitch, YouTube, Kick and TikTok chat from a web dashboard with no install. Emotes, price, open source, alerts and TTS, with a source for every cell.',
    sleepyChatHeading: 'All-Chat vs SleepyChat',
    sleepyChatIntro:
      'The two are the closest match on our comparison: both are hosted, need no install, and give you an OBS overlay and a dock for Twitch, YouTube, Kick and TikTok chat. We make All-Chat; every SleepyChat cell links to its own pages.',
    sleepyChatPickThem:
      'You want text-to-speech at the centre of the stream. SleepyChat keeps TTS from all four platforms in one queue next to its custom animated alerts.',
    sleepyChatPickUs:
      'You want open source. All-Chat is AGPL-3.0 and can be self-hosted with Docker Compose. It also reads Discord and renders 7TV, BTTV and FFZ emotes, and its merged chat is free, where the SleepyChat free tier has usage limits.',
    restreamName: 'Restream Chat',
    restreamMetaTitle: 'All-Chat vs Restream Chat: Multistream Chat With TikTok',
    restreamMetaDescription:
      'All-Chat and Restream Chat side by side: platforms, TikTok chat, OBS overlay and dock, price, alerts and TTS, with a source for every cell.',
    restreamHeading: 'All-Chat vs Restream Chat',
    restreamIntro:
      'Restream Chat is the chat tool built into Restream, the service that sends one stream to several platforms. All-Chat does not relay video; it merges chat however your stream reaches the platforms. We make All-Chat; every Restream cell links to its own pages.',
    restreamPickThem:
      'You already multistream through Restream and want to reply and relay messages across platforms from one window.',
    restreamPickUs:
      'You stream to TikTok, or you do not send your stream through Restream. Restream Chat does not read TikTok chat, and its messages arrive only while you stream through Restream. All-Chat reads TikTok LIVE chat and works with any way of going live, and it adds 7TV, BTTV and FFZ emotes, alerts and a chat bot.',
  },
} as const
