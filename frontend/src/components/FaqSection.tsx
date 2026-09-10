/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * FaqSection — the visible landing-page FAQ. Renders the shared
 * `FAQ_MESSAGE_STEMS` as a collapsed <details>/<summary> accordion to keep
 * the page compact; the matching FAQPage JSON-LD is emitted from the home
 * route (`app/page.tsx`).
 *
 * The full answer text stays in the DOM even while collapsed (a client component
 * still server-renders its initial markup), so the structured-data verbatim match
 * Google requires is unaffected.
 *
 * Lanes restyle: a .lanes-section (so it owns a snap frame like the other
 * sections) with the black-panel/hard-border vocabulary — no rounded cards.
 */
import { ChevronDown } from 'lucide-react'
import { FAQ_MESSAGE_STEMS } from '@/lib/faq'
import { getTranslations } from '@/lib/i18n'

export function FaqSection() {
  const t = getTranslations()
  return (
    <section className="lanes-section faq">
      <span className="mono-label">{t('marketing.faq.label')}</span>
      <h2>{t('marketing.faq.heading')}</h2>
      <div className="faq-list">
        {FAQ_MESSAGE_STEMS.map((stem) => (
          <details key={stem} className="faq-item group">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-3">
              {t(`marketing.faq.${stem}Question`)}
              <ChevronDown
                className="h-4 w-4 shrink-0 transition-transform group-open:rotate-180"
                aria-hidden="true"
              />
            </summary>
            <p>{t(`marketing.faq.${stem}Answer`)}</p>
          </details>
        ))}
      </div>
    </section>
  )
}
