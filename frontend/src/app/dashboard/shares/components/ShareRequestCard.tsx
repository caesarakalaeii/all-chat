'use client';

import { useState } from 'react';
import { ShareRequest } from '@/lib/types/share';
import { formatDistanceToNow } from 'date-fns';
import { PlatformBadge } from './PlatformBadge';
import { StatusBadge } from './StatusBadge';
import { AcceptModal } from './AcceptModal';
import { AddSourceModal } from './AddSourceModal';

interface ShareRequestCardProps {
  request: ShareRequest;
  onUpdate: () => void;
}

export function ShareRequestCard({ request, onUpdate }: ShareRequestCardProps) {
  const [showAcceptModal, setShowAcceptModal] = useState(false);
  const [showAddSourceModal, setShowAddSourceModal] = useState(false);
  const [acceptedShare, setAcceptedShare] = useState<{ senderName: string; senderOverlayId: string } | null>(null);

  return (
    <div className="bg-white rounded-lg shadow hover:shadow-md transition-shadow p-4">
      {/* User info */}
      <div className="flex items-center mb-3">
        {request.sender && (
          <>
            <img
              src={request.sender.profile_image_url || '/default-avatar.png'}
              alt={request.sender.username}
              className="w-10 h-10 rounded-full"
            />
            <div className="ml-3">
              <p className="font-medium">{request.sender.display_name}</p>
              <p className="text-sm text-gray-500">@{request.sender.username}</p>
            </div>
          </>
        )}
        {!request.sender && (
          <div className="flex items-center">
            <div className="w-10 h-10 rounded-full bg-gray-200"></div>
            <div className="ml-3">
              <p className="text-sm text-gray-500">Loading user info...</p>
            </div>
          </div>
        )}
      </div>

      {/* Platform badges */}
      {request.overlay_sources && request.overlay_sources.length > 0 && (
        <div className="flex gap-2 mb-3 flex-wrap">
          {request.overlay_sources.map((source, idx) => (
            <PlatformBadge key={idx} source={source} />
          ))}
        </div>
      )}

      {/* Timestamp */}
      <p className="text-xs text-gray-500">
        {formatDistanceToNow(new Date(request.created_at), { addSuffix: true })}
      </p>

      {/* Status indicator */}
      <div className="mt-3 pt-3 border-t">
        <StatusBadge status={request.status} />
      </div>

      {/* Action buttons (for pending requests) */}
      {request.status === 'pending' && (
        <div className="mt-3 flex gap-2">
          <button
            className="flex-1 px-3 py-1.5 text-sm bg-blue-500 text-white rounded hover:bg-blue-600 transition-colors"
            onClick={() => setShowAcceptModal(true)}
          >
            Accept
          </button>
          <button
            className="flex-1 px-3 py-1.5 text-sm bg-gray-200 text-gray-700 rounded hover:bg-gray-300 transition-colors"
            onClick={() => {
              // Phase 15: Reject action (implement in future plan)
              console.log('Reject not implemented yet (Phase 15)');
            }}
          >
            Reject
          </button>
        </div>
      )}

      {/* AcceptModal */}
      {showAcceptModal && (
        <AcceptModal
          request={request}
          onClose={() => setShowAcceptModal(false)}
          onAccepted={(senderOverlayId) => {
            setAcceptedShare({
              senderName: request.sender?.display_name || 'User',
              senderOverlayId,
            });
            setShowAcceptModal(false);
            setShowAddSourceModal(true);
          }}
        />
      )}

      {/* AddSourceModal */}
      {showAddSourceModal && acceptedShare && (
        <AddSourceModal
          senderName={acceptedShare.senderName}
          senderOverlayId={acceptedShare.senderOverlayId}
          onClose={() => {
            setShowAddSourceModal(false);
            setAcceptedShare(null);
            onUpdate();
          }}
          onAdded={() => {
            setShowAddSourceModal(false);
            setAcceptedShare(null);
            onUpdate();
          }}
        />
      )}
    </div>
  );
}
