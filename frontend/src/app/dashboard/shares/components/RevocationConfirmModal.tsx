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
import { toastManager } from '@/lib/toast'
import { sharesApi } from '@/lib/api/shares'
import { Button } from '@/components/ui/button'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { useTranslations } from '@/lib/i18n'

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
  const t = useTranslations()
  const [loading, setLoading] = useState(false)

  const handleRevoke = async () => {
    setLoading(true)
    try {
      await sharesApi.revokeShare(shareId)
      toastManager.add({ title: t('dashboard.shares.revokedToast'), type: 'success' })
      onRevoked()
      onClose()
    } catch (err) {
      toastManager.add({ title: t('dashboard.shares.revokeFailedToast'), type: 'error' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <AlertDialog.Root open onOpenChange={(open) => !open && onClose()}>
      <AlertDialog.Content>
        <AlertDialog.Title className="mb-4 text-xl">
          {t('dashboard.shares.revokeTitle', { partner: partnerName })}
        </AlertDialog.Title>
        <AlertDialog.Description className="mb-6 text-base">
          {t('dashboard.shares.revokeBody')}
        </AlertDialog.Description>
        {/* Cancel first in DOM order: least-destructive action receives initial focus */}
        <div className="flex gap-3">
          <Button variant="outline" className="flex-1" onClick={onClose} disabled={loading}>
            {t('dashboard.shares.revokeCancel')}
          </Button>
          <Button
            variant="destructive"
            className="flex-1"
            onClick={handleRevoke}
            disabled={loading}
          >
            {loading ? t('dashboard.shares.revoking') : t('dashboard.shares.revokeConfirm')}
          </Button>
        </div>
      </AlertDialog.Content>
    </AlertDialog.Root>
  )
}
