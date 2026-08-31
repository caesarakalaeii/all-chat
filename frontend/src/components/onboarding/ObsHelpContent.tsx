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
 * The canonical "add your overlay to OBS" instructions, shared by the
 * onboarding setup guide (step 4) and the editor's OBS help dialog so the
 * copy can never drift between the two.
 */

import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

export function ObsHelpContent() {
  const t = useTranslations()
  return (
    <ol className="list-decimal space-y-1.5 pl-5 text-sm text-text-sub">
      <li>
        {interpolateElements(t('onboarding.obs.step1'), {
          plus: <strong className="text-text">{t('onboarding.obs.step1Plus')}</strong>,
          browser: <strong className="text-text">{t('onboarding.obs.step1Browser')}</strong>,
        })}
      </li>
      <li>{t('onboarding.obs.step2')}</li>
      <li>{t('onboarding.obs.step3')}</li>
    </ol>
  )
}
