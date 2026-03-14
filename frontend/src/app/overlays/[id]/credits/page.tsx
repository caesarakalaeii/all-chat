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

import { use, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import clsx from 'clsx'
import dynamic from 'next/dynamic'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import type { Overlay, CreditRollConfig } from '@/lib/types/overlay'

const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="flex h-[400px] items-center justify-center rounded-lg border border-slate-700 bg-slate-900">
      <div className="text-sm text-slate-400">Loading editor...</div>
    </div>
  ),
})

const ThemeMarketplaceModal = dynamic(
  () => import('@/components/theme-marketplace/ThemeMarketplaceModal'),
  { ssr: false }
)

export default function CreditRollConfigPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const { token } = useAuthStore()

  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [config, setConfig] = useState<Partial<CreditRollConfig>>({
    enabled: false,
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

  useEffect(() => {
    if (!token) {
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
  }, [id, token, router])

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
      <div className="flex min-h-screen items-center justify-center bg-slate-900">
        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-twitch"></div>
      </div>
    )
  }

  if (!overlay) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-900">
        <div className="text-center">
          <p className="text-lg text-red-500">Overlay not found</p>
          <a href="/dashboard" className="mt-4 inline-block text-twitch hover:underline">
            Return to Dashboard
          </a>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-slate-900">
      {/* Notification */}
      {notification && (
        <div className="animate-slide-in fixed top-4 right-4 z-50">
          <div
            className={clsx(
              'rounded-lg border p-4 shadow-lg',
              notification.type === 'success'
                ? 'border-green-500/30 bg-green-500/20 text-green-400'
                : 'border-red-500/30 bg-red-500/20 text-red-400'
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
              <button
                onClick={() => setNotification(null)}
                className="ml-2 text-slate-400 hover:text-white"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
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
            className="mb-4 inline-flex items-center gap-2 text-slate-400 transition-colors hover:text-white"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M10 19l-7-7m0 0l7-7m-7 7h18"
              />
            </svg>
            Back to Overlay
          </a>
          <h1 className="text-3xl font-bold text-white">Credit Roll Settings</h1>
          <p className="mt-2 text-slate-400">
            Configure end-of-stream credits to showcase viewers who supported your stream with subs,
            donations, raids, and more.
          </p>
        </div>

        {/* Main Toggle */}
        <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-semibold text-white">Enable Credit Roll</h2>
              <p className="mt-1 text-sm text-slate-400">
                Show end-of-stream credits with leaderboards and highlights
              </p>
            </div>
            <button
              onClick={() => setConfig({ ...config, enabled: !config.enabled })}
              className={clsx(
                'relative inline-flex h-8 w-14 items-center rounded-full transition-colors',
                config.enabled ? 'bg-green-600' : 'bg-slate-600'
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
        </div>

        {config.enabled && (
          <>
            {/* Event Types */}
            <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
              <h3 className="mb-4 text-lg font-semibold text-white">Event Types to Include</h3>
              <p className="mb-4 text-sm text-slate-400">
                Select which types of events should appear in the credit roll leaderboards
              </p>
              <div className="grid grid-cols-2 gap-4">
                {[
                  { key: 'include_subs', label: 'Subscriptions', icon: '⭐' },
                  { key: 'include_resubs', label: 'Resubscriptions', icon: '🔄' },
                  { key: 'include_gift_subs', label: 'Gift Subs', icon: '🎁' },
                  { key: 'include_bits', label: 'Bits/Cheers', icon: '💎' },
                  { key: 'include_raids', label: 'Raids', icon: '⚔️' },
                  { key: 'include_super_chats', label: 'Super Chats', icon: '💰' },
                  { key: 'include_memberships', label: 'Memberships', icon: '👑' },
                  { key: 'include_follows', label: 'Follows', icon: '❤️' },
                ].map(({ key, label, icon }) => (
                  <label
                    key={key}
                    className="bg-slate-750 flex cursor-pointer items-center gap-3 rounded-lg border border-slate-600 p-3 transition-colors hover:border-slate-500"
                  >
                    <input
                      type="checkbox"
                      checked={config[key as keyof typeof config] as boolean}
                      onChange={(e) => setConfig({ ...config, [key]: e.target.checked })}
                      className="h-5 w-5 rounded border-slate-600 bg-slate-700 text-twitch focus-visible:ring-twitch focus-visible:ring-offset-slate-800"
                    />
                    <span className="text-2xl">{icon}</span>
                    <span className="font-medium text-white">{label}</span>
                  </label>
                ))}
              </div>
            </div>

            {/* Leaderboard Settings */}
            <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
              <h3 className="mb-4 text-lg font-semibold text-white">Leaderboard Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">
                    Top N Users per Category
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="50"
                    value={config.leaderboard_top_n || 10}
                    onChange={(e) =>
                      setConfig({ ...config, leaderboard_top_n: parseInt(e.target.value) })
                    }
                    className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                  />
                  <p className="mt-1 text-xs text-slate-400">
                    Show top 1-50 users in each leaderboard category
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">Sort By</label>
                  <select
                    value={config.leaderboard_sort_by || 'value'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        leaderboard_sort_by: e.target.value as 'value' | 'count',
                      })
                    }
                    className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                  >
                    <option value="value">Total Value (monetary amount)</option>
                    <option value="count">Count (number of events)</option>
                  </select>
                </div>
              </div>
            </div>

            {/* Display Settings */}
            <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
              <h3 className="mb-4 text-lg font-semibold text-white">Display Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">Theme</label>
                  <select
                    value={config.theme || 'cinematic'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        theme: e.target.value as 'classic' | 'cinematic' | 'modern',
                      })
                    }
                    className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                  >
                    <option value="classic">Classic</option>
                    <option value="cinematic">Cinematic</option>
                    <option value="modern">Modern</option>
                  </select>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">
                    Scroll Speed (1-100)
                  </label>
                  <input
                    type="range"
                    min="1"
                    max="100"
                    value={config.scroll_speed || 50}
                    onChange={(e) =>
                      setConfig({ ...config, scroll_speed: parseInt(e.target.value) })
                    }
                    className="w-full"
                  />
                  <p className="mt-1 text-xs text-slate-400">
                    Current: {config.scroll_speed || 50}
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">
                    Display Duration (seconds)
                  </label>
                  <input
                    type="number"
                    min="10"
                    max="300"
                    value={config.display_duration_seconds || 60}
                    onChange={(e) =>
                      setConfig({ ...config, display_duration_seconds: parseInt(e.target.value) })
                    }
                    className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                  />
                  <p className="mt-1 text-xs text-slate-400">
                    How long to show the credit roll (10-300 seconds)
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-white">
                    Background Opacity (0-1)
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.1"
                    value={config.background_opacity || 0.8}
                    onChange={(e) =>
                      setConfig({ ...config, background_opacity: parseFloat(e.target.value) })
                    }
                    className="w-full"
                  />
                  <p className="mt-1 text-xs text-slate-400">
                    Current: {config.background_opacity || 0.8}
                  </p>
                </div>
              </div>
            </div>

            {/* Clips Settings */}
            <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
              <div className="mb-4 flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-white">Twitch Clips</h3>
                  <p className="mt-1 text-sm text-slate-400">Show clips during credit roll</p>
                </div>
                <button
                  onClick={() => setConfig({ ...config, clips_enabled: !config.clips_enabled })}
                  className={clsx(
                    'relative inline-flex h-8 w-14 items-center rounded-full transition-colors',
                    config.clips_enabled ? 'bg-green-600' : 'bg-slate-600'
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
                    <label className="mb-2 block text-sm font-medium text-white">
                      Maximum Clips
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="20"
                      value={config.clips_max_count || 5}
                      onChange={(e) =>
                        setConfig({ ...config, clips_max_count: parseInt(e.target.value) })
                      }
                      className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                    />
                  </div>
                  <div>
                    <label className="mb-2 block text-sm font-medium text-white">
                      Fallback Days
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={config.clips_fallback_days || 7}
                      onChange={(e) =>
                        setConfig({ ...config, clips_fallback_days: parseInt(e.target.value) })
                      }
                      className="w-full rounded-lg border border-slate-600 bg-slate-700 px-4 py-2 text-white focus-visible:border-twitch focus-visible:outline-none"
                    />
                    <p className="mt-1 text-xs text-slate-400">
                      If no clips from this stream, show clips from last N days
                    </p>
                  </div>
                  <div>
                    <label className="bg-slate-750 flex cursor-pointer items-center gap-3 rounded-lg border border-slate-600 p-3 transition-colors hover:border-slate-500">
                      <input
                        type="checkbox"
                        checked={config.clips_muted ?? true}
                        onChange={(e) => setConfig({ ...config, clips_muted: e.target.checked })}
                        className="h-5 w-5 rounded border-slate-600 bg-slate-700 text-twitch focus-visible:ring-twitch focus-visible:ring-offset-slate-800"
                      />
                      <div className="flex-1">
                        <span className="font-medium text-white">Mute Clips Audio</span>
                        <p className="mt-1 text-xs text-slate-400">
                          Required for browser autoplay. Unmuting may require viewer interaction.
                        </p>
                      </div>
                    </label>
                  </div>
                </div>
              )}
            </div>
          </>
        )}

        {/* CSS Customization Section */}
        <div className="mb-6 rounded-lg border border-slate-700 bg-slate-800 p-6">
          <div className="mb-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <h3 className="text-lg font-semibold text-white">Custom CSS Editor</h3>
              <label className="flex items-center gap-2 text-sm text-slate-300">
                <input
                  type="checkbox"
                  checked={useCustomCss}
                  onChange={(e) => setUseCustomCss(e.target.checked)}
                  className="h-5 w-5 rounded border-slate-600 bg-slate-700 text-twitch focus-visible:ring-twitch focus-visible:ring-offset-slate-800"
                />
                Enable Custom CSS
              </label>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setShowThemeMarketplace(true)}
                className="flex items-center gap-2 rounded-lg bg-purple-600 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-purple-700"
              >
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01"
                  />
                </svg>
                Browse Themes
              </button>
              <button
                type="button"
                onClick={() => {
                  setCustomCss('')
                  setUseCustomCss(false)
                }}
                className="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-200 transition-colors hover:bg-slate-600"
              >
                Reset
              </button>
            </div>
          </div>

          <MonacoCSSEditor
            value={customCss}
            onChange={setCustomCss}
            height="400px"
            placeholder="/* Enter your custom CSS for credit roll */"
          />

          <p className="mt-4 text-sm text-slate-400">
            Customize your credit roll appearance with CSS. Browse themes or write your own styles.
            See{' '}
            <a
              href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/credit-roll-themes"
              target="_blank"
              rel="noreferrer"
              className="text-twitch hover:underline"
            >
              credit roll theme docs
            </a>{' '}
            for examples and CSS selectors.
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex gap-4">
          <button
            onClick={handleSave}
            disabled={saving}
            className="flex-1 rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-colors hover:bg-purple-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save Settings'}
          </button>
          <button
            onClick={() => router.push(`/overlays/${id}`)}
            className="rounded-lg bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600"
          >
            Cancel
          </button>
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
