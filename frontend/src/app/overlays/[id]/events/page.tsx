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

import { use, useEffect, useState } from 'react'
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
  // System
  enable_token_warnings: boolean
  // Settings
  tiktok_like_aggregation_window_seconds: number
  event_display_duration_multiplier: number
}

type Tab = 'global' | 'twitch' | 'youtube' | 'kick' | 'tiktok'

const TABS: { id: Tab; label: string }[] = [
  { id: 'global', label: 'Global' },
  { id: 'twitch', label: 'Twitch' },
  { id: 'youtube', label: 'YouTube' },
  { id: 'kick', label: 'Kick' },
  { id: 'tiktok', label: 'TikTok' },
]

const TAB_COLOR: Record<Tab, string> = {
  global: 'text-text',
  twitch: PLATFORM_COLORS.twitch.text,
  youtube: PLATFORM_COLORS.youtube.text,
  kick: PLATFORM_COLORS.kick.text,
  tiktok: PLATFORM_COLORS.tiktok.text,
}

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
  return (
    <div className="flex items-center justify-between border-b border-border py-3.5 last:border-0">
      <div className="flex-1 pr-4">
        <p className="text-sm font-medium text-text">{label}</p>
        <p className="mt-0.5 text-xs text-text-sub">{description}</p>
      </div>
      <label className="relative inline-flex shrink-0 cursor-pointer items-center">
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
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
      .catch(() => toastManager.add({ title: 'Failed to load event settings', type: 'error' }))
      .finally(() => setLoading(false))
  }, [id, user, router])

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
            Back to Overlay
          </button>
          <h1 className="text-2xl font-bold text-text">Event Display Settings</h1>
          <p className="mt-1 text-sm text-text-sub">
            Control which platform events appear on your overlay.
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
            <p className="text-destructive mb-4">Failed to load event settings</p>
            <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
              Back to Overlay
            </Button>
          </Card>
        ) : (
          <Card className="overflow-hidden">
            {/* Platform tabs */}
            <div className="flex overflow-x-auto border-b border-border">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={cn(
                    'shrink-0 border-b-2 px-4 py-3 text-sm font-medium transition-colors focus-visible:outline-none',
                    activeTab === tab.id
                      ? cn(TAB_COLOR[tab.id], 'border-current')
                      : 'border-transparent text-text-sub hover:text-text'
                  )}
                >
                  {tab.label}
                </button>
              ))}
            </div>

            <div className="p-6">
              {/* Global tab */}
              {activeTab === 'global' && (
                <div>
                  <p className="mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    System Events
                  </p>
                  <EventToggle
                    label="Token Warnings"
                    description="Display OAuth authentication errors on overlay (requires token-refresh-service)"
                    value={settings.enable_token_warnings}
                    onChange={(v) => update('enable_token_warnings', v)}
                  />
                  <p className="mt-6 mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    Display Settings
                  </p>
                  <NumberInput
                    label="Event Duration Multiplier"
                    description="Multiply all event display durations (0.5 = half time, 2.0 = double time)"
                    value={settings.event_display_duration_multiplier}
                    min={0.1}
                    max={5}
                    step={0.1}
                    onChange={(v) => update('event_display_duration_multiplier', v)}
                  />
                  <div className="mt-6 space-y-1.5 rounded-lg border border-border bg-surface-2 p-4 text-xs text-text-sub">
                    <p className="mb-2 font-semibold text-text">About event tiers</p>
                    <p>
                      • <strong className="text-text">High-value</strong> — subs, large donations,
                      raids: 30+ seconds
                    </p>
                    <p>
                      • <strong className="text-text">Medium-value</strong> — follows, small gifts:
                      15 seconds
                    </p>
                    <p>
                      • <strong className="text-text">Low-value</strong> — likes, shares: 5–10
                      seconds
                    </p>
                    <p>
                      • Style with CSS classes:{' '}
                      <code className="rounded bg-surface px-1">.event-tier-high</code>,{' '}
                      <code className="rounded bg-surface px-1">.event-type-raid</code>
                    </p>
                  </div>
                </div>
              )}

              {/* Twitch tab */}
              {activeTab === 'twitch' && (
                <div>
                  {[
                    {
                      key: 'enable_twitch_subs',
                      label: 'Subscriptions',
                      desc: 'New subscriptions and resubscriptions',
                    },
                    {
                      key: 'enable_twitch_resubs',
                      label: 'Resubscriptions',
                      desc: 'Monthly resubscription notices with streak information',
                    },
                    {
                      key: 'enable_twitch_gift_subs',
                      label: 'Gift Subscriptions',
                      desc: 'Gift subs and mystery gift bombs',
                    },
                    {
                      key: 'enable_twitch_bits',
                      label: 'Bits / Cheers',
                      desc: 'Bits cheered in chat',
                    },
                    {
                      key: 'enable_twitch_raids',
                      label: 'Raids',
                      desc: 'Incoming raids from other channels',
                    },
                    {
                      key: 'enable_twitch_channel_points',
                      label: 'Channel Points',
                      desc: 'Channel point reward redemptions (requires EventSub service)',
                    },
                    {
                      key: 'enable_twitch_follows',
                      label: 'Follows',
                      desc: 'New channel followers (requires EventSub service)',
                    },
                  ].map(({ key, label, desc }) => (
                    <EventToggle
                      key={key}
                      label={label}
                      description={desc}
                      value={settings[key as keyof EventSettings] as boolean}
                      onChange={(v) => update(key as keyof EventSettings, v)}
                    />
                  ))}
                </div>
              )}

              {/* YouTube tab */}
              {activeTab === 'youtube' && (
                <div>
                  {[
                    {
                      key: 'enable_youtube_super_chat',
                      label: 'Super Chat',
                      desc: 'Paid Super Chat messages',
                    },
                    {
                      key: 'enable_youtube_super_sticker',
                      label: 'Super Stickers',
                      desc: 'Paid Super Sticker purchases',
                    },
                    {
                      key: 'enable_youtube_members',
                      label: 'New Members',
                      desc: 'New channel memberships',
                    },
                    {
                      key: 'enable_youtube_member_milestones',
                      label: 'Member Milestones',
                      desc: 'Membership anniversary celebrations',
                    },
                    {
                      key: 'enable_youtube_member_gifts',
                      label: 'Membership Gifts',
                      desc: 'Gifted memberships',
                    },
                  ].map(({ key, label, desc }) => (
                    <EventToggle
                      key={key}
                      label={label}
                      description={desc}
                      value={settings[key as keyof EventSettings] as boolean}
                      onChange={(v) => update(key as keyof EventSettings, v)}
                    />
                  ))}
                </div>
              )}

              {/* Kick tab */}
              {activeTab === 'kick' && (
                <div>
                  {[
                    {
                      key: 'enable_kick_subs',
                      label: 'Subscriptions',
                      desc: 'Kick channel subscriptions',
                    },
                    {
                      key: 'enable_kick_gifts',
                      label: 'Gifts & Donations',
                      desc: 'Gift subscriptions and donations',
                    },
                  ].map(({ key, label, desc }) => (
                    <EventToggle
                      key={key}
                      label={label}
                      description={desc}
                      value={settings[key as keyof EventSettings] as boolean}
                      onChange={(v) => update(key as keyof EventSettings, v)}
                    />
                  ))}
                  <p className="mt-4 border-t border-border pt-4 text-xs text-text-sub">
                    ⚠️ Kick events require reverse-engineering and may not be available yet.
                  </p>
                </div>
              )}

              {/* TikTok tab */}
              {activeTab === 'tiktok' && (
                <div>
                  {[
                    {
                      key: 'enable_tiktok_likes',
                      label: 'Likes',
                      desc: 'Likes sent during stream (aggregated)',
                    },
                    {
                      key: 'enable_tiktok_gifts',
                      label: 'Gifts',
                      desc: 'Virtual gifts sent with diamond values',
                    },
                    {
                      key: 'enable_tiktok_follows',
                      label: 'Follows',
                      desc: 'New followers during stream',
                    },
                    {
                      key: 'enable_tiktok_shares',
                      label: 'Shares',
                      desc: 'Stream shares to other platforms',
                    },
                  ].map(({ key, label, desc }) => (
                    <EventToggle
                      key={key}
                      label={label}
                      description={desc}
                      value={settings[key as keyof EventSettings] as boolean}
                      onChange={(v) => update(key as keyof EventSettings, v)}
                    />
                  ))}
                  <p className="mt-6 mb-3 text-xs font-semibold tracking-wide text-text-sub uppercase">
                    Advanced
                  </p>
                  <NumberInput
                    label="Like Aggregation Window (seconds)"
                    description="Likes are collected in this window to prevent spam"
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
                  {saving ? 'Saving…' : 'Save Settings'}
                </Button>
                <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
                  Cancel
                </Button>
              </div>
            </div>
          </Card>
        )}
      </div>
    </div>
  )
}
