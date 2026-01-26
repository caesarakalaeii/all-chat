// Event Settings Page - Configure overlay event display preferences
'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAuthStore } from '@/lib/stores/auth-store';

interface EventSettings {
  id: string;
  overlay_id: string;
  // Twitch
  enable_twitch_subs: boolean;
  enable_twitch_resubs: boolean;
  enable_twitch_gift_subs: boolean;
  enable_twitch_bits: boolean;
  enable_twitch_raids: boolean;
  enable_twitch_channel_points: boolean;
  // YouTube
  enable_youtube_super_chat: boolean;
  enable_youtube_super_sticker: boolean;
  enable_youtube_members: boolean;
  enable_youtube_member_milestones: boolean;
  enable_youtube_member_gifts: boolean;
  // Kick
  enable_kick_subs: boolean;
  enable_kick_gifts: boolean;
  // TikTok
  enable_tiktok_likes: boolean;
  enable_tiktok_gifts: boolean;
  enable_tiktok_follows: boolean;
  enable_tiktok_shares: boolean;
  // System
  enable_token_warnings: boolean;
  // Settings
  tiktok_like_aggregation_window_seconds: number;
  event_display_duration_multiplier: number;
}

export default function EventSettingsPage({ params }: { params: { id: string } }) {
  const { token } = useAuthStore();
  const [settings, setSettings] = useState<EventSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'twitch' | 'youtube' | 'kick' | 'tiktok'>('twitch');
  const router = useRouter();

  // Load event settings
  useEffect(() => {
    if (!token) {
      router.push('/');
      return;
    }

    const loadSettings = async () => {
      try {
        const res = await fetch(`/api/v1/overlays/${params.id}/event-settings`, {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (!res.ok) {
          throw new Error('Failed to load event settings');
        }

        const data = await res.json();
        setSettings(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load settings');
      } finally {
        setLoading(false);
      }
    };

    loadSettings();
  }, [params.id, token, router]);

  const handleSave = async () => {
    if (!settings || !token) return;

    setSaving(true);
    setError(null);

    try {
      const res = await fetch(`/api/v1/overlays/${params.id}/event-settings`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify(settings),
      });

      if (!res.ok) {
        throw new Error('Failed to save event settings');
      }

      const updated = await res.json();
      setSettings(updated);
      alert('Event settings saved successfully!');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  const updateSetting = (key: keyof EventSettings, value: boolean | number) => {
    if (!settings) return;
    setSettings({ ...settings, [key]: value });
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-900 text-white p-8">
        <div className="max-w-4xl mx-auto">
          <p>Loading event settings...</p>
        </div>
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="min-h-screen bg-gray-900 text-white p-8">
        <div className="max-w-4xl mx-auto">
          <p className="text-red-400">Failed to load event settings: {error}</p>
          <Link href={`/overlays/${params.id}`} className="text-blue-400 hover:underline mt-4 block">
            Back to Overlay
          </Link>
        </div>
      </div>
    );
  }

  const EventToggle = ({ label, description, value, onChange }: {
    label: string;
    description: string;
    value: boolean;
    onChange: (value: boolean) => void;
  }) => (
    <div className="flex items-center justify-between py-3 border-b border-gray-700">
      <div className="flex-1">
        <h3 className="font-medium text-white">{label}</h3>
        <p className="text-sm text-gray-400 mt-1">{description}</p>
      </div>
      <label className="relative inline-flex items-center cursor-pointer ml-4">
        <input
          type="checkbox"
          checked={value}
          onChange={(e) => onChange(e.target.checked)}
          className="sr-only peer"
        />
        <div className="w-11 h-6 bg-gray-700 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600"></div>
      </label>
    </div>
  );

  return (
    <div className="min-h-screen bg-gray-900 text-white p-8">
      <div className="max-w-4xl mx-auto">
        <div className="mb-6">
          <Link href={`/overlays/${params.id}`} className="text-blue-400 hover:underline">
            ← Back to Overlay
          </Link>
        </div>

        <div className="bg-gray-800 rounded-lg shadow-xl p-6">
          <h1 className="text-3xl font-bold mb-2">Event Display Settings</h1>
          <p className="text-gray-400 mb-6">
            Control which platform events (subscriptions, donations, raids, follows, etc.) are displayed on your overlay.
          </p>

          {error && (
            <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded mb-6">
              {error}
            </div>
          )}

          {/* Platform Tabs */}
          <div className="flex space-x-4 border-b border-gray-700 mb-6">
            {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map((platform) => (
              <button
                key={platform}
                onClick={() => setActiveTab(platform)}
                className={`pb-3 px-4 font-medium transition-colors ${
                  activeTab === platform
                    ? 'text-blue-400 border-b-2 border-blue-400'
                    : 'text-gray-400 hover:text-gray-200'
                }`}
              >
                {platform.charAt(0).toUpperCase() + platform.slice(1)}
              </button>
            ))}
          </div>

          {/* Twitch Events */}
          {activeTab === 'twitch' && (
            <div className="space-y-0">
              <EventToggle
                label="Subscriptions"
                description="New subscriptions and resubscriptions"
                value={settings.enable_twitch_subs}
                onChange={(v) => updateSetting('enable_twitch_subs', v)}
              />
              <EventToggle
                label="Resubscriptions"
                description="Monthly resubscription notices with streak information"
                value={settings.enable_twitch_resubs}
                onChange={(v) => updateSetting('enable_twitch_resubs', v)}
              />
              <EventToggle
                label="Gift Subscriptions"
                description="Gift subs and mystery gift bombs"
                value={settings.enable_twitch_gift_subs}
                onChange={(v) => updateSetting('enable_twitch_gift_subs', v)}
              />
              <EventToggle
                label="Bits / Cheers"
                description="Bits cheered in chat"
                value={settings.enable_twitch_bits}
                onChange={(v) => updateSetting('enable_twitch_bits', v)}
              />
              <EventToggle
                label="Raids"
                description="Incoming raids from other channels"
                value={settings.enable_twitch_raids}
                onChange={(v) => updateSetting('enable_twitch_raids', v)}
              />
              <EventToggle
                label="Channel Points"
                description="Channel point reward redemptions (requires EventSub service)"
                value={settings.enable_twitch_channel_points}
                onChange={(v) => updateSetting('enable_twitch_channel_points', v)}
              />
            </div>
          )}

          {/* YouTube Events */}
          {activeTab === 'youtube' && (
            <div className="space-y-0">
              <EventToggle
                label="Super Chat"
                description="Paid Super Chat messages"
                value={settings.enable_youtube_super_chat}
                onChange={(v) => updateSetting('enable_youtube_super_chat', v)}
              />
              <EventToggle
                label="Super Stickers"
                description="Paid Super Sticker purchases"
                value={settings.enable_youtube_super_sticker}
                onChange={(v) => updateSetting('enable_youtube_super_sticker', v)}
              />
              <EventToggle
                label="New Members"
                description="New channel memberships"
                value={settings.enable_youtube_members}
                onChange={(v) => updateSetting('enable_youtube_members', v)}
              />
              <EventToggle
                label="Member Milestones"
                description="Membership anniversary celebrations"
                value={settings.enable_youtube_member_milestones}
                onChange={(v) => updateSetting('enable_youtube_member_milestones', v)}
              />
              <EventToggle
                label="Membership Gifts"
                description="Gifted memberships"
                value={settings.enable_youtube_member_gifts}
                onChange={(v) => updateSetting('enable_youtube_member_gifts', v)}
              />
            </div>
          )}

          {/* Kick Events */}
          {activeTab === 'kick' && (
            <div className="space-y-0">
              <EventToggle
                label="Subscriptions"
                description="Kick channel subscriptions"
                value={settings.enable_kick_subs}
                onChange={(v) => updateSetting('enable_kick_subs', v)}
              />
              <EventToggle
                label="Gifts & Donations"
                description="Gift subscriptions and donations"
                value={settings.enable_kick_gifts}
                onChange={(v) => updateSetting('enable_kick_gifts', v)}
              />
              <div className="py-4 text-sm text-gray-400">
                <p>⚠️ Kick events require reverse-engineering and may not be available yet.</p>
              </div>
            </div>
          )}

          {/* TikTok Events */}
          {activeTab === 'tiktok' && (
            <div className="space-y-0">
              <EventToggle
                label="Likes"
                description="Likes sent during stream (aggregated over 30-second windows)"
                value={settings.enable_tiktok_likes}
                onChange={(v) => updateSetting('enable_tiktok_likes', v)}
              />
              <EventToggle
                label="Gifts"
                description="Virtual gifts sent with diamond values"
                value={settings.enable_tiktok_gifts}
                onChange={(v) => updateSetting('enable_tiktok_gifts', v)}
              />
              <EventToggle
                label="Follows"
                description="New followers during stream"
                value={settings.enable_tiktok_follows}
                onChange={(v) => updateSetting('enable_tiktok_follows', v)}
              />
              <EventToggle
                label="Shares"
                description="Stream shares to other platforms"
                value={settings.enable_tiktok_shares}
                onChange={(v) => updateSetting('enable_tiktok_shares', v)}
              />

              {/* System Events */}
              <div className="pt-4 mt-4 border-t border-gray-700">
                <h3 className="font-medium text-white mb-4">System Events</h3>
                <EventToggle
                  label="Token Warnings"
                  description="Display OAuth authentication errors on overlay (requires token-refresh-service)"
                  value={settings.enable_token_warnings}
                  onChange={(v) => updateSetting('enable_token_warnings', v)}
                />
              </div>

              {/* Advanced Settings */}
              <div className="pt-4 mt-4 border-t border-gray-700">
                <h3 className="font-medium text-white mb-4">Advanced Settings</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">
                      Like Aggregation Window (seconds)
                    </label>
                    <p className="text-xs text-gray-400 mb-2">
                      Likes are collected and updated in this time window to prevent spam
                    </p>
                    <input
                      type="number"
                      min="10"
                      max="60"
                      value={settings.tiktok_like_aggregation_window_seconds}
                      onChange={(e) => updateSetting('tiktok_like_aggregation_window_seconds', parseInt(e.target.value))}
                      className="bg-gray-700 border border-gray-600 text-white rounded px-3 py-2 w-32"
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Global Settings */}
          <div className="mt-8 pt-6 border-t border-gray-700">
            <h3 className="font-medium text-white mb-4">Global Settings</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Event Display Duration Multiplier
                </label>
                <p className="text-xs text-gray-400 mb-2">
                  Multiply all event display durations by this factor (0.5 = half time, 2.0 = double time)
                </p>
                <input
                  type="number"
                  min="0.1"
                  max="5"
                  step="0.1"
                  value={settings.event_display_duration_multiplier}
                  onChange={(e) => updateSetting('event_display_duration_multiplier', parseFloat(e.target.value))}
                  className="bg-gray-700 border border-gray-600 text-white rounded px-3 py-2 w-32"
                />
              </div>
            </div>
          </div>

          {/* Save Button */}
          <div className="mt-8 flex gap-4">
            <button
              onClick={handleSave}
              disabled={saving}
              className="bg-blue-600 hover:bg-blue-700 disabled:bg-gray-600 text-white font-medium py-2 px-6 rounded transition-colors"
            >
              {saving ? 'Saving...' : 'Save Settings'}
            </button>
            <Link
              href={`/overlays/${params.id}`}
              className="bg-gray-700 hover:bg-gray-600 text-white font-medium py-2 px-6 rounded transition-colors inline-block"
            >
              Cancel
            </Link>
          </div>

          {/* Info Box */}
          <div className="mt-8 bg-blue-900/30 border border-blue-500/50 rounded-lg p-4">
            <h3 className="font-medium text-blue-300 mb-2">ℹ️ About Event Display</h3>
            <ul className="text-sm text-gray-300 space-y-2">
              <li>• <strong>High-value events</strong> (subs, large donations, raids): Display for 30+ seconds</li>
              <li>• <strong>Medium-value events</strong> (follows, small gifts): Display for 15 seconds</li>
              <li>• <strong>Low-value events</strong> (likes, shares): Display for 5-10 seconds</li>
              <li>• <strong>TikTok likes</strong> are aggregated over 30-second windows to prevent spam</li>
              <li>• Events can be styled with custom CSS using classes like <code className="bg-gray-700 px-1 rounded">.event-tier-high</code></li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
