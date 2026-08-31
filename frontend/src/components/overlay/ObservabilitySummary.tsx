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

import { Check, Minus } from 'lucide-react'
import clsx from 'clsx'

import { PlatformGlyph } from '@/components/overlay/PlatformGlyph'
import type { SourceInfo } from '@/components/PlatformStatusIndicators'
import { sourceKey } from '@/core/overlayStreamCore'
import { useTranslations } from '@/lib/i18n'
import type { EventSettings, PublicOverlayConfig } from '@/lib/types/overlay'

// The wire flag paired with the stem of its catalog key, so the label is looked
// up as t(`viewerOverlay.observability.event${stem}`). `as const satisfies`
// rather than a type annotation: an annotation widens the stems to string and a
// typo would stop failing tsc.
const EVENT_TOGGLES = [
  { key: 'enable_twitch_subs', messageStem: 'TwitchSubs' },
  { key: 'enable_twitch_resubs', messageStem: 'TwitchResubs' },
  { key: 'enable_twitch_gift_subs', messageStem: 'TwitchGiftSubs' },
  { key: 'enable_twitch_bits', messageStem: 'TwitchBits' },
  { key: 'enable_twitch_raids', messageStem: 'TwitchRaids' },
  { key: 'enable_twitch_channel_points', messageStem: 'TwitchChannelPoints' },
  { key: 'enable_twitch_follows', messageStem: 'TwitchFollows' },
  { key: 'enable_twitch_watch_streaks', messageStem: 'TwitchWatchStreaks' },
  { key: 'enable_youtube_super_chat', messageStem: 'YoutubeSuperChat' },
  { key: 'enable_youtube_super_sticker', messageStem: 'YoutubeSuperSticker' },
  { key: 'enable_youtube_members', messageStem: 'YoutubeMembers' },
  { key: 'enable_youtube_member_milestones', messageStem: 'YoutubeMemberMilestones' },
  { key: 'enable_youtube_member_gifts', messageStem: 'YoutubeMemberGifts' },
  { key: 'enable_kick_subs', messageStem: 'KickSubs' },
  { key: 'enable_kick_gifts', messageStem: 'KickGifts' },
  { key: 'enable_tiktok_likes', messageStem: 'TiktokLikes' },
  { key: 'enable_tiktok_gifts', messageStem: 'TiktokGifts' },
  { key: 'enable_tiktok_follows', messageStem: 'TiktokFollows' },
  { key: 'enable_tiktok_shares', messageStem: 'TiktokShares' },
  { key: 'enable_tiktok_treasure_chests', messageStem: 'TiktokTreasureChests' },
  { key: 'enable_token_warnings', messageStem: 'TokenWarnings' },
] as const satisfies ReadonlyArray<{ key: keyof EventSettings; messageStem: string }>

function Card({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-border bg-surface p-3">
      <h3 className="mb-2 text-xs font-semibold tracking-wide text-text-sub uppercase">{title}</h3>
      {children}
    </div>
  )
}

interface ObservabilitySummaryProps {
  config: PublicOverlayConfig | null
  sources: Map<string, SourceInfo>
  /** Keyed by `sourceKey()`, so look up with the platform as well as the id. */
  activeChannels: Set<string>
  eventSettings: EventSettings | null
  observedEventTypes: Set<string>
}

/** Read-only overview of how the overlay is configured: sources, enabled events, emotes, filters. */
export function ObservabilitySummary({
  config,
  sources,
  activeChannels,
  eventSettings,
  observedEventTypes,
}: ObservabilitySummaryProps) {
  const t = useTranslations()
  const filter = config?.filter_settings ?? {}
  const sourceList = Array.from(sources.values())

  return (
    <div className="grid gap-3 border-b border-border bg-bg p-3 md:grid-cols-2 lg:grid-cols-4">
      <Card title={t('viewerOverlay.observability.sources', { count: sourceList.length })}>
        {sourceList.length === 0 ? (
          <p className="text-sm text-text-dim">{t('viewerOverlay.observability.noSources')}</p>
        ) : (
          <ul className="space-y-1">
            {sourceList.map((s) => {
              const key = sourceKey(s.platform, s.channelId)
              const isLive = activeChannels.has(key)
              return (
                <li key={key} className="flex items-center gap-2 text-sm text-text">
                  <PlatformGlyph platform={s.platform} className="h-4 w-4 shrink-0" />
                  <span className="min-w-0 flex-1 truncate">{s.channelName}</span>
                  <span
                    className={clsx(
                      'shrink-0 rounded px-1.5 text-[10px] font-semibold uppercase',
                      isLive ? 'bg-kick/15 text-kick' : 'bg-surface-2 text-text-dim'
                    )}
                  >
                    {isLive
                      ? t('viewerOverlay.observability.sourceLive')
                      : t('viewerOverlay.observability.sourceIdle')}
                  </span>
                </li>
              )
            })}
          </ul>
        )}
      </Card>

      <Card title={t('viewerOverlay.observability.configuredEvents')}>
        {eventSettings ? (
          <ul className="grid grid-cols-1 gap-x-3 gap-y-0.5 sm:grid-cols-2">
            {EVENT_TOGGLES.map(({ key, messageStem }) => {
              const on = eventSettings[key] === true
              const label = t(`viewerOverlay.observability.event${messageStem}`)
              return (
                <li key={key} className="flex items-center gap-1.5 text-xs">
                  {on ? (
                    <Check className="h-3 w-3 shrink-0 text-kick" />
                  ) : (
                    <Minus className="h-3 w-3 shrink-0 text-text-dim" />
                  )}
                  <span className={on ? 'text-text' : 'text-text-dim line-through'}>{label}</span>
                </li>
              )
            })}
          </ul>
        ) : observedEventTypes.size > 0 ? (
          <div className="flex flex-wrap gap-1">
            {Array.from(observedEventTypes).map((t) => (
              <span
                key={t}
                className="rounded bg-surface-2 px-1.5 py-0.5 text-[10px] text-text-sub"
              >
                {t}
              </span>
            ))}
          </div>
        ) : (
          <p className="text-sm text-text-dim">
            {t('viewerOverlay.observability.eventsUnavailable')}
          </p>
        )}
      </Card>

      <Card title={t('viewerOverlay.observability.emotes')}>
        <dl className="space-y-1 text-sm">
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.sevenTvSet')}</dt>
            <dd className="min-w-0 truncate text-text">
              {config?.seventv_emote_set_id || t('viewerOverlay.observability.sevenTvDefault')}
            </dd>
          </div>
        </dl>
      </Card>

      <Card title={t('viewerOverlay.observability.filters')}>
        <dl className="space-y-1 text-sm">
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.bannedWords')}</dt>
            <dd className="text-text tabular-nums">{filter.banned_words?.length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.bannedUsers')}</dt>
            <dd className="text-text tabular-nums">{filter.banned_users?.length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.minLength')}</dt>
            <dd className="text-text tabular-nums">{filter.min_message_length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.hideCommands')}</dt>
            <dd className="text-text">
              {filter.hide_commands
                ? t('viewerOverlay.observability.yes')
                : t('viewerOverlay.observability.no')}
            </dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">{t('viewerOverlay.observability.sayHiFilter')}</dt>
            <dd className="text-text">
              {filter.hide_youtube_say_hi
                ? t('viewerOverlay.observability.yes')
                : t('viewerOverlay.observability.no')}
            </dd>
          </div>
          <p className="pt-1 text-[10px] text-text-dim">
            {t('viewerOverlay.observability.filtersNote')}
          </p>
        </dl>
      </Card>
    </div>
  )
}
