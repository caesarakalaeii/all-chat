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
 * Language switch in the homepage nav (ADR-0078). Deliberately ungated: a
 * signed-out visitor cannot pass the ui_languages probe the /settings card
 * uses, and the homepage is where they decide whether to sign in at all.
 */

'use client'

import {
  LOCALE_NATIVE_NAMES,
  SUPPORTED_LOCALES,
  chooseLocale,
  isSupportedLocale,
  useActiveLocale,
  useTranslations,
} from '@/lib/i18n'

export function LanguageToggle() {
  const t = useTranslations()
  const locale = useActiveLocale()

  // A native select, not a button per language: the nav row has no spare
  // pixels at 360px, and a closed select stays one code wide however many
  // languages there are.
  return (
    <select
      className="lanes-lang"
      aria-label={t('marketing.lanes.languageLabel')}
      value={locale}
      onChange={(event) => {
        if (isSupportedLocale(event.target.value)) void chooseLocale(event.target.value)
      }}
    >
      {SUPPORTED_LOCALES.map((code) => (
        <option
          key={code}
          value={code}
          lang={code}
          aria-label={`${code}, ${LOCALE_NATIVE_NAMES[code]}`}
        >
          {code}
        </option>
      ))}
    </select>
  )
}
