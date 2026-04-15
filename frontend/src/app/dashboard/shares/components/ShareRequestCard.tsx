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
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

interface ShareRequestCardProps {
  request: ShareRequest
  onUpdate: () => void
}

export function ShareRequestCard({ request, onUpdate }: ShareRequestCardProps) {
  const [showAcceptModal, setShowAcceptModal] = useState(false)
  const senderPlatform = request.overlay_sources?.[0]?.platform
  const [showAddSourceModal, setShowAddSourceModal] = useState(false)
  const [showRevokeModal, setShowRevokeModal] = useState(false)
  const [acceptedShare, setAcceptedShare] = useState<{
    senderName: string
    senderOverlayId: string
  } | null>(null)

  return (
    <Card className="p-4 shadow-lg transition-all duration-200 hover:scale-[1.02] hover:shadow-xl">
      {/* User info */}
      <div className="mb-3 flex items-center">
        {request.sender && (
          <>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={request.sender.profile_image_url || '/default-avatar.png'}
              alt={request.sender.username}
              className="h-10 w-10 rounded-full"
            />
            <div className="ml-3">
              <p className="font-medium text-text">{request.sender.display_name}</p>
              <p className="text-sm text-text-sub">@{request.sender.username}</p>
            </div>
          </>
        )}
        {!request.sender && (
          <div className="flex items-center">
            <div className="h-10 w-10 rounded-full bg-surface-2"></div>
            <div className="ml-3">
              <p className="text-sm text-text-sub">Loading user info...</p>
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

      {/* Timestamp */}
      <p className="text-xs text-text-dim">
        {formatDistanceToNow(new Date(request.created_at), { addSuffix: true })}
      </p>

      {/* Status indicator */}
      <div className="mt-3 border-t border-border pt-3">
        <StatusBadge status={request.status} />
        {request.status === 'accepted' && (
          <Button
            variant="destructive"
            size="sm"
            className="mt-2 w-full"
            onClick={() => setShowRevokeModal(true)}
          >
            Revoke
          </Button>
        )}
      </div>

      {/* Action buttons (for pending requests) */}
      {request.status === 'pending' && (
        <div className="mt-3 flex gap-2">
          <Button
            variant="gradient"
            size="sm"
            className="flex-1"
            onClick={() => setShowAcceptModal(true)}
          >
            Accept
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="flex-1"
            onClick={() => {
              // Phase 15: Reject action (implement in future plan)
              console.log('Reject not implemented yet (Phase 15)')
            }}
          >
            Reject
          </Button>
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
              senderName: request.sender?.display_name || 'User',
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
          partnerName={request.sender?.display_name || 'User'}
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
    </Card>
  )
}
