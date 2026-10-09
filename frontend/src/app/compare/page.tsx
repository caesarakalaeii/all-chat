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

import { GuidePage } from '@/components/guides/GuidePage'
import { getTranslations, type MessageKey } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// getTranslations, not useTranslations: this is a Server Component.
const t = getTranslations()

export const metadata = {
  title: t('guides.compare.metaTitle'),
  description: t('guides.compare.metaDescription'),
  alternates: { canonical: '/compare' },
}

// The day every source below was last read. Re-read them all before moving it:
// a newer date over stale cells is worse than an old date.
const LAST_CHECKED = '2026-10-08'

const ISSUES_URL = 'https://github.com/caesarakalaeii/all-chat/issues'

// Footnotes are numbered in this order. Each cell names the sources that back
// it; a cell about another tool with no source must be notDocumented.
const SOURCES = [
  {
    id: 'allChatReadme',
    url: 'https://github.com/caesarakalaeii/all-chat#readme',
    labelKey: 'guides.compare.sourceAllChatReadme',
  },
  {
    id: 'streamlabsChatBox',
    url: 'https://streamlabs.com/stream-widgets/chat-box',
    labelKey: 'guides.compare.sourceStreamlabsChatBox',
  },
  {
    id: 'streamlabsMultistream',
    url: 'https://streamlabs.com/multistream',
    labelKey: 'guides.compare.sourceStreamlabsMultistream',
  },
  {
    id: 'streamlabsWidgets',
    url: 'https://streamlabs.com/stream-widgets',
    labelKey: 'guides.compare.sourceStreamlabsWidgets',
  },
  {
    id: 'streamlabsTiktok',
    url: 'https://support.streamlabs.com/hc/en-us/articles/24730653249947-How-to-Go-Live-on-TikTok-from-a-Desktop',
    labelKey: 'guides.compare.sourceStreamlabsTiktok',
  },
  {
    id: 'streamlabsDesktopRepo',
    url: 'https://github.com/streamlabs/desktop',
    labelKey: 'guides.compare.sourceStreamlabsDesktopRepo',
  },
  {
    id: 'streamlabsTts',
    url: 'https://streamlabs.com/content-hub/post/how-to-add-text-to-speech-to-donations-to-your-stream',
    labelKey: 'guides.compare.sourceStreamlabsTts',
  },
  {
    id: 'seLiveGuide',
    url: 'https://support.streamelements.com/hc/en-us/articles/18771457842066-SE-Live-The-Complete-Guide-Setup-Features-Multistreaming-Troubleshooting',
    labelKey: 'guides.compare.sourceSeLiveGuide',
  },
  {
    id: 'seMultichatOverlay',
    url: 'https://support.streamelements.com/hc/en-us/community/posts/24720886083730-Multichat-Overlay',
    labelKey: 'guides.compare.sourceSeMultichatOverlay',
  },
  {
    id: 'seAlerts',
    url: 'https://support.streamelements.com/hc/en-us/articles/16789217829778-Setting-Up-Twitch-Alerts-with-StreamElements-Overlays',
    labelKey: 'guides.compare.sourceSeAlerts',
  },
  {
    id: 'seTts',
    url: 'https://support.streamelements.com/hc/en-us/articles/10474564060818-Stream-Store-The-Complete-Guide-Perks-Sound-Commands-TTS',
    labelKey: 'guides.compare.sourceSeTts',
  },
  {
    id: 'ssnReadme',
    url: 'https://github.com/steveseguin/social_stream',
    labelKey: 'guides.compare.sourceSsnReadme',
  },
  {
    id: 'ssnFeatures',
    url: 'https://socialstream.ninja/docs/features.html',
    labelKey: 'guides.compare.sourceSsnFeatures',
  },
  {
    id: 'ssnParameters',
    url: 'https://github.com/steveseguin/social_stream/blob/main/parameters.md',
    labelKey: 'guides.compare.sourceSsnParameters',
  },
  {
    id: 'sleepyChatHome',
    url: 'https://sleepychat.org/',
    labelKey: 'guides.compare.sourceSleepyChatHome',
  },
  {
    id: 'sleepyChatObs',
    url: 'https://sleepychat.org/guides/obs-multi-stream-chat-browser-dock-overlay/',
    labelKey: 'guides.compare.sourceSleepyChatObs',
  },
  {
    id: 'sleepyChatPricing',
    url: 'https://sleepychat.org/pricing/',
    labelKey: 'guides.compare.sourceSleepyChatPricing',
  },
  {
    id: 'sleepyChatRepo',
    url: 'https://github.com/sleepylessons/sleepychat-multistream-chat',
    labelKey: 'guides.compare.sourceSleepyChatRepo',
  },
  {
    id: 'restreamRead',
    url: 'https://support.restream.io/en/articles/2388185-read-incoming-chat-messages',
    labelKey: 'guides.compare.sourceRestreamRead',
  },
  {
    id: 'restreamChat',
    url: 'https://restream.io/chat',
    labelKey: 'guides.compare.sourceRestreamChat',
  },
  {
    id: 'restreamEncoders',
    url: 'https://support.restream.io/en/articles/9288891-how-to-use-restream-chat-with-encoders',
    labelKey: 'guides.compare.sourceRestreamEncoders',
  },
  {
    id: 'restreamEmbed',
    url: 'https://support.restream.io/en/articles/2524916-embed-chat-on-your-streaming-software',
    labelKey: 'guides.compare.sourceRestreamEmbed',
  },
  {
    id: 'restreamFree',
    url: 'https://support.restream.io/en/articles/9127747-can-i-use-restream-for-free-yes',
    labelKey: 'guides.compare.sourceRestreamFree',
  },
] as const satisfies readonly { id: string; url: string; labelKey: MessageKey }[]

type SourceId = (typeof SOURCES)[number]['id']

const COLUMNS = [
  { id: 'platforms', labelKey: 'guides.compare.colPlatforms' },
  { id: 'install', labelKey: 'guides.compare.colInstall' },
  { id: 'overlay', labelKey: 'guides.compare.colOverlay' },
  { id: 'dock', labelKey: 'guides.compare.colDock' },
  { id: 'emotes', labelKey: 'guides.compare.colEmotes' },
  { id: 'price', labelKey: 'guides.compare.colPrice' },
  { id: 'openSource', labelKey: 'guides.compare.colOpenSource' },
  { id: 'selfHost', labelKey: 'guides.compare.colSelfHost' },
  { id: 'alerts', labelKey: 'guides.compare.colAlerts' },
  { id: 'tts', labelKey: 'guides.compare.colTts' },
] as const satisfies readonly { id: string; labelKey: MessageKey }[]

type ColumnId = (typeof COLUMNS)[number]['id']

interface Cell {
  textKey: MessageKey
  sources: readonly SourceId[]
}

const cell = (textKey: MessageKey, ...sources: SourceId[]): Cell => ({ textKey, sources })

const NOT_DOCUMENTED = cell('guides.compare.notDocumented')

const ROWS: readonly { nameKey: MessageKey; cells: Readonly<Record<ColumnId, Cell>> }[] = [
  {
    nameKey: 'guides.compare.toolAllChat',
    cells: {
      platforms: cell('guides.compare.allChatPlatforms', 'allChatReadme'),
      install: cell('guides.compare.allChatInstall', 'allChatReadme'),
      overlay: cell('guides.compare.yes', 'allChatReadme'),
      dock: cell('guides.compare.allChatDock', 'allChatReadme'),
      emotes: cell('guides.compare.allThree', 'allChatReadme'),
      price: cell('guides.compare.allChatPrice'),
      openSource: cell('guides.compare.allChatOpenSource', 'allChatReadme'),
      selfHost: cell('guides.compare.allChatSelfHost', 'allChatReadme'),
      alerts: cell('guides.compare.allChatAlerts'),
      tts: cell('guides.compare.allChatTts'),
    },
  },
  {
    nameKey: 'guides.compare.toolStreamlabs',
    cells: {
      platforms: cell(
        'guides.compare.streamlabsPlatforms',
        'streamlabsChatBox',
        'streamlabsTiktok'
      ),
      install: cell(
        'guides.compare.streamlabsInstall',
        'streamlabsChatBox',
        'streamlabsMultistream'
      ),
      overlay: cell('guides.compare.streamlabsOverlay', 'streamlabsChatBox'),
      dock: cell('guides.compare.streamlabsDock', 'streamlabsMultistream'),
      emotes: cell('guides.compare.allThree', 'streamlabsChatBox'),
      price: cell('guides.compare.streamlabsPrice', 'streamlabsWidgets', 'streamlabsMultistream'),
      openSource: cell('guides.compare.streamlabsOpenSource', 'streamlabsDesktopRepo'),
      selfHost: NOT_DOCUMENTED,
      alerts: cell('guides.compare.streamlabsAlerts', 'streamlabsWidgets'),
      tts: cell('guides.compare.streamlabsTts', 'streamlabsTts'),
    },
  },
  {
    nameKey: 'guides.compare.toolSeLive',
    cells: {
      platforms: cell('guides.compare.seLivePlatforms', 'seLiveGuide'),
      install: cell('guides.compare.seLiveInstall', 'seLiveGuide'),
      overlay: cell('guides.compare.seLiveOverlay', 'seMultichatOverlay'),
      dock: cell('guides.compare.seLiveDock', 'seLiveGuide'),
      emotes: NOT_DOCUMENTED,
      price: cell('guides.compare.seLivePrice', 'seLiveGuide'),
      openSource: NOT_DOCUMENTED,
      selfHost: NOT_DOCUMENTED,
      alerts: cell('guides.compare.seLiveAlerts', 'seAlerts'),
      tts: cell('guides.compare.seLiveTts', 'seTts'),
    },
  },
  {
    nameKey: 'guides.compare.toolSsn',
    cells: {
      platforms: cell('guides.compare.ssnPlatforms', 'ssnReadme', 'ssnFeatures'),
      install: cell('guides.compare.ssnInstall', 'ssnReadme'),
      overlay: cell('guides.compare.yes', 'ssnReadme'),
      dock: cell('guides.compare.ssnDock', 'ssnReadme'),
      emotes: cell('guides.compare.allThree', 'ssnParameters'),
      price: cell('guides.compare.ssnPrice', 'ssnReadme'),
      openSource: cell('guides.compare.ssnOpenSource', 'ssnReadme'),
      selfHost: NOT_DOCUMENTED,
      alerts: cell('guides.compare.ssnAlerts', 'ssnReadme'),
      tts: cell('guides.compare.ssnTts', 'ssnReadme', 'ssnFeatures'),
    },
  },
  {
    nameKey: 'guides.compare.toolSleepyChat',
    cells: {
      platforms: cell('guides.compare.sleepyChatPlatforms', 'sleepyChatHome'),
      install: cell('guides.compare.sleepyChatInstall', 'sleepyChatObs'),
      overlay: cell('guides.compare.yes', 'sleepyChatObs'),
      dock: cell('guides.compare.yes', 'sleepyChatObs'),
      emotes: NOT_DOCUMENTED,
      price: cell('guides.compare.sleepyChatPrice', 'sleepyChatPricing'),
      openSource: cell('guides.compare.sleepyChatOpenSource', 'sleepyChatRepo'),
      selfHost: NOT_DOCUMENTED,
      alerts: cell('guides.compare.sleepyChatAlerts', 'sleepyChatHome'),
      tts: cell('guides.compare.sleepyChatTts', 'sleepyChatHome'),
    },
  },
  {
    nameKey: 'guides.compare.toolRestream',
    cells: {
      platforms: cell('guides.compare.restreamPlatforms', 'restreamRead'),
      install: cell('guides.compare.restreamInstall', 'restreamChat'),
      overlay: cell('guides.compare.restreamOverlay', 'restreamEmbed'),
      dock: cell('guides.compare.yes', 'restreamEncoders'),
      emotes: NOT_DOCUMENTED,
      price: cell('guides.compare.restreamPrice', 'restreamFree', 'restreamRead'),
      openSource: NOT_DOCUMENTED,
      selfHost: NOT_DOCUMENTED,
      alerts: NOT_DOCUMENTED,
      tts: cell('guides.compare.restreamTts', 'restreamChat'),
    },
  },
]

function SourceRefs({ ids }: { ids: readonly SourceId[] }) {
  if (ids.length === 0) return null
  return (
    <sup className="whitespace-nowrap">
      {ids.map((id) => {
        const number = SOURCES.findIndex((source) => source.id === id) + 1
        return (
          <a
            key={id}
            href={`#source-${number}`}
            aria-label={t('guides.compare.sourceRefLabel', { number })}
            className="ml-1"
          >
            [{number}]
          </a>
        )
      })}
    </sup>
  )
}

const lastChecked = <time dateTime={LAST_CHECKED}>{LAST_CHECKED}</time>

export default function ComparePage() {
  return (
    <GuidePage
      path="/compare"
      breadcrumb={t('guides.compare.breadcrumb')}
      heading={t('guides.compare.heading')}
      intro={t('guides.compare.intro')}
    >
      <section id="table">
        <p>{interpolateElements(t('guides.compare.lastChecked'), { date: lastChecked })}</p>
        {/* Wider than the panel on purpose: on a phone the table scrolls inside
            this box instead of crushing eleven columns into the viewport. */}
        <div className="my-4 overflow-x-auto border-2 border-white">
          <table className="w-full min-w-4xl text-left text-sm">
            <caption className="text-sub px-3 py-2 text-left text-xs">
              {interpolateElements(t('guides.compare.caption'), { date: lastChecked })}
            </caption>
            <thead>
              <tr>
                <th scope="col">{t('guides.compare.colTool')}</th>
                {COLUMNS.map((column) => (
                  <th key={column.id} scope="col" className="align-bottom">
                    {t(column.labelKey)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr key={row.nameKey} className="align-top">
                  <th scope="row">{t(row.nameKey)}</th>
                  {COLUMNS.map((column) => {
                    const { textKey, sources } = row.cells[column.id]
                    return (
                      <td key={column.id} className="text-sub">
                        {t(textKey)}
                        <SourceRefs ids={sources} />
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section id="better-pick">
        <h2>{t('guides.compare.betterHeading')}</h2>
        <p>{t('guides.compare.betterIntro')}</p>
        <ul>
          <li>{t('guides.compare.betterAlerts')}</li>
          <li>{t('guides.compare.betterTts')}</li>
          <li>{t('guides.compare.betterPlatforms')}</li>
          <li>{t('guides.compare.betterRestream')}</li>
        </ul>
      </section>

      <section id="all-chat">
        <h2>{t('guides.compare.fitHeading')}</h2>
        <p>{t('guides.compare.fitBody')}</p>
        <p>
          {interpolateElements(t('guides.compare.corrections'), {
            issue: (
              <a href={ISSUES_URL} target="_blank" rel="noopener noreferrer">
                {t('guides.compare.correctionsLinkText')}
              </a>
            ),
          })}
        </p>
      </section>

      <section id="sources">
        <h2>{t('guides.compare.sourcesHeading')}</h2>
        <ol className="list-decimal space-y-1 pl-6 text-sm">
          {SOURCES.map(({ id, url, labelKey }, index) => (
            <li key={id} id={`source-${index + 1}`}>
              {t(labelKey)}:
              <a href={url} target="_blank" rel="noopener noreferrer" className="ml-1 break-all">
                {url}
              </a>
            </li>
          ))}
        </ol>
      </section>
    </GuidePage>
  )
}
