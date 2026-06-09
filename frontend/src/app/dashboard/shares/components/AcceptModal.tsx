/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * AcceptModal Component
 *
 * Modal for accepting share requests with overlay selection and expiry options.
 */

'use client'

import { useState, useEffect } from 'react'
import clsx from 'clsx'
import { sharesApi } from '@/lib/api/shares'
import { overlaysApi } from '@/lib/api/overlays'
import { PlatformBadge } from './PlatformBadge'
import { Button } from '@/components/ui/button'
import type { ShareRequest } from '@/lib/types/share'
import type { Overlay } from '@/lib/types/overlay'
import { trackEvent } from '@/lib/analytics'
import toast from 'react-hot-toast'

interface AcceptModalProps {
  request: ShareRequest
  onClose: () => void
  onAccepted: (senderOverlayId: string) => void
  senderPlatform?: string // 'twitch' | 'youtube' | 'kick' | 'tiktok'
}

export function AcceptModal({ request, onClose, onAccepted, senderPlatform }: AcceptModalProps) {
  const [overlays, setOverlays] = useState<Overlay[]>([])
  const [selectedOverlay, setSelectedOverlay] = useState<string>('')
  const [expiryOption, setExpiryOption] = useState<'this_stream' | 'custom' | 'unlimited'>(
    'this_stream'
  )
  const [customHours, setCustomHours] = useState<string>('24')

  const isKickUser = senderPlatform === 'kick'
  const [loading, setLoading] = useState(false)
  const [loadingOverlays, setLoadingOverlays] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // If Kick platform, switch to 'unlimited' since stream lifecycle detection is not supported
  useEffect(() => {
    if (isKickUser && expiryOption === 'this_stream') {
      setExpiryOption('unlimited')
    }
  }, [isKickUser, expiryOption])

  // Fetch user's overlays on mount
  useEffect(() => {
    const fetchOverlays = async () => {
      try {
        setLoadingOverlays(true)
        const data = await overlaysApi.list()
        setOverlays(data)

        if (data.length > 0) {
          setSelectedOverlay(data[0].id)
        } else {
          setError('Create an overlay first to accept shares')
        }
      } catch (err) {
        console.error('Failed to fetch overlays:', err)
        setError('Failed to load overlays')
      } finally {
        setLoadingOverlays(false)
      }
    }

    fetchOverlays()
  }, [])

  // Validation logic
  const isValidCustomHours = () => {
    if (expiryOption !== 'custom') return true
    const hours = parseInt(customHours, 10)
    return !isNaN(hours) && hours >= 1 && hours <= 168
  }

  const canSubmit = selectedOverlay && isValidCustomHours() && !loading && !error

  const handleAccept = async () => {
    if (!canSubmit) return

    try {
      setLoading(true)

      const expiryHours = expiryOption === 'custom' ? parseInt(customHours, 10) : undefined
      const response = await sharesApi.acceptRequest(
        request.id,
        selectedOverlay,
        expiryOption,
        expiryHours
      )

      trackEvent('share_accepted')
      toast.success(`Share accepted from ${request.sender?.display_name || 'user'}!`)
      onAccepted(response.sender_overlay_id)
      onClose()
    } catch (err: any) {
      console.error('Failed to accept share:', err)

      const errorMessage = err.response?.data?.error || err.message || 'Failed to accept share'

      if (errorMessage.toLowerCase().includes('circular share')) {
        toast.error('Cannot accept: This would create a circular share dependency')
      } else {
        toast.error(errorMessage)
      }
    } finally {
      setLoading(false)
    }
  }

  // Show error modal if no overlays
  if (error && overlays.length === 0) {
    return (
      <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
        <div className="relative w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl">
          <h2 className="mb-4 text-xl font-semibold text-text">Cannot Accept Share</h2>
          <p className="mb-6 text-text-sub">{error}</p>
          <Button variant="outline" className="w-full" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <div className="relative w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl">
        {/* Title */}
        <h2 className="mb-4 text-xl font-semibold text-text">
          {request.sender?.display_name || 'User'} wants to share with you
        </h2>

        {/* Platform badges */}
        {request.overlay_sources && request.overlay_sources.length > 0 && (
          <div className="mb-4 flex flex-wrap gap-2">
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
              <label
                htmlFor="overlay-select"
                className="mb-2 block text-sm font-medium text-text-sub"
              >
                Share back which overlay? <span className="text-red-400">*</span>
              </label>
              <select
                id="overlay-select"
                value={selectedOverlay}
                onChange={(e) => setSelectedOverlay(e.target.value)}
                className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-text placeholder-text-dim transition-all duration-200 focus-visible:border-blue-500 focus-visible:ring-2 focus-visible:ring-blue-500/20 focus-visible:outline-none"
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
              <label className="mb-2 block text-sm font-medium text-text-sub">
                When should the share expire?
              </label>
              <div className="space-y-2">
                {/* This stream */}
                <label
                  className={clsx(
                    'flex cursor-pointer items-start',
                    isKickUser && 'cursor-not-allowed opacity-50'
                  )}
                >
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
                    <div className="text-sm font-medium text-text">
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
                <label className="flex cursor-pointer items-start">
                  <input
                    type="radio"
                    name="expiry"
                    value="custom"
                    checked={expiryOption === 'custom'}
                    onChange={(e) => setExpiryOption(e.target.value as any)}
                    className="mt-1 mr-2 accent-blue-500"
                  />
                  <div className="flex-1">
                    <div className="text-sm font-medium text-text">Custom duration</div>
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
                            className={clsx(
                              'w-24 rounded-lg border bg-surface-2 px-2 py-1 text-sm text-text transition-all duration-200 focus-visible:ring-2 focus-visible:ring-blue-500/20 focus-visible:outline-none',
                              !isValidCustomHours()
                                ? 'border-red-500 focus-visible:border-red-500'
                                : 'border-border focus-visible:border-blue-500'
                            )}
                          />
                          <span className="text-sm text-text-sub">hours (1-168)</span>
                        </div>
                        {!isValidCustomHours() && (
                          <p className="mt-1 text-xs text-red-400">
                            Must be between 1 and 168 hours
                          </p>
                        )}
                      </div>
                    )}
                  </div>
                </label>

                {/* Unlimited */}
                <label className="flex cursor-pointer items-start">
                  <input
                    type="radio"
                    name="expiry"
                    value="unlimited"
                    checked={expiryOption === 'unlimited'}
                    onChange={(e) => setExpiryOption(e.target.value as any)}
                    className="mt-1 mr-2 accent-blue-500"
                  />
                  <div>
                    <div className="text-sm font-medium text-text">Unlimited</div>
                    <div className="text-xs text-text-dim">Never expires</div>
                  </div>
                </label>
              </div>
            </div>

            {/* Action buttons */}
            <div className="flex gap-3">
              <Button variant="outline" className="flex-1" onClick={onClose} disabled={loading}>
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
  )
}
