'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import toast from 'react-hot-toast';
import { BanModal } from '@/components/admin/BanModal';

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

  // Handle impersonation
  const handleImpersonate = async (userId: string) => {
    if (!confirm('Are you sure you want to impersonate this user? This will replace your current session.')) {
      return;
    }

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

      // Store the impersonation token
      localStorage.setItem('jwt_token', data.token);
      localStorage.setItem('impersonating', 'true');
      localStorage.setItem('impersonated_user', data.username);

      // Redirect to home page
      router.push('/');
    } catch (err) {
      console.error('Failed to impersonate user:', err);
      alert('Failed to start impersonation. Please try again.');
    } finally {
      setImpersonating(false);
    }
  };

  // Handle ban user
  const handleBanUser = async (reason: string) => {
    if (!userToBan) return;

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
        const error = await response.json();
        throw new Error(error.error || 'Failed to ban user');
      }

      toast.success(`${userToBan.username} banned successfully`);
      await refetchUsers();

      // Clear selected user if it was the banned one
      if (selectedUser?.id === userToBan.id) {
        setSelectedUser(null);
      }
    } catch (err: any) {
      toast.error(err.message || 'Failed to ban user');
      throw err;
    }
  };

  // Handle unban user
  const handleUnbanUser = async (userId: string, username: string) => {
    if (!confirm(`Are you sure you want to unban ${username}?`)) {
      return;
    }

    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/v1/admin/users/${userId}/unban`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to unban user');
      }

      toast.success(`${username} unbanned successfully`);
      await refetchUsers();
    } catch (err: any) {
      toast.error(err.message || 'Failed to unban user');
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

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading users...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-md p-4">
        <p className="text-red-800">{error}</p>
      </div>
    );
  }

  const bannedCount = users.filter((u) => u.is_banned).length;
  const activeCount = users.filter((u) => !u.is_banned).length;

  return (
    <div className="px-4 py-6 sm:px-0">
      <div className="mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Users</h1>
        <p className="mt-2 text-sm text-gray-600">
          Manage and view all users in the system
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Users List */}
        <div className="lg:col-span-2">
          <div className="bg-white shadow overflow-hidden sm:rounded-lg">
            <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
              <h3 className="text-lg leading-6 font-medium text-gray-900">
                All Users ({users.length})
              </h3>

              {/* Search Input */}
              <div className="mt-4">
                <input
                  type="text"
                  placeholder="Search by username, display name, or platform ID..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>

              {/* Filter Tabs */}
              <div className="mt-4 flex space-x-4 border-b border-gray-200">
                <button
                  onClick={() => setFilter('all')}
                  className={`pb-2 px-1 text-sm font-medium border-b-2 ${
                    filter === 'all'
                      ? 'border-blue-500 text-blue-600'
                      : 'border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300'
                  }`}
                >
                  All ({users.length})
                </button>
                <button
                  onClick={() => setFilter('active')}
                  className={`pb-2 px-1 text-sm font-medium border-b-2 ${
                    filter === 'active'
                      ? 'border-blue-500 text-blue-600'
                      : 'border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300'
                  }`}
                >
                  Active ({activeCount})
                </button>
                <button
                  onClick={() => setFilter('banned')}
                  className={`pb-2 px-1 text-sm font-medium border-b-2 ${
                    filter === 'banned'
                      ? 'border-blue-500 text-blue-600'
                      : 'border-transparent text-gray-600 hover:text-gray-900 hover:border-gray-300'
                  }`}
                >
                  Banned ({bannedCount})
                </button>
              </div>
            </div>
            <ul className="divide-y divide-gray-200">
              {displayUsers.map((user) => (
                <li
                  key={user.id}
                  className={`px-4 py-4 hover:bg-gray-50 cursor-pointer transition-colors ${
                    selectedUser?.id === user.id ? 'bg-blue-50' : ''
                  }`}
                  onClick={() => setSelectedUser(user)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex-1">
                      <div className="flex items-center">
                        <p className="text-sm font-medium text-gray-900">
                          {user.display_name}
                        </p>
                        <div className="ml-2 flex space-x-1">
                          {user.is_banned && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800 border border-red-200">
                              BANNED
                            </span>
                          )}
                          {user.twitch_id && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800">
                              Twitch
                            </span>
                          )}
                          {user.youtube_id && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-800">
                              YouTube
                            </span>
                          )}
                          {user.kick_id && (
                            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                              Kick
                            </span>
                          )}
                        </div>
                      </div>
                      <p className="text-sm text-gray-500">@{user.username}</p>
                      <p className="text-xs text-gray-400 mt-1">
                        Joined {new Date(user.created_at).toLocaleDateString()}
                      </p>
                    </div>
                    <div>
                      <svg className="h-5 w-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
                      </svg>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* User Details Panel */}
        <div className="lg:col-span-1">
          {selectedUser ? (
            <div className="bg-white shadow sm:rounded-lg">
              <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
                <h3 className="text-lg leading-6 font-medium text-gray-900">
                  User Details
                </h3>
              </div>
              <div className="px-4 py-5 sm:p-6">
                <dl className="space-y-4">
                  <div>
                    <dt className="text-sm font-medium text-gray-500">ID</dt>
                    <dd className="mt-1 text-sm text-gray-900 font-mono">{selectedUser.id}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-gray-500">Username</dt>
                    <dd className="mt-1 text-sm text-gray-900">{selectedUser.username}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-gray-500">Display Name</dt>
                    <dd className="mt-1 text-sm text-gray-900">{selectedUser.display_name}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-gray-500">Auth Provider</dt>
                    <dd className="mt-1 text-sm text-gray-900 capitalize">{selectedUser.auth_provider}</dd>
                  </div>
                  <div>
                    <dt className="text-sm font-medium text-gray-500">Connected Platforms</dt>
                    <dd className="mt-2 space-y-1">
                      {selectedUser.twitch_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-purple-600">Twitch:</span>
                          <span className="ml-2 text-gray-700 font-mono text-xs">{selectedUser.twitch_id}</span>
                        </div>
                      )}
                      {selectedUser.youtube_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-red-600">YouTube:</span>
                          <span className="ml-2 text-gray-700 font-mono text-xs">{selectedUser.youtube_id}</span>
                        </div>
                      )}
                      {selectedUser.kick_id && (
                        <div className="flex items-center text-sm">
                          <span className="font-medium text-green-600">Kick:</span>
                          <span className="ml-2 text-gray-700 font-mono text-xs">{selectedUser.kick_id}</span>
                        </div>
                      )}
                    </dd>
                  </div>
                </dl>

                <div className="mt-6 pt-6 border-t border-gray-200">
                  <button
                    onClick={() => handleImpersonate(selectedUser.id)}
                    disabled={impersonating}
                    className="w-full px-4 py-2 bg-orange-600 hover:bg-orange-700 disabled:bg-gray-400 disabled:cursor-not-allowed text-white font-medium rounded-md transition-colors flex items-center justify-center gap-2"
                  >
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                    </svg>
                    View as {selectedUser.username}
                  </button>
                  <p className="mt-2 text-xs text-gray-500 text-center">
                    Temporarily act as this user to debug issues
                  </p>
                </div>

                {/* Ban/Unban Section */}
                <div className="mt-6 pt-6 border-t border-gray-200">
                  {selectedUser.is_banned ? (
                    <>
                      <div className="mb-3 p-3 bg-red-50 border border-red-200 rounded-md">
                        <p className="text-sm font-medium text-red-800">
                          Banned: {selectedUser.banned_reason}
                        </p>
                        <p className="text-xs text-red-600 mt-1">
                          {selectedUser.banned_at && `Banned on ${new Date(selectedUser.banned_at).toLocaleString()}`}
                        </p>
                      </div>
                      <button
                        onClick={() => handleUnbanUser(selectedUser.id, selectedUser.username)}
                        className="w-full px-4 py-2 bg-green-600 hover:bg-green-700 text-white font-medium rounded-md transition-colors"
                      >
                        Unban User
                      </button>
                    </>
                  ) : (
                    <button
                      onClick={() => {
                        setUserToBan(selectedUser);
                        setShowBanModal(true);
                      }}
                      className="w-full px-4 py-2 bg-red-600 hover:bg-red-700 text-white font-medium rounded-md transition-colors"
                    >
                      Ban User
                    </button>
                  )}
                </div>

                <div className="mt-6 pt-6 border-t border-gray-200">
                  <h4 className="text-sm font-medium text-gray-500 mb-2">
                    Overlays ({userOverlays.length})
                  </h4>
                  {userOverlays.length > 0 ? (
                    <ul className="space-y-2">
                      {userOverlays.map((overlay) => (
                        <li key={overlay.id}>
                          <Link
                            href={`/overlays/${overlay.id}`}
                            className="block px-3 py-2 bg-gray-50 rounded-md hover:bg-gray-100 transition-colors"
                          >
                            <div className="text-sm font-medium text-gray-900">{overlay.name}</div>
                            <div className="text-xs text-gray-500">{overlay.sources_count} sources</div>
                          </Link>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-gray-500 italic">No overlays yet</p>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="bg-white shadow sm:rounded-lg">
              <div className="px-4 py-5 sm:p-6 text-center">
                <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
                <p className="mt-2 text-sm text-gray-500">
                  Select a user to view details
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Ban Modal */}
      <BanModal
        isOpen={showBanModal}
        onClose={() => {
          setShowBanModal(false);
          setUserToBan(null);
        }}
        onConfirm={handleBanUser}
        username={userToBan?.username || ''}
      />
    </div>
  );
}
