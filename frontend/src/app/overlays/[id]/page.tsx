/**
 * Overlay Editor Page
 *
 * Edit an existing overlay and manage its chat sources.
 *
 * Features:
 * - Display overlay info (name, description, status)
 * - List all chat sources (Twitch, YouTube, etc.)
 * - Add new chat sources
 * - Remove existing sources
 * - Navigate to preview
 *
 * Client Component for dynamic data fetching and interactions.
 */

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { overlaysApi } from '@/lib/api/overlays';
import type { Overlay, ChatSource } from '@/lib/types/overlay';

export default function OverlayEditorPage({ params }: { params: { id: string } }) {
  const router = useRouter();
  const { token } = useAuthStore();

  const [overlay, setOverlay] = useState<Overlay | null>(null);
  const [sources, setSources] = useState<ChatSource[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAddSource, setShowAddSource] = useState(false);
  const [newSourcePlatform, setNewSourcePlatform] = useState<string>('twitch');
  const [newSourceChannel, setNewSourceChannel] = useState('');

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

  const handleAddSource = async () => {
    if (!newSourceChannel.trim()) return;

    try {
      await overlaysApi.addSource(params.id, {
        platform: newSourcePlatform as any,
        channel_id: newSourceChannel
      });
      const updatedSources = await overlaysApi.getSources(params.id);
      setSources(updatedSources);
      setShowAddSource(false);
      setNewSourceChannel('');
    } catch (error) {
      console.error('Failed to add source:', error);
      alert('Failed to add source');
    }
  };

  const handleRemoveSource = async (sourceId: string) => {
    if (!confirm('Remove this source?')) return;

    try {
      await overlaysApi.removeSource(params.id, sourceId);
      const updatedSources = await overlaysApi.getSources(params.id);
      setSources(updatedSources);
    } catch (error) {
      console.error('Failed to remove source:', error);
      alert('Failed to remove source');
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
      default:
        return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
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
        <div className="container mx-auto px-4 py-4">
          <a href="/dashboard" className="text-2xl font-bold text-white">
            All-Chat
          </a>
        </div>
      </nav>

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
            <div className="mb-6 p-4 bg-gray-750 rounded-lg border border-gray-600">
              <h3 className="text-sm font-medium text-gray-300 mb-3">Add New Source</h3>
              <div className="flex gap-4">
                <select
                  value={newSourcePlatform}
                  onChange={(e) => setNewSourcePlatform(e.target.value)}
                  className="px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                >
                  <option value="twitch">Twitch</option>
                  <option value="youtube">YouTube</option>
                  <option value="kick" disabled>
                    Kick (Coming Soon)
                  </option>
                  <option value="tiktok" disabled>
                    TikTok (Coming Soon)
                  </option>
                </select>
                <input
                  type="text"
                  value={newSourceChannel}
                  onChange={(e) => setNewSourceChannel(e.target.value)}
                  placeholder="Channel ID or username"
                  className="flex-1 px-4 py-2 bg-gray-700 text-white border border-gray-600 rounded-lg focus:outline-none focus:border-twitch"
                />
                <button
                  onClick={handleAddSource}
                  className="bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors"
                >
                  Add
                </button>
                <button
                  onClick={() => setShowAddSource(false)}
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
              <p className="text-sm">Add Twitch or YouTube sources to start aggregating chat</p>
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
                    <span
                      className={`text-xs font-semibold uppercase ${
                        getPlatformColor(source.platform).split(' ')[1]
                      }`}
                    >
                      {source.platform}
                    </span>
                    <span className="text-white font-medium">{source.channel_id}</span>
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
    </div>
  );
}
