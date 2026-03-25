'use client'

import { useState } from 'react'

interface BanModalProps {
  isOpen: boolean
  onClose: () => void
  onConfirm: (reason: string) => Promise<void>
  username: string
}

export function BanModal({ isOpen, onClose, onConfirm, username }: BanModalProps) {
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState(false)

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!reason.trim()) return

    setLoading(true)
    try {
      await onConfirm(reason)
      onClose()
      setReason('')
    } catch (error) {
      // Error handled by parent
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-[8px]"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-xl border border-border bg-surface p-6"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-xl font-bold text-text">Ban User: {username}</h2>
        <form onSubmit={handleSubmit}>
          <div className="mb-4">
            <label className="mb-2 block text-sm font-medium text-text">Reason for ban *</label>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              className="w-full rounded-lg border border-border bg-bg px-3 py-2 text-text focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
              rows={3}
              placeholder="Spam, abuse, ToS violation, etc..."
              required
            />
          </div>
          <div className="flex justify-end space-x-3">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-border px-4 py-2 text-text transition-colors hover:bg-surface-2 disabled:opacity-50"
              disabled={loading}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded-lg bg-youtube px-4 py-2 text-white transition-colors hover:bg-youtube/80 disabled:opacity-50"
              disabled={loading || !reason.trim()}
            >
              {loading ? 'Banning...' : 'Ban User'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
