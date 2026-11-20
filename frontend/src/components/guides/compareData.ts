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
 * The data behind /compare and the per-tool /compare/<tool> pages: one table,
 * read by both, so a cell cannot say one thing on one page and another on the
 * next.
 */

import type { MessageKey } from '@/lib/i18n'

// The tools that get their own "All-Chat vs <tool>" page. `id` names the row
// and the `guides.vs.<id>*` copy; `slug` is the URL segment.
export const VS_TOOLS = [
  { id: 'multistream', slug: 'multistream-gg' },
  { id: 'ssn', slug: 'social-stream-ninja' },
  { id: 'streamlabs', slug: 'streamlabs' },
  { id: 'seLive', slug: 'streamelements' },
  { id: 'sleepyChat', slug: 'sleepychat' },
  { id: 'restream', slug: 'restream-chat' },
] as const

export type VsToolId = (typeof VS_TOOLS)[number]['id']

// The day every source below was last read. Re-read them all before moving it:
// a newer date over stale cells is worse than an old date.
export const LAST_CHECKED = '2026-10-08'

// Footnotes are numbered in this order. Each cell names the sources that back
// it; a cell about another tool with no source must be notDocumented.
export const SOURCES = [
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
  {
    id: 'multistreamHome',
    url: 'https://multistream.gg/',
    labelKey: 'guides.compare.sourceMultistreamHome',
  },
  {
    id: 'multistreamDownload',
    url: 'https://multistream.gg/download',
    labelKey: 'guides.compare.sourceMultistreamDownload',
  },
  {
    id: 'multistreamOverlay',
    url: 'https://multistream.gg/tutorials/add-an-overlay-to-obs',
    labelKey: 'guides.compare.sourceMultistreamOverlay',
  },
  {
    id: 'multistreamChats',
    url: 'https://multistream.gg/guides/see-all-your-stream-chats-in-one-window',
    labelKey: 'guides.compare.sourceMultistreamChats',
  },
  {
    id: 'multistreamPricing',
    url: 'https://multistream.gg/pricing',
    labelKey: 'guides.compare.sourceMultistreamPricing',
  },
  {
    id: 'multistreamTts',
    url: 'https://multistream.gg/tutorials/read-chat-out-loud',
    labelKey: 'guides.compare.sourceMultistreamTts',
  },
] as const satisfies readonly { id: string; url: string; labelKey: MessageKey }[]

export type SourceId = (typeof SOURCES)[number]['id']

export const COLUMNS = [
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

export const ROWS: readonly {
  id: 'allChat' | VsToolId
  nameKey: MessageKey
  cells: Readonly<Record<ColumnId, Cell>>
}[] = [
  {
    id: 'allChat',
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
      alerts: cell('guides.compare.allChatAlerts', 'allChatReadme'),
      tts: cell('guides.compare.allChatTts'),
    },
  },
  {
    id: 'streamlabs',
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
    id: 'seLive',
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
    id: 'ssn',
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
    id: 'sleepyChat',
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
    id: 'restream',
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
  {
    id: 'multistream',
    nameKey: 'guides.compare.toolMultistream',
    cells: {
      platforms: cell('guides.compare.multistreamPlatforms', 'multistreamHome'),
      install: cell('guides.compare.multistreamInstall', 'multistreamDownload'),
      overlay: cell('guides.compare.multistreamOverlay', 'multistreamOverlay'),
      dock: cell('guides.compare.multistreamDock', 'multistreamOverlay'),
      emotes: NOT_DOCUMENTED,
      price: cell('guides.compare.multistreamPrice', 'multistreamPricing', 'multistreamChats'),
      openSource: NOT_DOCUMENTED,
      selfHost: NOT_DOCUMENTED,
      alerts: cell('guides.compare.multistreamAlerts', 'multistreamHome'),
      tts: cell('guides.compare.multistreamTts', 'multistreamTts'),
    },
  },
]
