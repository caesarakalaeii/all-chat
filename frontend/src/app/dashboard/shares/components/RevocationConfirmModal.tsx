'use client';

import { useState } from 'react';
import toast from 'react-hot-toast';
import { sharesApi } from '@/lib/api/shares';

interface RevocationConfirmModalProps {
  partnerName: string;
  shareId: string;
  onClose: () => void;
  onRevoked: () => void;
}

export function RevocationConfirmModal({
  partnerName, shareId, onClose, onRevoked
}: RevocationConfirmModalProps) {
  const [loading, setLoading] = useState(false);

  const handleRevoke = async () => {
    setLoading(true);
    try {
      await sharesApi.revokeShare(shareId);
      toast.success('Share revoked');
      onRevoked();
      onClose();
    } catch (err) {
      toast.error('Failed to revoke share');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <h2 className="text-xl font-semibold mb-4">
          Revoke share with {partnerName}?
        </h2>
        <p className="text-gray-600 mb-6">
          This will stop message delivery immediately.
        </p>
        <div className="flex gap-3">
          <button
            onClick={onClose}
            disabled={loading}
            className="flex-1 px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 disabled:opacity-50 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={handleRevoke}
            disabled={loading}
            className="flex-1 px-4 py-2 text-sm font-medium text-white bg-red-500 rounded-lg hover:bg-red-600 disabled:opacity-50 transition-colors"
          >
            {loading ? 'Revoking...' : 'Revoke'}
          </button>
        </div>
      </div>
    </div>
  );
}
