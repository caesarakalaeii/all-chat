/**
 * AcceptModal Component
 *
 * Modal for accepting share requests with overlay selection and expiry options.
 */

'use client';

import { useState, useEffect } from 'react';
import { sharesApi } from '@/lib/api/shares';
import { overlaysApi } from '@/lib/api/overlays';
import { PlatformBadge } from './PlatformBadge';
import { Button } from '@/components/ui/button';
import type { ShareRequest } from '@/lib/types/share';
import type { Overlay } from '@/lib/types/overlay';
import toast from 'react-hot-toast';

interface AcceptModalProps {
  request: ShareRequest;
  onClose: () => void;
  onAccepted: (senderOverlayId: string) => void;
  senderPlatform?: string; // 'twitch' | 'youtube' | 'kick' | 'tiktok'
}

export function AcceptModal({ request, onClose, onAccepted, senderPlatform }: AcceptModalProps) {
  const [overlays, setOverlays] = useState<Overlay[]>([]);
  const [selectedOverlay, setSelectedOverlay] = useState<string>('');
  const [expiryOption, setExpiryOption] = useState<'this_stream' | 'custom' | 'unlimited'>('this_stream');
  const [customHours, setCustomHours] = useState<string>('24');

  const isKickUser = senderPlatform === 'kick';
  const [loading, setLoading] = useState(false);
  const [loadingOverlays, setLoadingOverlays] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // If Kick platform, switch to 'unlimited' since stream lifecycle detection is not supported
  useEffect(() => {
    if (isKickUser && expiryOption === 'this_stream') {
      setExpiryOption('unlimited');
    }
  }, [isKickUser]);

  // Fetch user's overlays on mount
  useEffect(() => {
    const fetchOverlays = async () => {
      try {
        setLoadingOverlays(true);
        const data = await overlaysApi.list();
        setOverlays(data);

        if (data.length > 0) {
          setSelectedOverlay(data[0].id);
        } else {
          setError('Create an overlay first to accept shares');
        }
      } catch (err) {
        console.error('Failed to fetch overlays:', err);
        setError('Failed to load overlays');
      } finally {
        setLoadingOverlays(false);
      }
    };

    fetchOverlays();
  }, []);

  // Validation logic
  const isValidCustomHours = () => {
    if (expiryOption !== 'custom') return true;
    const hours = parseInt(customHours, 10);
    return !isNaN(hours) && hours >= 1 && hours <= 168;
  };

  const canSubmit = selectedOverlay && isValidCustomHours() && !loading && !error;

  const handleAccept = async () => {
    if (!canSubmit) return;

    try {
      setLoading(true);

      const expiryHours = expiryOption === 'custom' ? parseInt(customHours, 10) : undefined;
      const response = await sharesApi.acceptRequest(
        request.id,
        selectedOverlay,
        expiryOption,
        expiryHours
      );

      toast.success(`Share accepted from ${request.sender?.display_name || 'user'}!`);
      onAccepted(response.sender_overlay_id);
      onClose();
    } catch (err: any) {
      console.error('Failed to accept share:', err);

      const errorMessage = err.response?.data?.error || err.message || 'Failed to accept share';

      if (errorMessage.toLowerCase().includes('circular share')) {
        toast.error('Cannot accept: This would create a circular share dependency');
      } else {
        toast.error(errorMessage);
      }
    } finally {
      setLoading(false);
    }
  };

  // Show error modal if no overlays
  if (error && overlays.length === 0) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
        <div className="relative w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl">
          <h2 className="text-xl font-semibold text-text mb-4">Cannot Accept Share</h2>
          <p className="text-text-sub mb-6">{error}</p>
          <Button variant="outline" className="w-full" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
      <div className="relative w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl">
        {/* Title */}
        <h2 className="text-xl font-semibold text-text mb-4">
          {request.sender?.display_name || 'User'} wants to share with you
        </h2>

        {/* Platform badges */}
        {request.overlay_sources && request.overlay_sources.length > 0 && (
          <div className="flex gap-2 mb-4 flex-wrap">
            {request.overlay_sources.map((source, idx) => (
              <PlatformBadge key={idx} source={source} />
            ))}
          </div>
        )}

        {loadingOverlays ? (
          <div className="py-8 text-center text-text-sub">Loading overlays...</div>
        ) : (
          <>
            {/* Overlay dropdown */}
            <div className="mb-4">
              <label htmlFor="overlay-select" className="block text-sm font-medium text-text-sub mb-2">
                Share back which overlay? <span className="text-red-400">*</span>
              </label>
              <select
                id="overlay-select"
                value={selectedOverlay}
                onChange={(e) => setSelectedOverlay(e.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder-text-dim transition-all duration-200 focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/20"
              >
                {overlays.map((overlay) => (
                  <option key={overlay.id} value={overlay.id}>
                    {overlay.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Expiry options */}
            <div className="mb-6">
              <label className="block text-sm font-medium text-text-sub mb-2">
                When should the share expire?
              </label>
              <div className="space-y-2">
                {/* This stream */}
                <label className={`flex items-start cursor-pointer${isKickUser ? ' opacity-50 cursor-not-allowed' : ''}`}>
                  <input
                    type="radio"
                    name="expiry"
                    value="this_stream"
                    checked={expiryOption === 'this_stream'}
                    onChange={(e) => setExpiryOption(e.target.value as any)}
                    disabled={isKickUser}
                    className="mt-1 mr-2 accent-blue-500"
                  />
                  <div>
                    <div className="font-medium text-sm text-text">
                      This stream
                      {isKickUser && (
                        <span className="ml-1 text-xs text-text-dim">
                          (not available for Kick — stream detection not yet supported)
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-text-dim">Expires when your stream ends</div>
                  </div>
                </label>

                {/* Custom duration */}
                <label className="flex items-start cursor-pointer">
                  <input
                    type="radio"
                    name="expiry"
                    value="custom"
                    checked={expiryOption === 'custom'}
                    onChange={(e) => setExpiryOption(e.target.value as any)}
                    className="mt-1 mr-2 accent-blue-500"
                  />
                  <div className="flex-1">
                    <div className="font-medium text-sm text-text">Custom duration</div>
                    {expiryOption === 'custom' && (
                      <div className="mt-2">
                        <div className="flex items-center gap-2">
                          <input
                            type="number"
                            min="1"
                            max="168"
                            value={customHours}
                            onChange={(e) => setCustomHours(e.target.value)}
                            placeholder="hours"
                            className={`w-24 rounded-lg px-2 py-1 text-sm text-text bg-surface-2 border transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 ${
                              !isValidCustomHours() ? 'border-red-500 focus:border-red-500' : 'border-border focus:border-blue-500'
                            }`}
                          />
                          <span className="text-sm text-text-sub">hours (1-168)</span>
                        </div>
                        {!isValidCustomHours() && (
                          <p className="text-xs text-red-400 mt-1">Must be between 1 and 168 hours</p>
                        )}
                      </div>
                    )}
                  </div>
                </label>

                {/* Unlimited */}
                <label className="flex items-start cursor-pointer">
                  <input
                    type="radio"
                    name="expiry"
                    value="unlimited"
                    checked={expiryOption === 'unlimited'}
                    onChange={(e) => setExpiryOption(e.target.value as any)}
                    className="mt-1 mr-2 accent-blue-500"
                  />
                  <div>
                    <div className="font-medium text-sm text-text">Unlimited</div>
                    <div className="text-xs text-text-dim">Never expires</div>
                  </div>
                </label>
              </div>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3">
              <Button
                variant="outline"
                className="flex-1"
                onClick={onClose}
                disabled={loading}
              >
                Cancel
              </Button>
              <Button
                variant="gradient"
                className="flex-1"
                onClick={handleAccept}
                disabled={!canSubmit}
              >
                {loading ? 'Accepting...' : 'Accept'}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
