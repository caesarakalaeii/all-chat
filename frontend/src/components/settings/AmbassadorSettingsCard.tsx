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
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'

interface ShowcaseState {
  is_ambassador: boolean
  tagline: string | null
  sort_order: number
  featured_consent: boolean
}

export function AmbassadorSettingsCard() {
  const t = useTranslations()
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
        title: next
          ? t('settings.ambassador.toastFeatured')
          : t('settings.ambassador.toastUnfeatured'),
        type: 'success',
      })
    } catch {
      setConsent(prev ?? false) // rollback
      toastManager.add({ title: t('settings.ambassador.toastFailed'), type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card className="p-6">
      <h2 className="mb-4 text-lg font-semibold text-text">{t('settings.ambassador.heading')}</h2>
      <p className="mb-4 text-sm text-text-sub">{t('settings.ambassador.body')}</p>
      <div className="flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-text">{t('settings.ambassador.featureToggle')}</p>
          {tagline && (
            <p className="mt-1 truncate text-xs text-text-sub">
              {t('settings.ambassador.cardReads', { tagline })}
            </p>
          )}
        </div>
        <Switch.Root
          checked={consent ?? false}
          onCheckedChange={toggle}
          disabled={saving || consent === null}
          aria-label={t('settings.ambassador.featureToggle')}
        >
          <Switch.Thumb />
        </Switch.Root>
      </div>
    </Card>
  )
}
