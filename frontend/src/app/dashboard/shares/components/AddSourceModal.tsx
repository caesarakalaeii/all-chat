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
 * AddSourceModal Component
 *
 * Modal prompting user to add the shared overlay as a source to one of their overlays.
 */

'use client'

import { useState, useEffect } from 'react'
import { overlaysApi } from '@/lib/api/overlays'
import { Dialog, DialogTitle } from '@/components/ui/dialog'
import type { Overlay } from '@/lib/types/overlay'
import { trackEvent } from '@/lib/analytics'
import { toastManager } from '@/lib/toast'
import { useTranslations } from '@/lib/i18n'

interface AddSourceModalProps {
  senderName: string
  senderOverlayId: string
  onClose: () => void
  onAdded?: () => void
}

export function AddSourceModal({
  senderName,
  senderOverlayId,
  onClose,
  onAdded,
}: AddSourceModalProps) {
  const t = useTranslations()
  const [overlays, setOverlays] = useState<Overlay[]>([])
  const [selectedOverlay, setSelectedOverlay] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [loadingOverlays, setLoadingOverlays] = useState(true)

  // Fetch user's overlays on mount
  useEffect(() => {
    const fetchOverlays = async () => {
      try {
        setLoadingOverlays(true)
        const data = await overlaysApi.list()
        setOverlays(data)

        if (data.length > 0) {
          setSelectedOverlay(data[0].id)
        }
      } catch (err) {
        console.error('Failed to fetch overlays:', err)
        toastManager.add({ title: t('dashboard.shares.loadOverlaysFailed'), type: 'error' })
      } finally {
        setLoadingOverlays(false)
      }
    }

    fetchOverlays()
  }, [])

  const handleAdd = async () => {
    if (!selectedOverlay) return

    try {
      setLoading(true)

      await overlaysApi.addSource(selectedOverlay, {
        platform: 'shared_overlay',
        channel_id: senderOverlayId,
        channel_name: `${senderName}'s overlay`,
      })
      trackEvent('source_added', { platform: 'shared_overlay' })

      toastManager.add({
        title: t('dashboard.shares.addSourceToast', { sender: senderName }),
        type: 'success',
      })

      if (onAdded) {
        onAdded()
      }
      onClose()
    } catch (err: any) {
      console.error('Failed to add shared overlay:', err)
      trackEvent('source_add_failed', { platform: 'shared_overlay' })
      toastManager.add({ title: err?.message || 'Failed to add shared overlay', type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Content size="sm" className="lanes-app rounded-none border-white bg-black">
        {/* Title */}
        <DialogTitle className="mb-4 pr-8 text-xl">
          {t('dashboard.shares.addSourceTitle', { sender: senderName })}
        </DialogTitle>

        {loadingOverlays ? (
          <div className="text-sub py-8 text-center">{t('dashboard.shares.loadingOverlays')}</div>
        ) : (
          <>
            {/* Preview text */}
            <p className="text-sub mb-4 text-sm">
              {t('dashboard.shares.addSourcePreview', { sender: senderName })}
            </p>

            {/* Overlay dropdown */}
            <div className="mb-6">
              <label
                htmlFor="target-overlay-select"
                className="text-sub mb-2 block text-sm font-medium"
              >
                {t('dashboard.shares.addSourceSelectLabel')}
              </label>
              <select
                id="target-overlay-select"
                value={selectedOverlay}
                onChange={(e) => setSelectedOverlay(e.target.value)}
                className="lanes-input w-full px-3 py-2 text-sm"
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
              <button className="lanes-btn ghost flex-1" onClick={onClose} disabled={loading}>
                {t('dashboard.shares.addSourceSkip')}
              </button>
              <button
                className="lanes-btn flex-1"
                onClick={handleAdd}
                disabled={loading || !selectedOverlay}
              >
                {loading
                  ? t('dashboard.shares.addSourceAdding')
                  : t('dashboard.shares.addSourceAdd')}
              </button>
            </div>
          </>
        )}
      </Dialog.Content>
    </Dialog.Root>
  )
}
