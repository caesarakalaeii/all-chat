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

import { Field } from '@/components/ui/field'
import { Switch } from '@/components/ui/switch'
import { overlaysApi } from '@/lib/api/overlays'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ChatSource, VerticalCompanionMode, YouTubeSourceConfig } from '@/lib/types/overlay'

const MULTI_STREAM_STRATEGIES = new Set(['all', 'title_match_all'])

/**
 * Merges the channel's simultaneous vertical broadcast (landscape + vertical
 * streaming gives two broadcasts with separate chats). Free for everyone: it is the
 * behaviour a streamer expects, unlike the premium strategies that follow any number
 * of independent broadcasts. Stored as config.vertical_companion: absent/"" merges
 * a broadcast whose title says it is vertical, "on" merges a second broadcast
 * whatever its title, "off" never merges.
 */
export function YouTubeVerticalCompanionToggle({
  source,
  overlayId,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  onSaved: () => void
}) {
  const t = useTranslations()
  const config = (source.config ?? {}) as YouTubeSourceConfig
  const stored: VerticalCompanionMode = config.vertical_companion ?? ''
  // Holds the saved value until the parent refetches the source, so a switch does
  // not snap back in between (same pattern as YouTubeOfficialApiToggle).
  const [saved, setSaved] = useState<{
    config: ChatSource['config']
    value: VerticalCompanionMode
  } | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mode = saved !== null && saved.config === source.config ? saved.value : stored

  if (config.stream_select && MULTI_STREAM_STRATEGIES.has(config.stream_select)) {
    return (
      <p className="text-xs text-text-sub">
        {t('overlayEditor.verticalCompanion.multiStreamNote')}
      </p>
    )
  }

  async function save(next: VerticalCompanionMode) {
    setSaving(true)
    setError(null)
    try {
      const nextConfig: Record<string, unknown> = { ...source.config, vertical_companion: next }
      // The default needs no key; keep stored configs as they were.
      if (next === '') delete nextConfig.vertical_companion
      await overlaysApi.updateSourceConfig(overlayId, source.id, nextConfig)
      setSaved({ config: source.config, value: next })
      toastManager.add({ title: t('overlayEditor.toasts.verticalCompanionSaved'), type: 'success' })
      onSaved()
    } catch {
      setError(t('overlayEditor.verticalCompanion.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-3">
      <Field.Root invalid={error !== null}>
        <div className="flex items-center justify-between gap-3">
          <Field.Label className="text-xs text-text-sub">
            {t('overlayEditor.verticalCompanion.label')}
          </Field.Label>
          <Switch.Root
            checked={mode !== 'off'}
            disabled={saving}
            className="data-disabled:cursor-not-allowed data-disabled:opacity-50"
            onCheckedChange={(next: boolean) => void save(next ? '' : 'off')}
          >
            <Switch.Thumb />
          </Switch.Root>
        </div>
        <Field.Description className="text-xs">
          {t('overlayEditor.verticalCompanion.description')}
        </Field.Description>
        {error && (
          <Field.Error match role="alert" className="text-xs">
            {error}
          </Field.Error>
        )}
      </Field.Root>
      {mode !== 'off' && (
        <Field.Root>
          <div className="flex items-center justify-between gap-3">
            <Field.Label className="text-xs text-text-sub">
              {t('overlayEditor.verticalCompanion.unmarkedLabel')}
            </Field.Label>
            <Switch.Root
              checked={mode === 'on'}
              disabled={saving}
              className="data-disabled:cursor-not-allowed data-disabled:opacity-50"
              onCheckedChange={(next: boolean) => void save(next ? 'on' : '')}
            >
              <Switch.Thumb />
            </Switch.Root>
          </div>
          <Field.Description className="text-xs">
            {t('overlayEditor.verticalCompanion.unmarkedDescription')}
          </Field.Description>
        </Field.Root>
      )}
    </div>
  )
}
