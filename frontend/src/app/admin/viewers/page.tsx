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
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog } from '@/components/ui/dialog';
import { toastManager } from '@/lib/toast';

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
  const [unbanDialogViewer, setUnbanDialogViewer] = useState<ViewerSession | null>(null);

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
      toastManager.add({ title: 'Failed to load viewers', type: 'error' });
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
      toastManager.add({ title: `${selectedViewer.username} banned successfully`, type: 'success' });
      setShowBanModal(false);
      setSelectedViewer(null);
      setBanReason('');
      fetchViewers();
    } catch (error) {
      console.error('Failed to ban viewer:', error);
      toastManager.add({ title: 'Failed to ban viewer', type: 'error' });
    } finally {
      setBanningId(null);
    }
  };

  const handleUnban = async (viewerId: string, username: string) => {
    try {
      setBanningId(viewerId);
      await apiClient.post(`/api/v1/admin/viewers/${viewerId}/unban`, {});
      toastManager.add({ title: `${username} unbanned successfully`, type: 'success' });
      setUnbanDialogViewer(null);
      fetchViewers();
    } catch (error) {
      console.error('Failed to unban viewer:', error);
      toastManager.add({ title: 'Failed to unban viewer', type: 'error' });
    } finally {
      setBanningId(null);
    }
  };

  const bannedCount = viewers.filter((v) => v.is_banned).length;
  const activeCount = viewers.filter((v) => !v.is_banned).length;

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-text">Viewer Management</h1>
          <p className="mt-1 text-sm text-text-sub">Manage viewer sessions and bans</p>
        </div>
        <span className="text-text-sub text-sm">{viewers.length} total</span>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <Card className="p-4">
          <div className="text-xs text-text-sub">Total Viewers</div>
          <div className="text-2xl font-bold text-text">{viewers.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-text-sub">Banned</div>
          <div className="text-2xl font-bold text-destructive">{bannedCount}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-text-sub">Active</div>
          <div className="text-2xl font-bold text-kick">{activeCount}</div>
        </Card>
      </div>

      {/* Viewers Table */}
      {loading ? (
        <Card className="p-6 space-y-3">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </Card>
      ) : (
        <Card className="overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-surface-2 border-b border-border">
                <tr>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Username</th>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Platform</th>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Last Message</th>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Msg Count (1m/1h)</th>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Status</th>
                  <th className="text-left px-4 py-3 text-text-sub font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {viewers.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-text-dim">
                      No viewer sessions found
                    </td>
                  </tr>
                ) : (
                  viewers.map((viewer) => (
                    <tr key={viewer.id} className="hover:bg-surface-2 transition-colors">
                      <td className="px-4 py-3">
                        <div className="text-sm font-medium text-text">{viewer.username}</div>
                        <div className="text-xs text-text-sub">{viewer.display_name}</div>
                      </td>
                      <td className="px-4 py-3">
                        <span className="text-sm text-text-sub capitalize">{viewer.platform}</span>
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {viewer.last_message_at
                          ? formatDistanceToNow(new Date(viewer.last_message_at), { addSuffix: true })
                          : 'Never'}
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {viewer.message_count_1min}/{viewer.message_count_1hour}
                      </td>
                      <td className="px-4 py-3">
                        {viewer.is_banned ? (
                          <div>
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-destructive/10 text-destructive">
                              BANNED
                            </span>
                            {viewer.banned_reason && (
                              <div className="text-xs text-text-dim mt-1">
                                Reason: {viewer.banned_reason}
                              </div>
                            )}
                          </div>
                        ) : (
                          <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-kick/10 text-kick">
                            Active
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {viewer.is_banned ? (
                          <Dialog.Root
                            open={unbanDialogViewer?.id === viewer.id}
                            onOpenChange={(open) => {
                              if (!open) setUnbanDialogViewer(null);
                            }}
                          >
                            <Dialog.Trigger
                              render={
                                <Button
                                  variant="outline"
                                  size="sm"
                                  disabled={banningId === viewer.id}
                                  onClick={() => setUnbanDialogViewer(viewer)}
                                >
                                  {banningId === viewer.id ? 'Unbanning...' : 'Unban'}
                                </Button>
                              }
                            />
                            <Dialog.Content showCloseButton={false}>
                              <Dialog.Title>Unban &ldquo;{viewer.username}&rdquo;?</Dialog.Title>
                              <Dialog.Description>
                                This will restore their ability to send messages.
                              </Dialog.Description>
                              <div className="flex gap-3 justify-end mt-6">
                                <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                                <Button
                                  variant="default"
                                  disabled={banningId === viewer.id}
                                  onClick={() => handleUnban(viewer.id, viewer.username)}
                                >
                                  Unban Viewer
                                </Button>
                              </div>
                            </Dialog.Content>
                          </Dialog.Root>
                        ) : (
                          <Button
                            variant="destructive"
                            size="sm"
                            disabled={banningId === viewer.id}
                            onClick={() => handleBanClick(viewer)}
                          >
                            Ban
                          </Button>
                        )}
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Ban Modal — Dialog with reason textarea */}
      <Dialog.Root
        open={showBanModal}
        onOpenChange={(open) => {
          if (!open) {
            setShowBanModal(false);
            setSelectedViewer(null);
            setBanReason('');
          }
        }}
      >
        <Dialog.Content showCloseButton={false}>
          <Dialog.Title>Ban Viewer &ldquo;{selectedViewer?.username}&rdquo;?</Dialog.Title>
          <Dialog.Description>
            This will prevent {selectedViewer?.username} from sending messages.
          </Dialog.Description>
          <div className="mt-4">
            <label className="block text-sm font-medium text-text-sub mb-2">
              Reason (optional)
            </label>
            <textarea
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              placeholder="Enter reason for ban..."
              className="w-full bg-surface-2 text-text border border-border rounded-lg px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring placeholder:text-text-dim resize-none"
              rows={3}
            />
          </div>
          <div className="flex gap-3 justify-end mt-6">
            <Dialog.Close
              render={
                <Button variant="outline" disabled={banningId === selectedViewer?.id}>
                  Cancel
                </Button>
              }
            />
            <Button
              variant="destructive"
              disabled={banningId === selectedViewer?.id}
              onClick={handleBan}
            >
              {banningId === selectedViewer?.id ? 'Banning...' : 'Ban Viewer'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  );
}
