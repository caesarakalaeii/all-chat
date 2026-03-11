'use client'

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
      <main className="max-w-lg mx-auto px-4 py-12">
        <Card className="p-8">
          <h1 className="text-2xl font-bold text-text mb-2">Create Overlay</h1>
          <p className="text-text-sub text-sm mb-8">
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
                onChange={e => setName(e.target.value)}
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
            <div className="flex gap-3 justify-end">
              <Button
                variant="outline"
                type="button"
                onClick={() => router.back()}
              >
                Cancel
              </Button>
              <Button
                variant="gradient"
                type="submit"
                disabled={isSubmitting || !name.trim()}
              >
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
