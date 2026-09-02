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
 * The canonical "add the chat monitor to OBS as a dock" instructions.
 *
 * A sibling of `ObsHelpContent` rather than a mode of it. The two describe
 * different OBS features that happen to share a vendor: that one adds a browser
 * SOURCE that renders on stream, this one adds a browser DOCK that renders
 * beside the mixer. Overloading one component with both would let a change
 * meant for the source drift into the dock copy, which is the drift
 * ObsHelpContent exists to prevent in the first place.
 *
 * Streamlabs' custom browser dock takes the same URL and the same steps, so
 * this is not OBS-specific beyond the menu names.
 */

import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

export function ObsDockHelpContent() {
  const t = useTranslations()
  return (
    <ol className="list-decimal space-y-1.5 pl-5 text-sm text-text-sub">
      <li>
        {interpolateElements(t('onboarding.obsDock.step1'), {
          menu: <strong className="text-text">{t('onboarding.obsDock.step1Menu')}</strong>,
        })}
      </li>
      <li>{t('onboarding.obsDock.step2')}</li>
      <li>{t('onboarding.obsDock.step3')}</li>
    </ol>
  )
}
