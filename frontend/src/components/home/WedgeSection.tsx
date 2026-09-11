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
 * WedgeSection — the THEM / ALL·CHAT manifesto comparison (mockup G).
 * The ✕/✓ markers come from the CSS ::before content, not markup.
 */

'use client'

import { useTranslations } from '@/lib/i18n'

const THEM_STEMS = ['them1', 'them2', 'them3', 'them4', 'them5'] as const
const US_STEMS = ['us1', 'us2', 'us3', 'us4', 'us5'] as const

export function WedgeSection() {
  const t = useTranslations()

  return (
    <section className="lanes-section wedge" data-reveal>
      <span className="mono-label">{t('marketing.wedge.label')}</span>
      <h2>
        {t('marketing.wedge.headingTop')}
        <br />
        {t('marketing.wedge.headingMiddle')}{' '}
        <span className="accent">{t('marketing.wedge.headingAccent')}</span>
      </h2>
      <div className="wedge-grid">
        <div className="wcol bad">
          <h3>{t('marketing.wedge.themTitle')}</h3>
          <ul>
            {THEM_STEMS.map((stem) => (
              <li key={stem}>{t(`marketing.wedge.${stem}`)}</li>
            ))}
          </ul>
        </div>
        <div className="wcol good panel-fill">
          <h3>{t('marketing.wedge.usTitle')}</h3>
          <ul>
            {US_STEMS.map((stem) => (
              <li key={stem}>{t(`marketing.wedge.${stem}`)}</li>
            ))}
          </ul>
        </div>
      </div>
    </section>
  )
}
