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

import { DataExportButton } from '@/components/settings/DataExportButton'
import { viewerApi } from '@/lib/api/viewer'
import { useTranslations } from '@/lib/i18n'

/** The viewer's own data export on /settings/viewer (DSGVO Art. 15/20). */
export function ViewerDataExportCard() {
  const t = useTranslations()
  return (
    <div className="lanes-panel p-6">
      <h2 className="mb-1 text-lg">{t('settings.dataExport.viewerHeading')}</h2>
      <p className="text-sub mb-4 text-sm">{t('settings.dataExport.viewerBody')}</p>
      <DataExportButton load={viewerApi.exportData} filename="allchat-viewer-data-export" />
    </div>
  )
}
