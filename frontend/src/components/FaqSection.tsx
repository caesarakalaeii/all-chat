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
 * FaqSection — the visible landing-page FAQ. Renders the shared `FAQ_ITEMS`; the
 * matching FAQPage JSON-LD is emitted from the home route (`app/page.tsx`).
 */
import { FAQ_ITEMS } from '@/lib/faq'

export function FaqSection() {
  return (
    <section className="mx-auto max-w-3xl px-4 pb-16">
      <h2 className="mb-8 text-center text-2xl font-bold text-text">Frequently asked questions</h2>
      <dl className="space-y-3">
        {FAQ_ITEMS.map((item) => (
          <div key={item.question} className="rounded-xl border border-border bg-surface p-5">
            <dt className="mb-1.5 font-semibold text-text">{item.question}</dt>
            <dd className="text-sm text-text-sub">{item.answer}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
