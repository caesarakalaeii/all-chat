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
 * eslint.i18n.config.mjs for the gate that keeps new copy flowing in here,
 * and ADR-0055 for the catalog itself.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('landing page header', () => {
  it('keeps the sticky header labels', () => {
    expect(t('marketing.header.homeLabel')).toBe('All-Chat home')
    expect(t('marketing.header.docs')).toBe('Docs')
    expect(t('marketing.header.dashboard')).toBe('Dashboard')
    expect(t('marketing.header.signIn')).toBe('Sign in')
  })
})

describe('lanes hero, logged out', () => {
  it('keeps the manifesto pitch', () => {
    expect(t('marketing.lanes.kicker')).toBe('A DECLARATION OF WAR ON CHAT TOOLING')
    expect(t('marketing.lanes.titleTop')).toBe('EVERY CHAT.')
    expect(t('marketing.lanes.titleBottom')).toBe('ONE URL.')
    expect(t('marketing.lanes.totalLabel')).toBe('MSG DELIVERED')
  })

  it('keeps the CTA bar', () => {
    expect(t('marketing.lanes.cta')).toBe('GET YOUR OVERLAY — FREE →')
    expect(t('marketing.lanes.ctaNote')).toBe("lane height ≈ share of this week's messages")
    expect(t('marketing.lanes.howItWorks')).toBe('how it works ↓')
  })

  it('keeps one sign-in label per platform', () => {
    // Both the button text and its aria-label, which are the same string, so
    // migrating them cannot drift the accessible name from the visible one.
    expect(t('marketing.lanes.signInWith', { platform: 'Twitch' })).toBe('Sign in with Twitch')
    expect(t('marketing.lanes.signInWith', { platform: 'YouTube' })).toBe('Sign in with YouTube')
    expect(t('marketing.lanes.signInWith', { platform: 'Kick' })).toBe('Sign in with Kick')
  })

  it('keeps every marquee string free of placeholder syntax', () => {
    // The lanes read these as decorative chatter; a stray {param} would leak
    // into the marquee unresolved because the render passes no params.
    const platforms = {
      twitch: 'flowTwitch',
      youtube: 'flowYoutube',
      tiktok: 'flowTiktok',
      kick: 'flowKick',
      discord: 'flowDiscord',
    } as const
    for (const group of Object.values(platforms)) {
      for (let i = 1; i <= 22; i++) {
        const key = `marketing.${group}.m${i}`
        const value = t(key as Parameters<typeof t>[0])
        if (value === key) continue // platform ran out of curated strings
        expect(value).not.toMatch(/[{}]/)
      }
    }
  })
})

describe('lanes hero, logged in', () => {
  it('keeps the returning-user welcome', () => {
    expect(t('marketing.lanes.navDashboardChip')).toBe('▸ dashboard')
    expect(t('marketing.lanes.backToDashboard')).toBe('BACK TO YOUR DASHBOARD →')
    expect(t('marketing.lanes.welcomeNote', { name: 'Ada', count: 3 })).toBe(
      'welcome back, Ada — 3 overlays are live, yours is one click away'
    )
  })

  it('keeps the collapsed explore row', () => {
    expect(t('marketing.explore.summary')).toBe('Explore All-Chat')
    expect(t('marketing.explore.extension')).toBe('Browser extension')
    expect(t('marketing.explore.api')).toBe('Developer API')
    expect(t('marketing.explore.docsAndFaq')).toBe('Docs & FAQ')
  })
})

describe('convergence band', () => {
  it('keeps the manifesto copy', () => {
    expect(t('marketing.convergence.label')).toBe('THE WHOLE PRODUCT')
    expect(t('marketing.convergence.headingTop')).toBe('FIVE CHATS IN.')
    expect(t('marketing.convergence.headingBottom')).toBe('ONE WINDOW OUT.')
    expect(
      t('marketing.convergence.body', {
        oneUrl: 'one URL',
        platforms: 'Twitch, YouTube, TikTok, Kick and Discord',
        emotes: '7TV, BTTV and FFZ emotes',
      })
    ).toBe(
      "Your chat tool should weigh one URL and nothing else. All-Chat merges your Twitch, YouTube, TikTok, Kick and Discord chat into a single OBS browser source. Every message keeps its platform color, 7TV, BTTV and FFZ emotes render natively, animated — because chat without emotes isn't chat."
    )
  })

  it('keeps the preview frame chrome', () => {
    expect(t('marketing.convergence.liveLabel')).toBe('● LIVE PREVIEW')
    expect(t('marketing.convergence.frameUrl')).toBe('allch.at/overlay/8f3e…')
  })
})

describe('wedge band', () => {
  it('keeps the headings', () => {
    expect(t('marketing.wedge.label')).toBe('EVERY MULTICHAT TOOL MAKES YOU PAY SOMETHING')
    expect(t('marketing.wedge.headingTop')).toBe('MONEY. DISK SPACE.')
    expect(t('marketing.wedge.headingMiddle')).toBe('PATIENCE.')
    expect(t('marketing.wedge.headingAccent')).toBe('NOT HERE.')
    expect(t('marketing.wedge.themTitle')).toBe('THEM')
    expect(t('marketing.wedge.usTitle')).toBe('ALL·CHAT')
  })

  it('keeps all five THEM bullets and all five ALL·CHAT bullets', () => {
    for (let i = 1; i <= 5; i++) {
      expect(t(`marketing.wedge.them${i}` as Parameters<typeof t>[0])).not.toBe('')
      expect(t(`marketing.wedge.us${i}` as Parameters<typeof t>[0])).not.toBe('')
    }
    expect(t('marketing.wedge.them1')).toBe(
      'download a desktop app first — your chat tool should weigh one URL, not 200 MB'
    )
    expect(t('marketing.wedge.us1')).toBe('nothing to install — it is a URL')
  })
})

describe('numbers band', () => {
  it('keeps the row labels', () => {
    expect(t('marketing.numbers.label')).toBe('THIS WEEK, BY PLATFORM')
    expect(t('marketing.numbers.platformLabel', { platform: 'TWITCH', share: '49' })).toBe(
      'TWITCH · MSGS/WK · 49%'
    )
    expect(t('marketing.numbers.totalLabel')).toBe('MESSAGES DELIVERED')
    expect(t('marketing.numbers.usersLabel')).toBe('STREAMERS ON BOARD')
    expect(t('marketing.numbers.overlaysLabel')).toBe('OVERLAYS LIVE RIGHT NOW')
  })
})

describe('steps band', () => {
  it('keeps the three steps', () => {
    expect(t('marketing.steps.label')).toBe('THE ENTIRE SETUP — NO, REALLY')
    expect(t('marketing.steps.signInTitle')).toBe('Sign in')
    expect(t('marketing.steps.addChannelsTitle')).toBe('Add your channels')
    expect(t('marketing.steps.pasteUrlTitle')).toBe('Paste one URL')
  })
})

describe('final band', () => {
  it('keeps the closing manifesto', () => {
    expect(t('marketing.final.line1')).toBe('NOTHING TO INSTALL.')
    expect(t('marketing.final.line2')).toBe('NOTHING TO PAY.')
    expect(t('marketing.final.line3')).toBe('EVERYTHING TO READ.')
    expect(t('marketing.final.cta')).toBe('GET YOUR OVERLAY — FREE →')
    expect(t('marketing.final.micro')).toBe('free · no download · agpl-3.0, self-hostable')
  })

  it('keeps the welcome-back swap', () => {
    expect(t('marketing.final.welcomeBack', { name: 'Ada' })).toBe('welcome back, Ada.')
    expect(t('marketing.final.welcomeMicro')).toBe('your chat never stopped')
  })
})

describe('ambassadors band', () => {
  it('keeps the band headings', () => {
    expect(t('marketing.ambassadors.eyebrow')).toBe('Ambassadors')
    expect(t('marketing.ambassadors.title')).toBe('Streamers who run on All-Chat')
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

  it('keeps the first FAQ stem pair', () => {
    // Both the visible FAQ and the FAQPage JSON-LD read these keys; this lock
    // is what keeps the structured data from drifting from the page.
    expect(t('marketing.faq.platformsQuestion')).toBe('Which platforms can I combine?')
    expect(t('marketing.faq.platformsAnswer')).toBe(
      'Twitch, YouTube, Kick, TikTok, and Discord — in any combination, all in a single overlay.'
    )
  })
})

describe('upgrade page', () => {
  it('keeps the hero', () => {
    expect(t('marketing.upgrade.badge')).toBe('All-Chat Premium')
    expect(t('marketing.upgrade.title')).toBe('Unlock the full power of your overlay')
  })

  it('keeps the premium feature list', () => {
    expect(t('marketing.upgrade.moderationTitle')).toBe('Moderate from your overlay')
    expect(t('marketing.upgrade.ttsTitle')).toBe('ElevenLabs text-to-speech')
    expect(t('marketing.upgrade.streamSelectionTitle')).toBe('YouTube stream selection')
    expect(t('marketing.upgrade.sharedChatTitle')).toBe('Shared chat')
    expect(t('marketing.upgrade.flairsTitle')).toBe('Viewer flairs')
  })
})

describe('login failures', () => {
  it('keeps the failure and error notices', () => {
    expect(t('marketing.login.failedTitle')).toBe('Login failed')
    expect(t('marketing.login.failedBody')).toBe('No auth URL returned. Try again.')
    expect(t('marketing.login.errorTitle')).toBe('Login error')
    expect(t('marketing.login.errorBody', { platform: 'Twitch' })).toBe(
      'Failed to initiate Twitch login.'
    )
  })
})

describe('theme switcher copy', () => {
  it('keeps the carousel labels', () => {
    expect(t('marketing.themeSwitcher.heading')).toBe('Themes')
    expect(t('marketing.themeSwitcher.carouselLabel')).toBe('Featured themes')
    expect(t('marketing.themeSwitcher.dotsGroupLabel')).toBe('Choose a theme')
  })

  it('keeps the rotation control labels', () => {
    expect(t('marketing.themeSwitcher.pauseLabel')).toBe('Pause theme rotation')
    expect(t('marketing.themeSwitcher.resumeLabel')).toBe('Resume theme rotation')
    expect(t('marketing.themeSwitcher.showThemeLabel', { theme: 'Minimal' })).toBe('Show Minimal')
  })

  it('keeps the captions with link placeholders', () => {
    expect(
      t('marketing.themeSwitcher.customiseCaption', {
        count: '16',
        gui: 'point-and-click',
        css: 'write your own CSS',
      })
    ).toBe('16 built-in themes — restyle any point-and-click, or write your own CSS.')
    expect(
      t('marketing.themeSwitcher.portCaption', {
        discord: 'Ask on Discord',
      })
    ).toBe("Coming from another tool? Ask on Discord and we'll port your theme.")
  })
})
