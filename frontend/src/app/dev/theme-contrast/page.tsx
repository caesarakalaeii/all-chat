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
 * Theme-contrast harness — DEV ONLY (404s in production).
 *
 * Renders every bundled theme through the real `ThemePreview` so the message-text
 * WCAG gate (tests/e2e/theme-contrast.spec.ts) can measure all of them on one page.
 * This decouples the gate from the marketing homepage, which now only teases a
 * curated few — so the landing page can stay uncluttered while contrast coverage
 * stays complete. Not linked from anywhere; excluded from search and prod builds.
 */

import type { Metadata } from 'next'
import { notFound } from 'next/navigation'
import ThemePreview from '@/components/theme-marketplace/ThemePreview'
import { getBundledThemes } from '@/lib/theme-marketplace/bundled-themes'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'

export const dynamic = 'force-dynamic'
export const metadata: Metadata = { robots: { index: false, follow: false } }

export default function ThemeContrastHarness() {
  if (process.env.NODE_ENV === 'production') notFound()

  const themes = getBundledThemes()

  return (
    <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-8 p-6">
      <header>
        <h1 className="text-xl font-bold text-text">Theme contrast harness</h1>
        <p className="text-sm text-text-sub">
          Dev-only. Renders every bundled theme for the message-text WCAG gate. {themes.length}{' '}
          themes.
        </p>
      </header>

      {themes.map((theme) => (
        <section key={theme.id} data-theme-harness={theme.id}>
          <h2 className="mb-2 text-sm font-semibold text-text">
            {theme.name} <span className="font-mono text-xs text-text-dim">{theme.id}</span>
          </h2>
          <ThemePreview
            css={theme.css}
            messages={SAMPLE_PREVIEW_MESSAGES}
            themeId={theme.id}
            height={260}
          />
        </section>
      ))}
    </main>
  )
}
