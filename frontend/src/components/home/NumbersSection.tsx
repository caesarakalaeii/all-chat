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
 * NumbersSection — this week by platform, then the all-time totals row
 * (mockup G). Platform rows are zero-guarded like the old stat strip: a
 * platform that delivered nothing this week does not render, so a quiet week
 * never shows an embarrassing 0%. The white totals row always shows.
 */

'use client'

import { useTranslations, formatNumber } from '@/lib/i18n'

export interface NumbersSectionProps {
  /** Weekly message count per platform; undefined until the stats fetch lands. */
  platforms: Record<string, number> | null
  /** Ticking all-time counter, formatted by the parent (shared with the hero). */
  totalDisplay: string
  users: number
  overlaysLive: number
}

export function NumbersSection({
  platforms,
  totalDisplay,
  users,
  overlaysLive,
}: NumbersSectionProps) {
  const t = useTranslations()

  // Zero-guard: only platforms that actually delivered messages this week.
  const active = Object.keys(platforms ?? {})
    .filter((platform) => (platforms?.[platform] ?? 0) > 0)
    .sort((a, b) => (platforms?.[b] ?? 0) - (platforms?.[a] ?? 0))
  const weeklyTotal = active.reduce((sum, platform) => sum + (platforms?.[platform] ?? 0), 0)

  return (
    <section className="lanes-section" data-reveal>
      <span className="mono-label">{t('marketing.numbers.label')}</span>
      {active.length > 0 && (
        <div className="numbers five">
          {active.map((platform) => {
            const count = platforms?.[platform] ?? 0
            const share = Math.round((count / weeklyTotal) * 100)
            return (
              <div key={platform}>
                <div className="figure" style={{ color: `var(--lanes-${platform})` }}>
                  {formatNumber(count)}
                </div>
                <div className="figure-caption">
                  {t('marketing.numbers.platformLabel', {
                    platform: platform.toUpperCase(),
                    share,
                  })}
                </div>
              </div>
            )
          })}
        </div>
      )}
      <div className="numbers">
        <div>
          <div className="figure" style={{ color: '#fff' }}>
            {totalDisplay}
          </div>
          <div className="figure-caption">{t('marketing.numbers.totalLabel')}</div>
        </div>
        <div>
          <div className="figure" style={{ color: '#fff' }}>
            {formatNumber(users)}
          </div>
          <div className="figure-caption">{t('marketing.numbers.usersLabel')}</div>
        </div>
        <div>
          <div className="figure" style={{ color: '#fff' }}>
            {formatNumber(overlaysLive)}
          </div>
          <div className="figure-caption">{t('marketing.numbers.overlaysLabel')}</div>
        </div>
      </div>
    </section>
  )
}
