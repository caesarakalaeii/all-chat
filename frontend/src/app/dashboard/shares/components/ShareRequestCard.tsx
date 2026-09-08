'use client'

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

import { useState } from 'react'
import { ShareRequest } from '@/lib/types/share'
import { formatDistanceToNow } from 'date-fns'
import { PlatformBadge } from './PlatformBadge'
import { StatusBadge } from './StatusBadge'
import { AcceptModal } from './AcceptModal'
import { AddSourceModal } from './AddSourceModal'
import { RevocationConfirmModal } from './RevocationConfirmModal'
import { useTranslations } from '@/lib/i18n'

interface ShareRequestCardProps {
  request: ShareRequest
  onUpdate: () => void
}

export function ShareRequestCard({ request, onUpdate }: ShareRequestCardProps) {
  const t = useTranslations()
  const [showAcceptModal, setShowAcceptModal] = useState(false)
  const senderPlatform = request.overlay_sources?.[0]?.platform
  const [showAddSourceModal, setShowAddSourceModal] = useState(false)
  const [showRevokeModal, setShowRevokeModal] = useState(false)
  const [acceptedShare, setAcceptedShare] = useState<{
    senderName: string
    senderOverlayId: string
  } | null>(null)

  return (
    <div className="lanes-panel p-4">
      {/* User info */}
      <div className="mb-3 flex items-center">
        {request.sender && (
          <>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={request.sender.profile_image_url || '/default-avatar.png'}
              alt={request.sender.username}
              className="h-10 w-10 border-2 border-white object-cover"
            />
            <div className="ml-3">
              <p className="font-medium">{request.sender.display_name}</p>
              <p className="text-sub text-sm">@{request.sender.username}</p>
            </div>
          </>
        )}
        {!request.sender && (
          <div className="flex items-center">
            <div className="h-10 w-10 border-2 border-white bg-white/10"></div>
            <div className="ml-3">
              <p className="text-sub text-sm">{t('dashboard.shares.loadingUser')}</p>
            </div>
          </div>
        )}
      </div>

      {/* Platform badges */}
      {request.overlay_sources && request.overlay_sources.length > 0 && (
        <div className="mb-3 flex flex-wrap gap-2">
          {request.overlay_sources.map((source, idx) => (
            <PlatformBadge key={idx} source={source} />
          ))}
        </div>
      )}

      <p className="text-dim text-xs">
        {formatDistanceToNow(new Date(request.created_at), { addSuffix: true })}
      </p>

      {/* Status indicator */}
      <div className="mt-3 border-t border-white/20 pt-3">
        <StatusBadge status={request.status} />
        {request.status === 'accepted' && (
          <button
            className="lanes-btn danger sm mt-2 w-full"
            onClick={() => setShowRevokeModal(true)}
          >
            {t('dashboard.shares.revoke')}
          </button>
        )}
      </div>

      {/* Action buttons (for pending requests) */}
      {request.status === 'pending' && (
        <div className="mt-3 flex gap-2">
          <button className="lanes-btn sm flex-1" onClick={() => setShowAcceptModal(true)}>
            {t('dashboard.shares.accept')}
          </button>
          <button
            className="lanes-btn ghost sm flex-1"
            onClick={() => {
              console.log('Reject not implemented yet (Phase 15)')
            }}
          >
            {t('dashboard.shares.reject')}
          </button>
        </div>
      )}

      {/* AcceptModal */}
      {showAcceptModal && (
        <AcceptModal
          request={request}
          senderPlatform={senderPlatform}
          onClose={() => setShowAcceptModal(false)}
          onAccepted={(senderOverlayId) => {
            setAcceptedShare({
              senderName: request.sender?.display_name || t('dashboard.shares.userFallbackName'),
              senderOverlayId,
            })
            setShowAcceptModal(false)
            setShowAddSourceModal(true)
          }}
        />
      )}

      {/* RevocationConfirmModal */}
      {showRevokeModal && (
        <RevocationConfirmModal
          partnerName={request.sender?.display_name || t('dashboard.shares.userFallbackName')}
          shareId={request.id}
          onClose={() => setShowRevokeModal(false)}
          onRevoked={() => {
            setShowRevokeModal(false)
            onUpdate()
          }}
        />
      )}

      {/* AddSourceModal */}
      {showAddSourceModal && acceptedShare && (
        <AddSourceModal
          senderName={acceptedShare.senderName}
          senderOverlayId={acceptedShare.senderOverlayId}
          onClose={() => {
            setShowAddSourceModal(false)
            setAcceptedShare(null)
            onUpdate()
          }}
          onAdded={() => {
            setShowAddSourceModal(false)
            setAcceptedShare(null)
            onUpdate()
          }}
        />
      )}
    </div>
  )
}
