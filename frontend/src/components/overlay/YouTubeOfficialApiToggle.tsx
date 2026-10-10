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

import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { Field } from '@/components/ui/field'
import { Switch } from '@/components/ui/switch'
import { ApiError } from '@/lib/api/client'
import { overlaysApi } from '@/lib/api/overlays'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ChatSource, YouTubeSourceConfig } from '@/lib/types/overlay'

/**
 * Premium opt-in that hands a channel the streamer owns to the Data API listener,
 * which sees unlisted and members-only broadcasts. The server re-checks premium
 * and channel ownership, so `isPremium` only decides what the UI offers.
 */
export function YouTubeOfficialApiToggle({
  source,
  overlayId,
  isPremium,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  isPremium: boolean
  onSaved: () => void
}) {
  const t = useTranslations()
  const stored = ((source.config ?? {}) as YouTubeSourceConfig).official_api === true
  // Holds the saved value until the parent refetches the source, so the switch
  // does not snap back in between. Keyed on the config object it was saved
  // against: once a fresh config arrives, the stored value is the truth again.
  const [saved, setSaved] = useState<{ config: ChatSource['config']; value: boolean } | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const checked = saved !== null && saved.config === source.config ? saved.value : stored
  // A lapsed opt-in stays stored server-side; switching it off must stay possible.
  const locked = !isPremium && !checked

  async function save(next: boolean) {
    setSaving(true)
    setError(null)
    try {
      await overlaysApi.updateSourceConfig(overlayId, source.id, {
        ...source.config,
        official_api: next,
      })
      setSaved({ config: source.config, value: next })
      toastManager.add({
        title: t(
          next
            ? 'overlayEditor.toasts.officialApiEnabled'
            : 'overlayEditor.toasts.officialApiDisabled'
        ),
        type: 'success',
      })
      onSaved()
    } catch (err) {
      // 403 (not premium) and 409 (channel not connected) carry copy written for
      // the streamer in `message`; anything else is an outage, not their choice.
      const message =
        err instanceof ApiError && (err.status === 403 || err.status === 409)
          ? typeof err.data?.message === 'string'
            ? err.data.message
            : err.message
          : null
      setError(message ?? t('overlayEditor.officialApi.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-2">
      <Field.Root invalid={error !== null}>
        <div className="flex items-center justify-between gap-3">
          <Field.Label className="text-xs text-text-sub">
            {t('overlayEditor.officialApi.label')}
          </Field.Label>
          <Switch.Root
            checked={checked}
            disabled={saving || locked}
            // Base UI renders the root as a <span> with data-disabled, which the
            // primitive's `disabled:` variants never match.
            className="data-disabled:cursor-not-allowed data-disabled:opacity-50"
            onCheckedChange={(next: boolean) => void save(next)}
          >
            <Switch.Thumb />
          </Switch.Root>
        </div>
        <Field.Description className="text-xs">
          {t('overlayEditor.officialApi.description')}
        </Field.Description>
        {error && (
          <Field.Error match role="alert" className="text-xs">
            {error}
          </Field.Error>
        )}
      </Field.Root>
      {!isPremium && (
        <p className="text-xs text-text-dim">
          {checked ? (
            t('overlayEditor.officialApi.lapsed')
          ) : (
            <>
              <PremiumUpsellLink />
              {t('overlayEditor.officialApi.upsellSuffix')}
            </>
          )}
        </p>
      )}
    </div>
  )
}
