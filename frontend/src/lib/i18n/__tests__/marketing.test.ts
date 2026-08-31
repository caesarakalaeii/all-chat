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
 * Copy lock for the public landing page, the FAQ and the upgrade pitch. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 *
 * The FAQ answers are load bearing twice over: FaqSection renders them and
 * app/page.tsx copies them into FAQPage JSON-LD, where Google requires the
 * structured text to match the visible answer verbatim. Both read the catalog,
 * so this lock is what keeps them identical.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('landing page header', () => {
  it('keeps the sticky header labels', () => {
    expect(t('marketing.header.homeLabel')).toBe('All-Chat home')
    expect(t('marketing.header.wordmark')).toBe('all-chat')
    expect(t('marketing.header.docs')).toBe('Docs')
    expect(t('marketing.header.dashboard')).toBe('Dashboard')
    expect(t('marketing.header.signIn')).toBe('Sign in')
  })
})

describe('landing hero, logged out', () => {
  it('keeps the pitch', () => {
    expect(t('marketing.hero.eyebrow')).toBe('Free · open source · every platform')
    expect(t('marketing.hero.title')).toBe('One overlay. Every platform.')
    expect(t('marketing.hero.subtitle')).toBe(
      'Every message from Twitch, YouTube, Kick, TikTok and Discord in one OBS overlay.'
    )
    expect(t('marketing.hero.reassurance')).toBe('Free & open source · No bots · Just a URL in OBS')
  })

  it('keeps one sign-in label per platform', () => {
    // Both the button text and its aria-label, which are the same string, so
    // migrating them cannot drift the accessible name from the visible one.
    expect(t('marketing.hero.signInWith', { platform: 'Twitch' })).toBe('Sign in with Twitch')
    expect(t('marketing.hero.signInWith', { platform: 'YouTube' })).toBe('Sign in with YouTube')
    expect(t('marketing.hero.signInWith', { platform: 'Kick' })).toBe('Sign in with Kick')
  })

  it('keeps the stat strip caption', () => {
    expect(t('marketing.hero.statsCaption')).toBe('messages delivered this week')
  })
})

describe('landing hero, logged in', () => {
  it('keeps the returning-user welcome', () => {
    expect(t('marketing.hero.welcomeEyebrow')).toBe('Welcome back')
    expect(t('marketing.hero.welcomeTitle', { name: 'Ada' })).toBe('Welcome back, Ada.')
    expect(t('marketing.hero.goToDashboard')).toBe('Go to Dashboard')
    expect(t('marketing.hero.adminDashboard')).toBe('Welcome aboard, captain!')
  })

  it('keeps the collapsed explore row', () => {
    expect(t('marketing.explore.summary')).toBe('Explore All-Chat')
    expect(t('marketing.explore.extension')).toBe('Browser extension')
    expect(t('marketing.explore.api')).toBe('Developer API')
    expect(t('marketing.explore.docsAndFaq')).toBe('Docs & FAQ')
  })
})

describe('why All-Chat band', () => {
  it('keeps the band headings', () => {
    expect(t('marketing.why.eyebrow')).toBe('Why All-Chat')
    expect(t('marketing.why.title')).toBe('Built for multistreamers')
  })

  it('keeps the three feature cards', () => {
    expect(t('marketing.why.themesTitle')).toBe('16 themes, full control')
    expect(t('marketing.why.themesBody')).toBe(
      'From Win98 retro to cyberpunk neon — pick a built-in theme, tweak it point-and-click, or write your own CSS.'
    )
    expect(t('marketing.why.emotesTitle')).toBe('Every emote, everywhere')
    expect(t('marketing.why.emotesBody')).toBe(
      '7TV, BTTV, FFZ plus native Twitch and YouTube emotes — they all render correctly in your overlay.'
    )
    expect(t('marketing.why.resourcesTitle')).toBe('Smart resource usage')
    expect(t('marketing.why.resourcesBody')).toBe(
      'Only polls a platform while your overlay is live in OBS. Switch scenes and All-Chat stands down.'
    )
  })

  it('keeps the three-step strip', () => {
    expect(t('marketing.steps.heading')).toBe('Live in 3 steps:')
    expect(t('marketing.steps.signIn')).toBe('Sign in')
    expect(t('marketing.steps.addChannels')).toBe('Add your channels')
    expect(t('marketing.steps.pasteUrl')).toBe('Paste the URL in OBS')
  })
})

describe('ambassadors band', () => {
  it('keeps the band headings', () => {
    expect(t('marketing.ambassadors.eyebrow')).toBe('Ambassadors')
    expect(t('marketing.ambassadors.title')).toBe('Streamers who run on All-Chat')
  })
})

describe('beyond the overlay band', () => {
  it('keeps the band headings', () => {
    expect(t('marketing.beyond.eyebrow')).toBe('Beyond the overlay')
    expect(t('marketing.beyond.title')).toBe('Do more with All-Chat')
  })

  it('keeps the extension card', () => {
    expect(t('marketing.beyond.extensionTitle')).toBe('Browser extension')
    expect(t('marketing.beyond.extensionBody')).toBe(
      'Give your viewers unified cross-platform chat right in their browser — it replaces native Twitch, YouTube, and Kick chat.'
    )
    expect(t('marketing.beyond.firefox')).toBe('Firefox')
    expect(t('marketing.beyond.chrome')).toBe('Chrome')
    expect(t('marketing.beyond.githubReleases')).toBe('GitHub Releases')
  })

  it('keeps the API card', () => {
    expect(t('marketing.beyond.apiTitle')).toBe('Build on the API')
    expect(t('marketing.beyond.apiBody')).toBe(
      "One unified chat WebSocket — every platform, one message format. There's a public test stream you can hook up in seconds, no account needed."
    )
    expect(t('marketing.beyond.apiCta')).toBe('Read the API docs')
  })
})

describe('landing footer', () => {
  it('keeps the footer tagline and links', () => {
    expect(t('marketing.footer.tagline')).toBe(
      'Free. Open source. Built for streamers who refuse to pick just one platform.'
    )
    expect(t('marketing.footer.github')).toBe('GitHub')
    expect(t('marketing.footer.discord')).toBe('Discord')
    expect(t('marketing.footer.docs')).toBe('Docs')
    expect(t('marketing.footer.api')).toBe('API')
    expect(t('marketing.footer.privacy')).toBe('Privacy Policy')
    expect(t('marketing.footer.terms')).toBe('Terms of Service')
    expect(t('marketing.footer.impressum')).toBe('Impressum')
  })
})

describe('landing FAQ', () => {
  it('keeps the section heading', () => {
    expect(t('marketing.faq.heading')).toBe('Frequently asked questions')
  })

  it('keeps every question and answer verbatim', () => {
    expect(t('marketing.faq.platformsQuestion')).toBe('Which platforms can I combine?')
    expect(t('marketing.faq.platformsAnswer')).toBe(
      'Twitch, YouTube, Kick, TikTok, and Discord — in any combination, all in a single overlay.'
    )
    expect(t('marketing.faq.obsQuestion')).toBe('How do I add All-Chat to OBS?')
    expect(t('marketing.faq.obsAnswer')).toBe(
      'Create an overlay, add your chat sources, then paste the overlay URL into an OBS Browser Source. No plugins or bots required.'
    )
    expect(t('marketing.faq.freeQuestion')).toBe('Is All-Chat free?')
    expect(t('marketing.faq.freeAnswer')).toBe(
      'Yes. All-Chat is free and open source under the AGPL-3.0 license.'
    )
    expect(t('marketing.faq.premiumQuestion')).toBe('Why are some features premium?')
    expect(t('marketing.faq.premiumAnswer')).toBe(
      'Premium covers what costs real money or scarce quota to run, plus a few power-user extras: text-to-speech streams audio to your overlay, which is far more expensive to deliver than regular chat messages; YouTube moderation actions and poll announcements posted to chat consume strictly limited platform quotas; YouTube stream selection is an advanced option very few channels need; shared chat is gated to prevent abuse; and viewer flairs are cosmetic perks for supporters. Premium is funded through Patreon and keeps All-Chat running for everyone.'
    )
    expect(t('marketing.faq.emotesQuestion')).toBe('Which emotes are supported?')
    expect(t('marketing.faq.emotesAnswer')).toBe(
      '7TV, BTTV, and FFZ, alongside native Twitch and YouTube emotes — they all render correctly in your overlay.'
    )
    expect(t('marketing.faq.customizeQuestion')).toBe('Can I customize how the overlay looks?')
    expect(t('marketing.faq.customizeAnswer')).toBe(
      'Yes. Choose from 16 built-in themes or write your own CSS for full control over fonts, colors, and layout.'
    )
    expect(t('marketing.faq.privacyQuestion')).toBe(
      'Does All-Chat track my viewers or use cookies?'
    )
    expect(t('marketing.faq.privacyAnswer')).toBe(
      'No. Usage analytics are cookieless and self-hosted, and chat messages are automatically deleted after about an hour.'
    )
    expect(t('marketing.faq.extensionQuestion')).toBe('Is there a browser extension?')
    expect(t('marketing.faq.extensionAnswer')).toBe(
      'Yes. The All-Chat browser extension replaces native Twitch, YouTube, and Kick chat so your viewers can follow along across platforms.'
    )
  })
})

describe('upgrade page', () => {
  it('keeps the hero', () => {
    expect(t('marketing.upgrade.badge')).toBe('All-Chat Premium')
    expect(t('marketing.upgrade.title')).toBe('Unlock the full power of your overlay')
    expect(t('marketing.upgrade.body')).toBe(
      'Premium is funded entirely through Patreon — it keeps All-Chat running and unlocks the features that make multistream moderation effortless. Back the project once, and premium applies automatically to your account.'
    )
    expect(t('marketing.upgrade.subscribe')).toBe('Subscribe on Patreon')
    expect(t('marketing.upgrade.connectPatreon')).toBe('Already a patron? Connect Patreon')
  })

  it('keeps the six premium feature entries', () => {
    expect(t('marketing.upgrade.moderationTitle')).toBe('Moderate from your overlay')
    expect(t('marketing.upgrade.moderationBody')).toBe(
      'Moderate straight from the monitor view — no second dashboard. Delete, timeout, ban and unban on Twitch, Kick and Discord; timeout and ban on YouTube. (TikTok has no moderation API.)'
    )
    expect(t('marketing.upgrade.moderatorsTitle')).toBe('Let your moderators help')
    expect(t('marketing.upgrade.moderatorsBody')).toBe(
      'Hand the monitor view to the moderators you already trust. They act with their own platform accounts, so Twitch, YouTube and Kick check their moderator role on every action — and they never need a plan of their own.'
    )
    expect(t('marketing.upgrade.ttsTitle')).toBe('ElevenLabs text-to-speech')
    expect(t('marketing.upgrade.ttsBody')).toBe(
      'Read chat aloud with high-quality ElevenLabs voices, with full control over priority and pronunciation.'
    )
    expect(t('marketing.upgrade.streamSelectionTitle')).toBe('YouTube stream selection')
    expect(t('marketing.upgrade.streamSelectionBody')).toBe(
      'Pick exactly which YouTube broadcast an overlay listens to instead of relying on auto-detection.'
    )
    expect(t('marketing.upgrade.sharedChatTitle')).toBe('Shared chat')
    expect(t('marketing.upgrade.sharedChatBody')).toBe(
      'Combine several channels into one shared conversation across your overlays.'
    )
    expect(t('marketing.upgrade.flairsTitle')).toBe('Viewer flairs')
    expect(t('marketing.upgrade.flairsBody')).toBe(
      'Stand out in any chat you appear in with premium cosmetics like animated name gradients.'
    )
  })

  it('keeps the three how-it-works steps whole', () => {
    expect(t('marketing.upgrade.howItWorks')).toBe('How it works')
    // Each step keeps its linked words inside the sentence as a placeholder, so
    // a translator can move the link anywhere the target language needs it.
    expect(t('marketing.upgrade.step1')).toBe('Back All-Chat on {patreon} at the premium tier.')
    expect(t('marketing.upgrade.step1Patreon')).toBe('Patreon')
    expect(t('marketing.upgrade.step2')).toBe(
      'Connect your Patreon account on the {settings} page.'
    )
    expect(t('marketing.upgrade.step2Settings')).toBe('Premium settings')
    expect(t('marketing.upgrade.step3')).toBe(
      'Premium unlocks automatically — no codes, no waiting.'
    )
  })

  it('keeps the viewer premium footnote whole', () => {
    // The trailing full stop sits outside the link in the markup, so the
    // sentence keeps it and interpolateElements wraps only the linked run.
    expect(t('marketing.upgrade.viewerFootnote')).toBe('Just want viewer cosmetics? {link}.')
    expect(t('marketing.upgrade.viewerFootnoteLink')).toBe('See viewer premium')
  })
})

describe('login failures', () => {
  it('collapses the three byte-identical login failures to one key', () => {
    // handleTwitchLogin / handleYouTubeLogin / handleKickLogin raised the same
    // two strings each. One key, so a reword cannot land on one platform only.
    expect(t('marketing.login.failedTitle')).toBe('Login failed')
    expect(t('marketing.login.failedBody')).toBe('No auth URL returned. Try again.')
  })

  it('parameterises the per-platform login error', () => {
    expect(t('marketing.login.errorTitle')).toBe('Login error')
    expect(t('marketing.login.errorBody', { platform: 'Twitch' })).toBe(
      'Failed to initiate Twitch login.'
    )
    expect(t('marketing.login.errorBody', { platform: 'YouTube' })).toBe(
      'Failed to initiate YouTube login.'
    )
    expect(t('marketing.login.errorBody', { platform: 'Kick' })).toBe(
      'Failed to initiate Kick login.'
    )
  })
})
