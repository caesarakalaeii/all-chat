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
 * FaqSection — the visible landing-page FAQ. Renders the shared `FAQ_ITEMS` as a
 * collapsed <details>/<summary> accordion to keep the page compact; the matching
 * FAQPage JSON-LD is emitted from the home route (`app/page.tsx`).
 *
 * The full answer text stays in the DOM even while collapsed (a client component
 * still server-renders its initial markup), so the structured-data verbatim match
 * Google requires is unaffected.
 */
import { ChevronDown } from 'lucide-react'
import { FAQ_ITEMS } from '@/lib/faq'

export function FaqSection() {
  return (
    <section className="mx-auto max-w-3xl px-4 py-16">
      <h2 className="mb-8 text-center text-2xl font-bold text-text">Frequently asked questions</h2>
      <div className="space-y-3">
        {FAQ_ITEMS.map((item) => (
          <details key={item.question} className="group rounded-xl border border-border bg-surface">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-3 p-5 font-semibold text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none [&::-webkit-details-marker]:hidden">
              {item.question}
              <ChevronDown
                className="h-4 w-4 shrink-0 text-text-sub transition-transform group-open:rotate-180"
                aria-hidden="true"
              />
            </summary>
            <p className="px-5 pb-5 text-sm text-text-sub">{item.answer}</p>
          </details>
        ))}
      </div>
    </section>
  )
}
