/**
 * AddSourceModal Component
 *
 * Modal prompting user to add the shared overlay as a source to one of their overlays.
 */

'use client';

import { useState, useEffect } from 'react';
import { overlaysApi } from '@/lib/api/overlays';
import type { Overlay } from '@/lib/types/overlay';
import toast from 'react-hot-toast';

interface AddSourceModalProps {
  senderName: string;
  senderOverlayId: string;
  onClose: () => void;
  onAdded?: () => void;
}

export function AddSourceModal({ senderName, senderOverlayId, onClose, onAdded }: AddSourceModalProps) {
  const [overlays, setOverlays] = useState<Overlay[]>([]);
  const [selectedOverlay, setSelectedOverlay] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [loadingOverlays, setLoadingOverlays] = useState(true);

  // Fetch user's overlays on mount
  useEffect(() => {
    const fetchOverlays = async () => {
      try {
        setLoadingOverlays(true);
        const data = await overlaysApi.list();
        setOverlays(data);

        if (data.length > 0) {
          setSelectedOverlay(data[0].id);
        }
      } catch (err) {
        console.error('Failed to fetch overlays:', err);
        toast.error('Failed to load overlays');
      } finally {
        setLoadingOverlays(false);
      }
    };

    fetchOverlays();
  }, []);

  const handleAdd = async () => {
    if (!selectedOverlay) return;

    try {
      setLoading(true);

      // Phase 16 will implement this endpoint
      // For now, just log and close modal
      console.log('Adding shared overlay source:', {
        overlayId: selectedOverlay,
        sharedOverlayId: senderOverlayId,
      });

      // TODO Phase 16: Uncomment when endpoint exists
      // await overlaysApi.addSource(selectedOverlay, {
      //   type: 'shared_overlay',
      //   shared_overlay_id: senderOverlayId,
      // });

      toast.success(`Added ${senderName}'s overlay!`);

      if (onAdded) {
        onAdded();
      }
      onClose();
    } catch (err: any) {
      console.error('Failed to add shared overlay:', err);
      toast.error('Failed to add shared overlay');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-60">
      <div className="bg-white rounded-lg shadow-xl max-w-sm w-full mx-4 p-6">
        {/* Title */}
        <h2 className="text-xl font-semibold mb-4">
          Add {senderName}&apos;s overlay to one of yours?
        </h2>

        {loadingOverlays ? (
          <div className="py-8 text-center text-gray-500">Loading overlays...</div>
        ) : (
          <>
            {/* Preview text */}
            <p className="text-sm text-gray-600 mb-4">
              {senderName}&apos;s overlay (shared chat)
            </p>

            {/* Overlay dropdown */}
            <div className="mb-6">
              <label htmlFor="target-overlay-select" className="block text-sm font-medium text-gray-700 mb-2">
                Add to which overlay?
              </label>
              <select
                id="target-overlay-select"
                value={selectedOverlay}
                onChange={(e) => setSelectedOverlay(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                {overlays.map((overlay) => (
                  <option key={overlay.id} value={overlay.id}>
                    {overlay.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3">
              <button
                onClick={onClose}
                disabled={loading}
                className="flex-1 px-4 py-2 text-gray-600 hover:text-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Skip
              </button>
              <button
                onClick={handleAdd}
                disabled={loading || !selectedOverlay}
                className="flex-1 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {loading ? 'Adding...' : 'Add'}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
