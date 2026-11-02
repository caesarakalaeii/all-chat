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
 * The viewer-facing /browser-extension page, in the homepage's lanes art
 * direction: the platform lanes as the hero, a mock stream page showing the
 * merged chat panel, the viewer-vs-streamer wedge, a sourced comparison with
 * other chat tools, and a closing install band.
 *
 * The comparison cells state only what each tool's own pages say (checked
 * 2026-11-02, sources linked in the table). Re-check them before changing
 * the copy; the claim is "nothing else does this for viewers", not "nothing
 * else merges chat".
 */

'use client'

import { useEffect, useRef, useState, useSyncExternalStore } from 'react'
import Link from 'next/link'
import { renderFeedMessage } from '@/components/home/ConvergenceSection'
import { LaneText, MARQUEE_ROWS } from '@/components/home/LanesHero'
import { LanguageToggle } from '@/components/home/LanguageToggle'
import { useReducedMotion } from '@/hooks/useReducedMotion'
import { useReveal } from '@/hooks/useReveal'
import { trackEvent } from '@/lib/analytics'
import { DISCORD_INVITE_URL, EXTENSION_CHROME_URL, EXTENSION_FIREFOX_URL } from '@/lib/constants'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { cn } from '@/lib/utils'

const EXTENSION_REPO_URL = 'https://github.com/caesarakalaeii/all-chat-extension'

// The demo panel's chatter: platform, mockup handle (fixtures, not catalog
// copy) and a marketing.feed message stem. Platforms alternate so every
// screenful mixes at least four of them.
const DEMO_FEED = [
  { platform: 'twitch', user: 'nightowl_tv', stem: 'm1' },
  { platform: 'youtube', user: 'PixelPioneer', stem: 'm2' },
  { platform: 'kick', user: 'greenmachine', stem: 'm3' },
  { platform: 'tiktok', user: 'scroll.and.chill', stem: 'm5' },
  { platform: 'twitch', user: 'lurkerlou', stem: 'm4' },
  { platform: 'discord', user: 'raidboss_rin', stem: 'm10' },
  { platform: 'youtube', user: 'VODEnjoyer', stem: 'm6' },
  { platform: 'kick', user: 'clipit_quick', stem: 'm7' },
  { platform: 'tiktok', user: 'fyp_famous', stem: 'm9' },
  { platform: 'twitch', user: 'emotecollector', stem: 'm8' },
] as const

const DEMO_INITIAL = 6
const DEMO_MAX = 8

// One row per tool; `us` is the All-Chat row. Sources are each tool's own
// pages, the same standard as /compare.
const COMPARE_ROWS = [
  {
    stem: 'Ssn',
    sources: [{ url: 'https://socialstream.ninja/', label: 'socialstream.ninja' }],
  },
  {
    stem: 'Restream',
    sources: [
      {
        url: 'https://support.restream.io/en/articles/2379624-what-is-restream-chat',
        label: 'support.restream.io',
      },
      {
        url: 'https://sleepychat.org/guides/obs-multi-stream-chat-browser-dock-overlay/',
        label: 'sleepychat.org',
      },
    ],
  },
  {
    stem: 'Shared',
    sources: [
      {
        url: 'https://blog.twitch.tv/en/2024/09/24/bring-your-communities-together-with-shared-chat',
        label: 'blog.twitch.tv',
      },
    ],
  },
  {
    stem: 'Emotes',
    sources: [
      { url: 'https://7tv.app/', label: '7tv.app' },
      { url: 'https://www.frankerfacez.com/', label: 'frankerfacez.com' },
    ],
  },
  {
    stem: 'Chatterino',
    sources: [{ url: 'https://github.com/Chatterino/chatterino2', label: 'github.com/Chatterino' }],
  },
  {
    stem: 'Us',
    sources: [{ url: EXTENSION_REPO_URL, label: 'github.com/caesarakalaeii' }],
  },
] as const

const FEATURES = [
  { num: '01', stem: 'Merged' },
  { num: '02', stem: 'Send' },
  { num: '03', stem: 'Emotes' },
  { num: '04', stem: 'Engage' },
  { num: '05', stem: 'Native' },
  { num: '06', stem: 'Open' },
] as const

const THEM_STEMS = ['them1', 'them2', 'them3', 'them4'] as const
const US_STEMS = ['us1', 'us2', 'us3', 'us4'] as const

// The browser does not change under a mounted page; the store exists for its
// server snapshot (Chrome first, then the client corrects it on hydration).
const subscribeNever = () => () => {}

export function BrowserExtensionClient() {
  const t = useTranslations()
  const pageRef = useRef<HTMLDivElement>(null)
  useReveal(pageRef)
  const reducedMotion = useReducedMotion()
  const isFirefox = useSyncExternalStore(
    subscribeNever,
    () => /firefox/i.test(navigator.userAgent),
    () => false
  )

  // Same loop as the homepage demo feed: a message every 0.9-2.7s, last 8 kept.
  const [cursor, setCursor] = useState(DEMO_INITIAL)
  useEffect(() => {
    if (reducedMotion) return
    let timer: number | undefined
    const loop = () => {
      setCursor((count) => count + 1)
      timer = window.setTimeout(loop, 900 + Math.random() * 1800)
    }
    timer = window.setTimeout(loop, 900 + Math.random() * 1800)
    return () => window.clearTimeout(timer)
  }, [reducedMotion])
  const visibleCount = Math.min(cursor, DEMO_MAX)
  const firstKey = cursor - visibleCount

  // The visitor's own browser gets the big button; the other store stays one
  // link away.
  const stores = [
    { dest: 'ext_chrome', url: EXTENSION_CHROME_URL, cta: 'ctaChrome', alt: 'altChrome' },
    { dest: 'ext_firefox', url: EXTENSION_FIREFOX_URL, cta: 'ctaFirefox', alt: 'altFirefox' },
  ] as const
  const [primary, secondary] = isFirefox ? [stores[1], stores[0]] : stores

  const storeLink = (
    store: (typeof stores)[number],
    location: string,
    className: string,
    label: string
  ) => (
    <a
      href={store.url}
      target="_blank"
      rel="noopener noreferrer"
      className={className}
      onClick={() => trackEvent('outbound_click', { dest: store.dest, location })}
    >
      {label}
    </a>
  )

  return (
    <div ref={pageRef} className={cn('lanes-home', archivoBlack.variable, spaceMono.variable)}>
      <div className="lanes-chrome lanes-tl">
        <Link href="/">{t('marketing.lanes.brand')}</Link>
      </div>
      <nav className="lanes-chrome lanes-tr" aria-label={t('marketing.lanes.navLabel')}>
        <Link className="nav-themes" href="/">
          {t('marketing.extensionPage.navStreamers')}
        </Link>
        <Link href="/docs">{t('marketing.lanes.navDocs')}</Link>
        <a href={DISCORD_INVITE_URL}>{t('marketing.lanes.navDiscord')}</a>
        <LanguageToggle />
      </nav>

      <main id="main-content" tabIndex={-1} className="min-h-screen">
        <header className="lanes-stage ext-stage">
          <div className="lanes-track">
            {MARQUEE_ROWS.map(({ platform, rows }) => (
              <div key={platform} className="lane" data-p={platform}>
                <LaneText platform={platform} rows={rows} />
              </div>
            ))}
          </div>

          <div className="lanes-headline">
            <div className="warkicker">{t('marketing.extensionPage.kicker')}</div>
            <h1>
              {t('marketing.extensionPage.titleTop')}
              <br />
              {t('marketing.extensionPage.titleBottom')}{' '}
              <span className="lanes-category">{t('marketing.extensionPage.titleCategory')}</span>
            </h1>
            <p className="lanes-manifesto">
              {t('marketing.extensionPage.manifestoLine1')}
              <br />
              {t('marketing.extensionPage.manifestoLine2')}
            </p>
          </div>

          <div className="ctabar">
            <span className="note">{t('marketing.extensionPage.ctaNote')}</span>
            {storeLink(primary, 'hero', 'cta-go', t(`marketing.extensionPage.${primary.cta}`))}
            {storeLink(secondary, 'hero', '', t(`marketing.extensionPage.${secondary.alt}`))}
          </div>
        </header>

        <section className="lanes-section ext-demo" data-reveal>
          <span className="mono-label">{t('marketing.extensionPage.demoLabel')}</span>
          <h2>
            {t('marketing.extensionPage.demoHeadingTop')}
            <br />
            <span className="accent">{t('marketing.extensionPage.demoHeadingAccent')}</span>
          </h2>
          <p>{t('marketing.extensionPage.demoBody')}</p>

          <div
            className="ext-window panel-fill"
            role="img"
            aria-label={t('marketing.extensionPage.demoAlt')}
          >
            <div className="ext-window-bar">
              <span className="ext-dots">
                <i />
                <i />
                <i />
              </span>
              <span className="ext-url">{t('marketing.extensionPage.demoUrl')}</span>
            </div>
            <div className="ext-window-body">
              <div className="ext-video">
                <span className="ext-live">{t('marketing.extensionPage.demoLive')}</span>
                <span className="ext-video-title">
                  {t('marketing.extensionPage.demoStreamTitle')}
                </span>
              </div>
              <div className="ext-panel">
                <div className="ext-tabs">
                  <span className="on">{t('marketing.extensionPage.demoTabAllChat')}</span>
                  <span>{t('marketing.extensionPage.demoTabNative')}</span>
                </div>
                <div className="chat-feed">
                  {Array.from({ length: visibleCount }, (_, i) => {
                    const entry = DEMO_FEED[(firstKey + i) % DEMO_FEED.length]
                    return (
                      <div
                        key={firstKey + i}
                        className="msg"
                        style={{ '--pc': `var(--lanes-${entry.platform})` } as React.CSSProperties}
                      >
                        <span className="ext-src">{t(`common.platforms.${entry.platform}`)}</span>
                        <span className="who">{entry.user}</span>
                        <span className="txt">
                          {renderFeedMessage(t(`marketing.feed.${entry.stem}`))}
                        </span>
                      </div>
                    )
                  })}
                </div>
                <div className="ext-input">{t('marketing.extensionPage.demoInput')}</div>
              </div>
            </div>
          </div>
        </section>

        <section className="lanes-section wedge" data-reveal>
          <span className="mono-label">{t('marketing.extensionPage.wedgeLabel')}</span>
          <h2>
            {t('marketing.extensionPage.wedgeHeadingTop')}
            <br />
            {t('marketing.extensionPage.wedgeHeadingMiddle')}{' '}
            <span className="accent">{t('marketing.extensionPage.wedgeHeadingAccent')}</span>
          </h2>
          <div className="wedge-grid">
            <div className="wcol bad">
              <h3>{t('marketing.extensionPage.themTitle')}</h3>
              <ul>
                {THEM_STEMS.map((stem) => (
                  <li key={stem}>{t(`marketing.extensionPage.${stem}`)}</li>
                ))}
              </ul>
            </div>
            <div className="wcol good panel-fill">
              <h3>{t('marketing.extensionPage.usTitle')}</h3>
              <ul>
                {US_STEMS.map((stem) => (
                  <li key={stem}>{t(`marketing.extensionPage.${stem}`)}</li>
                ))}
              </ul>
            </div>
          </div>
        </section>

        <section className="lanes-section ext-compare" data-reveal>
          <span className="mono-label">{t('marketing.extensionPage.compareLabel')}</span>
          <h2>
            {t('marketing.extensionPage.compareHeadingTop')}
            <br />
            <span className="accent">{t('marketing.extensionPage.compareHeadingAccent')}</span>
          </h2>
          <table>
            <thead>
              <tr>
                <th scope="col">{t('marketing.extensionPage.compareToolHeader')}</th>
                <th scope="col">{t('marketing.extensionPage.compareViewerHeader')}</th>
                <th scope="col">{t('marketing.extensionPage.compareSourceHeader')}</th>
              </tr>
            </thead>
            <tbody>
              {COMPARE_ROWS.map((row) => (
                <tr key={row.stem} className={row.stem === 'Us' ? 'us' : undefined}>
                  <th scope="row">{t(`marketing.extensionPage.row${row.stem}Tool`)}</th>
                  <td>{t(`marketing.extensionPage.row${row.stem}Viewer`)}</td>
                  <td>
                    <div className="ext-sources">
                      {row.sources.map((source) => (
                        <a
                          key={source.url}
                          href={source.url}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          {source.label}
                        </a>
                      ))}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="ext-footnote">
            {interpolateElements(t('marketing.extensionPage.compareFootnote'), {
              discord: <a href={DISCORD_INVITE_URL}>{t('marketing.lanes.navDiscord')}</a>,
              compare: <Link href="/compare">{t('marketing.extensionPage.compareLinkText')}</Link>,
            })}
          </p>
        </section>

        <section className="lanes-section" data-reveal>
          <span className="mono-label">{t('marketing.extensionPage.featuresLabel')}</span>
          <div className="steps ext-features">
            {FEATURES.map((feature) => (
              <div key={feature.num} className="step">
                <div className="stepnum">{feature.num}</div>
                <h3>{t(`marketing.extensionPage.feature${feature.stem}Title`)}</h3>
                <p>{t(`marketing.extensionPage.feature${feature.stem}Body`)}</p>
              </div>
            ))}
          </div>
        </section>

        <div className="final ext-final" data-reveal>
          <h2>
            {t('marketing.extensionPage.titleTop')}
            <br />
            <span className="accent">{t('marketing.extensionPage.titleBottom')}</span>
          </h2>
          <div className="final-signins">
            {storeLink(primary, 'final', 'cta-go', t(`marketing.extensionPage.${primary.cta}`))}
            {storeLink(
              secondary,
              'final',
              'lanes-signin',
              t(`marketing.extensionPage.${secondary.cta}`)
            )}
          </div>
          <p className="micro">{t('marketing.extensionPage.finalMicro')}</p>
          <p className="ext-where">{t('marketing.extensionPage.finalWhere')}</p>
          <p className="ext-where">
            {interpolateElements(t('marketing.extensionPage.streamerBody'), {
              dashboard: (
                <Link href="/dashboard">{t('marketing.extensionPage.streamerLinkText')}</Link>
              ),
            })}
          </p>
        </div>
      </main>
    </div>
  )
}
