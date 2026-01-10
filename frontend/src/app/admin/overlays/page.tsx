'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';

interface Overlay {
  id: string;
  name: string;
  user_id: string;
  created_at: string;
  updated_at: string;
  sources_count?: number;
}

interface OverlaySource {
  id: string;
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok';
  channel_id: string;
  channel_name: string;
  is_active: boolean;
  created_at: string;
}

export default function OverlaysPage() {
  const [overlays, setOverlays] = useState<Overlay[]>([]);
  const [selectedOverlay, setSelectedOverlay] = useState<Overlay | null>(null);
  const [sources, setSources] = useState<OverlaySource[]>([]);
  const [loading, setLoading] = useState(true);
  const [sourcesLoading, setSourcesLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  // Fetch all overlays
  useEffect(() => {
    async function fetchOverlays() {
      try {
        const token = localStorage.getItem('jwt_token');
        if (!token) {
          setError('Not authenticated');
          setLoading(false);
          return;
        }

        const response = await fetch('/api/v1/admin/overlays', {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data = await response.json();
        setOverlays(data);
        setLoading(false);
      } catch (err) {
        console.error('Failed to load overlays:', err);
        setError('Failed to load overlays');
        setLoading(false);
      }
    }

    fetchOverlays();
  }, []);

  // Fetch sources for selected overlay
  useEffect(() => {
    async function fetchSources() {
      if (!selectedOverlay) {
        setSources([]);
        return;
      }

      setSourcesLoading(true);
      try {
        const token = localStorage.getItem('jwt_token');
        const response = await fetch(`/api/v1/overlays/${selectedOverlay.id}/sources`, {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          setSources(data);
        } else {
          console.error('Failed to fetch sources:', response.statusText);
          setSources([]);
        }
      } catch (err) {
        console.error('Failed to fetch sources:', err);
        setSources([]);
      } finally {
        setSourcesLoading(false);
      }
    }

    fetchSources();
  }, [selectedOverlay]);

  const getPlatformColor = (platform: string) => {
    switch (platform) {
      case 'twitch': return 'bg-purple-100 text-purple-800';
      case 'youtube': return 'bg-red-100 text-red-800';
      case 'kick': return 'bg-green-100 text-green-800';
      case 'tiktok': return 'bg-pink-100 text-pink-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  // Filter overlays by search term
  const filteredOverlays = overlays.filter((o) => {
    if (!searchTerm) return true;
    const term = searchTerm.toLowerCase();
    return (
      o.name.toLowerCase().includes(term) ||
      o.id.toLowerCase().includes(term) ||
      o.user_id.toLowerCase().includes(term)
    );
  });

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading overlays...</div>
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

  return (
    <div className="px-4 py-6 sm:px-0">
      <div className="mb-6">
        <h1 className="text-3xl font-bold text-gray-900">Overlays</h1>
        <p className="mt-2 text-sm text-gray-600">
          Manage overlays and their connected chat sources
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Overlays List */}
        <div className="lg:col-span-2">
          <div className="bg-white shadow overflow-hidden sm:rounded-lg">
            <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
              <h3 className="text-lg leading-6 font-medium text-gray-900">
                All Overlays ({overlays.length})
              </h3>

              {/* Search Input */}
              <div className="mt-4">
                <input
                  type="text"
                  placeholder="Search by overlay name, ID, or user ID..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="w-full px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
              </div>
            </div>
            <ul className="divide-y divide-gray-200">
              {filteredOverlays.map((overlay) => (
                <li
                  key={overlay.id}
                  className={`px-4 py-4 hover:bg-gray-50 cursor-pointer transition-colors ${
                    selectedOverlay?.id === overlay.id ? 'bg-blue-50' : ''
                  }`}
                  onClick={() => setSelectedOverlay(overlay)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex-1">
                      <div className="flex items-center">
                        <p className="text-sm font-medium text-gray-900">
                          {overlay.name}
                        </p>
                        <span className="ml-2 inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800">
                          {overlay.sources_count || 0} sources
                        </span>
                      </div>
                      <p className="text-sm text-gray-500 font-mono text-xs mt-1">
                        ID: {overlay.id}
                      </p>
                      <p className="text-xs text-gray-400 mt-1">
                        Created {new Date(overlay.created_at).toLocaleDateString()}
                      </p>
                    </div>
                    <div className="flex items-center space-x-2">
                      <Link
                        href={`/overlay/${overlay.id}`}
                        target="_blank"
                        className="text-blue-600 hover:text-blue-800 text-sm"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                        </svg>
                      </Link>
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

        {/* Overlay Details & Sources */}
        <div className="lg:col-span-1">
          {selectedOverlay ? (
            <div className="space-y-4">
              {/* Overlay Details */}
              <div className="bg-white shadow sm:rounded-lg">
                <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
                  <h3 className="text-lg leading-6 font-medium text-gray-900">
                    Overlay Details
                  </h3>
                </div>
                <div className="px-4 py-5 sm:p-6">
                  <dl className="space-y-4">
                    <div>
                      <dt className="text-sm font-medium text-gray-500">Name</dt>
                      <dd className="mt-1 text-sm text-gray-900">{selectedOverlay.name}</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-gray-500">ID</dt>
                      <dd className="mt-1 text-sm text-gray-900 font-mono text-xs">{selectedOverlay.id}</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-gray-500">User ID</dt>
                      <dd className="mt-1 text-sm text-gray-900 font-mono text-xs">{selectedOverlay.user_id}</dd>
                    </div>
                  </dl>
                </div>
              </div>

              {/* Sources */}
              <div className="bg-white shadow sm:rounded-lg">
                <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
                  <h3 className="text-lg leading-6 font-medium text-gray-900">
                    Connected Sources ({sources.length})
                  </h3>
                </div>
                <div className="px-4 py-5 sm:p-6">
                  {sourcesLoading ? (
                    <p className="text-sm text-gray-500">Loading sources...</p>
                  ) : sources.length > 0 ? (
                    <ul className="space-y-3">
                      {sources.map((source) => (
                        <li key={source.id} className="border border-gray-200 rounded-lg p-3">
                          <div className="flex items-start justify-between">
                            <div className="flex-1">
                              <div className="flex items-center space-x-2">
                                <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${getPlatformColor(source.platform)}`}>
                                  {source.platform.charAt(0).toUpperCase() + source.platform.slice(1)}
                                </span>
                                {source.is_active ? (
                                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800">
                                    Active
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800">
                                    Inactive
                                  </span>
                                )}
                              </div>
                              <p className="mt-1 text-sm font-medium text-gray-900">
                                {source.channel_name}
                              </p>
                              <p className="mt-1 text-xs text-gray-500 font-mono">
                                {source.channel_id}
                              </p>
                              <p className="mt-1 text-xs text-gray-400">
                                Added {new Date(source.created_at).toLocaleDateString()}
                              </p>
                            </div>
                          </div>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-gray-500 italic">No sources connected</p>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="bg-white shadow sm:rounded-lg">
              <div className="px-4 py-5 sm:p-6 text-center">
                <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
                <p className="mt-2 text-sm text-gray-500">
                  Select an overlay to view details
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
