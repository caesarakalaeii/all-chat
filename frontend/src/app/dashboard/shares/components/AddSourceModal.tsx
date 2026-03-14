/**
 * AddSourceModal Component
 *
 * Modal prompting user to add the shared overlay as a source to one of their overlays.
 */

'use client';

import { useState, useEffect } from 'react';
import { overlaysApi } from '@/lib/api/overlays';
import { Button } from '@/components/ui/button';
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

      await overlaysApi.addSource(selectedOverlay, {
        platform: 'shared_overlay',
        channel_id: senderOverlayId,
        channel_name: `${senderName}'s overlay`,
      });

      toast.success(`Added ${senderName}'s overlay!`);

      if (onAdded) {
        onAdded();
      }
      onClose();
    } catch (err: any) {
      console.error('Failed to add shared overlay:', err);
      toast.error(err?.message || 'Failed to add shared overlay');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-[60] flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="relative w-full max-w-sm rounded-xl border border-border bg-surface p-6 shadow-2xl">
        {/* Title */}
        <h2 className="text-xl font-semibold text-text mb-4">
          Add {senderName}&apos;s overlay to one of yours?
        </h2>

        {loadingOverlays ? (
          <div className="py-8 text-center text-text-sub">Loading overlays...</div>
        ) : (
          <>
            {/* Preview text */}
            <p className="text-sm text-text-sub mb-4">
              {senderName}&apos;s overlay (shared chat)
            </p>

            {/* Overlay dropdown */}
            <div className="mb-6">
              <label htmlFor="target-overlay-select" className="block text-sm font-medium text-text-sub mb-2">
                Add to which overlay?
              </label>
              <select
                id="target-overlay-select"
                value={selectedOverlay}
                onChange={(e) => setSelectedOverlay(e.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text transition-all duration-200 focus-visible:border-blue-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/20"
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
              <Button
                variant="ghost"
                className="flex-1"
                onClick={onClose}
                disabled={loading}
              >
                Skip
              </Button>
              <Button
                variant="gradient"
                className="flex-1"
                onClick={handleAdd}
                disabled={loading || !selectedOverlay}
              >
                {loading ? 'Adding...' : 'Add'}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
