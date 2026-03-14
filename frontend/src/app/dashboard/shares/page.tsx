'use client';

import { useEffect, useState } from 'react';
import clsx from 'clsx';
import { sharesApi } from '@/lib/api/shares';
import { ShareRequest } from '@/lib/types/share';
import { ShareRequestCard } from './components/ShareRequestCard';
import { AddSourceModal } from './components/AddSourceModal';
import toast from 'react-hot-toast';

export default function ShareRequestsPage() {
  const [requests, setRequests] = useState<ShareRequest[]>([]);
  const [filter, setFilter] = useState<'pending' | 'history'>('pending');
  const [loading, setLoading] = useState(true);
  const [unseenAcceptances, setUnseenAcceptances] = useState<ShareRequest[]>([]);
  const [showUnseenPrompt, setShowUnseenPrompt] = useState(false);

  useEffect(() => {
    fetchRequests();
    checkUnseenAcceptances();
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

  async function checkUnseenAcceptances() {
    try {
      const unseen = await sharesApi.getUnseenAcceptances();
      if (unseen.length > 0) {
        setUnseenAcceptances(unseen);
        setShowUnseenPrompt(true);
      }
    } catch (error) {
      console.error('Failed to check unseen acceptances:', error);
      // Fail silently - this is a secondary feature
    }
  }

  const handleCloseUnseenPrompt = async () => {
    if (unseenAcceptances.length > 0) {
      try {
        await sharesApi.markAcceptanceSeen(unseenAcceptances[0].id);
        // Remove the first acceptance and continue
        const remaining = unseenAcceptances.slice(1);
        setUnseenAcceptances(remaining);
        if (remaining.length === 0) {
          setShowUnseenPrompt(false);
        }
      } catch (error) {
        console.error('Failed to mark acceptance seen:', error);
        toast.error('Failed to update notification status');
      }
    }
  }

  const handleAddedSource = async () => {
    if (unseenAcceptances.length > 0) {
      try {
        await sharesApi.markAcceptanceSeen(unseenAcceptances[0].id);
        const remaining = unseenAcceptances.slice(1);
        setUnseenAcceptances(remaining);
        if (remaining.length === 0) {
          setShowUnseenPrompt(false);
        }
      } catch (error) {
        console.error('Failed to mark acceptance seen:', error);
        toast.error('Failed to update notification status');
      }
    }
  };

  // Filter: Pending vs History tabs
  const displayRequests = requests.filter(r => {
    if (filter === 'pending') return r.status === 'pending';
    return ['accepted', 'rejected', 'expired', 'revoked'].includes(r.status);
  });

  // Sort: Most recent first
  const sortedRequests = displayRequests.sort((a, b) =>
    new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  );

  const pendingCount = requests.filter(r => r.status === 'pending').length;
  const historyCount = requests.length - pendingCount;

  return (
    <div className="px-4 py-6">
      {/* Add Source Modal for unseen acceptances */}
      {showUnseenPrompt && unseenAcceptances.length > 0 && (
        <AddSourceModal
          senderName={unseenAcceptances[0].sender_display_name || 'Unknown User'}
          senderOverlayId={unseenAcceptances[0].sender_overlay_id}
          onClose={handleCloseUnseenPrompt}
          onAdded={handleAddedSource}
        />
      )}

      <h1 className="text-2xl font-semibold text-text mb-6">Share Requests</h1>

      {/* Tab Filters */}
      <div className="flex space-x-4 border-b border-border mb-6">
        <button
          onClick={() => setFilter('pending')}
          className={clsx(
            'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
            filter === 'pending'
              ? 'border-blue-500 text-text'
              : 'border-transparent text-text-sub hover:text-text'
          )}
        >
          Pending ({pendingCount})
        </button>
        <button
          onClick={() => setFilter('history')}
          className={clsx(
            'border-b-2 px-1 pb-2 text-sm font-medium transition-colors',
            filter === 'history'
              ? 'border-blue-500 text-text'
              : 'border-transparent text-text-sub hover:text-text'
          )}
        >
          History ({historyCount})
        </button>
      </div>

      {/* Loading state */}
      {loading && (
        <div className="text-center py-8 text-text-sub">
          Loading requests...
        </div>
      )}

      {/* Empty state */}
      {!loading && sortedRequests.length === 0 && (
        <div className="text-center py-8 text-text-sub">
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
