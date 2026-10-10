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

import { Button } from '@/components/ui/button'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { ApiError } from '@/lib/api/client'
import { overlaysApi } from '@/lib/api/overlays'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import type { ChatSource, YouTubeSourceConfig } from '@/lib/types/overlay'

/**
 * Pins one YouTube video to a source, for streams the channel page never lists
 * (unlisted). The server parses and verifies the link, so the raw input is sent
 * as typed and its 400/422 text is what the streamer sees.
 */
export function YouTubeStreamLinkField({
  source,
  overlayId,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  onSaved: () => void
}) {
  const t = useTranslations()
  const pinned = ((source.config ?? {}) as YouTubeSourceConfig).stream_id ?? ''
  const [link, setLink] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function save(streamId: string) {
    setSaving(true)
    setError(null)
    try {
      await overlaysApi.updateSourceConfig(overlayId, source.id, {
        ...source.config,
        stream_id: streamId,
      })
      setLink('')
      toastManager.add({
        title: t(
          streamId
            ? 'overlayEditor.toasts.streamLinkSaved'
            : 'overlayEditor.toasts.streamLinkCleared'
        ),
        type: 'success',
      })
      onSaved()
    } catch (err) {
      // 400 (unparsable) and 422 (another channel's video, or unverifiable) carry
      // copy written for the streamer; anything else is an outage, not their input.
      const userFacing = err instanceof ApiError && (err.status === 400 || err.status === 422)
      setError(userFacing ? err.message : t('overlayEditor.streamLink.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form
      className="space-y-2"
      onSubmit={(event) => {
        event.preventDefault()
        void save(link.trim())
      }}
    >
      <Field.Root invalid={error !== null}>
        <Field.Label className="text-xs text-text-sub">
          {t('overlayEditor.streamLink.label')}
        </Field.Label>
        <Input
          size="sm"
          value={link}
          autoComplete="off"
          spellCheck={false}
          placeholder={t('overlayEditor.streamLink.placeholder')}
          onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
            setLink(event.target.value)
            setError(null)
          }}
        />
        <Field.Description className="text-xs">
          {t('overlayEditor.streamLink.description')}
        </Field.Description>
        {error && (
          <Field.Error match role="alert" className="text-xs">
            {error}
          </Field.Error>
        )}
      </Field.Root>
      <p className="text-xs wrap-break-word text-text-dim">
        {pinned
          ? t('overlayEditor.streamLink.currentPin', { videoId: pinned })
          : t('overlayEditor.streamLink.notPinned')}
      </p>
      <div className="flex gap-2">
        <Button
          type="submit"
          size="sm"
          variant="outline"
          className="flex-1"
          disabled={saving || !link.trim()}
        >
          {saving ? t('overlayEditor.streamLink.saving') : t('overlayEditor.streamLink.save')}
        </Button>
        {pinned && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            disabled={saving}
            onClick={() => void save('')}
          >
            {t('overlayEditor.streamLink.clear')}
          </Button>
        )}
      </div>
    </form>
  )
}
