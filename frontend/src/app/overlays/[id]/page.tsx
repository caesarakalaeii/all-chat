/**
 * Overlay Editor Page
 *
 * Edit an existing overlay and manage its chat sources.
 *
 * Features:
 * - Display overlay info (name, description, status)
 * - List all chat sources (Twitch, YouTube, etc.)
 * - Add new chat sources via OAuth login
 * - Add sources manually (advanced)
 * - Remove existing sources
 * - Navigate to preview
 *
 * Client Component for dynamic data fetching and interactions.
 */

'use client';

import { useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { overlaysApi } from '@/lib/api/overlays';
import type { Overlay, ChatSource } from '@/lib/types/overlay';
import { BetaWarning } from '@/components/BetaWarning';

export default function OverlayEditorPage({ params }: { params: { id: string } }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token, user } = useAuthStore();

  const [overlay, setOverlay] = useState<Overlay | null>(null);
  const [sources, setSources] = useState<ChatSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddSource, setShowAddSource] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showTikTokInput, setShowTikTokInput] = useState(false);
  const [tiktokUsername, setTikTokUsername] = useState('');
  const [newSourcePlatform, setNewSourcePlatform] = useState<string>('twitch');
  const [newSourceChannel, setNewSourceChannel] = useState('');
  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | null>(null);

  // Load overlay and sources
  useEffect(() => {
    if (!token) {
      router.push('/');
      return;
    }

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(params.id);
        setOverlay(overlayData);

        // Try to fetch sources, but handle 404 gracefully (endpoint may not be implemented yet)
        try {
          const sourcesData = await overlaysApi.getSources(params.id);
          setSources(sourcesData);
        } catch (sourcesError) {
          console.warn('Sources endpoint not available yet, starting with empty sources');
          setSources([]);
        }
      } catch (error) {
        console.error('Failed to load overlay:', error);
        setOverlay(null);
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, [params.id, token, router]);

  // Handle OAuth callback redirects
  useEffect(() => {
    const sourceAdded = searchParams.get('source_added');
    const error = searchParams.get('error');

    if (sourceAdded) {
      setNotification({
        type: 'success',
        message: `Successfully added ${sourceAdded} source!`
      });
      // Refresh sources
      overlaysApi.getSources(params.id).then(setSources).catch(console.error);
      // Clean up URL
      window.history.replaceState({}, '', `/overlays/${params.id}`);
      // Auto-hide notification after 5 seconds
      setTimeout(() => setNotification(null), 5000);
    } else if (error === 'failed_to_add_source') {
      setNotification({
        type: 'error',
        message: 'Failed to add source. Please try again or use manual entry.'
      });
      // Clean up URL
      window.history.replaceState({}, '', `/overlays/${params.id}`);
      // Auto-hide notification after 5 seconds
      setTimeout(() => setNotification(null), 5000);
    }
  }, [searchParams, params.id]);

  const handleOAuthAddSource = async (platform: 'twitch' | 'youtube' | 'kick' | 'tiktok') => {
    // TikTok uses username input (no OAuth)
    if (platform === 'tiktok') {
      setShowTikTokInput(true);
      return;
    }

    // Show beta warning for YouTube
    if (platform === 'youtube') {
      setShowBetaWarning(platform);
      return;
    }

    proceedWithOAuthAddSource(platform);
  };

  const proceedWithOAuthAddSource = async (platform: 'youtube' | 'twitch' | 'kick') => {
    try {
      // Get OAuth URL (or short-circuit response) from backend
      const response = await fetch(`/api/v1/auth/${platform}/add-source/${params.id}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to get OAuth URL');
      }

      const data = await response.json();

      if (data.source_added) {
        setNotification({
          type: 'success',
          message: `Successfully added ${data.source_added} source!`
        });
        overlaysApi.getSources(params.id).then(setSources).catch(console.error);
        setShowAddSource(false);
        setTimeout(() => setNotification(null), 5000);
        return;
      }

      if (data.auth_url) {
        // Redirect to OAuth provider when server requires a new auth flow
        window.location.href = data.auth_url;
        return;
      }

      throw new Error('Server response missing auth_url');
    } catch (error) {
      console.error('Failed to initiate OAuth:', error);
      setNotification({
        type: 'error',
        message: 'Failed to start login process. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    }
  };

  const handleAddSourceManually = async () => {
    if (!newSourceChannel.trim()) return;

    try {
      let channelId = newSourceChannel.trim();

      // If YouTube, resolve URL/handle to channel ID first
      if (newSourcePlatform === 'youtube') {
        try {
          const response = await fetch('/api/v1/overlays/youtube/resolve', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
              'Authorization': `Bearer ${token}`,
            },
            body: JSON.stringify({ input: newSourceChannel }),
          });

          if (!response.ok) {
            const error = await response.json();
            alert(error.error || 'Failed to resolve YouTube channel');
            return;
          }

          const resolved = await response.json();
          channelId = resolved.channel_id;
          console.log(`Resolved YouTube input to channel: ${resolved.title} (${channelId})`);
        } catch (error) {
          console.error('Failed to resolve YouTube channel:', error);
          alert('Failed to resolve YouTube channel. Please enter a valid YouTube URL, handle (@username), or channel ID.');
          return;
        }
      }

      await overlaysApi.addSource(params.id, {
        platform: newSourcePlatform as any,
        channel_id: channelId
      });
      const updatedSources = await overlaysApi.getSources(params.id);
      setSources(updatedSources);
      setShowAddSource(false);
      setShowAdvanced(false);
      setNewSourceChannel('');
      setNotification({
        type: 'success',
        message: 'Source added successfully!'
      });
      setTimeout(() => setNotification(null), 5000);
    } catch (error) {
      console.error('Failed to add source:', error);
      setNotification({
        type: 'error',
        message: 'Failed to add source. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    }
  };

  const handleAddTikTokSource = async () => {
    if (!tiktokUsername.trim()) return;

    try {
      const username = tiktokUsername.trim().toLowerCase().replace(/^@/, '');

      await overlaysApi.addSource(params.id, {
        platform: 'tiktok',
        channel_id: username
      });
      const updatedSources = await overlaysApi.getSources(params.id);
      setSources(updatedSources);
      setShowAddSource(false);
      setShowTikTokInput(false);
      setTikTokUsername('');
      setNotification({
        type: 'success',
        message: 'TikTok source added successfully!'
      });
      setTimeout(() => setNotification(null), 5000);
    } catch (error) {
      console.error('Failed to add TikTok source:', error);
      setNotification({
        type: 'error',
        message: 'Failed to add TikTok source. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    }
  };

  const handleRemoveSource = async (sourceId: string) => {
    if (!confirm('Remove this source?')) return;

    try {
      await overlaysApi.removeSource(params.id, sourceId);
      const updatedSources = await overlaysApi.getSources(params.id);
      setSources(updatedSources);
      setNotification({
        type: 'success',
        message: 'Source removed successfully!'
      });
      setTimeout(() => setNotification(null), 5000);
    } catch (error) {
      console.error('Failed to remove source:', error);
      setNotification({
        type: 'error',
        message: 'Failed to remove source. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    }
  };

  const handleTogglePublicForViewers = async () => {
    if (!overlay) return;

    try {
      const updatedOverlay = await overlaysApi.update(params.id, {
        is_public_for_viewers: !overlay.is_public_for_viewers
      });
      setOverlay(updatedOverlay);
      setNotification({
        type: 'success',
        message: updatedOverlay.is_public_for_viewers
          ? 'Overlay is now public for viewers (browser extension can connect)'
          : 'Overlay is now private (browser extension cannot connect)'
      });
      setTimeout(() => setNotification(null), 5000);
    } catch (error) {
      console.error('Failed to update overlay:', error);
      setNotification({
        type: 'error',
        message: 'Failed to update overlay setting. Please try again.'
      });
      setTimeout(() => setNotification(null), 5000);
    }
  };

  const getPlatformColor = (platform: string): string => {
    switch (platform) {
      case 'twitch':
        return 'bg-twitch/20 text-purple-400 border-twitch/30';
      case 'youtube':
        return 'bg-red-500/20 text-red-400 border-red-500/30';
      case 'kick':
        return 'bg-green-500/20 text-green-400 border-green-500/30';
      case 'tiktok':
        return 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30';
      default:
        return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
    }
  };

  const getPlatformIcon = (platform: string) => {
    switch (platform) {
      case 'twitch':
        return (
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
          </svg>
        );
      case 'youtube':
        return (
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
          </svg>
        );
      case 'kick':
        return (
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0zm5.894 16.97L12 12.053l-5.894 4.917V7.03L12 11.947l5.894-4.917v9.94z"/>
          </svg>
        );
      case 'tiktok':
        return (
          <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12.525.02c1.31-.02 2.61-.01 3.91-.02.08 1.53.63 3.09 1.75 4.17 1.12 1.11 2.7 1.62 4.24 1.79v4.03c-1.44-.05-2.89-.35-4.2-.97-.57-.26-1.1-.59-1.62-.93-.01 2.92.01 5.84-.02 8.75-.08 1.4-.54 2.79-1.35 3.94-1.31 1.92-3.58 3.17-5.91 3.21-1.43.08-2.86-.31-4.08-1.03-2.02-1.19-3.44-3.37-3.65-5.71-.02-.5-.03-1-.01-1.49.18-1.9 1.12-3.72 2.58-4.96 1.66-1.44 3.98-2.13 6.15-1.72.02 1.48-.04 2.96-.04 4.44-.99-.32-2.15-.23-3.02.37-.63.41-1.11 1.04-1.36 1.75-.21.51-.15 1.07-.14 1.61.24 1.64 1.82 3.02 3.5 2.87 1.12-.01 2.19-.66 2.77-1.61.19-.33.4-.67.41-1.06.1-1.79.06-3.57.07-5.36.01-4.03-.01-8.05.02-12.07z"/>
          </svg>
        );
      default:
        return null;
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
      {/* Navbar */}
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <a href="/dashboard" className="text-2xl font-bold text-white">
            All-Chat
          </a>
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 text-gray-400 hover:text-white transition-colors"
          >
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
              <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.603-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.463-1.11-1.463-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"/>
            </svg>
            <span>GitHub</span>
          </a>
        </div>
      </nav>

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
      <div className="container mx-auto px-4 py-8">
        {/* Header */}
        <div className="flex justify-between items-start mb-8">
          <div>
            <div className="flex items-center gap-4 mb-2">
              <h1 className="text-3xl font-bold text-white">{overlay.name}</h1>
              <span
                className={`px-3 py-1 rounded-full text-sm ${
                  overlay.is_active
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-gray-600 text-gray-400'
                }`}
              >
                {overlay.is_active ? 'Active' : 'Inactive'}
              </span>
            </div>
            {overlay.description && <p className="text-gray-400 mt-2">{overlay.description}</p>}

            {/* Public for Viewers Toggle */}
            <div className="mt-4 flex items-center gap-3">
              <button
                onClick={handleTogglePublicForViewers}
                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                  overlay.is_public_for_viewers ? 'bg-green-600' : 'bg-gray-600'
                }`}
                title={overlay.is_public_for_viewers ? 'Viewers can connect via browser extension' : 'Browser extension connections disabled'}
              >
                <span
                  className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                    overlay.is_public_for_viewers ? 'translate-x-6' : 'translate-x-1'
                  }`}
                />
              </button>
              <div>
                <p className="text-sm font-medium text-white">
                  Public for Viewers {overlay.is_public_for_viewers && <span className="text-green-400">✓</span>}
                </p>
                <p className="text-xs text-gray-400">
                  {overlay.is_public_for_viewers
                    ? 'Browser extension can connect to this overlay'
                    : 'Browser extension connections are disabled'}
                </p>
              </div>
            </div>
          </div>

          <button
            onClick={() => router.push(`/overlays/${params.id}/preview`)}
            className="bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
              />
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              />
            </svg>
            Preview
          </button>
        </div>

        {/* Chat Sources Section */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-xl font-semibold text-white">Chat Sources</h2>
            <button
              onClick={() => setShowAddSource(!showAddSource)}
              className="bg-gray-700 hover:bg-gray-600 text-white font-semibold py-2 px-4 rounded-lg transition-colors"
            >
              + Add Source
            </button>
          </div>

          {/* Add Source Form */}
          {showAddSource && (
            <div className="mb-6 p-6 bg-gray-750 rounded-lg border border-gray-600">
              <h3 className="text-lg font-medium text-white mb-4">Add New Source</h3>

              {/* OAuth Buttons */}
              <div className="mb-6">
                <p className="text-sm text-gray-400 mb-3">
                  Login with your streaming platform to automatically add your channel:
                </p>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    onClick={() => handleOAuthAddSource('twitch')}
                    className="flex items-center justify-center gap-2 px-4 py-3 bg-twitch hover:bg-purple-700 text-white font-semibold rounded-lg transition-colors"
                  >
                    {getPlatformIcon('twitch')}
                    Login with Twitch
                  </button>
                  <button
                    onClick={() => handleOAuthAddSource('youtube')}
                    className="flex items-center justify-center gap-2 px-4 py-3 bg-red-600 hover:bg-red-700 text-white font-semibold rounded-lg transition-colors"
                  >
                    {getPlatformIcon('youtube')}
                    Login with YouTube
                  </button>
                  <button
                    onClick={() => handleOAuthAddSource('kick')}
                    className="flex items-center justify-center gap-2 px-4 py-3 bg-green-600 hover:bg-green-700 text-white font-semibold rounded-lg transition-colors"
                  >
                    {getPlatformIcon('kick')}
                    Login with Kick
                  </button>
                </div>
              </div>

              {/* TikTok Username Input */}
              <div className="mb-6">
                <button
                  onClick={() => setShowTikTokInput(!showTikTokInput)}
                  className="flex items-center justify-center gap-2 px-4 py-3 bg-cyan-600 hover:bg-cyan-700 text-white font-semibold rounded-lg transition-colors w-full"
                >
                  {getPlatformIcon('tiktok')}
                  Add TikTok by Username
                  <span className="ml-2 text-xs bg-yellow-500 text-black px-2 py-0.5 rounded-full">BETA</span>
                </button>

                {showTikTokInput && (
                  <div className="mt-3 p-4 bg-gray-700/50 rounded-lg border border-gray-600">
                    <p className="text-xs text-gray-400 mb-3">
                      Enter TikTok username (no OAuth required - uses unofficial library)
                    </p>
                    <div className="flex gap-3">
                      <input
                        type="text"
                        value={tiktokUsername}
                        onChange={(e) => setTikTokUsername(e.target.value)}
                        placeholder="TikTok username (e.g., @username or username)"
                        className="flex-1 px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-cyan-500"
                        onKeyPress={(e) => e.key === 'Enter' && handleAddTikTokSource()}
                      />
                      <button
                        onClick={handleAddTikTokSource}
                        className="bg-cyan-600 hover:bg-cyan-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors"
                      >
                        Add
                      </button>
                    </div>
                  </div>
                )}
              </div>

              {/* Divider */}
              <div className="relative my-6">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-600"></div>
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-2 bg-gray-750 text-gray-400">OR</span>
                </div>
              </div>

              {/* Advanced: Manual Entry (Admin Only) */}
              {user?.is_admin && (
                <div>
                  <button
                    onClick={() => setShowAdvanced(!showAdvanced)}
                    className="flex items-center gap-2 text-gray-400 hover:text-white transition-colors text-sm mb-3"
                  >
                  <svg
                    className={`w-4 h-4 transition-transform ${showAdvanced ? 'rotate-90' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                  </svg>
                  Advanced: Enter Channel ID Manually
                </button>

                {showAdvanced && (
                  <div className="pl-6 space-y-3">
                    <p className="text-xs text-gray-500 mb-3">
                      Enter a specific channel ID or username. For YouTube, you can also enter a URL or @handle.
                    </p>
                    <div className="flex gap-3">
                      <select
                        value={newSourcePlatform}
                        onChange={(e) => setNewSourcePlatform(e.target.value)}
                        className="px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                      >
                        <option value="twitch">Twitch</option>
                        <option value="youtube">YouTube</option>
                        <option value="kick">Kick</option>
                        <option value="tiktok">TikTok</option>
                      </select>
                      <input
                        type="text"
                        value={newSourceChannel}
                        onChange={(e) => setNewSourceChannel(e.target.value)}
                        placeholder="Channel ID, username, or URL"
                        className="flex-1 px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                        onKeyPress={(e) => e.key === 'Enter' && handleAddSourceManually()}
                      />
                      <button
                        onClick={handleAddSourceManually}
                        className="bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors"
                      >
                        Add
                      </button>
                    </div>
                  </div>
                )}
                </div>
              )}

              {/* Cancel Button */}
              <div className="mt-4 flex justify-end">
                <button
                  onClick={() => {
                    setShowAddSource(false);
                    setShowAdvanced(false);
                    setShowTikTokInput(false);
                    setTikTokUsername('');
                    setNewSourceChannel('');
                  }}
                  className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}

          {/* Sources List */}
          {!sources || sources.length === 0 ? (
            <div className="text-center py-12 text-gray-500">
              <p className="mb-2">No chat sources added yet</p>
              <p className="text-sm">Add Twitch, YouTube, Kick, or TikTok sources to start aggregating chat</p>
            </div>
          ) : (
            <div className="space-y-3">
              {sources?.map((source) => (
                <div
                  key={source.id}
                  className={`flex items-center justify-between p-4 bg-gray-750 rounded-lg border ${getPlatformColor(
                    source.platform
                  )}`}
                >
                  <div className="flex items-center gap-4">
                    <div className="flex items-center gap-2">
                      {getPlatformIcon(source.platform)}
                      <span
                        className={`text-xs font-semibold uppercase ${
                          getPlatformColor(source.platform).split(' ')[1]
                        }`}
                      >
                        {source.platform}
                      </span>
                    </div>
                    <span className="text-white font-medium">
                      {source.channel_name || source.channel_id}
                    </span>
                  </div>
                  <button
                    onClick={() => handleRemoveSource(source.id)}
                    className="text-red-400 hover:text-red-300 transition-colors"
                    title="Remove source"
                  >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                      />
                    </svg>
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Beta Warning Modal */}
      {showBetaWarning && (
        <BetaWarning
          platform={showBetaWarning}
          onCancel={() => setShowBetaWarning(null)}
          onContinue={() => {
            const platform = showBetaWarning;
            setShowBetaWarning(null);
            proceedWithOAuthAddSource(platform);
          }}
        />
      )}
    </div>
  );
}
