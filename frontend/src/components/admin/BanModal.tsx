'use client';

import { useState } from 'react';

interface BanModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (reason: string) => Promise<void>;
  username: string;
}

export function BanModal({ isOpen, onClose, onConfirm, username }: BanModalProps) {
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!reason.trim()) return;

    setLoading(true);
    try {
      await onConfirm(reason);
      onClose();
      setReason('');
    } catch (error) {
      // Error handled by parent
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-lg bg-white p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-xl font-bold">Ban User: {username}</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="mb-2 block text-sm font-medium">Reason for ban *</label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="w-full rounded-md border border-slate-300 px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
              rows={3}
              placeholder="Spam, abuse, ToS violation, etc..."
              required
            />
          </div>
          <div className="flex justify-end space-x-3">
            <button
              type="button"
              onClick={onClose}
              className="rounded px-4 py-2 text-slate-700 hover:bg-slate-100 disabled:opacity-50"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded bg-red-600 px-4 py-2 text-white hover:bg-red-700 disabled:opacity-50"
              disabled={loading || !reason.trim()}
            >
              {loading ? 'Banning...' : 'Ban User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
