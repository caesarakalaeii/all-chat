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
import { useTranslations } from '@/lib/i18n'

function NewOverlayContent() {
  const t = useTranslations()
  const router = useRouter()
  const { createOverlay } = useOverlayStore()

  const [name, setName] = useState('')
  const [nameError, setNameError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim()) {
      setNameError(t('overlayEditor.create.nameRequired'))
      return
    }
    setNameError('')
    setIsSubmitting(true)
    try {
      const overlay = await createOverlay({ name: name.trim() })
      trackEvent('overlay_created')
      toastManager.add({
        title: t('overlayEditor.toasts.created', { name: overlay.name }),
        type: 'success',
      })
      router.push(`/overlays/${overlay.id}`)
    } catch (err) {
      toastManager.add({
        title: t('overlayEditor.toasts.createFailed'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-lg px-4 py-12">
        <Card className="p-8">
          <h1 className="mb-2 text-2xl font-bold text-text">{t('overlayEditor.create.heading')}</h1>
          <p className="mb-8 text-sm text-text-sub">{t('overlayEditor.create.body')}</p>
          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <label htmlFor="overlay-name" className="text-sm font-medium text-text">
                {t('overlayEditor.create.nameLabel')}
              </label>
              <Input
                id="overlay-name"
                type="text"
                placeholder={t('overlayEditor.create.namePlaceholder')}
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                minLength={1}
                maxLength={100}
                aria-describedby={nameError ? 'name-error' : undefined}
              />
              {nameError && (
                <p id="name-error" className="text-sm text-destructive" role="alert">
                  {nameError}
                </p>
              )}
            </div>
            <div className="flex justify-end gap-3">
              <Button variant="outline" type="button" onClick={() => router.back()}>
                {t('overlayEditor.create.cancel')}
              </Button>
              <Button variant="gradient" type="submit" disabled={isSubmitting || !name.trim()}>
                {isSubmitting ? (
                  <span className="flex items-center gap-2">
                    <Skeleton className="h-4 w-24 rounded" />
                  </span>
                ) : (
                  t('overlayEditor.create.submit')
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
