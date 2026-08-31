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
 * Credit Roll Configuration Page
 *
 * Configure end-of-stream credit roll settings for an overlay.
 *
 * Features:
 * - Enable/disable credit roll
 * - Select which event types to include (subs, bits, raids, etc.)
 * - Configure leaderboard settings
 * - Customize display settings (scroll speed, duration, theme)
 * - Clips integration settings
 *
 * Client Component for form interactions and API calls.
 */

'use client'

import { use, useEffect, useId, useState } from 'react'
import { useRouter } from 'next/navigation'
import clsx from 'clsx'
import dynamic from 'next/dynamic'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import { trackEvent } from '@/lib/analytics'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Overlay, CreditRollConfig } from '@/lib/types/overlay'
import { getTranslations, useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// Read through getTranslations() rather than the hook: dynamic()'s `loading`
// callback is not a component, so it cannot call one.
const EDITOR_LOADING_LABEL = getTranslations()('overlayEditor.credits.loadingEditor')

const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div
      className={
        // eslint-disable-next-line tailwindcss/no-unnecessary-arbitrary-value -- px is intentional here: this is a fixed-size loading placeholder, matched to the Monaco editor it is replaced by, sized to the layout it sits in rather than to the reading text, so it must not grow with the root font size the way the suggested rem-relative utility would
        'flex h-[400px] items-center justify-center rounded-lg border border-border bg-bg'
      }
    >
      <div className="text-sm text-text-dim">{EDITOR_LOADING_LABEL}</div>
    </div>
  ),
})

// Which event kinds can feed the credit-roll leaderboards. The config key and
// the icon live here; the labels are in the catalog under
// `overlayEditor.credits.event<Stem>`. `as const satisfies` keeps each stem a
// string literal so a typo fails tsc at the lookup rather than at runtime.
const CREDIT_EVENT_TYPES = [
  { key: 'include_subs', messageStem: 'Subs', icon: '⭐' },
  { key: 'include_resubs', messageStem: 'Resubs', icon: '🔄' },
  { key: 'include_gift_subs', messageStem: 'GiftSubs', icon: '🎁' },
  { key: 'include_bits', messageStem: 'Bits', icon: '💎' },
  { key: 'include_raids', messageStem: 'Raids', icon: '⚔️' },
  { key: 'include_super_chats', messageStem: 'SuperChats', icon: '💰' },
  { key: 'include_memberships', messageStem: 'Memberships', icon: '👑' },
  { key: 'include_follows', messageStem: 'Follows', icon: '❤️' },
] as const satisfies ReadonlyArray<{
  key: keyof CreditRollConfig
  messageStem: string
  icon: string
}>

const ThemeMarketplaceModal = dynamic(
  () => import('@/components/theme-marketplace/ThemeMarketplaceModal'),
  { ssr: false }
)

export default function CreditRollConfigPage({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
  const { id } = use(params)
  const router = useRouter()
  const { user } = useAuthStore()
  const fieldId = useId()

  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [config, setConfig] = useState<Partial<CreditRollConfig>>({
    enabled: true,
    include_subs: true,
    include_resubs: true,
    include_gift_subs: true,
    include_bits: true,
    include_raids: true,
    include_channel_points: false,
    include_super_chats: true,
    include_memberships: true,
    include_follows: true,
    leaderboard_top_n: 10,
    leaderboard_sort_by: 'value',
    scroll_speed: 50,
    display_duration_seconds: 60,
    background_opacity: 0.8,
    theme: 'cinematic',
    clips_enabled: false,
    clips_max_count: 5,
    clips_fallback_days: 7,
    clips_muted: true,
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [notification, setNotification] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)
  const [customCss, setCustomCss] = useState('')
  const [useCustomCss, setUseCustomCss] = useState(false)
  const [showThemeMarketplace, setShowThemeMarketplace] = useState(false)
  const [copiedCreditsUrl, setCopiedCreditsUrl] = useState(false)

  useEffect(() => {
    if (!user) {
      router.push('/')
      return
    }

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(id)
        setOverlay(overlayData)

        try {
          const configData = await overlaysApi.getCreditRollConfig(id)
          setConfig(configData)
          const css = configData.custom_css || ''
          setCustomCss(css)
          setUseCustomCss(Boolean(css.trim().length))
        } catch (error) {
          console.warn('Credit roll config not found, using defaults')
        }
      } catch (error) {
        console.error('Failed to load overlay:', error)
        setNotification({
          type: 'error',
          message: 'Failed to load overlay data',
        })
      } finally {
        setLoading(false)
      }
    }

    loadData()
  }, [id, user, router])

  const handleSave = async () => {
    setSaving(true)
    try {
      await overlaysApi.updateCreditRollConfig(id, {
        ...config,
        custom_css: useCustomCss ? customCss : '',
      })
      setNotification({
        type: 'success',
        message: 'Credit roll settings saved successfully!',
      })
      setTimeout(() => setNotification(null), 5000)
    } catch (error) {
      console.error('Failed to save config:', error)
      setNotification({
        type: 'error',
        message: 'Failed to save settings. Please try again.',
      })
      setTimeout(() => setNotification(null), 5000)
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex items-center justify-center pt-32">
          <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-twitch"></div>
        </div>
      </div>
    )
  }

  if (!overlay) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex items-center justify-center pt-32">
          <div className="text-center">
            <p className="text-lg text-youtube">{t('overlayEditor.credits.notFound')}</p>
            <a href="/dashboard" className="mt-4 inline-block text-twitch hover:underline">
              {t('overlayEditor.credits.returnToDashboard')}
            </a>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />

      {/* Notification */}
      {notification && (
        <div className="animate-slide-in fixed top-4 right-4 z-50">
          <div
            className={clsx(
              'rounded-lg border p-4 shadow-lg',
              notification.type === 'success'
                ? 'border-kick/30 bg-kick/10 text-kick'
                : 'border-youtube/30 bg-youtube/10 text-youtube'
            )}
          >
            <div className="flex items-center gap-3">
              {notification.type === 'success' ? (
                <svg
                  className="h-5 w-5 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              ) : (
                <svg
                  className="h-5 w-5 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              )}
              <p className="font-medium">{notification.message}</p>
              <Button
                onClick={() => setNotification(null)}
                variant="ghost"
                size="icon-xs"
                className="ml-2"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <div className="container mx-auto max-w-4xl px-4 py-8">
        {/* Header */}
        <div className="mb-8">
          <a
            href={`/overlays/${id}`}
            className="mb-4 inline-flex items-center gap-2 text-text-sub transition-colors hover:text-text"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M10 19l-7-7m0 0l7-7m-7 7h18"
              />
            </svg>
            {t('overlayEditor.credits.backToOverlay')}
          </a>
          <h1 className="text-3xl font-bold text-text">{t('overlayEditor.credits.heading')}</h1>
          <p className="mt-2 text-text-sub">{t('overlayEditor.credits.intro')}</p>
          <div className="mt-4">
            <Button
              variant="outline"
              className="inline-flex items-center gap-2"
              onClick={() => {
                const url = `${window.location.origin}/overlay/${id}/credits`
                navigator.clipboard.writeText(url).then(() => {
                  trackEvent('obs_url_copied', { surface: 'credits' })
                  setCopiedCreditsUrl(true)
                  setTimeout(() => setCopiedCreditsUrl(false), 2000)
                })
              }}
            >
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                />
              </svg>
              {copiedCreditsUrl
                ? t('overlayEditor.credits.copiedObsUrl')
                : t('overlayEditor.credits.copyObsUrl')}
            </Button>
            <p className="mt-1 text-xs text-text-dim">{t('overlayEditor.credits.obsUrlHint')}</p>
          </div>
        </div>

        {/* Main Toggle */}
        <Card className="mb-6 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-semibold text-text">
                {t('overlayEditor.credits.enableHeading')}
              </h2>
              <p className="mt-1 text-sm text-text-sub">{t('overlayEditor.credits.enableHint')}</p>
            </div>
            <button
              onClick={() => setConfig({ ...config, enabled: !config.enabled })}
              className={clsx(
                'relative inline-flex h-8 w-14 items-center rounded-full transition-colors',
                config.enabled ? 'bg-kick' : 'bg-surface-2'
              )}
            >
              <span
                className={clsx(
                  'inline-block h-6 w-6 transform rounded-full bg-white transition-transform',
                  config.enabled ? 'translate-x-7' : 'translate-x-1'
                )}
              />
            </button>
          </div>
        </Card>

        {config.enabled && (
          <>
            {/* Event Types */}
            <Card className="mb-6 p-6">
              <h3 className="mb-4 text-lg font-semibold text-text">
                {t('overlayEditor.credits.eventTypesHeading')}
              </h3>
              <p className="mb-4 text-sm text-text-sub">
                {t('overlayEditor.credits.eventTypesHint')}
              </p>
              <div className="grid grid-cols-2 gap-4">
                {CREDIT_EVENT_TYPES.map(({ key, messageStem, icon }) => (
                  <label
                    key={key}
                    className="flex cursor-pointer items-center gap-3 rounded-lg border border-border bg-bg p-3 transition-colors hover:border-border-md"
                  >
                    <input
                      type="checkbox"
                      checked={config[key] as boolean}
                      onChange={(e) => setConfig({ ...config, [key]: e.target.checked })}
                      className="h-5 w-5 rounded border-border bg-surface-2 text-twitch accent-twitch focus-visible:ring-twitch"
                    />
                    <span className="text-2xl">{icon}</span>
                    <span className="font-medium text-text">
                      {t(`overlayEditor.credits.event${messageStem}`)}
                    </span>
                  </label>
                ))}
              </div>
            </Card>

            {/* Leaderboard Settings */}
            <Card className="mb-6 p-6">
              <h3 className="mb-4 text-lg font-semibold text-text">
                {t('overlayEditor.credits.leaderboardHeading')}
              </h3>
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor={`${fieldId}-top-n`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.topNLabel')}
                  </label>
                  <Input
                    id={`${fieldId}-top-n`}
                    type="number"
                    min="1"
                    max="50"
                    value={config.leaderboard_top_n || 10}
                    onChange={(e) =>
                      setConfig({ ...config, leaderboard_top_n: parseInt(e.target.value) })
                    }
                    aria-describedby={`${fieldId}-top-n-hint`}
                  />
                  <p id={`${fieldId}-top-n-hint`} className="mt-1 text-xs text-text-dim">
                    {t('overlayEditor.credits.topNHint')}
                  </p>
                </div>
                <div>
                  <label
                    htmlFor={`${fieldId}-sort-by`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.sortByLabel')}
                  </label>
                  <select
                    id={`${fieldId}-sort-by`}
                    value={config.leaderboard_sort_by || 'value'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        leaderboard_sort_by: e.target.value as 'value' | 'count',
                      })
                    }
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                  >
                    <option value="value">{t('overlayEditor.credits.sortByTotalValue')}</option>
                    <option value="count">{t('overlayEditor.credits.sortByCount')}</option>
                  </select>
                </div>
              </div>
            </Card>

            {/* Display Settings */}
            <Card className="mb-6 p-6">
              <h3 className="mb-4 text-lg font-semibold text-text">
                {t('overlayEditor.credits.displayHeading')}
              </h3>
              <div className="space-y-4">
                <div>
                  <label
                    htmlFor={`${fieldId}-theme`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.themeLabel')}
                  </label>
                  <select
                    id={`${fieldId}-theme`}
                    value={config.theme || 'cinematic'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        theme: e.target.value as 'classic' | 'cinematic' | 'modern',
                      })
                    }
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                  >
                    <option value="classic">{t('overlayEditor.credits.themeClassic')}</option>
                    <option value="cinematic">{t('overlayEditor.credits.themeCinematic')}</option>
                    <option value="modern">{t('overlayEditor.credits.themeModern')}</option>
                  </select>
                </div>
                <div>
                  <label
                    htmlFor={`${fieldId}-scroll-speed`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.scrollSpeedLabel')}
                  </label>
                  <input
                    id={`${fieldId}-scroll-speed`}
                    type="range"
                    min="1"
                    max="100"
                    value={config.scroll_speed || 50}
                    onChange={(e) =>
                      setConfig({ ...config, scroll_speed: parseInt(e.target.value) })
                    }
                    className="w-full accent-twitch"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    {t('overlayEditor.credits.currentValue', {
                      value: config.scroll_speed || 50,
                    })}
                  </p>
                </div>
                <div>
                  <label
                    htmlFor={`${fieldId}-duration`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.durationLabel')}
                  </label>
                  <Input
                    id={`${fieldId}-duration`}
                    type="number"
                    min="10"
                    max="300"
                    value={config.display_duration_seconds || 60}
                    onChange={(e) =>
                      setConfig({ ...config, display_duration_seconds: parseInt(e.target.value) })
                    }
                    aria-describedby={`${fieldId}-duration-hint`}
                  />
                  <p id={`${fieldId}-duration-hint`} className="mt-1 text-xs text-text-dim">
                    {t('overlayEditor.credits.durationHint')}
                  </p>
                </div>
                <div>
                  <label
                    htmlFor={`${fieldId}-opacity`}
                    className="mb-2 block text-sm font-medium text-text"
                  >
                    {t('overlayEditor.credits.opacityLabel')}
                  </label>
                  <input
                    id={`${fieldId}-opacity`}
                    type="range"
                    min="0"
                    max="1"
                    step="0.1"
                    value={config.background_opacity || 0.8}
                    onChange={(e) =>
                      setConfig({ ...config, background_opacity: parseFloat(e.target.value) })
                    }
                    className="w-full accent-twitch"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    {t('overlayEditor.credits.currentValue', {
                      value: config.background_opacity || 0.8,
                    })}
                  </p>
                </div>
              </div>
            </Card>

            {/* Clips Settings */}
            <Card className="mb-6 p-6">
              <div className="mb-4 flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-text">
                    {t('overlayEditor.credits.clipsHeading')}
                  </h3>
                  <p className="mt-1 text-sm text-text-sub">
                    {t('overlayEditor.credits.clipsHint')}
                  </p>
                </div>
                <button
                  onClick={() => setConfig({ ...config, clips_enabled: !config.clips_enabled })}
                  className={clsx(
                    'relative inline-flex h-8 w-14 items-center rounded-full transition-colors',
                    config.clips_enabled ? 'bg-kick' : 'bg-surface-2'
                  )}
                >
                  <span
                    className={clsx(
                      'inline-block h-6 w-6 transform rounded-full bg-white transition-transform',
                      config.clips_enabled ? 'translate-x-7' : 'translate-x-1'
                    )}
                  />
                </button>
              </div>

              {config.clips_enabled && (
                <div className="space-y-4">
                  <div>
                    <label
                      htmlFor={`${fieldId}-clips-max`}
                      className="mb-2 block text-sm font-medium text-text"
                    >
                      {t('overlayEditor.credits.maxClipsLabel')}
                    </label>
                    <Input
                      id={`${fieldId}-clips-max`}
                      type="number"
                      min="1"
                      max="20"
                      value={config.clips_max_count || 5}
                      onChange={(e) =>
                        setConfig({ ...config, clips_max_count: parseInt(e.target.value) })
                      }
                    />
                  </div>
                  <div>
                    <label
                      htmlFor={`${fieldId}-clips-fallback`}
                      className="mb-2 block text-sm font-medium text-text"
                    >
                      {t('overlayEditor.credits.fallbackDaysLabel')}
                    </label>
                    <Input
                      id={`${fieldId}-clips-fallback`}
                      type="number"
                      min="1"
                      max="30"
                      value={config.clips_fallback_days || 7}
                      onChange={(e) =>
                        setConfig({ ...config, clips_fallback_days: parseInt(e.target.value) })
                      }
                      aria-describedby={`${fieldId}-clips-fallback-hint`}
                    />
                    <p id={`${fieldId}-clips-fallback-hint`} className="mt-1 text-xs text-text-dim">
                      {t('overlayEditor.credits.fallbackDaysHint')}
                    </p>
                  </div>
                  <div>
                    <label className="flex cursor-pointer items-center gap-3 rounded-lg border border-border bg-bg p-3 transition-colors hover:border-border-md">
                      <input
                        type="checkbox"
                        checked={config.clips_muted ?? true}
                        onChange={(e) => setConfig({ ...config, clips_muted: e.target.checked })}
                        className="h-5 w-5 rounded border-border bg-surface-2 text-twitch accent-twitch focus-visible:ring-twitch"
                      />
                      <span className="flex-1 font-medium text-text">
                        {t('overlayEditor.credits.muteClipsLabel')}
                        <span className="mt-1 block text-xs font-normal text-text-dim">
                          {t('overlayEditor.credits.muteClipsHint')}
                        </span>
                      </span>
                    </label>
                  </div>
                </div>
              )}
            </Card>
          </>
        )}

        {/* CSS Customization Section */}
        <Card className="mb-6 p-6">
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h3 className="text-lg font-semibold text-text">
                {t('overlayEditor.credits.cssHeading')}
              </h3>
              <label className="flex items-center gap-2 text-sm text-text-sub">
                <input
                  type="checkbox"
                  checked={useCustomCss}
                  onChange={(e) => setUseCustomCss(e.target.checked)}
                  className="h-5 w-5 rounded border-border bg-surface-2 text-twitch accent-twitch focus-visible:ring-twitch"
                />
                {t('overlayEditor.credits.cssEnable')}
              </label>
            </div>
            <div className="flex gap-2">
              <Button variant="default" onClick={() => setShowThemeMarketplace(true)}>
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01"
                  />
                </svg>
                {t('overlayEditor.credits.cssBrowseThemes')}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setCustomCss('')
                  setUseCustomCss(false)
                }}
              >
                {t('overlayEditor.credits.cssReset')}
              </Button>
            </div>
          </div>

          <MonacoCSSEditor
            value={customCss}
            onChange={setCustomCss}
            height="400px"
            placeholder={t('overlayEditor.credits.cssEditorPlaceholder')}
          />

          <p className="mt-4 text-sm text-text-sub">
            {interpolateElements(t('overlayEditor.credits.cssHint'), {
              docsLink: (
                <a
                  href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/credit-roll-themes"
                  target="_blank"
                  rel="noreferrer"
                  className="text-twitch hover:underline"
                >
                  {t('overlayEditor.credits.cssDocsLink')}
                </a>
              ),
            })}
          </p>
        </Card>

        {/* Action Buttons */}
        <div className="flex gap-4">
          <Button className="flex-1" variant="gradient" onClick={handleSave} disabled={saving}>
            {saving ? t('overlayEditor.credits.saving') : t('overlayEditor.credits.save')}
          </Button>
          <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
            {t('overlayEditor.credits.cancel')}
          </Button>
        </div>
      </div>

      {/* Theme Marketplace Modal */}
      <ThemeMarketplaceModal
        isOpen={showThemeMarketplace}
        onClose={() => setShowThemeMarketplace(false)}
        onApplyTheme={(css) => {
          setCustomCss(css)
          setUseCustomCss(true)
          setShowThemeMarketplace(false)
        }}
        themeType="creditroll"
      />
    </div>
  )
}
