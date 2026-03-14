'use client'

import { useState } from 'react'
import toast from 'react-hot-toast'
import { sharesApi } from '@/lib/api/shares'
import { Button } from '@/components/ui/button'

interface RevocationConfirmModalProps {
  partnerName: string
  shareId: string
  onClose: () => void
  onRevoked: () => void
}

export function RevocationConfirmModal({
  partnerName,
  shareId,
  onClose,
  onRevoked,
}: RevocationConfirmModalProps) {
  const [loading, setLoading] = useState(false)

  const handleRevoke = async () => {
    setLoading(true)
    try {
      await sharesApi.revokeShare(shareId)
      toast.success('Share revoked')
      onRevoked()
      onClose()
    } catch (err) {
      toast.error('Failed to revoke share')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <div className="relative w-full max-w-md rounded-xl border border-border bg-surface p-6 shadow-2xl">
        <h2 className="mb-4 text-xl font-semibold text-text">Revoke share with {partnerName}?</h2>
        <p className="mb-6 text-text-sub">This will stop message delivery immediately.</p>
        <div className="flex gap-3">
          <Button variant="outline" className="flex-1" onClick={onClose} disabled={loading}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            className="flex-1"
            onClick={handleRevoke}
            disabled={loading}
          >
            {loading ? 'Revoking...' : 'Revoke'}
          </Button>
        </div>
      </div>
    </div>
  )
}
