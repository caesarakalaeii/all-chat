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
 * Ambassador settings card (ADR-0041)
 *
 * Shown in Settings only to streamers who hold the admin-granted ambassador role.
 * It lets them opt in/out of the public "Featured Ambassadors" homepage showcase —
 * being an ambassador grants premium + early access immediately, but the public
 * card stays hidden until the streamer flips this consent switch themselves.
 * The tagline/order are admin-curated and shown here read-only for context.
 */

'use client'

import { useEffect, useState } from 'react'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { apiClient } from '@/lib/api/client'
import { toastManager } from '@/lib/toast'

interface ShowcaseState {
  is_ambassador: boolean
  tagline: string | null
  sort_order: number
  featured_consent: boolean
}

export function AmbassadorSettingsCard() {
  // null while loading — keeps the switch disabled until we know the real state.
  const [consent, setConsent] = useState<boolean | null>(null)
  const [tagline, setTagline] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    apiClient
      .get<ShowcaseState>('/api/v1/ambassadors/me/showcase')
      .then((s) => {
        if (!cancelled) {
          setConsent(s.featured_consent)
          setTagline(s.tagline)
        }
      })
      .catch(() => {
        if (!cancelled) setConsent(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const toggle = async (next: boolean) => {
    const prev = consent
    setConsent(next) // optimistic
    setSaving(true)
    try {
      await apiClient.put('/api/v1/ambassadors/me/showcase', { featured_consent: next })
      toastManager.add({
        title: next ? 'You will now appear on the homepage' : 'Removed from the homepage showcase',
        type: 'success',
      })
    } catch {
      setConsent(prev ?? false) // rollback
      toastManager.add({ title: 'Failed to update showcase setting', type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card className="p-6">
      <h2 className="mb-4 text-lg font-semibold text-text">Ambassador</h2>
      <p className="mb-4 text-sm text-text-sub">
        You&rsquo;re an All-Chat ambassador. Choose whether to be featured on the public homepage.
      </p>
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-text">Feature me on the homepage</p>
          {tagline && (
            <p className="mt-1 truncate text-xs text-text-sub">
              Your card reads: &ldquo;{tagline}&rdquo;
            </p>
          )}
        </div>
        <Switch.Root
          checked={consent ?? false}
          onCheckedChange={toggle}
          disabled={saving || consent === null}
          aria-label="Feature me on the homepage"
        >
          <Switch.Thumb />
        </Switch.Root>
      </div>
    </Card>
  )
}
