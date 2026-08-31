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

import Link from 'next/link'
import type { ReactNode } from 'react'
import { AppNav } from '@/components/AppNav'
import { Code, Pre } from '@/components/docs/prose'
import { JsonLd } from '@/components/JsonLd'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { getTranslations, type MessageKey } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('metadata.docs.title'),
  description: t('metadata.docs.description'),
  alternates: { canonical: '/docs' },
}

// Structured data: the "Get your overlay live" section is a genuine step-by-step
// procedure, so it maps to schema.org HowTo (rich-result eligible). Breadcrumbs
// give Google the Home > Documentation trail. Emitted from this server page, so
// both land in the initial HTML for crawlers.
const howToLd = {
  '@context': 'https://schema.org',
  '@type': 'HowTo',
  name: 'Get your All-Chat overlay live in OBS',
  description:
    'Set up a multi-platform chat overlay in OBS with All-Chat, combining Twitch, YouTube, Kick, TikTok and Discord chat into one browser source.',
  step: [
    {
      '@type': 'HowToStep',
      position: 1,
      name: 'Sign in',
      text: 'Sign in at allch.at with your Twitch, YouTube, or Kick account.',
    },
    {
      '@type': 'HowToStep',
      position: 2,
      name: 'Create an overlay and connect your platforms',
      text: 'In the dashboard, create an overlay and connect the platforms you stream on. Each connected platform becomes a chat source on that overlay.',
    },
    {
      '@type': 'HowToStep',
      position: 3,
      name: 'Add the overlay to OBS',
      text: 'Copy the overlay browser-source URL and add it to OBS as a Browser Source. Chat appears the moment the overlay connects.',
    },
  ],
}

const breadcrumbLd = {
  '@context': 'https://schema.org',
  '@type': 'BreadcrumbList',
  itemListElement: [
    { '@type': 'ListItem', position: 1, name: 'Home', item: 'https://allch.at' },
    { '@type': 'ListItem', position: 2, name: 'Documentation', item: 'https://allch.at/docs' },
  ],
}

/** A "variable / default / effect" row table for the CSS custom properties. */
interface CssVar {
  /** The custom property. A CSS identifier, so not copy. */
  name: string
  /** Its default value. Also CSS, also not copy. */
  default: string
  /** What it changes, which is the only translatable part of a row. */
  effectKey: MessageKey
}
function CssVarTable({ rows }: { rows: readonly CssVar[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssVarsColumnVariable')}</th>
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssVarsColumnDefault')}</th>
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssVarsColumnEffect')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.name}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.default}</td>
              <td className="px-4 py-2 text-text-sub">{t(r.effectKey)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** A "selector / kind / targets" hook table. */
interface Hook {
  /** The CSS selector. Not copy. */
  selector: string
  /** 'class' or 'attribute'. Describes the selector, so not copy either. */
  kind: string
  /** What it targets, which is the only translatable part of a row. */
  targetsKey: MessageKey
}
function HookTable({ rows }: { rows: readonly Hook[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssHooksColumnSelector')}</th>
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssHooksColumnKind')}</th>
            <th className="px-4 py-2 font-semibold">{t('docs.guide.cssHooksColumnTargets')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.selector} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.selector}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.kind}</td>
              <td className="px-4 py-2 text-text-sub">{t(r.targetsKey)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function DevCallout({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-surface-2 p-5 text-sm text-text-sub">
      {children}
    </div>
  )
}

const CSS_VARIABLES: readonly CssVar[] = [
  { name: '--chat-font-size', default: '1rem', effectKey: 'docs.guideCssVars.chatFontSize' },
  { name: '--chat-font-family', default: 'inherit', effectKey: 'docs.guideCssVars.chatFontFamily' },
  {
    name: '--chat-message-color',
    default: '#ffffff',
    effectKey: 'docs.guideCssVars.chatMessageColor',
  },
  { name: '--chat-message-gap', default: '0.75rem', effectKey: 'docs.guideCssVars.chatMessageGap' },
  {
    name: '--chat-bubble-border-radius',
    default: '0.5rem',
    effectKey: 'docs.guideCssVars.chatBubbleBorderRadius',
  },
  {
    name: '--chat-bubble-padding',
    default: '0.75rem',
    effectKey: 'docs.guideCssVars.chatBubblePadding',
  },
  { name: '--chat-avatar-size', default: '2.5rem', effectKey: 'docs.guideCssVars.chatAvatarSize' },
  {
    name: '--chat-username-font-size',
    default: '0.875rem',
    effectKey: 'docs.guideCssVars.chatUsernameFontSize',
  },
  { name: '--chat-emote-scale', default: '1', effectKey: 'docs.guideCssVars.chatEmoteScale' },
  { name: '--chat-show-avatars', default: 'block', effectKey: 'docs.guideCssVars.chatShowAvatars' },
  { name: '--chat-show-badges', default: 'flex', effectKey: 'docs.guideCssVars.chatShowBadges' },
  {
    name: '--chat-show-timestamps',
    default: 'block',
    effectKey: 'docs.guideCssVars.chatShowTimestamps',
  },
]

const CSS_HOOKS: readonly Hook[] = [
  {
    selector: '.overlay-live-body',
    kind: 'class',
    targetsKey: 'docs.guideCssHooks.overlayLiveBody',
  },
  { selector: '.chat-message', kind: 'class', targetsKey: 'docs.guideCssHooks.chatMessage' },
  { selector: '.chat-username', kind: 'class', targetsKey: 'docs.guideCssHooks.chatUsername' },
  { selector: '.platform-badge', kind: 'class', targetsKey: 'docs.guideCssHooks.platformBadge' },
  { selector: '.event-message', kind: 'class', targetsKey: 'docs.guideCssHooks.eventMessage' },
  {
    selector: '[data-platform="twitch"]',
    kind: 'attribute',
    targetsKey: 'docs.guideCssHooks.dataPlatformTwitch',
  },
  {
    selector: '[data-username="…"]',
    kind: 'attribute',
    targetsKey: 'docs.guideCssHooks.dataUsername',
  },
  {
    selector: '[data-feed-anchor="top|bottom"]',
    kind: 'attribute',
    targetsKey: 'docs.guideCssHooks.dataFeedAnchorTopBottom',
  },
  {
    selector: '[data-feed-order="newest-last|newest-first"]',
    kind: 'attribute',
    targetsKey: 'docs.guideCssHooks.dataFeedOrderNewestLastNewestFirst',
  },
]

// The anchor is a URL fragment people bookmark and link to, so it stays a
// literal; only the label is copy.
const toc = [
  { id: 'what-is-all-chat', labelKey: 'docs.guide.tocWhatIsAllChat' },
  { id: 'getting-started', labelKey: 'docs.guide.tocGettingStarted' },
  { id: '24-7-irl', labelKey: 'docs.guide.tocIrl' },
  { id: 'monitor', labelKey: 'docs.guide.tocMonitor' },
  { id: 'moderation', labelKey: 'docs.guide.tocModeration' },
  { id: 'engagement', labelKey: 'docs.guide.tocEngagement' },
  { id: 'events-credits', labelKey: 'docs.guide.tocEventsCredits' },
  { id: 'sharing', labelKey: 'docs.guide.tocSharing' },
  { id: 'themes', labelKey: 'docs.guide.tocThemes' },
  { id: 'customize', labelKey: 'docs.guide.tocCustomize' },
  { id: 'custom-css', labelKey: 'docs.guide.tocCustomCss' },
  { id: 'fonts', labelKey: 'docs.guide.tocFonts' },
  { id: 'premium', labelKey: 'docs.guide.tocPremium' },
] as const satisfies readonly { id: string; labelKey: MessageKey }[]

// Values the overlay's CSS and chat parser understand. Not copy: a translated
// selector matches nothing and a translated command is a command nobody typed.
const PASSIVE_QUERY_PARAM = '?passive=true'
const ROOT_SELECTOR = ':root'
const IMPORT_AT_RULE = '@import'
const VOTE_COMMAND = '!vote 2'
const PREDICT_COMMAND = '!predict 1 500'

// The Discord link's text is the platform name, which common.platforms already
// holds for all five.
const DISCORD_LINK_KEY = 'common.platforms.discord'

// The code samples. Not copy: a translated CSS property or URL produces a
// sample that does not work when pasted. Hoisted out of the JSX so the i18n
// gate, which cannot tell a <Pre> child from a paragraph, does not have to.
const PASSIVE_URL_EXAMPLE = `https://allch.at/overlay/<overlay-id>?passive=true`

const CSS_VARIABLES_EXAMPLE = `:root {
  --chat-font-size: 20px;
  --chat-message-gap: 12px;
  --chat-show-timestamps: none;   /* hide timestamps */
}`

const FEED_ANCHOR_CSS_EXAMPLE = `/* Fade the far end of the feed, whichever end that is */
[data-feed-anchor="top"]    .overlay-live-body > div:last-child  { opacity: 0.6; }
[data-feed-anchor="bottom"] .overlay-live-body > div:first-child { opacity: 0.6; }

/* Custom entry animation that flips with Invert Message Order */
@keyframes my-entry {
  from { opacity: 0; transform: translateY(calc(40px * var(--msg-enter-dir, 1))); }
  to   { opacity: 1; transform: none; }
}`

const PLATFORM_STRIPE_CSS_EXAMPLE = `.chat-message[data-platform="twitch"]  { border-left: 4px solid #9146FF; }
.chat-message[data-platform="youtube"] { border-left: 4px solid #FF0000; }
.chat-message[data-platform="kick"]    { border-left: 4px solid #53FC18; }

/* Make raid alerts pop */
.event-message[class*="raid"] { transform: scale(1.1); }`

const FONT_IMPORT_EXAMPLE = `@import url('https://fonts.googleapis.com/css2?family=Press+Start+2P&display=swap');

:root { --chat-font-family: 'Press Start 2P', monospace; }`

// Each perk is one whole sentence opening with an emphasised label. The five
// share a shape, so they are a table rather than five near-identical <li>s.
const PREMIUM_PERKS = [
  {
    sentenceKey: 'docs.guide.premiumModeration',
    labelKey: 'docs.guide.premiumModerationEmphasis',
  },
  { sentenceKey: 'docs.guide.premiumTts', labelKey: 'docs.guide.premiumTtsEmphasis' },
  { sentenceKey: 'docs.guide.premiumYoutube', labelKey: 'docs.guide.premiumYoutubeEmphasis' },
  {
    sentenceKey: 'docs.guide.premiumSharedChat',
    labelKey: 'docs.guide.premiumSharedChatEmphasis',
  },
  { sentenceKey: 'docs.guide.premiumFlairs', labelKey: 'docs.guide.premiumFlairsEmphasis' },
] as const satisfies readonly { sentenceKey: MessageKey; labelKey: MessageKey }[]

export default function DocsPage() {
  return (
    <div className="min-h-screen bg-bg transition-colors duration-300">
      <JsonLd data={howToLd} />
      <JsonLd data={breadcrumbLd} />
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-4xl px-4 py-12">
        <div className="rounded-xl border border-border bg-surface p-8 transition-colors duration-300 md:p-12">
          <div className="mb-8 space-y-2">
            <p className="text-xs font-semibold tracking-[0.2em] text-twitch uppercase">
              {t('docs.guide.eyebrow')}
            </p>
            <h1 className="text-3xl font-bold text-text">{t('docs.guide.heading')}</h1>
            <p className="text-sm text-text-dim">{t('docs.guide.intro')}</p>
            <p className="text-sm text-text-sub">
              {interpolateElements(t('docs.guide.apiPrompt'), {
                api: (
                  <Link href="/docs/api" className="text-twitch underline underline-offset-2">
                    {t('docs.guide.apiLinkText')}
                  </Link>
                ),
              })}
            </p>
          </div>

          {/* Table of contents */}
          <nav className="mb-10 rounded-lg border border-border bg-surface-2 p-5">
            <p className="mb-2 text-xs font-semibold tracking-[0.15em] text-text-dim uppercase">
              {t('docs.guide.tocHeading')}
            </p>
            <ul className="grid gap-1 sm:grid-cols-2">
              {toc.map(({ id, labelKey }) => (
                <li key={id}>
                  <a href={`#${id}`} className="text-sm text-twitch hover:underline">
                    {t(labelKey)}
                  </a>
                </li>
              ))}
            </ul>
          </nav>

          <div className="legal-prose space-y-10 leading-relaxed text-text-sub">
            {/* What is All-Chat */}
            <section id="what-is-all-chat">
              <h2>{t('docs.guide.whatIsHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.whatIsBody'), {
                  twitch: <strong>{t('common.platforms.twitch')}</strong>,
                  youtube: <strong>{t('common.platforms.youtube')}</strong>,
                  kick: <strong>{t('common.platforms.kick')}</strong>,
                  tiktok: <strong>{t('common.platforms.tiktok')}</strong>,
                  discord: <strong>{t('common.platforms.discord')}</strong>,
                })}
              </p>
            </section>

            {/* Getting started */}
            <section id="getting-started">
              <h2>{t('docs.guide.startHeading')}</h2>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>
                  {interpolateElements(t('docs.guide.startSignIn'), {
                    home: <Link href="/">{t('docs.guide.startSignInLinkText')}</Link>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.startCreate'), {
                    overlay: <strong>{t('docs.guide.startCreateOverlayEmphasis')}</strong>,
                    source: <em>{t('docs.guide.startCreateSourceEmphasis')}</em>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.startObs'), {
                    browserSource: <strong>{t('docs.guide.startObsEmphasis')}</strong>,
                  })}
                </li>
                <li>{t('docs.guide.startDemandDriven')}</li>
              </ol>
            </section>

            {/* 24/7 & IRL streams */}
            <section id="24-7-irl">
              <h2>{t('docs.guide.irlHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.irlIntro'), {
                  passiveParam: <Code>{PASSIVE_QUERY_PARAM}</Code>,
                })}
              </p>
              <Pre>{PASSIVE_URL_EXAMPLE}</Pre>
              <p>
                {interpolateElements(t('docs.guide.irlExplainer'), {
                  passive: <strong>{t('docs.guide.irlExplainerEmphasis')}</strong>,
                })}
              </p>
              <h3>{t('docs.guide.irlWhenLiveHeading')}</h3>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>{t('docs.guide.irlStepPassiveUrl')}</li>
                <li>
                  {interpolateElements(t('docs.guide.irlStepOpenMonitor'), {
                    monitor: <strong>{t('docs.guide.irlStepOpenMonitorEmphasis')}</strong>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.irlStepRediscover'), {
                    rediscover: <strong>{t('docs.guide.irlStepRediscoverEmphasis')}</strong>,
                  })}
                </li>
                <li>{t('docs.guide.irlStepKeepOpen')}</li>
              </ol>
              <p>
                {interpolateElements(t('docs.guide.irlRefreshNote'), {
                  negation: <em>{t('docs.guide.irlRefreshNoteNegationEmphasis')}</em>,
                  rediscover: <strong>{t('docs.guide.irlStepRediscoverEmphasis')}</strong>,
                  paused: <strong>{t('docs.guide.irlRefreshNotePausedEmphasis')}</strong>,
                })}
              </p>
            </section>

            {/* Chat monitor */}
            <section id="monitor">
              <h2>{t('docs.guide.monitorHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.monitorIntro'), {
                  monitorView: <strong>{t('docs.guide.monitorIntroEmphasis')}</strong>,
                })}
              </p>
              <ul>
                <li>
                  {interpolateElements(t('docs.guide.monitorSend'), {
                    send: <strong>{t('docs.guide.monitorSendEmphasis')}</strong>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.monitorRediscover'), {
                    rediscover: <strong>{t('docs.guide.monitorRediscoverEmphasis')}</strong>,
                    passiveOverlay: (
                      <a href="#24-7-irl">{t('docs.guide.monitorPassiveOverlayEmphasis')}</a>
                    ),
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.monitorDisplay'), {
                    display: <strong>{t('docs.guide.monitorDisplayEmphasis')}</strong>,
                  })}
                </li>
              </ul>
            </section>

            {/* Moderation */}
            <section id="moderation">
              <h2>{t('docs.guide.moderationHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.moderationBody'), {
                  controls: <strong>{t('docs.guide.moderationControlsEmphasis')}</strong>,
                  del: <strong>{t('docs.guide.moderationDeleteEmphasis')}</strong>,
                  timeout: <strong>{t('docs.guide.moderationTimeoutEmphasis')}</strong>,
                  ban: <strong>{t('docs.guide.moderationBanEmphasis')}</strong>,
                  unban: <strong>{t('docs.guide.moderationUnbanEmphasis')}</strong>,
                })}
              </p>
              <p>
                {interpolateElements(t('docs.guide.moderationEnable'), {
                  enable: <strong>{t('docs.guide.moderationEnableEmphasis')}</strong>,
                  premium: <a href="#premium">{t('docs.guide.tocPremium')}</a>,
                })}
              </p>
            </section>

            {/* Engagement */}
            <section id="engagement">
              <h2>{t('docs.guide.engagementHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.engagementIntro'), {
                  polls: <strong>{t('docs.guide.engagementPollsEmphasis')}</strong>,
                  predictions: <strong>{t('docs.guide.engagementPredictionsEmphasis')}</strong>,
                  points: <strong>{t('docs.guide.engagementPointsEmphasis')}</strong>,
                })}
              </p>
              <ul>
                <li>
                  {interpolateElements(t('docs.guide.engagementSetup'), {
                    section: <strong>{t('docs.guide.engagementSectionEmphasis')}</strong>,
                    enablePoints: <strong>{t('docs.guide.engagementEnablePointsEmphasis')}</strong>,
                    pointsName: <strong>{t('docs.guide.engagementPointsNameEmphasis')}</strong>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.engagementRun'), {
                    section: <strong>{t('docs.guide.engagementSectionEmphasis')}</strong>,
                    startPoll: <strong>{t('docs.guide.engagementStartPollEmphasis')}</strong>,
                    closePoll: <strong>{t('docs.guide.engagementClosePollEmphasis')}</strong>,
                    startPrediction: (
                      <strong>{t('docs.guide.engagementStartPredictionEmphasis')}</strong>
                    ),
                    lockWagers: <strong>{t('docs.guide.engagementLockWagersEmphasis')}</strong>,
                    cancelRefund: <strong>{t('docs.guide.engagementCancelRefundEmphasis')}</strong>,
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.engagementJoin'), {
                    voteCommand: <Code>{VOTE_COMMAND}</Code>,
                    predictCommand: <Code>{PREDICT_COMMAND}</Code>,
                    participationPage: (
                      <strong>{t('docs.guide.engagementParticipationPageEmphasis')}</strong>
                    ),
                  })}
                </li>
                <li>
                  {interpolateElements(t('docs.guide.engagementWidgets'), {
                    pollWidget: <strong>{t('docs.guide.engagementPollWidgetEmphasis')}</strong>,
                    predictionWidget: (
                      <strong>{t('docs.guide.engagementPredictionWidgetEmphasis')}</strong>
                    ),
                  })}
                </li>
              </ul>
            </section>

            {/* Events & credit roll */}
            <section id="events-credits">
              <h2>{t('docs.guide.eventsHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.eventsBody'), {
                  events: <strong>{t('docs.guide.eventsEmphasis')}</strong>,
                  eventSettings: <strong>{t('docs.guide.eventSettingsEmphasis')}</strong>,
                })}
              </p>
              <p>
                {interpolateElements(t('docs.guide.creditsBody'), {
                  credits: <strong>{t('docs.guide.creditsEmphasis')}</strong>,
                  creditRoll: <strong>{t('docs.guide.creditRollEmphasis')}</strong>,
                  copyUrl: <strong>{t('docs.guide.creditsCopyUrlEmphasis')}</strong>,
                })}
              </p>
            </section>

            {/* Share an overlay */}
            <section id="sharing">
              <h2>{t('docs.guide.sharingHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.sharingBody'), {
                  share: <strong>{t('docs.guide.sharingShareEmphasis')}</strong>,
                  sharedOverlays: <strong>{t('docs.guide.sharingSharedOverlaysEmphasis')}</strong>,
                  premium: <a href="#premium">{t('docs.guide.tocPremium')}</a>,
                })}
              </p>
            </section>

            {/* Themes */}
            <section id="themes">
              <h2>{t('docs.guide.themesHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.themesIntro'), {
                  count: <strong>{t('docs.guide.themesCountEmphasis')}</strong>,
                  noCss: <strong>{t('docs.guide.themesNoCssEmphasis')}</strong>,
                })}
              </p>
              <ol className="list-decimal space-y-2 pl-6 text-text-sub">
                <li>{t('docs.guide.themesStepOpen')}</li>
                <li>
                  {interpolateElements(t('docs.guide.themesStepApply'), {
                    theme: <strong>{t('docs.guide.themesStepApplyEmphasis')}</strong>,
                  })}
                </li>
                <li>{t('docs.guide.themesStepSave')}</li>
              </ol>
              <p>
                {interpolateElements(t('docs.guide.themesPreview'), {
                  home: <Link href="/">{t('docs.guide.themesPreviewLinkText')}</Link>,
                })}
              </p>
            </section>

            {/* Make it your own (no-code) */}
            <section id="customize">
              <h2>{t('docs.guide.customizeHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.customizeIntro'), {
                  noCode: <strong>{t('docs.guide.customizeNoCodeEmphasis')}</strong>,
                  appearance: <strong>{t('docs.guide.customizeAppearanceEmphasis')}</strong>,
                })}
              </p>
              <ul>
                <li>{t('docs.guide.customizeFont')}</li>
                <li>{t('docs.guide.customizeSpacing')}</li>
                <li>{t('docs.guide.customizeAvatar')}</li>
                <li>{t('docs.guide.customizeVisibility')}</li>
                <li>{t('docs.guide.customizeColors')}</li>
                <li>{t('docs.guide.customizeEvents')}</li>
              </ul>
              <p>{t('docs.guide.customizeOutro')}</p>
            </section>

            {/* Custom CSS */}
            <section id="custom-css">
              <h2>{t('docs.guide.cssHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.cssIntro'), {
                  customCss: <strong>{t('docs.guide.cssCustomCssEmphasis')}</strong>,
                  expert: <strong>{t('docs.guide.cssExpertEmphasis')}</strong>,
                })}
              </p>
              <ul>
                <li>{t('docs.guide.cssScope')}</li>
                <li>
                  {interpolateElements(t('docs.guide.cssOrder'), {
                    after: <em>{t('docs.guide.cssOrderEmphasis')}</em>,
                  })}
                </li>
                <li>{t('docs.guide.cssPreview')}</li>
              </ul>

              <h3>{t('docs.guide.cssVarsHeading')}</h3>
              <p>
                {interpolateElements(t('docs.guide.cssVarsIntro'), {
                  root: <Code>{ROOT_SELECTOR}</Code>,
                })}
              </p>
              <CssVarTable rows={CSS_VARIABLES} />
              <Pre lang="css">{CSS_VARIABLES_EXAMPLE}</Pre>

              <h3>{t('docs.guide.cssHooksHeading')}</h3>
              <p>{t('docs.guide.cssHooksIntro')}</p>
              <HookTable rows={CSS_HOOKS} />
              <p>
                {interpolateElements(t('docs.guide.cssFeedAnchor'), {
                  feedAnchor: <strong>{t('docs.guide.cssFeedAnchorEmphasis')}</strong>,
                  edge: <em>{t('docs.guide.cssFeedAnchorEdgeEmphasis')}</em>,
                  invertOrder: <strong>{t('docs.guide.cssInvertOrderEmphasis')}</strong>,
                  endOfList: <em>{t('docs.guide.cssEndOfListEmphasis')}</em>,
                })}
              </p>
              <Pre lang="css">{FEED_ANCHOR_CSS_EXAMPLE}</Pre>
              <p>{t('docs.guide.cssExampleCaption')}</p>
              <Pre lang="css">{PLATFORM_STRIPE_CSS_EXAMPLE}</Pre>
              <DevCallout>
                {interpolateElements(t('docs.guide.cssCallout'), {
                  github: (
                    <a
                      href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-twitch hover:underline"
                    >
                      {t('docs.guide.cssCalloutGithubLinkText')}
                    </a>
                  ),
                  discord: (
                    <a
                      href={DISCORD_INVITE_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-twitch hover:underline"
                    >
                      {t(DISCORD_LINK_KEY)}
                    </a>
                  ),
                })}
              </DevCallout>
            </section>

            {/* Fonts */}
            <section id="fonts">
              <h2>{t('docs.guide.fontsHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.fontsIntro'), {
                  importRule: <Code>{IMPORT_AT_RULE}</Code>,
                })}
              </p>
              <p className="text-sm text-text-sub">{t('docs.guide.fontsFamilies')}</p>
              <Pre lang="css">{FONT_IMPORT_EXAMPLE}</Pre>
              <p>{t('docs.guide.fontsOutro')}</p>
            </section>

            {/* Premium */}
            <section id="premium">
              <h2>{t('docs.guide.premiumHeading')}</h2>
              <p>
                {interpolateElements(t('docs.guide.premiumIntro'), {
                  settings: <strong>{t('docs.guide.premiumSettingsEmphasis')}</strong>,
                })}
              </p>
              <ul>
                {PREMIUM_PERKS.map(({ sentenceKey, labelKey }) => (
                  <li key={sentenceKey}>
                    {interpolateElements(t(sentenceKey), {
                      label: <strong>{t(labelKey)}</strong>,
                    })}
                  </li>
                ))}
              </ul>
            </section>
          </div>

          {/* Footer */}
          <div className="mt-12 flex flex-col gap-3 border-t border-border pt-6 text-sm text-text-dim sm:flex-row sm:items-center sm:justify-between">
            <span>{t('docs.guide.footerCopyright', { year: new Date().getFullYear() })}</span>
            <div className="flex flex-wrap items-center gap-4">
              <Link href="/docs/api" className="transition-colors hover:text-text">
                {t('docs.guide.footerApiLink')}
              </Link>
              <Link href="/legal/privacy" className="transition-colors hover:text-text">
                {t('docs.guide.footerPrivacyLink')}
              </Link>
              <Link href="/legal/terms" className="transition-colors hover:text-text">
                {t('docs.guide.footerTermsLink')}
              </Link>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
