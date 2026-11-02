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

'use client'

import { trackEvent } from '@/lib/analytics'
import { EXTENSION_CHROME_URL, EXTENSION_FIREFOX_URL } from '@/lib/constants'
import { useTranslations } from '@/lib/i18n'

/** Store links for the browser extension. A client island so the clicks can be tracked. */
export function ExtensionInstallButtons() {
  const t = useTranslations()
  return (
    <div className="flex flex-col gap-3 sm:flex-row">
      <a
        href={EXTENSION_CHROME_URL}
        target="_blank"
        rel="noopener noreferrer"
        onClick={() => trackEvent('outbound_click', { dest: 'ext_chrome' })}
        className="lanes-btn"
      >
        {t('marketing.extensionPage.installChrome')}
      </a>
      <a
        href={EXTENSION_FIREFOX_URL}
        target="_blank"
        rel="noopener noreferrer"
        onClick={() => trackEvent('outbound_click', { dest: 'ext_firefox' })}
        className="lanes-btn ghost"
      >
        {t('marketing.extensionPage.installFirefox')}
      </a>
    </div>
  )
}
