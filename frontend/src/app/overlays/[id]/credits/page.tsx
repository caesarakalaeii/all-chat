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

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import dynamic from 'next/dynamic';
import { useAuthStore } from '@/lib/stores/auth-store';
import { overlaysApi } from '@/lib/api/overlays';
import type { Overlay, CreditRollConfig } from '@/lib/types/overlay';

const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="h-[400px] bg-gray-900 border border-gray-700 rounded-lg flex items-center justify-center">
      <div className="text-gray-400 text-sm">Loading editor...</div>
    </div>
  )
});

const ThemeMarketplaceModal = dynamic(
  () => import('@/components/theme-marketplace/ThemeMarketplaceModal'),
  { ssr: false }
);

export default function CreditRollConfigPage({ params }: { params: { id: string } }) {
  const router = useRouter();
  const { token } = useAuthStore();

  const [overlay, setOverlay] = useState<Overlay | null>(null);
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
  });
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [customCss, setCustomCss] = useState('');
  const [useCustomCss, setUseCustomCss] = useState(false);
  const [showThemeMarketplace, setShowThemeMarketplace] = useState(false);

  useEffect(() => {
    if (!token) {
      router.push('/');
      return;
    }

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(params.id);
        setOverlay(overlayData);

        try {
          const configData = await overlaysApi.getCreditRollConfig(params.id);
          setConfig(configData);
          const css = configData.custom_css || '';
          setCustomCss(css);
          setUseCustomCss(Boolean(css.trim().length));
        } catch (error) {
          console.warn('Credit roll config not found, using defaults');
        }
      } catch (error) {
        console.error('Failed to load overlay:', error);
        setNotification({
          type: 'error',
          message: 'Failed to load overlay data'
        });
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [params.id, token, router]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await overlaysApi.updateCreditRollConfig(params.id, {
        ...config,
        custom_css: useCustomCss ? customCss : ''
      });
      setNotification({
        type: 'success',
        message: 'Credit roll settings saved successfully!'
      });
      setTimeout(() => setNotification(null), 5000);
    } catch (error) {
      console.error('Failed to save config:', error);
      setNotification({
        type: 'error',
        message: 'Failed to save settings. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch"></div>
      </div>
    );
  }

  if (!overlay) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="text-center">
          <p className="text-red-500 text-lg">Overlay not found</p>
          <a href="/dashboard" className="text-twitch hover:underline mt-4 inline-block">
            Return to Dashboard
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Notification */}
      {notification && (
        <div className="fixed top-4 right-4 z-50 animate-slide-in">
          <div className={`rounded-lg p-4 shadow-lg border ${
            notification.type === 'success'
              ? 'bg-green-500/20 border-green-500/30 text-green-400'
              : 'bg-red-500/20 border-red-500/30 text-red-400'
          }`}>
            <div className="flex items-center gap-3">
              {notification.type === 'success' ? (
                <svg className="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                <svg className="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              )}
              <p className="font-medium">{notification.message}</p>
              <button
                onClick={() => setNotification(null)}
                className="ml-2 text-gray-400 hover:text-white"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        {/* Header */}
        <div className="mb-8">
          <a
            href={`/overlays/${params.id}`}
            className="text-gray-400 hover:text-white transition-colors inline-flex items-center gap-2 mb-4"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
            </svg>
            Back to Overlay
          </a>
          <h1 className="text-3xl font-bold text-white">Credit Roll Settings</h1>
          <p className="text-gray-400 mt-2">
            Configure end-of-stream credits to showcase viewers who supported your stream with subs, donations, raids, and more.
          </p>
        </div>

        {/* Main Toggle */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-xl font-semibold text-white">Enable Credit Roll</h2>
              <p className="text-sm text-gray-400 mt-1">
                Show end-of-stream credits with leaderboards and highlights
              </p>
            </div>
            <button
              onClick={() => setConfig({ ...config, enabled: !config.enabled })}
              className={`relative inline-flex h-8 w-14 items-center rounded-full transition-colors ${
                config.enabled ? 'bg-green-600' : 'bg-gray-600'
              }`}
            >
              <span
                className={`inline-block h-6 w-6 transform rounded-full bg-white transition-transform ${
                  config.enabled ? 'translate-x-7' : 'translate-x-1'
                }`}
              />
            </button>
          </div>
        </div>

        {config.enabled && (
          <>
            {/* Event Types */}
            <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
              <h3 className="text-lg font-semibold text-white mb-4">Event Types to Include</h3>
              <p className="text-sm text-gray-400 mb-4">
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
                  <label key={key} className="flex items-center gap-3 p-3 bg-gray-750 rounded-lg border border-gray-600 cursor-pointer hover:border-gray-500 transition-colors">
                    <input
                      type="checkbox"
                      checked={config[key as keyof typeof config] as boolean}
                      onChange={(e) => setConfig({ ...config, [key]: e.target.checked })}
                      className="w-5 h-5 rounded border-gray-600 bg-gray-700 text-twitch focus:ring-twitch focus:ring-offset-gray-800"
                    />
                    <span className="text-2xl">{icon}</span>
                    <span className="text-white font-medium">{label}</span>
                  </label>
                ))}
              </div>
            </div>

            {/* Leaderboard Settings */}
            <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
              <h3 className="text-lg font-semibold text-white mb-4">Leaderboard Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Top N Users per Category
                  </label>
                  <input
                    type="number"
                    min="1"
                    max="50"
                    value={config.leaderboard_top_n || 10}
                    onChange={(e) => setConfig({ ...config, leaderboard_top_n: parseInt(e.target.value) })}
                    className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                  />
                  <p className="text-xs text-gray-400 mt-1">Show top 1-50 users in each leaderboard category</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Sort By
                  </label>
                  <select
                    value={config.leaderboard_sort_by || 'value'}
                    onChange={(e) => setConfig({ ...config, leaderboard_sort_by: e.target.value as 'value' | 'count' })}
                    className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                  >
                    <option value="value">Total Value (monetary amount)</option>
                    <option value="count">Count (number of events)</option>
                  </select>
                </div>
              </div>
            </div>

            {/* Display Settings */}
            <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
              <h3 className="text-lg font-semibold text-white mb-4">Display Settings</h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Theme
                  </label>
                  <select
                    value={config.theme || 'cinematic'}
                    onChange={(e) => setConfig({ ...config, theme: e.target.value as 'classic' | 'cinematic' | 'modern' })}
                    className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                  >
                    <option value="classic">Classic</option>
                    <option value="cinematic">Cinematic</option>
                    <option value="modern">Modern</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Scroll Speed (1-100)
                  </label>
                  <input
                    type="range"
                    min="1"
                    max="100"
                    value={config.scroll_speed || 50}
                    onChange={(e) => setConfig({ ...config, scroll_speed: parseInt(e.target.value) })}
                    className="w-full"
                  />
                  <p className="text-xs text-gray-400 mt-1">Current: {config.scroll_speed || 50}</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Display Duration (seconds)
                  </label>
                  <input
                    type="number"
                    min="10"
                    max="300"
                    value={config.display_duration_seconds || 60}
                    onChange={(e) => setConfig({ ...config, display_duration_seconds: parseInt(e.target.value) })}
                    className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                  />
                  <p className="text-xs text-gray-400 mt-1">How long to show the credit roll (10-300 seconds)</p>
                </div>
                <div>
                  <label className="block text-sm font-medium text-white mb-2">
                    Background Opacity (0-1)
                  </label>
                  <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.1"
                    value={config.background_opacity || 0.8}
                    onChange={(e) => setConfig({ ...config, background_opacity: parseFloat(e.target.value) })}
                    className="w-full"
                  />
                  <p className="text-xs text-gray-400 mt-1">Current: {config.background_opacity || 0.8}</p>
                </div>
              </div>
            </div>

            {/* Clips Settings */}
            <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
              <div className="flex items-center justify-between mb-4">
                <div>
                  <h3 className="text-lg font-semibold text-white">Twitch Clips</h3>
                  <p className="text-sm text-gray-400 mt-1">Show clips during credit roll</p>
                </div>
                <button
                  onClick={() => setConfig({ ...config, clips_enabled: !config.clips_enabled })}
                  className={`relative inline-flex h-8 w-14 items-center rounded-full transition-colors ${
                    config.clips_enabled ? 'bg-green-600' : 'bg-gray-600'
                  }`}
                >
                  <span
                    className={`inline-block h-6 w-6 transform rounded-full bg-white transition-transform ${
                      config.clips_enabled ? 'translate-x-7' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              {config.clips_enabled && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-white mb-2">
                      Maximum Clips
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="20"
                      value={config.clips_max_count || 5}
                      onChange={(e) => setConfig({ ...config, clips_max_count: parseInt(e.target.value) })}
                      className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                    />
                  </div>
                  <div>
                    <label className="block text-sm font-medium text-white mb-2">
                      Fallback Days
                    </label>
                    <input
                      type="number"
                      min="1"
                      max="30"
                      value={config.clips_fallback_days || 7}
                      onChange={(e) => setConfig({ ...config, clips_fallback_days: parseInt(e.target.value) })}
                      className="w-full px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                    />
                    <p className="text-xs text-gray-400 mt-1">If no clips from this stream, show clips from last N days</p>
                  </div>
                </div>
              )}
            </div>
          </>
        )}

        {/* CSS Customization Section */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 mb-6">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <h3 className="text-lg font-semibold text-white">Custom CSS Editor</h3>
              <label className="flex items-center gap-2 text-sm text-gray-300">
                <input
                  type="checkbox"
                  checked={useCustomCss}
                  onChange={(e) => setUseCustomCss(e.target.checked)}
                  className="w-5 h-5 rounded border-gray-600 bg-gray-700 text-twitch focus:ring-twitch focus:ring-offset-gray-800"
                />
                Enable Custom CSS
              </label>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setShowThemeMarketplace(true)}
                className="bg-purple-600 hover:bg-purple-700 text-white text-sm font-semibold px-4 py-2 rounded-lg transition-colors flex items-center gap-2"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01" />
                </svg>
                Browse Themes
              </button>
              <button
                type="button"
                onClick={() => {
                  setCustomCss('');
                  setUseCustomCss(false);
                }}
                className="px-4 py-2 text-sm border border-gray-600 rounded-lg text-gray-200 hover:bg-gray-600 transition-colors"
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

          <p className="text-sm text-gray-400 mt-4">
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
            className="flex-1 bg-twitch hover:bg-purple-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? 'Saving...' : 'Save Settings'}
          </button>
          <button
            onClick={() => router.push(`/overlays/${params.id}`)}
            className="px-6 py-3 bg-gray-700 hover:bg-gray-600 text-white font-semibold rounded-lg transition-colors"
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
          setCustomCss(css);
          setUseCustomCss(true);
          setShowThemeMarketplace(false);
        }}
        themeType="creditroll"
      />
    </div>
  );
}
