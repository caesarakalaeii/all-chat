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
 * Admin Cosmetics Catalog Page
 *
 * Allows admins to manage the avatar frame and flair catalog.
 *
 * Features:
 * - Tabs: Frames | Flairs
 * - Entry list: thumbnail, name, Premium badge, delete button
 * - Add form: name, image URL (with blur preview), is_premium toggle, submit
 *
 * Route: /admin/cosmetics
 */

'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { apiClient } from '@/lib/api/client'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { toastManager } from '@/lib/toast'
import { useTranslations } from '@/lib/i18n'
import { cn } from '@/lib/utils'

interface CatalogEntry {
  id: string
  name: string
  image_url: string
  is_premium: boolean
}

interface CatalogListResponse {
  frames?: CatalogEntry[]
  flairs?: CatalogEntry[]
}

// The multiplication sign the delete button draws. Decoration beside an
// aria-label that says the same thing in words, so not copy.
const DELETE_GLYPH = '\u00D7'

export default function AdminCosmeticsPage() {
  const router = useRouter()
  const { user } = useAuthStore()
  const t = useTranslations()

  const [activeTab, setActiveTab] = useState<'frames' | 'flairs'>('frames')
  const [frames, setFrames] = useState<CatalogEntry[]>([])
  const [flairs, setFlairs] = useState<CatalogEntry[]>([])
  const [loading, setLoading] = useState(true)

  // Add form state
  const [addName, setAddName] = useState('')
  const [addImageUrl, setAddImageUrl] = useState('')
  const [addPreviewUrl, setAddPreviewUrl] = useState('')
  const [addIsPremium, setAddIsPremium] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const fetchFrames = useCallback(async () => {
    try {
      const response = await apiClient.get<CatalogListResponse>('/api/v1/admin/cosmetics/frames')
      setFrames(response.frames ?? [])
    } catch {
      toastManager.add({ title: t('admin.cosmetics.loadFramesError'), type: 'error' })
    }
  }, [t])

  const fetchFlairs = useCallback(async () => {
    try {
      const response = await apiClient.get<CatalogListResponse>('/api/v1/admin/cosmetics/flairs')
      setFlairs(response.flairs ?? [])
    } catch {
      toastManager.add({ title: t('admin.cosmetics.loadFlairsError'), type: 'error' })
    }
  }, [t])

  // Stable identity (both fetchers are dependency-free useCallbacks) so the
  // guard effect below can depend on it without refetching every render.
  //
  // No `setLoading(true)` up front: `loading` already starts true, the mount
  // effect is the only caller, and setting state synchronously from an effect
  // is what react-hooks/set-state-in-effect flags.
  const fetchAll = useCallback(async () => {
    try {
      await Promise.all([fetchFrames(), fetchFlairs()])
    } finally {
      setLoading(false)
    }
  }, [fetchFrames, fetchFlairs])

  useEffect(() => {
    if (!user?.is_admin) {
      router.push('/dashboard')
      return
    }
    fetchAll()
  }, [user, router, fetchAll])

  const handleDelete = async (id: string) => {
    try {
      if (activeTab === 'frames') {
        await apiClient.delete(`/api/v1/admin/cosmetics/frames/${id}`)
        toastManager.add({ title: t('admin.cosmetics.frameDeleted'), type: 'success' })
        await fetchFrames()
      } else {
        await apiClient.delete(`/api/v1/admin/cosmetics/flairs/${id}`)
        toastManager.add({ title: t('admin.cosmetics.flairDeleted'), type: 'success' })
        await fetchFlairs()
      }
    } catch {
      toastManager.add({ title: t('admin.cosmetics.deleteError'), type: 'error' })
    }
  }

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!addName.trim() || !addImageUrl.trim()) return

    setSubmitting(true)
    try {
      if (activeTab === 'frames') {
        await apiClient.post('/api/v1/admin/cosmetics/frames', {
          name: addName.trim(),
          image_url: addImageUrl.trim(),
          is_premium: addIsPremium,
        })
        toastManager.add({ title: t('admin.cosmetics.frameAdded'), type: 'success' })
        await fetchFrames()
      } else {
        await apiClient.post('/api/v1/admin/cosmetics/flairs', {
          name: addName.trim(),
          image_url: addImageUrl.trim(),
          is_premium: addIsPremium,
        })
        toastManager.add({ title: t('admin.cosmetics.flairAdded'), type: 'success' })
        await fetchFlairs()
      }
      // Clear form
      setAddName('')
      setAddImageUrl('')
      setAddPreviewUrl('')
      setAddIsPremium(false)
    } catch {
      toastManager.add({ title: t('admin.cosmetics.addError'), type: 'error' })
    } finally {
      setSubmitting(false)
    }
  }

  const currentEntries = activeTab === 'frames' ? frames : flairs
  // Whole keys per entry kind rather than one noun spliced into a sentence:
  // 'Frame' has to inflect and cannot if the render site owns the sentence.
  const isFrames = activeTab === 'frames'

  return (
    <div className="min-h-screen bg-bg">
      <div className="mx-auto max-w-4xl px-4 py-8">
        <div className="mb-6">
          <h1 className="text-2xl font-bold text-text">{t('admin.cosmetics.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('admin.cosmetics.intro')}</p>
        </div>

        {/* Tab bar */}
        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as 'frames' | 'flairs')}
          className="mb-6"
        >
          <TabsList variant="line" className="w-full justify-start border-b border-border">
            <TabsTrigger value="frames">{t('admin.cosmetics.tabFrames')}</TabsTrigger>
            <TabsTrigger value="flairs">{t('admin.cosmetics.tabFlairs')}</TabsTrigger>
          </TabsList>
        </Tabs>

        {/* Entry list */}
        {loading ? (
          <Card className="mb-6 space-y-3 p-6">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-lg" />
            ))}
          </Card>
        ) : (
          <Card className="mb-6 overflow-hidden">
            {currentEntries.length === 0 ? (
              <div className="px-4 py-8 text-center text-sm text-text-sub">
                {isFrames ? t('admin.cosmetics.emptyFrames') : t('admin.cosmetics.emptyFlairs')}
              </div>
            ) : (
              <div className="divide-y divide-border">
                {currentEntries.map((entry) => (
                  <div key={entry.id} className="flex items-center gap-3 px-4 py-3">
                    {/* Thumbnail */}
                    <div className="flex h-16 w-16 flex-shrink-0 items-center justify-center overflow-hidden rounded bg-surface-2">
                      {/* eslint-disable-next-line @next/next/no-img-element */}
                      <img
                        src={entry.image_url}
                        alt={entry.name}
                        className="h-16 w-16 object-contain"
                      />
                    </div>

                    {/* Name and premium badge */}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-medium text-text">{entry.name}</span>
                        {entry.is_premium && (
                          <Badge className="text-xs">{t('admin.cosmetics.badgePremium')}</Badge>
                        )}
                      </div>
                    </div>

                    {/* Delete button */}
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDelete(entry.id)}
                      className="text-text-sub hover:text-destructive"
                      aria-label={t('admin.cosmetics.deleteLabel', { name: entry.name })}
                    >
                      {DELETE_GLYPH}
                    </Button>
                  </div>
                ))}
              </div>
            )}
          </Card>
        )}

        {/* Add form */}
        <Card className="p-6">
          <h2 className="mb-4 text-base font-semibold text-text">
            {isFrames ? t('admin.cosmetics.addFrameHeading') : t('admin.cosmetics.addFlairHeading')}
          </h2>
          <form onSubmit={handleAdd} className="space-y-4">
            <div>
              <label className="mb-1 block text-xs text-text-sub" htmlFor="add-name">
                {t('admin.cosmetics.nameLabel')}
              </label>
              <Input
                id="add-name"
                value={addName}
                onChange={(e) => setAddName(e.target.value)}
                placeholder={
                  isFrames
                    ? t('admin.cosmetics.framePlaceholder')
                    : t('admin.cosmetics.flairPlaceholder')
                }
                required
              />
            </div>

            <div>
              <label className="mb-1 block text-xs text-text-sub" htmlFor="add-image-url">
                {t('admin.cosmetics.imageUrlLabel')}
              </label>
              <div className="flex items-center gap-3">
                <Input
                  id="add-image-url"
                  value={addImageUrl}
                  onChange={(e) => setAddImageUrl(e.target.value)}
                  onBlur={() => setAddPreviewUrl(addImageUrl)}
                  placeholder={t('admin.cosmetics.imageUrlPlaceholder')}
                  required
                  className="flex-1"
                />
                {addPreviewUrl && (
                  <div className="flex h-16 w-16 flex-shrink-0 items-center justify-center overflow-hidden rounded bg-surface-2">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={addPreviewUrl}
                      alt={t('admin.cosmetics.previewAlt')}
                      className="h-16 w-16 rounded object-contain"
                    />
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2">
              <input
                id="add-is-premium"
                type="checkbox"
                checked={addIsPremium}
                onChange={(e) => setAddIsPremium(e.target.checked)}
                className="rounded border-border"
              />
              <label htmlFor="add-is-premium" className="text-sm text-text">
                {t('admin.cosmetics.premiumOnlyLabel')}
              </label>
            </div>

            <Button type="submit" disabled={submitting || !addName.trim() || !addImageUrl.trim()}>
              {submitting
                ? t('admin.cosmetics.submittingButton')
                : isFrames
                  ? t('admin.cosmetics.submitFrame')
                  : t('admin.cosmetics.submitFlair')}
            </Button>
          </form>
        </Card>
      </div>
    </div>
  )
}
