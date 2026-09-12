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
 * The public homepage ("lanes" design), the upgrade page and the device-link
 * flow.
 *
 * The homepage is marketing art direction (mockup G): manifesto voice,
 * lowercase mono chrome, five platform lanes. Its copy lives under
 * `lanes`, `convergence`, `wedge`, `numbers`, `steps` and `final`.
 */

export const marketing = {
  header: {
    homeLabel: 'All-Chat home',
    docs: 'Docs',
    dashboard: 'Dashboard',
    signIn: 'Sign in',
  },
  lanes: {
    // The mid-dot wordmark is the marketing treatment of the brand; the app
    // chrome keeps common.brand.wordmark ('all-chat').
    brand: 'all·chat',
    navLabel: 'Main',
    navThemes: 'themes',
    navDocs: 'docs',
    navDiscord: 'discord',
    navDashboard: 'dashboard',
    // The logged-in nav chip; '▸' rides inside the key so no JSX carries it.
    navDashboardChip: '▸ dashboard',
    kicker: 'A DECLARATION OF WAR ON CHAT TOOLING',
    // Two display lines of one headline, not two sentences.
    titleTop: 'EVERY CHAT.',
    titleBottom: 'ONE URL.',
    // The multistream objection, answered in the same voice: one merged
    // chat means one community, and one mod queue means no extra load.
    manifesto: 'YOUR CHATS? MERGED. YOUR COMMUNITY? UNITED.',
    totalLabel: 'MSG DELIVERED',
    cta: 'GET YOUR OVERLAY — FREE →',
    ctaNote: "lane height ≈ share of this week's messages",
    howItWorks: 'how it works ↓',
    backToDashboard: 'BACK TO YOUR DASHBOARD →',
    welcomeNote: 'welcome back, {name} — {count} overlays are live, yours is one click away',
    // Doubles as each sign-in button's aria-label, so one key keeps the
    // accessible name and the visible label from ever drifting apart.
    signInWith: 'Sign in with {platform}',
    // Decorative marquee chatter per lane — curated strings, deliberately not
    // wired to live chat. Lives in the flat flow* groups below, not under
    // lanes.flow.<platform>, because the catalog caps nesting at three
    // segments (messages.test.ts, docs/frontend/I18N.md).
  },
  // Marquee chatter for the twitch lane. m-keys enumerated by the hero's
  // MARQUEE_MESSAGE_COUNTS table.
  flowTwitch: {
    m1: 'that clip was INSANE',
    m2: 'KEKW',
    m3: 'CLIP IT',
    m4: 'GIGACHAD',
    m5: 'W',
    m6: 'been lurking 3 years',
    m7: 'modCheck',
    m8: 'gg',
    m9: 'RAID INCOMING',
    m10: 'lets gooooo',
    m11: 'HOLY',
    m12: 'peepoHappy',
    m13: 'actual cinema',
    m14: 'SourPls SourPls',
    m15: 'OK dude',
    m16: 'chat is this real',
    m17: 'POGGERS',
    m18: 'no way that worked',
    m19: 'first time here, love it',
    m20: 'catJAM catJAM catJAM',
    m21: 'this overlay goes hard',
    m22: 'monkaS',
  },
  // Marquee chatter for the youtube lane.
  flowYoutube: {
    m1: 'hi from the VOD gang',
    m2: 'first time catching live',
    m3: 'audio is crisp today',
    m4: 'timestamp 2:14:33',
    m5: 'subbing now',
    m6: 'this overlay sold me',
    m7: 'great stream as always',
    m8: 'notification squad',
    m9: 'the editor is going to eat this VOD',
    m10: 'chat works here now?',
    m11: 'lurking from the TV app',
    m12: 'premiere hype',
    m13: 'this segment is going in the highlight reel',
    m14: 'joined from the community post',
    m15: 'POGGERS it actually merges',
    m16: 'late but here',
    m17: 'SourPls the stream quality',
    m18: 'chapter 3 is peak',
    m19: 'PepePls premiere starting',
    m20: 'replay the intro',
    m21: 'chat actually synced here',
    m22: 'this is my Roman Empire',
  },
  // Marquee chatter for the tiktok lane.
  flowTiktok: {
    m1: 'came from the fyp',
    m2: 'what game is this',
    m3: 'how is chat in one box',
    m4: 'the algorithm brought me here',
    m5: 'fyp fyp fyp',
    m6: 'live rn??',
    m7: 'saving this',
    m8: 'wait chat is real here',
    m9: 'typing from the bus',
    m10: 'my fyp knows me too well',
    m11: 'this is so satisfying to watch',
    m12: 'the overlay is so clean',
    m13: 'no way this is free',
    m14: 'watching instead of sleeping',
    m15: 'AYAYA finally live',
    m16: 'dropping a follow',
    m17: 'the vibes are immaculate',
    m18: 'KKona he actually did it',
    m19: 'comment section won today',
    m20: 'no bots just chat',
    m21: 'zooming to the end',
    m22: 'been here since 200 views',
  },
  // Marquee chatter for the kick lane.
  flowKick: {
    m1: 'W streamer',
    m2: 'OMEGALUL',
    m3: 'the grind never stops',
    m4: 'kick keep it',
    m5: 'W takes',
    m6: 'clip it quick',
    m7: 'EZ Clap',
    m8: 'this site actually respects chat',
    m9: 'no ads just vibes',
    m10: 'GIGACHAD move',
    m11: 'signing up after this stream',
    m12: 'the 95/5 split goes crazy',
    m13: 'WAYTOODANK',
    m14: 'PartyParrot kickoff',
    m15: 'no kick from me',
    m16: 'the chat speed here is unreal',
    m17: 'LuL streamer.exe stopped',
    m18: 'signed up in 30 seconds',
    m19: 'gambling less, chatting more',
    m20: 'this lane is my home now',
    m21: 'ratio the algorithm',
    m22: 'PETPET the mods',
  },
  // Marquee chatter for the discord lane.
  flowDiscord: {
    m1: 'discord games after the raid!',
    m2: 'gg everyone',
    m3: 'movie night friday',
    m4: 'nice stream',
    m5: 'clips channel is popping off',
    m6: 'GG',
    m7: 'watching from the server vc',
    m8: 'the bot announcements are elite',
    m9: 'lurking while i cook',
    m10: 'stream ping squad assemble',
    m11: 'this overlay in our events channel would go hard',
    m12: 'raid train forming',
    m13: 'peepoHey new here',
    m14: 'Stare at the chat merge',
    m15: 'reactor just said that',
    m16: 'movie night poll is up',
    m17: 'who else is multitasking',
    m18: 'the events calendar is stacked',
    m19: 'first stream on the server',
    m20: 'xdx the overlay works here',
    m21: 'lurking with dinner',
    m22: 'feels like the old internet',
  },
  convergence: {
    label: 'THE WHOLE PRODUCT',
    headingTop: 'FIVE CHATS IN.',
    headingBottom: 'ONE WINDOW OUT.',
    // Bold runs ride as {placeholders} with sibling *Emphasis keys, so word
    // order stays a translator decision. See emphasise()/interpolateElements.
    body: "Your chat tool should weigh {oneUrl} and nothing else. All-Chat merges your {platforms} chat into a single OBS browser source. Every message keeps its platform color, {emotes} render natively, animated — because chat without emotes isn't chat.",
    oneUrlEmphasis: 'one URL',
    platformsEmphasis: 'Twitch, YouTube, TikTok, Kick and Discord',
    emotesEmphasis: '7TV, BTTV and FFZ emotes',
    liveLabel: '● LIVE PREVIEW',
    frameUrl: 'allch.at/overlay/8f3e…',
    // The static demo feed's messages live in the flat marketing.feed group
    // below (three-segment cap, messages.test.ts). Usernames are handles
    // kept in the component.
  },
  // The static demo feed below the manifesto; one message per m-key.
  feed: {
    m1: 'that clip was INSANE GIGACHAD',
    m2: 'first time catching the stream live!',
    m3: 'W streamer W takes POGGERS',
    m4: 'lurking 3 years, this overlay is clean',
    m5: 'came from the fyp, what game is this',
    m6: 'hi from the VOD gang',
    m7: 'CLIP IT OMEGALUL',
    m8: 'peepoHappy the 7tv support!!',
    m9: 'how is chat from 4 apps in one box KEKW',
    m10: 'RAID INCOMING — welcome!',
  },
  wedge: {
    label: 'EVERY MULTICHAT TOOL MAKES YOU PAY SOMETHING',
    headingTop: 'MONEY. DISK SPACE.',
    headingMiddle: 'PATIENCE.',
    headingAccent: 'NOT HERE.',
    themTitle: 'THEM',
    usTitle: 'ALL·CHAT',
    them1: 'download a desktop app first — your chat tool should weigh one URL, not 200 MB',
    them2: 'install a browser extension and hand it your tabs',
    them3: 'a subscription to read your own chat — rent-seeking on infrastructure',
    them4: 'closed source — "trust us, bro"',
    them5: 'a dashboard you read, not what your viewers see',
    us1: 'nothing to install — it is a URL',
    us2: 'free — merged chat is infrastructure, not a premium tier',
    us3: 'AGPL-3.0 — audit it, fork it, self-host it',
    us4: '5 platforms, Discord included',
    us5: 'emotes are the language: 7TV · BTTV · FFZ, native and animated',
  },
  numbers: {
    label: 'THIS WEEK, BY PLATFORM',
    platformLabel: '{platform} · MSGS/WK · {share}%',
    totalLabel: 'MESSAGES DELIVERED',
    usersLabel: 'STREAMERS ON BOARD',
    overlaysLabel: 'OVERLAYS LIVE RIGHT NOW',
  },
  steps: {
    label: 'THE ENTIRE SETUP — NO, REALLY',
    signInTitle: 'Sign in',
    signInBody:
      'Twitch, YouTube or Kick account. No bot to invite, no tokens to paste, nothing to download.',
    addChannelsTitle: 'Add your channels',
    addChannelsBody:
      'Any mix of the five platforms. Five windows is four too many — one overlay carries all of them.',
    pasteUrlTitle: 'Paste one URL',
    pasteUrlBody:
      "That's the install. Drop the link into an OBS browser source; emotes, badges and themes come along.",
  },
  final: {
    line1: 'NOTHING TO INSTALL.',
    line2: 'NOTHING TO PAY.',
    line3: 'EVERYTHING TO READ.',
    cta: 'GET YOUR OVERLAY — FREE →',
    micro: 'free · no download · agpl-3.0, self-hostable',
    welcomeBack: 'welcome back, {name}.',
    welcomeMicro: 'your chat never stopped',
  },
  explore: {
    summary: 'Explore All-Chat',
    extension: 'Browser extension',
    api: 'Developer API',
    docsAndFaq: 'Docs & FAQ',
  },
  ambassadors: {
    eyebrow: 'Ambassadors',
    title: 'Streamers who run on All-Chat',
  },
  footer: {
    tagline: 'Free. Open source. Built for streamers who refuse to pick just one platform.',
    github: 'GitHub',
    discord: 'Discord',
    docs: 'Docs',
    api: 'API',
    privacy: 'Privacy Policy',
    terms: 'Terms of Service',
    impressum: 'Impressum',
  },
  // Rendered twice: by FaqSection, and verbatim into the FAQPage JSON-LD on the
  // home route. Google requires the structured text to match the visible
  // answer exactly, so both read these keys and neither restates them.
  faq: {
    label: 'FAQ',
    heading: 'Frequently asked questions',
    platformsQuestion: 'Which platforms can I combine?',
    platformsAnswer:
      'Twitch, YouTube, Kick, TikTok, and Discord — in any combination, all in a single overlay.',
    obsQuestion: 'How do I add All-Chat to OBS?',
    obsAnswer:
      'Create an overlay, add your chat sources, then paste the overlay URL into an OBS Browser Source. No plugins or bots required.',
    freeQuestion: 'Is All-Chat free?',
    freeAnswer: 'Yes. All-Chat is free and open source under the AGPL-3.0 license.',
    premiumQuestion: 'Why are some features premium?',
    premiumAnswer:
      'Premium covers what costs real money or scarce quota to run, plus a few power-user extras: text-to-speech streams audio to your overlay, which is far more expensive to deliver than regular chat messages; YouTube moderation actions and poll announcements posted to chat consume strictly limited platform quotas; YouTube stream selection is an advanced option very few channels need; shared chat is gated to prevent abuse; and viewer flairs are cosmetic perks for supporters. Premium is funded through Patreon and keeps All-Chat running for everyone.',
    emotesQuestion: 'Which emotes are supported?',
    emotesAnswer:
      '7TV, BTTV, and FFZ, alongside native Twitch and YouTube emotes — they all render correctly in your overlay.',
    customizeQuestion: 'Can I customize how the overlay looks?',
    customizeAnswer:
      'Yes. Choose from 16 built-in themes or write your own CSS for full control over fonts, colors, and layout.',
    privacyQuestion: 'Does All-Chat track my viewers or use cookies?',
    privacyAnswer:
      'No. Usage analytics are cookieless and self-hosted, and chat messages are automatically deleted after about an hour.',
    extensionQuestion: 'Is there a browser extension?',
    extensionAnswer:
      'Yes. The All-Chat browser extension replaces native Twitch, YouTube, and Kick chat so your viewers can follow along across platforms.',
  },
  upgrade: {
    badge: 'All-Chat Premium',
    title: 'Unlock the full power of your overlay',
    body: 'Premium is funded entirely through Patreon — it keeps All-Chat running and unlocks the features that make multistream moderation effortless. Back the project once, and premium applies automatically to your account.',
    subscribe: 'Subscribe on Patreon',
    connectPatreon: 'Already a patron? Connect Patreon',
    moderationTitle: 'Moderate from your overlay',
    moderationBody:
      'Moderate straight from the monitor view — no second dashboard. Delete, timeout, ban and unban on Twitch, Kick and Discord; timeout and ban on YouTube. (TikTok has no moderation API.)',
    moderatorsTitle: 'Let your moderators help',
    moderatorsBody:
      'Hand the monitor view to the moderators you already trust. They act with their own platform accounts, so Twitch, YouTube and Kick check their moderator role on every action — and they never need a plan of their own.',
    ttsTitle: 'ElevenLabs text-to-speech',
    ttsBody:
      'Read chat aloud with high-quality ElevenLabs voices, with full control over priority and pronunciation.',
    streamSelectionTitle: 'YouTube stream selection',
    streamSelectionBody:
      'Pick exactly which YouTube broadcast an overlay listens to instead of relying on auto-detection.',
    sharedChatTitle: 'Shared chat',
    sharedChatBody: 'Combine several channels into one shared conversation across your overlays.',
    flairsTitle: 'Viewer flairs',
    flairsBody:
      'Stand out in any chat you appear in with premium cosmetics like animated name gradients.',
    howItWorks: 'How it works',
    // The linked words stay inside their sentence as a placeholder so a
    // translator can put the link where the target language needs it.
    step1: 'Back All-Chat on {patreon} at the premium tier.',
    step1Patreon: 'Patreon',
    step2: 'Connect your Patreon account on the {settings} page.',
    step2Settings: 'Premium settings',
    step3: 'Premium unlocks automatically — no codes, no waiting.',
    viewerFootnote: 'Just want viewer cosmetics? {link}.',
    viewerFootnoteLink: 'See viewer premium',
  },
  login: {
    failedTitle: 'Login failed',
    failedBody: 'No auth URL returned. Try again.',
    errorTitle: 'Login error',
    errorBody: 'Failed to initiate {platform} login.',
  },
  // The landing page's rotating theme showcase. Not rendered by the lanes
  // homepage (the lanes nav links to /docs#themes instead); kept for reuse.
  themeSwitcher: {
    heading: 'Themes',
    carouselLabel: 'Featured themes',
    dotsGroupLabel: 'Choose a theme',
    pauseLabel: 'Pause theme rotation',
    resumeLabel: 'Resume theme rotation',
    showThemeLabel: 'Show {theme}',
    // The showcase table carried these as module data, so the gate never saw
    // them. They are what the dot buttons announce.
    showcaseMinimal: 'Minimal',
    showcaseComic: 'Comic',
    showcaseStickyNotes: 'Sticky Notes',
    showcaseModernDark: 'Modern Dark',
    // Whole sentences with the links as params: they were five and three JSX
    // runs, which a language reordering the clauses could not move.
    customiseCaption: '{count} built-in themes — restyle any {gui}, or {css}.',
    customiseGuiLink: 'point-and-click',
    customiseCssLink: 'write your own CSS',
    portCaption: "Coming from another tool? {discord} and we'll port your theme.",
    portDiscordLink: 'Ask on Discord',
  },
} as const
