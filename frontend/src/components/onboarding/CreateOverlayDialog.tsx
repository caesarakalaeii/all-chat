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

/**
 * Step 1 of the onboarding setup guide: create the first overlay without
 * leaving the dashboard, then continue in the editor where the remaining
 * steps live. The standalone /overlays/new page stays as the regular
 * (non-onboarding) path.
 */

import React, { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { useOverlayStore } from '@/lib/stores/overlay-store'
import { useOnboardingStore } from '@/lib/stores/onboarding-store'
import { trackEvent } from '@/lib/analytics'
import { toastManager } from '@/lib/toast'

export interface CreateOverlayDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function CreateOverlayDialog({ open, onOpenChange }: CreateOverlayDialogProps) {
  const router = useRouter()
  const createOverlay = useOverlayStore((s) => s.createOverlay)
  const setActiveOverlay = useOnboardingStore((s) => s.setActiveOverlay)
  const reportStepCompleted = useOnboardingStore((s) => s.reportStepCompleted)
  const [name, setName] = useState('My Stream')
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed || submitting) return
    setSubmitting(true)
    try {
      const overlay = await createOverlay({ name: trimmed })
      trackEvent('overlay_created')
      reportStepCompleted('create_overlay')
      setActiveOverlay(overlay.id)
      onOpenChange(false)
      router.push(`/overlays/${overlay.id}`)
    } catch {
      toastManager.add({
        title: 'Could not create the overlay',
        description: 'Please try again.',
      })
      setSubmitting(false)
    }
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogTitle>Create your overlay</DialogTitle>
        <DialogDescription>
          Give it a name — you&apos;ll connect your chat right after.
        </DialogDescription>
        <form onSubmit={handleSubmit} className="mt-4 space-y-4">
          <Field.Root>
            <Field.Label>Overlay name</Field.Label>
            <Field.Control
              render={
                <Input
                  value={name}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setName(e.target.value)}
                  maxLength={100}
                />
              }
            />
          </Field.Root>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="gradient" disabled={!name.trim() || submitting}>
              {submitting ? 'Creating…' : 'Create overlay'}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog.Root>
  )
}
