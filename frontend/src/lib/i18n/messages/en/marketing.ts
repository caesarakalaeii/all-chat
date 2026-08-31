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
 * The public homepage, the upgrade page and the device-link flow.
 */

export const marketing = {
  header: {
    homeLabel: 'All-Chat home',
    wordmark: 'all-chat',
    docs: 'Docs',
    dashboard: 'Dashboard',
    signIn: 'Sign in',
  },
  hero: {
    eyebrow: 'Free · open source · every platform',
    title: 'One overlay. Every platform.',
    subtitle: 'Every message from Twitch, YouTube, Kick, TikTok and Discord in one OBS overlay.',
    reassurance: 'Free & open source · No bots · Just a URL in OBS',
    // Doubles as each button's aria-label, so one key keeps the accessible
    // name and the visible label from ever drifting apart.
    signInWith: 'Sign in with {platform}',
    statsCaption: 'messages delivered this week',
    welcomeEyebrow: 'Welcome back',
    welcomeTitle: 'Welcome back, {name}.',
    goToDashboard: 'Go to Dashboard',
    adminDashboard: 'Welcome aboard, captain!',
  },
  explore: {
    summary: 'Explore All-Chat',
    extension: 'Browser extension',
    api: 'Developer API',
    docsAndFaq: 'Docs & FAQ',
  },
  why: {
    eyebrow: 'Why All-Chat',
    title: 'Built for multistreamers',
    themesTitle: '16 themes, full control',
    themesBody:
      'From Win98 retro to cyberpunk neon — pick a built-in theme, tweak it point-and-click, or write your own CSS.',
    emotesTitle: 'Every emote, everywhere',
    emotesBody:
      '7TV, BTTV, FFZ plus native Twitch and YouTube emotes — they all render correctly in your overlay.',
    resourcesTitle: 'Smart resource usage',
    resourcesBody:
      'Only polls a platform while your overlay is live in OBS. Switch scenes and All-Chat stands down.',
  },
  steps: {
    heading: 'Live in 3 steps:',
    signIn: 'Sign in',
    addChannels: 'Add your channels',
    pasteUrl: 'Paste the URL in OBS',
  },
  ambassadors: {
    eyebrow: 'Ambassadors',
    title: 'Streamers who run on All-Chat',
  },
  beyond: {
    eyebrow: 'Beyond the overlay',
    title: 'Do more with All-Chat',
    extensionTitle: 'Browser extension',
    extensionBody:
      'Give your viewers unified cross-platform chat right in their browser — it replaces native Twitch, YouTube, and Kick chat.',
    firefox: 'Firefox',
    chrome: 'Chrome',
    githubReleases: 'GitHub Releases',
    apiTitle: 'Build on the API',
    apiBody:
      "One unified chat WebSocket — every platform, one message format. There's a public test stream you can hook up in seconds, no account needed.",
    apiCta: 'Read the API docs',
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
} as const
