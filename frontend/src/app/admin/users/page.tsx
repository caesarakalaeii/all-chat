'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog } from '@/components/ui/dialog';
import { toastManager } from '@/lib/toast';

interface User {
  id: string;
  username: string;
  display_name: string;
  auth_provider: string;
  profile_image_url: string;
  created_at: string;
  twitch_id?: string;
  youtube_id?: string;
  kick_id?: string;
  is_banned: boolean;
  banned_at?: string;
  banned_reason?: string;
  banned_by?: string;
}

interface UserOverlay {
  id: string;
  name: string;
  sources_count: number;
}

export default function UsersPage() {
  const router = useRouter();
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [userOverlays, setUserOverlays] = useState<UserOverlay[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [impersonating, setImpersonating] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [filter, setFilter] = useState<'all' | 'active' | 'banned'>('all');
  const [showBanModal, setShowBanModal] = useState(false);
  const [userToBan, setUserToBan] = useState<User | null>(null);
  const [banReason, setBanReason] = useState('');
  const [banLoading, setBanLoading] = useState(false);
  const [impersonateDialogUser, setImpersonateDialogUser] = useState<User | null>(null);
  const [unbanDialogUser, setUnbanDialogUser] = useState<User | null>(null);

  // Fetch all users from the database
  useEffect(() => {
    async function fetchUsers() {
      try {
        const token = localStorage.getItem('jwt_token');
        if (!token) {
          setError('Not authenticated');
          setLoading(false);
          return;
        }

        const response = await fetch('/api/v1/admin/users', {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data = await response.json();
        setUsers(data);
        setLoading(false);
      } catch (err) {
        console.error('Failed to load users:', err);
        setError('Failed to load users');
        setLoading(false);
      }
    }

    fetchUsers();
  }, []);

  // Fetch overlays for selected user
  useEffect(() => {
    async function fetchUserOverlays() {
      if (!selectedUser) {
        setUserOverlays([]);
        return;
      }

      try {
        const token = localStorage.getItem('jwt_token');
        const response = await fetch('/api/v1/overlays', {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const overlays = await response.json();
          setUserOverlays(overlays);
        }
      } catch (err) {
        console.error('Failed to fetch user overlays:', err);
      }
    }

    fetchUserOverlays();
  }, [selectedUser]);

  // Refetch users helper
  const refetchUsers = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch('/api/v1/admin/users', {
        headers: { 'Authorization': `Bearer ${token}` },
      });
      if (response.ok) {
        const data = await response.json();
        setUsers(data);
      }
    } catch (err) {
      console.error('Failed to refetch users:', err);
    }
  };

  // Handle impersonation (called from Dialog confirm)
  const handleImpersonate = async (userId: string) => {
    setImpersonating(true);
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/v1/admin/users/${userId}/impersonate`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('Failed to impersonate user');
      }

      const data = await response.json();

      // Save the original admin token before overwriting it
      localStorage.setItem('admin_token', token || '');

      // Store the impersonation token
      localStorage.setItem('jwt_token', data.token);
      localStorage.setItem('impersonating', 'true');
      localStorage.setItem('impersonated_user', data.username);

      // Redirect to home page
      router.push('/');
    } catch (err) {
      console.error('Failed to impersonate user:', err);
      toastManager.add({ title: 'Failed to start impersonation. Please try again.', type: 'error' });
    } finally {
      setImpersonating(false);
      setImpersonateDialogUser(null);
    }
  };

  // Handle ban user
  const handleBanUser = async (reason: string) => {
    if (!userToBan) return;

    setBanLoading(true);
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/v1/admin/users/${userToBan.id}/ban`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ reason }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to ban user');
      }

      toastManager.add({ title: `${userToBan.username} banned successfully`, type: 'success' });
      setShowBanModal(false);
      setUserToBan(null);
      setBanReason('');
      await refetchUsers();

      // Clear selected user if it was the banned one
      if (selectedUser?.id === userToBan.id) {
        setSelectedUser(null);
      }
    } catch (err: any) {
      toastManager.add({ title: err.message || 'Failed to ban user', type: 'error' });
    } finally {
      setBanLoading(false);
    }
  };

  // Handle unban user (called from Dialog confirm)
  const handleUnbanUser = async (userId: string, username: string) => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/v1/admin/users/${userId}/unban`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to unban user');
      }

      toastManager.add({ title: `${username} unbanned successfully`, type: 'success' });
      setUnbanDialogUser(null);
      await refetchUsers();
    } catch (err: any) {
      toastManager.add({ title: err.message || 'Failed to unban user', type: 'error' });
    }
  };

  // Filter and search users
  const displayUsers = users.filter((u) => {
    // Filter by ban status
    if (filter === 'banned' && !u.is_banned) return false;
    if (filter === 'active' && u.is_banned) return false;

    // Search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      return (
        u.username.toLowerCase().includes(term) ||
        u.display_name.toLowerCase().includes(term) ||
        u.twitch_id?.toLowerCase().includes(term) ||
        u.youtube_id?.toLowerCase().includes(term) ||
        u.kick_id?.toLowerCase().includes(term)
      );
    }

    return true;
  });

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-8">
        <Card className="p-4 border-destructive">
          <p className="text-destructive">{error}</p>
        </Card>
      </div>
    );
  }

  const bannedCount = users.filter((u) => u.is_banned).length;
  const activeCount = users.filter((u) => !u.is_banned).length;

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">Users</h1>
        <p className="mt-1 text-sm text-text-sub">
          Manage and view all users in the system
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Users List */}
        <div className="lg:col-span-2">
          {loading ? (
            <Card className="p-6 space-y-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full rounded-lg" />
              ))}
            </Card>
          ) : (
            <Card className="overflow-hidden">
              <div className="px-4 py-5 border-b border-border">
                <h3 className="text-base font-medium text-text">
                  All Users ({users.length})
                </h3>

                {/* Search Input */}
                <div className="mt-4">
                  <input
                    type="text"
                    placeholder="Search by username, display name, or platform ID..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="w-full px-4 py-2 border border-border rounded-lg bg-surface-2 text-text placeholder:text-text-dim focus:outline-none focus:ring-2 focus:ring-ring"
                  />
                </div>

                {/* Filter Tabs */}
                <div className="mt-4 flex space-x-4 border-b border-border">
                  <button
                    onClick={() => setFilter('all')}
                    className={`pb-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                      filter === 'all'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:text-text hover:border-border'
                    }`}
                  >
                    All ({users.length})
                  </button>
                  <button
                    onClick={() => setFilter('active')}
                    className={`pb-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                      filter === 'active'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:text-text hover:border-border'
                    }`}
                  >
                    Active ({activeCount})
                  </button>
                  <button
                    onClick={() => setFilter('banned')}
                    className={`pb-2 px-1 text-sm font-medium border-b-2 transition-colors ${
                      filter === 'banned'
                        ? 'border-primary text-primary'
                        : 'border-transparent text-text-sub hover:text-text hover:border-border'
                    }`}
                  >
                    Banned ({bannedCount})
                  </button>
                </div>
              </div>
              <ul className="divide-y divide-border">
                {displayUsers.map((user) => (
                  <li
                    key={user.id}
                    className={`px-4 py-4 hover:bg-surface-2 cursor-pointer transition-colors ${
                      selectedUser?.id === user.id ? 'bg-surface-2' : ''
                    }`}
                    onClick={() => setSelectedUser(user)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="flex items-center">
                          <p className="text-sm font-medium text-text">
                            {user.display_name}
                          </p>
                          <div className="ml-2 flex space-x-1">
                            {user.is_banned && (
                              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-destructive/10 text-destructive border border-destructive/20">
                                BANNED
                              </span>
                            )}
                            {user.twitch_id && (
                              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium text-twitch bg-badge-bg">
                                Twitch
                              </span>
                            )}
                            {user.youtube_id && (
                              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium text-youtube bg-badge-bg">
                                YouTube
                              </span>
                            )}
                            {user.kick_id && (
                              <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium text-kick bg-badge-bg">
                                Kick
                              </span>
                            )}
                          </div>
                        </div>
                        <p className="text-sm text-text-sub">@{user.username}</p>
                        <p className="text-xs text-text-dim mt-1">
                          Joined {new Date(user.created_at).toLocaleDateString()}
                        </p>
                      </div>
                      <div>
                        <svg className="h-5 w-5 text-text-dim" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
                        </svg>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>

        {/* User Details Panel */}
        <div className="lg:col-span-1">
          {selectedUser ? (
            <Card className="overflow-hidden">
              <div className="px-4 py-5 border-b border-border">
                <h3 className="text-base font-medium text-text">
                  User Details
                </h3>
              </div>
              <div className="px-4 py-5">
                <dl className="space-y-4">
                  <div>
                    <dt className="text-sm font-medium text-text-sub">ID</dt>
                    <dd className="mt-1 text-sm text-text font-mono break-all">{selectedUser.id}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Username</dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.username}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Display Name</dt>
                    <dd className="mt-1 text-sm text-text">{selectedUser.display_name}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Auth Provider</dt>
                    <dd className="mt-1 text-sm text-text capitalize">{selectedUser.auth_provider}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-text-sub">Connected Platforms</dt>
                    <dd className="mt-2 space-y-1">
                      {selectedUser.twitch_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-twitch">Twitch:</span>
                          <span className="ml-2 text-text-sub font-mono text-xs">{selectedUser.twitch_id}</span>
                        </div>
                      )}
                      {selectedUser.youtube_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-youtube">YouTube:</span>
                          <span className="ml-2 text-text-sub font-mono text-xs">{selectedUser.youtube_id}</span>
                        </div>
                      )}
                      {selectedUser.kick_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-kick">Kick:</span>
                          <span className="ml-2 text-text-sub font-mono text-xs">{selectedUser.kick_id}</span>
                        </div>
                      )}
                    </dd>
                  </div>
                </dl>

                {/* Impersonate — Dialog confirmation */}
                <div className="mt-6 pt-6 border-t border-border">
                  <Dialog.Root
                    open={impersonateDialogUser?.id === selectedUser.id}
                    onOpenChange={(open) => {
                      if (!open) setImpersonateDialogUser(null);
                    }}
                  >
                    <Dialog.Trigger
                      render={
                        <Button
                          variant="outline"
                          className="w-full flex items-center gap-2"
                          disabled={impersonating}
                          onClick={() => setImpersonateDialogUser(selectedUser)}
                        >
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                          </svg>
                          View as {selectedUser.username}
                        </Button>
                      }
                    />
                    <Dialog.Content showCloseButton={false}>
                      <Dialog.Title>Impersonate &ldquo;{selectedUser.username}&rdquo;?</Dialog.Title>
                      <Dialog.Description>
                        This will replace your current session. You can return to admin by using the stored admin token.
                      </Dialog.Description>
                      <div className="flex gap-3 justify-end mt-6">
                        <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                        <Button
                          variant="default"
                          disabled={impersonating}
                          onClick={() => handleImpersonate(selectedUser.id)}
                        >
                          {impersonating ? 'Switching...' : 'Impersonate'}
                        </Button>
                      </div>
                    </Dialog.Content>
                  </Dialog.Root>
                  <p className="mt-2 text-xs text-text-dim text-center">
                    Temporarily act as this user to debug issues
                  </p>
                </div>

                {/* Ban/Unban Section */}
                <div className="mt-6 pt-6 border-t border-border">
                  {selectedUser.is_banned ? (
                    <>
                      <div className="mb-3 p-3 bg-destructive/10 border border-destructive/20 rounded-lg">
                        <p className="text-sm font-medium text-destructive">
                          Banned: {selectedUser.banned_reason}
                        </p>
                        <p className="text-xs text-destructive/70 mt-1">
                          {selectedUser.banned_at && `Banned on ${new Date(selectedUser.banned_at).toLocaleString()}`}
                        </p>
                      </div>
                      {/* Unban — Dialog confirmation */}
                      <Dialog.Root
                        open={unbanDialogUser?.id === selectedUser.id}
                        onOpenChange={(open) => {
                          if (!open) setUnbanDialogUser(null);
                        }}
                      >
                        <Dialog.Trigger
                          render={
                            <Button
                              variant="outline"
                              className="w-full"
                              onClick={() => setUnbanDialogUser(selectedUser)}
                            >
                              Unban User
                            </Button>
                          }
                        />
                        <Dialog.Content showCloseButton={false}>
                          <Dialog.Title>Unban &ldquo;{selectedUser.username}&rdquo;?</Dialog.Title>
                          <Dialog.Description>
                            This will restore their access to the platform.
                          </Dialog.Description>
                          <div className="flex gap-3 justify-end mt-6">
                            <Dialog.Close render={<Button variant="outline">Cancel</Button>} />
                            <Button
                              variant="default"
                              onClick={() => handleUnbanUser(selectedUser.id, selectedUser.username)}
                            >
                              Unban User
                            </Button>
                          </div>
                        </Dialog.Content>
                      </Dialog.Root>
                    </>
                  ) : (
                    <Button
                      variant="destructive"
                      className="w-full"
                      onClick={() => {
                        setUserToBan(selectedUser);
                        setBanReason('');
                        setShowBanModal(true);
                      }}
                    >
                      Ban User
                    </Button>
                  )}
                </div>

                <div className="mt-6 pt-6 border-t border-border">
                  <h4 className="text-sm font-medium text-text-sub mb-2">
                    Overlays ({userOverlays.length})
                  </h4>
                  {userOverlays.length > 0 ? (
                    <ul className="space-y-2">
                      {userOverlays.map((overlay) => (
                        <li key={overlay.id}>
                          <Link
                            href={`/overlay/${overlay.id}`}
                            target="_blank"
                            className="block px-3 py-2 bg-surface-2 rounded-lg hover:bg-surface-2/80 transition-colors border border-border"
                          >
                            <div className="flex items-center justify-between">
                              <div>
                                <div className="text-sm font-medium text-text">{overlay.name}</div>
                                <div className="text-xs text-text-sub">{overlay.sources_count} sources</div>
                              </div>
                              <svg className="h-4 w-4 text-text-dim" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                              </svg>
                            </div>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-text-dim italic">No overlays yet</p>
                  )}
                </div>
              </div>
            </Card>
          ) : (
            <Card className="p-6 text-center">
              <svg className="mx-auto h-12 w-12 text-text-dim" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
              </svg>
              <p className="mt-2 text-sm text-text-sub">
                Select a user to view details
              </p>
            </Card>
          )}
        </div>
      </div>

      {/* Ban Modal — Dialog for ban with reason input */}
      <Dialog.Root
        open={showBanModal}
        onOpenChange={(open) => {
          if (!open) {
            setShowBanModal(false);
            setUserToBan(null);
            setBanReason('');
          }
        }}
      >
        <Dialog.Content showCloseButton={false}>
          <Dialog.Title>Ban &ldquo;{userToBan?.username}&rdquo;?</Dialog.Title>
          <Dialog.Description>
            This will prevent the user from accessing the platform.
          </Dialog.Description>
          <div className="mt-4">
            <label className="block text-sm font-medium text-text-sub mb-2">
              Reason for ban *
            </label>
            <textarea
              value={banReason}
              onChange={(e) => setBanReason(e.target.value)}
              className="w-full px-3 py-2 border border-border rounded-lg bg-surface-2 text-text placeholder:text-text-dim focus:outline-none focus:ring-2 focus:ring-ring resize-none"
              rows={3}
              placeholder="Spam, abuse, ToS violation, etc..."
            />
          </div>
          <div className="flex gap-3 justify-end mt-6">
            <Dialog.Close render={<Button variant="outline" disabled={banLoading}>Cancel</Button>} />
            <Button
              variant="destructive"
              disabled={banLoading || !banReason.trim()}
              onClick={() => handleBanUser(banReason)}
            >
              {banLoading ? 'Banning...' : 'Ban User'}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  );
}
