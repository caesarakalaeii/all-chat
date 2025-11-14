/**
 * Dashboard Page
 *
 * Main dashboard showing user's overlays.
 * Displays all overlays in a grid with create/edit/delete options.
 *
 * Features:
 * - List all user overlays
 * - Create new overlay button
 * - Navigate to overlay editor
 * - Navigate to overlay preview
 * - Display overlay status (active/inactive)
 *
 * This is a Client Component because it:
 * - Uses Zustand state management
 * - Fetches data on mount
 * - Handles user interactions
 */

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { useOverlayStore } from '@/lib/stores/overlay-store';
import { formatDistanceToNow } from 'date-fns';

export default function DashboardPage() {
  const router = useRouter();
  const { user, token, init } = useAuthStore();
  const { overlays, loading, fetchOverlays, deleteOverlay } = useOverlayStore();
  const [deleting, setDeleting] = useState<string | null>(null);

  // Initialize auth and fetch overlays
  useEffect(() => {
    init();
  }, [init]);

  useEffect(() => {
    if (!token) {
      router.push('/');
      return;
    }

    fetchOverlays();
  }, [token, fetchOverlays, router]);

  const handleDelete = async (overlayId: string, overlayName: string) => {
    if (!confirm(`Are you sure you want to delete "${overlayName}"? This action cannot be undone.`)) {
      return;
    }

    setDeleting(overlayId);
    try {
      await deleteOverlay(overlayId);
    } catch (error) {
      console.error('Failed to delete overlay:', error);
      alert('Failed to delete overlay');
    } finally {
      setDeleting(null);
    }
  };

  if (!user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch"></div>
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
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-3">
              {user.profile_image_url && (
                <img
                  src={user.profile_image_url}
                  alt={user.display_name}
                  className="w-8 h-8 rounded-full"
                />
              )}
              <span className="text-white">{user.display_name}</span>
            </div>
            <button
              onClick={() => {
                useAuthStore.getState().logout();
                router.push('/');
              }}
              className="text-gray-400 hover:text-white transition-colors"
            >
              Logout
            </button>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-3xl font-bold text-white">My Overlays</h1>
          <button
            onClick={() => router.push('/overlays/new')}
            className="bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 4v16m8-8H4"
              />
            </svg>
            Create Overlay
          </button>
        </div>

        {/* Overlay Grid */}
        {loading ? (
          <div className="text-center text-gray-400 py-20">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch mx-auto mb-4"></div>
            Loading overlays...
          </div>
        ) : !overlays || overlays.length === 0 ? (
          <div className="text-center py-20">
            <div className="text-6xl mb-6">📺</div>
            <p className="text-gray-400 text-lg mb-6">No overlays yet</p>
            <p className="text-gray-500 mb-8">Create your first overlay to get started!</p>
            <button
              onClick={() => router.push('/overlays/new')}
              className="bg-twitch hover:bg-purple-700 text-white font-semibold py-3 px-8 rounded-lg transition-colors"
            >
              Create Your First Overlay
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {overlays?.map((overlay) => (
              <div
                key={overlay.id}
                onClick={() => router.push(`/overlays/${overlay.id}`)}
                className="bg-gray-800 rounded-lg p-6 hover:bg-gray-750 transition-colors cursor-pointer border border-gray-700"
              >
                <div className="flex justify-between items-start mb-4">
                  <h3 className="text-xl font-semibold text-white">{overlay.name}</h3>
                  <div className="flex items-center gap-2">
                    <span
                      className={`px-2 py-1 rounded text-xs ${
                        overlay.is_active
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-gray-600 text-gray-400'
                      }`}
                    >
                      {overlay.is_active ? 'Active' : 'Inactive'}
                    </span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(overlay.id, overlay.name);
                      }}
                      disabled={deleting === overlay.id}
                      className="text-red-400 hover:text-red-300 transition-colors p-1 disabled:opacity-50"
                      title="Delete overlay"
                    >
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                        />
                      </svg>
                    </button>
                  </div>
                </div>

                {overlay.description && (
                  <p className="text-gray-400 text-sm mb-4 line-clamp-2">
                    {overlay.description}
                  </p>
                )}

                <div className="flex justify-between items-center text-xs text-gray-500">
                  <span>
                    Created {formatDistanceToNow(new Date(overlay.created_at), { addSuffix: true })}
                  </span>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      router.push(`/overlays/${overlay.id}/preview`);
                    }}
                    className="text-twitch hover:text-purple-400 transition-colors"
                  >
                    Preview →
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
