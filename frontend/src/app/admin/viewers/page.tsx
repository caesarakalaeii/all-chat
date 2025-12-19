/**
 * Admin Viewer Management Page
 *
 * Allows admins to view all viewer sessions and ban/unban users.
 *
 * Features:
 * - List all viewer sessions
 * - View message counts and rate limits
 * - Ban viewers with reason
 * - Unban viewers
 * - See ban status and reasons
 *
 * Route: /admin/viewers
 */

'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { apiClient } from '@/lib/api/client';
import { formatDistanceToNow } from 'date-fns';

interface ViewerSession {
  id: string;
  platform: string;
  platform_user_id: string;
  username: string;
  display_name: string;
  last_message_at: string | null;
  message_count_1min: number;
  message_count_1hour: number;
  is_banned: boolean;
  banned_at: string | null;
  banned_reason: string | null;
  created_at: string;
}

export default function AdminViewersPage() {
  const router = useRouter();
  const { user } = useAuthStore();

  const [viewers, setViewers] = useState<ViewerSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [banningId, setBanningId] = useState<string | null>(null);
  const [banReason, setBanReason] = useState('');
  const [showBanModal, setShowBanModal] = useState(false);
  const [selectedViewer, setSelectedViewer] = useState<ViewerSession | null>(null);

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard');
      return;
    }

    fetchViewers();
  }, [user, router]);

  const fetchViewers = async () => {
    try {
      setLoading(true);
      const response = await apiClient.get<{ viewers: ViewerSession[] }>('/api/v1/admin/viewers?limit=100');
      setViewers(response.viewers);
    } catch (error) {
      console.error('Failed to fetch viewers:', error);
      alert('Failed to load viewers');
    } finally {
      setLoading(false);
    }
  };

  const handleBanClick = (viewer: ViewerSession) => {
    setSelectedViewer(viewer);
    setBanReason('');
    setShowBanModal(true);
  };

  const handleBan = async () => {
    if (!selectedViewer) return;

    try {
      setBanningId(selectedViewer.id);
      await apiClient.post(`/api/v1/admin/viewers/${selectedViewer.id}/ban`, {
        reason: banReason || 'No reason provided'
      });
      setShowBanModal(false);
      setSelectedViewer(null);
      setBanReason('');
      fetchViewers();
    } catch (error) {
      console.error('Failed to ban viewer:', error);
      alert('Failed to ban viewer');
    } finally {
      setBanningId(null);
    }
  };

  const handleUnban = async (viewerId: string) => {
    if (!confirm('Are you sure you want to unban this viewer?')) return;

    try {
      setBanningId(viewerId);
      await apiClient.post(`/api/v1/admin/viewers/${viewerId}/unban`, {});
      fetchViewers();
    } catch (error) {
      console.error('Failed to unban viewer:', error);
      alert('Failed to unban viewer');
    } finally {
      setBanningId(null);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="text-white text-xl">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Navbar */}
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <a href="/admin" className="text-2xl font-bold text-white">
            All-Chat Admin
          </a>
          <div className="flex items-center gap-4">
            <a href="/admin/users" className="text-gray-400 hover:text-white">Users</a>
            <a href="/admin/overlays" className="text-gray-400 hover:text-white">Overlays</a>
            <a href="/admin/sources" className="text-gray-400 hover:text-white">Sources</a>
            <a href="/admin/viewers" className="text-white font-semibold">Viewers</a>
            <a href="/dashboard" className="text-gray-400 hover:text-white">Dashboard</a>
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        <div className="mb-6">
          <h1 className="text-3xl font-bold text-white mb-2">Viewer Management</h1>
          <p className="text-gray-400">Manage viewer sessions and bans</p>
        </div>

        {/* Viewers Table */}
        <div className="bg-gray-800 rounded-lg overflow-hidden">
          <table className="w-full">
            <thead className="bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left text-gray-300">Username</th>
                <th className="px-4 py-3 text-left text-gray-300">Platform</th>
                <th className="px-4 py-3 text-left text-gray-300">Last Message</th>
                <th className="px-4 py-3 text-left text-gray-300">Msg Count (1m/1h)</th>
                <th className="px-4 py-3 text-left text-gray-300">Status</th>
                <th className="px-4 py-3 text-left text-gray-300">Actions</th>
              </tr>
            </thead>
            <tbody>
              {viewers.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-gray-500">
                    No viewer sessions found
                  </td>
                </tr>
              ) : (
                viewers.map((viewer) => (
                  <tr key={viewer.id} className="border-t border-gray-700">
                    <td className="px-4 py-3">
                      <div className="text-white font-semibold">{viewer.username}</div>
                      <div className="text-gray-400 text-sm">{viewer.display_name}</div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-gray-300 capitalize">{viewer.platform}</span>
                    </td>
                    <td className="px-4 py-3 text-gray-400 text-sm">
                      {viewer.last_message_at
                        ? formatDistanceToNow(new Date(viewer.last_message_at), { addSuffix: true })
                        : 'Never'}
                    </td>
                    <td className="px-4 py-3 text-gray-400">
                      {viewer.message_count_1min}/{viewer.message_count_1hour}
                    </td>
                    <td className="px-4 py-3">
                      {viewer.is_banned ? (
                        <div>
                          <span className="inline-block bg-red-900 text-red-200 px-2 py-1 rounded text-sm">
                            BANNED
                          </span>
                          {viewer.banned_reason && (
                            <div className="text-gray-400 text-xs mt-1">
                              Reason: {viewer.banned_reason}
                            </div>
                          )}
                        </div>
                      ) : (
                        <span className="inline-block bg-green-900 text-green-200 px-2 py-1 rounded text-sm">
                          Active
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {viewer.is_banned ? (
                        <button
                          onClick={() => handleUnban(viewer.id)}
                          disabled={banningId === viewer.id}
                          className="bg-green-600 hover:bg-green-700 disabled:bg-gray-600 text-white px-3 py-1 rounded text-sm transition-colors"
                        >
                          {banningId === viewer.id ? 'Unbanning...' : 'Unban'}
                        </button>
                      ) : (
                        <button
                          onClick={() => handleBanClick(viewer)}
                          disabled={banningId === viewer.id}
                          className="bg-red-600 hover:bg-red-700 disabled:bg-gray-600 text-white px-3 py-1 rounded text-sm transition-colors"
                        >
                          Ban
                        </button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Stats */}
        <div className="mt-6 grid grid-cols-3 gap-4">
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Total Viewers</div>
            <div className="text-white text-2xl font-bold">{viewers.length}</div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Banned</div>
            <div className="text-red-400 text-2xl font-bold">
              {viewers.filter((v) => v.is_banned).length}
            </div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4">
            <div className="text-gray-400 text-sm">Active</div>
            <div className="text-green-400 text-2xl font-bold">
              {viewers.filter((v) => !v.is_banned).length}
            </div>
          </div>
        </div>
      </div>

      {/* Ban Modal */}
      {showBanModal && selectedViewer && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-gray-800 rounded-lg p-6 max-w-md w-full mx-4">
            <h2 className="text-xl font-bold text-white mb-4">Ban Viewer</h2>
            <p className="text-gray-300 mb-4">
              Ban <span className="font-semibold">{selectedViewer.username}</span> from sending messages?
            </p>
            <div className="mb-4">
              <label className="block text-gray-400 text-sm mb-2">
                Reason (optional)
              </label>
              <textarea
                value={banReason}
                onChange={(e) => setBanReason(e.target.value)}
                placeholder="Enter reason for ban..."
                className="w-full bg-gray-700 text-white rounded px-3 py-2 focus:outline-none focus:ring-2 focus:ring-red-500"
                rows={3}
              />
            </div>
            <div className="flex gap-3">
              <button
                onClick={handleBan}
                disabled={banningId === selectedViewer.id}
                className="flex-1 bg-red-600 hover:bg-red-700 disabled:bg-gray-600 text-white px-4 py-2 rounded font-semibold transition-colors"
              >
                {banningId === selectedViewer.id ? 'Banning...' : 'Ban Viewer'}
              </button>
              <button
                onClick={() => {
                  setShowBanModal(false);
                  setSelectedViewer(null);
                }}
                className="flex-1 bg-gray-700 hover:bg-gray-600 text-white px-4 py-2 rounded font-semibold transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
