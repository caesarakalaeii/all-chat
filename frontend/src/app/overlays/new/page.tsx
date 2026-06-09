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
import { useRouter } from 'next/navigation'
import { AppNav } from '@/components/AppNav'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/lib/toast'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useOverlayStore } from '@/lib/stores/overlay-store'
import { trackEvent } from '@/lib/analytics'
import { ProtectedRoute } from '@/components/ProtectedRoute'

function NewOverlayContent() {
  const router = useRouter()
  const { createOverlay } = useOverlayStore()

  const [name, setName] = useState('')
  const [nameError, setNameError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setNameError('Overlay name is required')
      return
    }
    setNameError('')
    setIsSubmitting(true)
    try {
      const overlay = await createOverlay({ name: name.trim() })
      trackEvent('overlay_created')
      toastManager.add({ title: `"${overlay.name}" created`, type: 'success' })
      router.push(`/overlays/${overlay.id}`)
    } catch (err) {
      toastManager.add({
        title: 'Failed to create overlay',
        description: 'Please try again.',
        type: 'error',
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="mx-auto max-w-lg px-4 py-12">
        <Card className="p-8">
          <h1 className="mb-2 text-2xl font-bold text-text">Create Overlay</h1>
          <p className="mb-8 text-sm text-text-sub">
            Give your overlay a name. You can add chat sources after creation.
          </p>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <label htmlFor="overlay-name" className="text-sm font-medium text-text">
                Overlay Name
              </label>
              <Input
                id="overlay-name"
                type="text"
                placeholder="e.g. Main Stream, TikTok Only"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                minLength={1}
                maxLength={100}
                aria-describedby={nameError ? 'name-error' : undefined}
              />
              {nameError && (
                <p id="name-error" className="text-destructive text-sm" role="alert">
                  {nameError}
                </p>
              )}
            </div>
            <div className="flex justify-end gap-3">
              <Button variant="outline" type="button" onClick={() => router.back()}>
                Cancel
              </Button>
              <Button variant="gradient" type="submit" disabled={isSubmitting || !name.trim()}>
                {isSubmitting ? (
                  <span className="flex items-center gap-2">
                    <Skeleton className="h-4 w-24 rounded" />
                  </span>
                ) : (
                  'Create Overlay'
                )}
              </Button>
            </div>
          </form>
        </Card>
      </main>
    </div>
  )
}

export default function NewOverlayPage() {
  return (
    <ProtectedRoute>
      <NewOverlayContent />
    </ProtectedRoute>
  )
}
