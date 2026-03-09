'use client';

import { useEffect, useState } from 'react';
import { sharesApi } from '@/lib/api/shares';
import { ShareRequest } from '@/lib/types/share';
import { ShareRequestCard } from './components/ShareRequestCard';
import toast from 'react-hot-toast';

export default function ShareRequestsPage() {
  const [requests, setRequests] = useState<ShareRequest[]>([]);
  const [filter, setFilter] = useState<'pending' | 'history'>('pending');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchRequests();
  }, []);

  async function fetchRequests() {
    try {
      setLoading(true);
      const data = await sharesApi.fetchIncoming();
      setRequests(data);
    } catch (error) {
      console.error('Failed to fetch share requests:', error);
      toast.error('Failed to load share requests');
    } finally {
      setLoading(false);
    }
  }

  // Filter: Pending vs History tabs
  const displayRequests = requests.filter(r => {
    if (filter === 'pending') return r.status === 'pending';
    return ['accepted', 'rejected', 'expired'].includes(r.status);
  });

  // Sort: Most recent first
  const sortedRequests = displayRequests.sort((a, b) =>
    new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );

  const pendingCount = requests.filter(r => r.status === 'pending').length;
  const historyCount = requests.length - pendingCount;

  return (
    <div className="px-4 py-6">
      <h1 className="text-2xl font-bold mb-6">Share Requests</h1>

      {/* Tab Filters */}
      <div className="flex space-x-4 border-b border-gray-200 mb-6">
        <button
          onClick={() => setFilter('pending')}
          className={`pb-2 px-1 text-sm font-medium border-b-2 transition-colors ${
            filter === 'pending'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-600 hover:text-gray-900'
          }`}
        >
          Pending ({pendingCount})
        </button>
        <button
          onClick={() => setFilter('history')}
          className={`pb-2 px-1 text-sm font-medium border-b-2 transition-colors ${
            filter === 'history'
              ? 'border-blue-500 text-blue-600'
              : 'border-transparent text-gray-600 hover:text-gray-900'
          }`}
        >
          History ({historyCount})
        </button>
      </div>

      {/* Loading state */}
      {loading && (
        <div className="text-center py-8 text-gray-500">
          Loading requests...
        </div>
      )}

      {/* Empty state */}
      {!loading && sortedRequests.length === 0 && (
        <div className="text-center py-8 text-gray-500">
          {filter === 'pending'
            ? 'No pending share requests'
            : 'No request history'}
        </div>
      )}

      {/* Card Grid */}
      {!loading && sortedRequests.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {sortedRequests.map(request => (
            <ShareRequestCard
              key={request.id}
              request={request}
              onUpdate={fetchRequests}
            />
          ))}
        </div>
      )}
    </div>
  );
}
