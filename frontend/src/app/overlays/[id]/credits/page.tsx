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
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import type { Overlay, CreditRollConfig } from '@/lib/types/overlay'

const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="flex h-[400px] items-center justify-center rounded-lg border border-border bg-bg">
      <div className="text-sm text-text-dim">Loading editor...</div>
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
            <p className="text-lg text-youtube">Overlay not found</p>
            <a href="/dashboard" className="mt-4 inline-block text-twitch hover:underline">
              Return to Dashboard
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
              <button
                onClick={() => setNotification(null)}
                className="ml-2 text-text-dim transition-colors hover:text-text"
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
            Back to Overlay
          </a>
          <h1 className="text-3xl font-bold text-text">Credit Roll Settings</h1>
          <p className="mt-2 text-text-sub">
            Configure end-of-stream credits to showcase viewers who supported your stream with subs,
            donations, raids, and more.
          </p>
          <div className="mt-4">
            <Button
              variant="outline"
              className="inline-flex items-center gap-2"
              onClick={() => {
                const url = `${window.location.origin}/overlay/${id}/credits`
                navigator.clipboard.writeText(url).then(() => {
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
              {copiedCreditsUrl ? 'Copied!' : 'Copy Credits OBS URL'}
            </Button>
            <p className="mt-1 text-xs text-text-dim">
              Add this URL as a Browser Source in OBS to display credits at end of stream
            </p>
          </div>
        </div>

        {/* Main Toggle */}
        <Card className="mb-6 p-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-semibold text-text">Enable Credit Roll</h2>
              <p className="mt-1 text-sm text-text-sub">
                Show end-of-stream credits with leaderboards and highlights
              </p>
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
              <h3 className="mb-4 text-lg font-semibold text-text">Event Types to Include</h3>
              <p className="mb-4 text-sm text-text-sub">
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
                    className="flex cursor-pointer items-center gap-3 rounded-lg border border-border bg-bg p-3 transition-colors hover:border-border-md"
                  >
                    <input
                      type="checkbox"
                      checked={config[key as keyof typeof config] as boolean}
                      onChange={(e) => setConfig({ ...config, [key]: e.target.checked })}
                      className="h-5 w-5 rounded border-border bg-surface-2 text-twitch accent-twitch focus-visible:ring-twitch"
                    />
                    <span className="text-2xl">{icon}</span>
                    <span className="font-medium text-text">{label}</span>
                  </label>
                ))}
              </div>
            </Card>

            {/* Leaderboard Settings */}
            <Card className="mb-6 p-6">
              <h3 className="mb-4 text-lg font-semibold text-text">Leaderboard Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">
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
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    Show top 1-50 users in each leaderboard category
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">Sort By</label>
                  <select
                    value={config.leaderboard_sort_by || 'value'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        leaderboard_sort_by: e.target.value as 'value' | 'count',
                      })
                    }
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                  >
                    <option value="value">Total Value (monetary amount)</option>
                    <option value="count">Count (number of events)</option>
                  </select>
                </div>
              </div>
            </Card>

            {/* Display Settings */}
            <Card className="mb-6 p-6">
              <h3 className="mb-4 text-lg font-semibold text-text">Display Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">Theme</label>
                  <select
                    value={config.theme || 'cinematic'}
                    onChange={(e) =>
                      setConfig({
                        ...config,
                        theme: e.target.value as 'classic' | 'cinematic' | 'modern',
                      })
                    }
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                  >
                    <option value="classic">Classic</option>
                    <option value="cinematic">Cinematic</option>
                    <option value="modern">Modern</option>
                  </select>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">
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
                    className="w-full accent-twitch"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    Current: {config.scroll_speed || 50}
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">
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
                    className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    How long to show the credit roll (10-300 seconds)
                  </p>
                </div>
                <div>
                  <label className="mb-2 block text-sm font-medium text-text">
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
                    className="w-full accent-twitch"
                  />
                  <p className="mt-1 text-xs text-text-dim">
                    Current: {config.background_opacity || 0.8}
                  </p>
                </div>
              </div>
            </Card>

            {/* Clips Settings */}
            <Card className="mb-6 p-6">
              <div className="mb-4 flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-text">Twitch Clips</h3>
                  <p className="mt-1 text-sm text-text-sub">Show clips during credit roll</p>
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
                    <label className="mb-2 block text-sm font-medium text-text">
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
                      className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                    />
                  </div>
                  <div>
                    <label className="mb-2 block text-sm font-medium text-text">
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
                      className="w-full rounded-lg border border-border bg-bg px-4 py-2 text-text focus-visible:border-twitch focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-twitch/50"
                    />
                    <p className="mt-1 text-xs text-text-dim">
                      If no clips from this stream, show clips from last N days
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
                      <div className="flex-1">
                        <span className="font-medium text-text">Mute Clips Audio</span>
                        <p className="mt-1 text-xs text-text-dim">
                          Required for browser autoplay. Unmuting may require viewer interaction.
                        </p>
                      </div>
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
              <h3 className="text-lg font-semibold text-text">Custom CSS Editor</h3>
              <label className="flex items-center gap-2 text-sm text-text-sub">
                <input
                  type="checkbox"
                  checked={useCustomCss}
                  onChange={(e) => setUseCustomCss(e.target.checked)}
                  className="h-5 w-5 rounded border-border bg-surface-2 text-twitch accent-twitch focus-visible:ring-twitch"
                />
                Enable Custom CSS
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
                Browse Themes
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setCustomCss('')
                  setUseCustomCss(false)
                }}
              >
                Reset
              </Button>
            </div>
          </div>

          <MonacoCSSEditor
            value={customCss}
            onChange={setCustomCss}
            height="400px"
            placeholder="/* Enter your custom CSS for credit roll */"
          />

          <p className="mt-4 text-sm text-text-sub">
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
        </Card>

        {/* Action Buttons */}
        <div className="flex gap-4">
          <Button
            className="flex-1"
            variant="gradient"
            onClick={handleSave}
            disabled={saving}
          >
            {saving ? 'Saving...' : 'Save Settings'}
          </Button>
          <Button variant="outline" onClick={() => router.push(`/overlays/${id}`)}>
            Cancel
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
