'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';

interface AdminStats {
  total_users: number;
  banned_users: number;
  active_overlays: number;
  total_sources: { [platform: string]: number };
}

export default function AdminDashboard() {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function fetchStats() {
      try {
        const token = localStorage.getItem('jwt_token');
        if (!token) {
          setLoading(false);
          return;
        }

        const response = await fetch('/api/v1/admin/stats', {
          headers: {
            'Authorization': `Bearer ${token}`,
          },
        });

        if (response.ok) {
          const data = await response.json();
          setStats(data);
        }
      } catch (err) {
        console.error('Failed to load stats:', err);
      } finally {
        setLoading(false);
      }
    }

    fetchStats();
  }, []);

  const totalSources = stats?.total_sources
    ? Object.values(stats.total_sources).reduce((a, b) => a + b, 0)
    : 0;

  return (
    <div className="px-4 py-6 sm:px-0">
      <h1 className="text-3xl font-bold text-gray-900 mb-6">Admin Dashboard</h1>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
        {/* Users Card */}
        <Link href="/admin/users">
          <div className="bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow cursor-pointer">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <svg className="h-6 w-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                  </svg>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Users
                    </dt>
                    <dd className="text-lg font-semibold text-gray-900">
                      View all users
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
            <div className="bg-gray-50 px-5 py-3">
              <div className="text-sm">
                <span className="font-medium text-blue-600 hover:text-blue-500">
                  View all →
                </span>
              </div>
            </div>
          </div>
        </Link>

        {/* Overlays Card */}
        <Link href="/admin/overlays">
          <div className="bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow cursor-pointer">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <svg className="h-6 w-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Overlays
                    </dt>
                    <dd className="text-lg font-semibold text-gray-900">
                      Manage overlays
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
            <div className="bg-gray-50 px-5 py-3">
              <div className="text-sm">
                <span className="font-medium text-green-600 hover:text-green-500">
                  View all →
                </span>
              </div>
            </div>
          </div>
        </Link>

        {/* Sources Card */}
        <Link href="/admin/sources">
          <div className="bg-white overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow cursor-pointer">
            <div className="p-5">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <svg className="h-6 w-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                </div>
                <div className="ml-5 w-0 flex-1">
                  <dl>
                    <dt className="text-sm font-medium text-gray-500 truncate">
                      Sources
                    </dt>
                    <dd className="text-lg font-semibold text-gray-900">
                      View all sources
                    </dd>
                  </dl>
                </div>
              </div>
            </div>
            <div className="bg-gray-50 px-5 py-3">
              <div className="text-sm">
                <span className="font-medium text-purple-600 hover:text-purple-500">
                  View all →
                </span>
              </div>
            </div>
          </div>
        </Link>
      </div>

      {/* Quick Stats */}
      <div className="mt-8">
        <h2 className="text-xl font-semibold text-gray-900 mb-4">Quick Stats</h2>
        {loading ? (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-600">Loading statistics...</p>
          </div>
        ) : stats ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {/* Total Users */}
            <div className="p-6 rounded-lg border bg-blue-50 border-blue-200">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600">Total Users</p>
                  <p className="text-3xl font-bold mt-1">{stats.total_users}</p>
                </div>
                <div className="text-4xl">👥</div>
              </div>
            </div>

            {/* Banned Users */}
            <div className="p-6 rounded-lg border bg-red-50 border-red-200">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600">Banned Users</p>
                  <p className="text-3xl font-bold mt-1">{stats.banned_users}</p>
                </div>
                <div className="text-4xl">🚫</div>
              </div>
            </div>

            {/* Active Overlays */}
            <div className="p-6 rounded-lg border bg-green-50 border-green-200">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600">Active Overlays</p>
                  <p className="text-3xl font-bold mt-1">{stats.active_overlays}</p>
                </div>
                <div className="text-4xl">📺</div>
              </div>
            </div>

            {/* Total Sources */}
            <div className="p-6 rounded-lg border bg-purple-50 border-purple-200">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm text-gray-600">Total Sources</p>
                  <p className="text-3xl font-bold mt-1">{totalSources}</p>
                </div>
                <div className="text-4xl">📡</div>
              </div>
            </div>
          </div>
        ) : (
          <div className="bg-white shadow rounded-lg p-6">
            <p className="text-gray-600">Failed to load statistics.</p>
          </div>
        )}
      </div>
    </div>
  );
}
