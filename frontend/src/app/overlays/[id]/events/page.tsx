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

// Event Settings Page - Configure overlay event display preferences
'use client'

import { use, useEffect, useId, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/lib/toast'
import { ChevronLeft } from 'lucide-react'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { cn } from '@/lib/utils'
import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

interface EventSettings {
  id: string
  overlay_id: string
  // Twitch
  enable_twitch_subs: boolean
  enable_twitch_resubs: boolean
  enable_twitch_gift_subs: boolean
  enable_twitch_bits: boolean
  enable_twitch_raids: boolean
  enable_twitch_channel_points: boolean
  enable_twitch_follows: boolean
  enable_twitch_watch_streaks: boolean
  // YouTube
  enable_youtube_super_chat: boolean
  enable_youtube_super_sticker: boolean
  enable_youtube_members: boolean
  enable_youtube_member_milestones: boolean
  enable_youtube_member_gifts: boolean
  // Kick
  enable_kick_subs: boolean
  enable_kick_gifts: boolean
  // TikTok
  enable_tiktok_likes: boolean
  enable_tiktok_gifts: boolean
  enable_tiktok_follows: boolean
  enable_tiktok_shares: boolean
  enable_tiktok_treasure_chests: boolean
  // System
  enable_token_warnings: boolean
  // Settings
  tiktok_like_aggregation_window_seconds: number
  event_display_duration_multiplier: number
}

type Tab = 'global' | 'twitch' | 'youtube' | 'kick' | 'tiktok'

const TAB_IDS: readonly Tab[] = ['global', 'twitch', 'youtube', 'kick', 'tiktok']

const TAB_COLOR: Record<Tab, string> = {
  global: 'text-text',
  twitch: PLATFORM_COLORS.twitch.text,
  youtube: PLATFORM_COLORS.youtube.text,
  kick: PLATFORM_COLORS.kick.text,
  tiktok: PLATFORM_COLORS.tiktok.text,
}

/**
 * The event toggles per platform tab, in render order.
 *
 * `messageStem` is a catalog key stem, not copy: the label is
 * overlayEditor.eventSettings.<stem>Label and the description is
 * <stem>Description. `as const satisfies` rather than a type annotation keeps
 * the stems literal, so a typo fails tsc at the t() call instead of resolving
 * to its own key name at runtime.
 */
const EVENT_TOGGLES = {
  twitch: [
    { key: 'enable_twitch_subs', messageStem: 'twitchSubs' },
    { key: 'enable_twitch_resubs', messageStem: 'twitchResubs' },
    { key: 'enable_twitch_gift_subs', messageStem: 'twitchGiftSubs' },
    { key: 'enable_twitch_bits', messageStem: 'twitchBits' },
    { key: 'enable_twitch_raids', messageStem: 'twitchRaids' },
    { key: 'enable_twitch_channel_points', messageStem: 'twitchChannelPoints' },
    { key: 'enable_twitch_follows', messageStem: 'twitchFollows' },
    { key: 'enable_twitch_watch_streaks', messageStem: 'twitchWatchStreaks' },
  ],
  youtube: [
    { key: 'enable_youtube_super_chat', messageStem: 'youtubeSuperChat' },
    { key: 'enable_youtube_super_sticker', messageStem: 'youtubeSuperSticker' },
    { key: 'enable_youtube_members', messageStem: 'youtubeMembers' },
    { key: 'enable_youtube_member_milestones', messageStem: 'youtubeMemberMilestones' },
    { key: 'enable_youtube_member_gifts', messageStem: 'youtubeMemberGifts' },
  ],
  kick: [
    { key: 'enable_kick_subs', messageStem: 'kickSubs' },
    { key: 'enable_kick_gifts', messageStem: 'kickGifts' },
  ],
  tiktok: [
    { key: 'enable_tiktok_likes', messageStem: 'tiktokLikes' },
    { key: 'enable_tiktok_gifts', messageStem: 'tiktokGifts' },
    { key: 'enable_tiktok_follows', messageStem: 'tiktokFollows' },
    { key: 'enable_tiktok_shares', messageStem: 'tiktokShares' },
    // Honest description rather than a promise. TikTok has not been observed
    // delivering the ENVELOPE frame a coin chest rides on: no chest reached us
    // across ~75 minutes of monitoring eight live rooms, one of them with 61
    // gifts, and no undecodable message resembling an envelope appeared either.
    // The cause is upstream and still unknown, so the toggle stays (it works the
    // moment a frame does arrive) but must not claim a feature we have never once
    // delivered. Drop the caveat from tiktokTreasureChestsDescription once a
    // chest is confirmed end to end.
    { key: 'enable_tiktok_treasure_chests', messageStem: 'tiktokTreasureChests' },
  ],
} as const satisfies Record<
  Exclude<Tab, 'global'>,
  ReadonlyArray<{ key: keyof EventSettings; messageStem: string }>
>

/**
 * The two CSS class names the tier explainer names. Protocol, not copy: they
 * are selectors a streamer writes in their own stylesheet.
 */
const TIER_CLASS = '.event-tier-high'
const EVENT_TYPE_CLASS = '.event-type-raid'

// ---------------------------------------------------------------------------
// EventToggle
// ---------------------------------------------------------------------------
function EventToggle({
  label,
  description,
  value,
  onChange,
}: {
  label: string
  description: string
  value: boolean
  onChange: (value: boolean) => void
}) {
  const descriptionId = useId()
  return (
    <div className="flex items-center justify-between border-b border-border py-3.5 last:border-0">
      <div className="flex-1 pr-4">
        <p className="text-sm font-medium text-text">{label}</p>
        <p id={descriptionId} className="mt-0.5 text-xs text-text-sub">
          {description}
        </p>
      </div>
      <label className="relative inline-flex shrink-0 cursor-pointer items-center">
        <span className="sr-only">{label}</span>
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
          aria-describedby={descriptionId}
          className="peer sr-only"
        />
        <div className="peer h-5 w-10 rounded-full border border-border bg-surface-2 peer-checked:bg-twitch peer-focus-visible:ring-2 peer-focus-visible:ring-twitch after:absolute after:top-[3px] after:left-[3px] after:h-3.5 after:w-3.5 after:rounded-full after:bg-white after:transition-transform after:content-[''] peer-checked:after:translate-x-5" />
      </label>
    </div>
  )
}

// ---------------------------------------------------------------------------
// NumberInput
// ---------------------------------------------------------------------------
function NumberInput({
  label,
  description,
  value,
  min,
  max,
  step,
  onChange,
}: {
  label: string
  description: string
  value: number
  min: number
  max: number
  step?: number
  onChange: (value: number) => void
}) {
  return (
    <div className="border-b border-border py-3 last:border-0">
      <label className="mb-0.5 block text-sm font-medium text-text">{label}</label>
      <p className="mb-2 text-xs text-text-sub">{description}</p>
      <input
        type="number"
        min={min}
        max={max}
        step={step ?? 1}
        value={value}
        onChange={(e) =>
          onChange(step && step < 1 ? parseFloat(e.target.value) : parseInt(e.target.value))
        }
        className="w-28 rounded-lg border border-border bg-surface-2 px-3 py-1.5 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      />
    </div>
  )
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------
export default function EventSettingsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const t = useTranslations()
  const { user } = useAuthStore()
  const router = useRouter()
  const [settings, setSettings] = useState<EventSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [activeTab, setActiveTab] = useState<Tab>('global')

  useEffect(() => {
    if (!user) {
      router.push('/')
      return
    }

    fetch(`/api/v1/overlays/${id}/event-settings`)
      .then((res) => (res.ok ? res.json() : Promise.reject('Failed to load')))
      .then(setSettings)
      .catch(() =>
        toastManager.add({ title: t('overlayEditor.eventSettings.loadFailed'), type: 'error' })
      )
      .finally(() => setLoading(false))
  }, [id, user, router, t])

  const update = (key: keyof EventSettings, value: boolean | number) => {
    if (!settings) return
    setSettings({ ...settings, [key]: value })
  }

  const handleSave = async () => {
    if (!settings || !user) return
    setSaving(true)
    try {
      const res = await fetch(`/api/v1/overlays/${id}/event-settings`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
      })
      if (!res.ok) throw new Error()
      setSettings(await res.json())
      toastManager.add({ title: 'Event settings saved', type: 'success' })
    } catch {
      toastManager.add({ title: 'Failed to save event settings', type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <div className="mx-auto max-w-3xl px-4 py-8">
        {/* Header */}
        <div className="mb-6">
          <button
            onClick={() => router.push(`/overlays/${id}`)}
            className="mb-3 flex items-center gap-1 rounded text-sm text-text-sub hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            <ChevronLeft className="size-4" />
            {t('overlayEditor.eventSettings.back')}
          </button>
          <h1 className="text-2xl font-bold text-text">
            {t('overlayEditor.eventSettings.heading')}
          </h1>
          <p className="mt-1 text-sm text-text-sub">
            {t('overlayEditor.eventSettings.subheading')}
          </p>
        </div>

        {loading ? (
          <Card className="space-y-4 p-6">
            <Skeleton className="h-5 w-40" />
            {[0, 1, 2, 3].map((i) => (
              <Skeleton key={i} className="h-14 w-full" />
            ))}
          </Card>
        ) : !settings ? (
          <Card className="p-6 text-center">
            <p className="text-destructive mb-4">{t('overlayEditor.eventSettings.loadFailed')}</p>
            <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
              {t('overlayEditor.eventSettings.back')}
            </Button>
          </Card>
        ) : (
          <Card className="overflow-hidden">
            {/* Platform tabs */}
            <div className="flex overflow-x-auto border-b border-border">
              {TAB_IDS.map((tab) => (
                <button
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={cn(
                    'shrink-0 border-b-2 px-4 py-3 text-sm font-medium transition-colors focus-visible:outline-none',
                    activeTab === tab
                      ? cn(TAB_COLOR[tab], 'border-current')
                      : 'border-transparent text-text-sub hover:text-text'
                  )}
                >
                  {tab === 'global'
                    ? t('overlayEditor.eventSettings.tabGlobal')
                    : t(`common.platforms.${tab}`)}
                </button>
              ))}
            </div>

            <div className="p-6">
              {/* Global tab */}
              {activeTab === 'global' && (
                <div>
                  <p className="mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    {t('overlayEditor.eventSettings.systemEventsHeading')}
                  </p>
                  <EventToggle
                    label={t('overlayEditor.eventSettings.tokenWarningsLabel')}
                    description={t('overlayEditor.eventSettings.tokenWarningsDescription')}
                    value={settings.enable_token_warnings}
                    onChange={(v) => update('enable_token_warnings', v)}
                  />
                  <p className="mt-6 mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    {t('overlayEditor.eventSettings.displaySettingsHeading')}
                  </p>
                  <NumberInput
                    label={t('overlayEditor.eventSettings.durationMultiplierLabel')}
                    description={t('overlayEditor.eventSettings.durationMultiplierDescription')}
                    value={settings.event_display_duration_multiplier}
                    min={0.1}
                    max={5}
                    step={0.1}
                    onChange={(v) => update('event_display_duration_multiplier', v)}
                  />
                  <div className="mt-6 space-y-1.5 rounded-lg border border-border bg-surface-2 p-4 text-xs text-text-sub">
                    <p className="mb-2 font-semibold text-text">
                      {t('overlayEditor.eventSettings.tiersHeading')}
                    </p>
                    <p>
                      {interpolateElements(t('overlayEditor.eventSettings.tierHigh'), {
                        tier: (
                          <strong className="text-text">
                            {t('overlayEditor.eventSettings.tierHighName')}
                          </strong>
                        ),
                      })}
                    </p>
                    <p>
                      {interpolateElements(t('overlayEditor.eventSettings.tierMedium'), {
                        tier: (
                          <strong className="text-text">
                            {t('overlayEditor.eventSettings.tierMediumName')}
                          </strong>
                        ),
                      })}
                    </p>
                    <p>
                      {interpolateElements(t('overlayEditor.eventSettings.tierLow'), {
                        tier: (
                          <strong className="text-text">
                            {t('overlayEditor.eventSettings.tierLowName')}
                          </strong>
                        ),
                      })}
                    </p>
                    <p>
                      {interpolateElements(t('overlayEditor.eventSettings.tierStyling'), {
                        tierClass: <code className="rounded bg-surface px-1">{TIER_CLASS}</code>,
                        typeClass: (
                          <code className="rounded bg-surface px-1">{EVENT_TYPE_CLASS}</code>
                        ),
                      })}
                    </p>
                  </div>
                </div>
              )}

              {/* Twitch tab */}
              {activeTab === 'twitch' && (
                <div>
                  {EVENT_TOGGLES.twitch.map(({ key, messageStem }) => (
                    <EventToggle
                      key={key}
                      label={t(`overlayEditor.eventSettings.${messageStem}Label`)}
                      description={t(`overlayEditor.eventSettings.${messageStem}Description`)}
                      value={settings[key] as boolean}
                      onChange={(v) => update(key, v)}
                    />
                  ))}
                </div>
              )}

              {/* YouTube tab */}
              {activeTab === 'youtube' && (
                <div>
                  {EVENT_TOGGLES.youtube.map(({ key, messageStem }) => (
                    <EventToggle
                      key={key}
                      label={t(`overlayEditor.eventSettings.${messageStem}Label`)}
                      description={t(`overlayEditor.eventSettings.${messageStem}Description`)}
                      value={settings[key] as boolean}
                      onChange={(v) => update(key, v)}
                    />
                  ))}
                </div>
              )}

              {/* Kick tab */}
              {activeTab === 'kick' && (
                <div>
                  {EVENT_TOGGLES.kick.map(({ key, messageStem }) => (
                    <EventToggle
                      key={key}
                      label={t(`overlayEditor.eventSettings.${messageStem}Label`)}
                      description={t(`overlayEditor.eventSettings.${messageStem}Description`)}
                      value={settings[key] as boolean}
                      onChange={(v) => update(key, v)}
                    />
                  ))}
                  <p className="mt-4 border-t border-border pt-4 text-xs text-text-sub">
                    {t('overlayEditor.eventSettings.kickCaveat')}
                  </p>
                </div>
              )}

              {/* TikTok tab */}
              {activeTab === 'tiktok' && (
                <div>
                  {EVENT_TOGGLES.tiktok.map(({ key, messageStem }) => (
                    <EventToggle
                      key={key}
                      label={t(`overlayEditor.eventSettings.${messageStem}Label`)}
                      description={t(`overlayEditor.eventSettings.${messageStem}Description`)}
                      value={settings[key] as boolean}
                      onChange={(v) => update(key, v)}
                    />
                  ))}
                  <p className="mt-6 mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    {t('overlayEditor.eventSettings.advancedHeading')}
                  </p>
                  <NumberInput
                    label={t('overlayEditor.eventSettings.likeWindowLabel')}
                    description={t('overlayEditor.eventSettings.likeWindowDescription')}
                    value={settings.tiktok_like_aggregation_window_seconds}
                    min={10}
                    max={60}
                    onChange={(v) => update('tiktok_like_aggregation_window_seconds', v)}
                  />
                </div>
              )}

              {/* Actions */}
              <div className="mt-6 flex gap-3 border-t border-border pt-6">
                <Button onClick={handleSave} disabled={saving}>
                  {saving
                    ? t('overlayEditor.eventSettings.saving')
                    : t('overlayEditor.eventSettings.save')}
                </Button>
                <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
                  {t('overlayEditor.eventSettings.cancel')}
                </Button>
              </div>
            </div>
          </Card>
        )}
      </div>
    </div>
  )
}
