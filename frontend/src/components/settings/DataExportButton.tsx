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
 * "Download my data" (DSGVO Art. 15/20). `load` fetches the export with the
 * caller's own credential; the result is saved as a dated JSON file.
 */

'use client'

import { useState } from 'react'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import { downloadJson } from '@/lib/utils/download'

export function DataExportButton({
  load,
  filename,
}: {
  load: () => Promise<unknown>
  /** File name without the date and extension, e.g. `allchat-data-export`. */
  filename: string
}) {
  const t = useTranslations()
  const [downloading, setDownloading] = useState(false)

  async function handleDownload() {
    setDownloading(true)
    try {
      const data = await load()
      downloadJson(`${filename}-${new Date().toISOString().slice(0, 10)}.json`, data)
    } catch {
      toastManager.add({
        title: t('settings.dataExport.failedToast'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    } finally {
      setDownloading(false)
    }
  }

  return (
    <button
      className="lanes-btn ghost"
      disabled={downloading}
      onClick={() => void handleDownload()}
    >
      <span>
        {downloading ? t('settings.dataExport.downloading') : t('settings.dataExport.download')}
      </span>
      <span aria-hidden="true">↓</span>
    </button>
  )
}
