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
 * along with this program. If not, <https://www.gnu.org/licenses/>.
 */

/**
 * StepsSection — the three-step setup strip (mockup G). The step numbers are
 * styling, not copy; the CSS calls them .stepnum.
 */

'use client'

import { useTranslations } from '@/lib/i18n'

const STEPS = [
  { num: '01', titleKey: 'signIn', bodyKey: 'signInBody' },
  { num: '02', titleKey: 'addChannels', bodyKey: 'addChannelsBody' },
  { num: '03', titleKey: 'pasteUrl', bodyKey: 'pasteUrlBody' },
] as const

export function StepsSection() {
  const t = useTranslations()

  return (
    <section className="lanes-section" data-reveal>
      <span className="mono-label">{t('marketing.steps.label')}</span>
      <div className="steps">
        {STEPS.map((step) => (
          <div key={step.num} className="step">
            <div className="stepnum">{step.num}</div>
            <h3>{t(`marketing.steps.${step.titleKey}Title`)}</h3>
            <p>{t(`marketing.steps.${step.bodyKey}`)}</p>
          </div>
        ))}
      </div>
    </section>
  )
}
