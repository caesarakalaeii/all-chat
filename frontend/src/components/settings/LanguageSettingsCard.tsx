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
 * UI language picker (ADR-0066).
 *
 * Which languages are offered is the `ui_languages` feature gate, resolved by
 * share-service on GET /api/v1/ui-languages: 200 offers every language, 403
 * offers English only and says why. English stays selectable in every state,
 * so somebody whose access lapsed is never stuck in a language they can no
 * longer switch away from.
 */

'use client'

import { useEffect, useId, useState } from 'react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ApiError, apiClient } from '@/lib/api/client'
import {
  DEFAULT_LOCALE,
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
  chooseLocale,
  isSupportedLocale,
  useActiveLocale,
  useTranslations,
} from '@/lib/i18n'

type Access = 'checking' | 'open' | 'beta' | 'unknown'

const LANGUAGE_ITEMS = SUPPORTED_LOCALES.map((code) => ({
  value: code,
  label: LOCALE_NATIVE_NAMES[code],
}))

export function LanguageSettingsCard() {
  const t = useTranslations()
  const locale = useActiveLocale()
  const selectId = useId()
  const [access, setAccess] = useState<Access>('checking')

  useEffect(() => {
    let cancelled = false
    apiClient
      .get<{ enabled: boolean }>('/api/v1/ui-languages')
      .then(() => {
        if (!cancelled) setAccess('open')
      })
      .catch((err: unknown) => {
        if (!cancelled)
          setAccess(err instanceof ApiError && err.status === 403 ? 'beta' : 'unknown')
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div id="language" className="lanes-panel scroll-mt-24 p-6">
      <h2 className="mb-4 text-lg">{t('settings.language.heading')}</h2>
      <p className="text-sub mb-4 text-sm">{t('settings.language.body')}</p>
      <div className="flex flex-col gap-2">
        <label htmlFor={selectId} className="text-sm font-medium">
          {t('settings.language.selectLabel')}
        </label>
        <Select
          items={LANGUAGE_ITEMS}
          value={locale}
          onValueChange={(value) => {
            if (value !== null && isSupportedLocale(value)) void chooseLocale(value)
          }}
        >
          <SelectTrigger id={selectId} className="w-56 rounded-none">
            <SelectValue />
          </SelectTrigger>
          <SelectContent className="rounded-none">
            {LANGUAGE_ITEMS.map((item) => (
              <SelectItem
                key={item.value}
                value={item.value}
                lang={item.value}
                disabled={item.value !== DEFAULT_LOCALE && access !== 'open'}
              >
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {access === 'beta' && <p className="text-sub text-xs">{t('settings.language.betaNote')}</p>}
        {access === 'unknown' && (
          <p className="text-sub text-xs">{t('settings.language.checkFailed')}</p>
        )}
      </div>
    </div>
  )
}
