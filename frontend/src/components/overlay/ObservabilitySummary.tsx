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
import type { EventSettings, PublicOverlayConfig } from '@/lib/types/overlay'

const EVENT_TOGGLES: Array<{ key: keyof EventSettings; label: string }> = [
  { key: 'enable_twitch_subs', label: 'Twitch Subs' },
  { key: 'enable_twitch_resubs', label: 'Twitch Resubs' },
  { key: 'enable_twitch_gift_subs', label: 'Twitch Gift Subs' },
  { key: 'enable_twitch_bits', label: 'Twitch Bits' },
  { key: 'enable_twitch_raids', label: 'Twitch Raids' },
  { key: 'enable_twitch_channel_points', label: 'Channel Points' },
  { key: 'enable_twitch_follows', label: 'Twitch Follows' },
  { key: 'enable_twitch_watch_streaks', label: 'Watch Streaks' },
  { key: 'enable_youtube_super_chat', label: 'YouTube Super Chat' },
  { key: 'enable_youtube_super_sticker', label: 'Super Sticker' },
  { key: 'enable_youtube_members', label: 'YouTube Members' },
  { key: 'enable_youtube_member_milestones', label: 'Member Milestones' },
  { key: 'enable_youtube_member_gifts', label: 'Member Gifts' },
  { key: 'enable_kick_subs', label: 'Kick Subs' },
  { key: 'enable_kick_gifts', label: 'Kick Gifts' },
  { key: 'enable_tiktok_likes', label: 'TikTok Likes' },
  { key: 'enable_tiktok_gifts', label: 'TikTok Gifts' },
  { key: 'enable_tiktok_follows', label: 'TikTok Follows' },
  { key: 'enable_tiktok_shares', label: 'TikTok Shares' },
  { key: 'enable_token_warnings', label: 'Token Warnings' },
]

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
  const filter = config?.filter_settings ?? {}
  const sourceList = Array.from(sources.values())

  return (
    <div className="grid gap-3 border-b border-border bg-bg p-3 md:grid-cols-2 lg:grid-cols-4">
      <Card title={`Sources (${sourceList.length})`}>
        {sourceList.length === 0 ? (
          <p className="text-sm text-text-dim">No sources configured.</p>
        ) : (
          <ul className="space-y-1">
            {sourceList.map((s) => (
              <li key={s.channelId} className="flex items-center gap-2 text-sm text-text">
                <PlatformGlyph platform={s.platform} className="h-4 w-4 shrink-0" />
                <span className="min-w-0 flex-1 truncate">{s.channelName}</span>
                <span
                  className={clsx(
                    'shrink-0 rounded px-1.5 text-[10px] font-semibold uppercase',
                    activeChannels.has(s.channelId)
                      ? 'bg-kick/15 text-kick'
                      : 'bg-surface-2 text-text-dim'
                  )}
                >
                  {activeChannels.has(s.channelId) ? 'live' : 'idle'}
                </span>
              </li>
            ))}
          </ul>
        )}
      </Card>

      <Card title="Configured Events">
        {eventSettings ? (
          <ul className="grid grid-cols-1 gap-x-3 gap-y-0.5 sm:grid-cols-2">
            {EVENT_TOGGLES.map(({ key, label }) => {
              const on = eventSettings[key] === true
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
            Event configuration unavailable; events appear here as they arrive.
          </p>
        )}
      </Card>

      <Card title="Emotes">
        <dl className="space-y-1 text-sm">
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">7TV set</dt>
            <dd className="min-w-0 truncate text-text">
              {config?.seventv_emote_set_id || 'per-source default'}
            </dd>
          </div>
        </dl>
      </Card>

      <Card title="Filters">
        <dl className="space-y-1 text-sm">
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">Banned words</dt>
            <dd className="text-text tabular-nums">{filter.banned_words?.length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">Banned users</dt>
            <dd className="text-text tabular-nums">{filter.banned_users?.length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">Min length</dt>
            <dd className="text-text tabular-nums">{filter.min_message_length ?? 0}</dd>
          </div>
          <div className="flex justify-between gap-2">
            <dt className="text-text-sub">Hide commands</dt>
            <dd className="text-text">{filter.hide_commands ? 'yes' : 'no'}</dd>
          </div>
          <p className="pt-1 text-[10px] text-text-dim">
            Filters are shown for reference; this view displays all messages.
          </p>
        </dl>
      </Card>
    </div>
  )
}
