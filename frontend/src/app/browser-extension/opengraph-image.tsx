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

import { getTranslations } from '@/lib/i18n'
import { SOCIAL_CARD_SIZE, renderLanesCard } from '@/lib/socialCard'

// Module scope, so getTranslations rather than the hook.
const t = getTranslations()

export const alt = t('marketing.extensionPage.socialCardAlt')
export const size = SOCIAL_CARD_SIZE
export const contentType = 'image/png'
export const revalidate = 3600

export default function Image() {
  return renderLanesCard({
    kicker: t('marketing.extensionPage.kicker'),
    titleTop: t('marketing.extensionPage.titleTop'),
    titleBottom: t('marketing.extensionPage.titleBottom'),
    chip: t('marketing.extensionPage.socialCardChip'),
    // "JUST WATCHING?" is three characters wider than the homepage's lines.
    titleSize: 100,
  })
}
