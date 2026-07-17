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
 * Shared axe-core scan helper for the a11y smoke suite (WCAG 2.2 AA gate).
 *
 * Product rule, codified once here so it never becomes a blanket disable:
 * overlay chat THEMES are streamer-chosen broadcast art rendered inside OBS.
 * They are NOT app UI and are exempt from app-UI contrast rules (they have
 * their own deliberate floor in theme-contrast.spec.ts). Viewer-defined
 * chatter name colors must never be contrast-gated. Everything else IS app
 * UI and gets the full WCAG 2.2 AA ruleset.
 *
 * Ratchet: tests/e2e/a11y-baseline.json holds the rule IDs that were already
 * violated when the gate was introduced, keyed by page. NEW rule IDs fail
 * immediately. When a batch fixes a rule, remove it from the baseline in the
 * same PR — the stale-entry report below names removable entries. The
 * baseline must only ever shrink.
 */

import { expect, type Page, type TestInfo } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'
import baselineJson from './a11y-baseline.json'

// The JSON carries a __doc string key (JSON has no comments); widen past it.
const baseline = baselineJson as unknown as Record<string, string[]>

/** Selectors rendering themed chat content (broadcast art, not app UI). */
const THEMED_CHAT_SELECTORS = ['.overlay-view', '[data-theme-preview]', '.theme-preview']

const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']

export async function expectNoNewA11yViolations(
  page: Page,
  pageKey: string,
  testInfo: TestInfo
): Promise<void> {
  // Freeze CSS animations/transitions before scanning: axe's color-contrast
  // check cannot resolve backgrounds mid-animation (fade-ins), which made
  // results depend on scan timing. Frozen pages give deterministic results.
  // Injected via evaluate, NOT page.addStyleTag: addStyleTag waits on load
  // events and rejects when an unrelated page error fires meanwhile (e.g. the
  // editor's Monaco CDN script being CSP-blocked in dev).
  await page.evaluate(() => {
    const style = document.createElement('style')
    style.textContent =
      '*, *::before, *::after { animation: none !important; transition: none !important; }'
    document.head.appendChild(style)
  })
  // Let async-hydrating widgets (comboboxes, editors, fetch-gated sections)
  // reach their final DOM so the scan sees the same page every run.
  await page.waitForLoadState('networkidle')

  let builder = new AxeBuilder({ page }).withTags(WCAG_TAGS)
  for (const selector of THEMED_CHAT_SELECTORS) {
    builder = builder.exclude(selector)
  }
  const results = await builder.analyze()

  const known = new Set(baseline[pageKey] ?? [])
  const found = new Map(results.violations.map((v) => [v.id, v]))

  const fresh = results.violations.filter((v) => !known.has(v.id))
  const stale = [...known].filter((id) => !found.has(id))

  // Attach the full report for debugging regardless of outcome.
  await testInfo.attach(`axe-${pageKey}`, {
    body: JSON.stringify(results.violations, null, 2),
    contentType: 'application/json',
  })

  if (stale.length > 0) {
    // Not a failure (fixes land in their own batches), but must not linger:
    // prune these ids from a11y-baseline.json in the PR that fixed them.
    console.warn(
      `[a11y] ${pageKey}: baseline entries no longer violated — prune from a11y-baseline.json: ${stale.join(', ')}`
    )
  }

  expect(
    fresh.map((v) => ({
      id: v.id,
      impact: v.impact,
      help: v.help,
      nodes: v.nodes.slice(0, 5).map((n) => n.target.join(' ')),
    })),
    `NEW a11y violations on ${pageKey} (not in a11y-baseline.json — fix them, do not extend the baseline)`
  ).toEqual([])
}
