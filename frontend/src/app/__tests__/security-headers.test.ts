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
 * Contract for the per-route framing policy declared in `next.config.js`.
 *
 * The overlay display widgets (chat / poll / prediction / credits) are public
 * OBS browser sources with no auth or viewer identity, so they must be
 * embeddable anywhere (`frame-ancestors *`). Everything else under `/overlay`
 * — notably the viewer participation page (login + points) and the
 * authenticated monitor — stays frame-locked (`frame-ancestors 'none'`), and
 * the rest of the app allows only same-origin framing (editor SplitView embed).
 *
 * Next.js applies header rules top-to-bottom and the last matching rule wins
 * per header key, so the assertions here pin both the values AND that the
 * embeddable widget rules are declared after the `/overlay/:id*` lockdown.
 */

import { createRequire } from 'node:module'

import { describe, expect, it } from 'vitest'

const require = createRequire(import.meta.url)
// next.config.js is a CommonJS module at the frontend root.
const nextConfig = require('../../../next.config.js') as {
  headers: () => Promise<Array<{ source: string; headers: Array<{ key: string; value: string }> }>>
}

const WIDGET_SOURCES = [
  '/overlay/:id',
  '/overlay/:id/poll',
  '/overlay/:id/prediction',
  '/overlay/:id/credits',
] as const

async function rules() {
  return nextConfig.headers()
}

function ruleFor(
  all: Awaited<ReturnType<typeof rules>>,
  source: string
): { source: string; headers: Array<{ key: string; value: string }> } | undefined {
  return all.find((r) => r.source === source)
}

function csp(rule?: { headers: Array<{ key: string; value: string }> }): string {
  return rule?.headers.find((h) => h.key === 'Content-Security-Policy')?.value ?? ''
}

function xfo(rule?: { headers: Array<{ key: string; value: string }> }): string[] {
  return (rule?.headers ?? []).filter((h) => h.key === 'X-Frame-Options').map((h) => h.value)
}

function indexOfSource(all: Awaited<ReturnType<typeof rules>>, source: string): number {
  return all.findIndex((r) => r.source === source)
}

describe('next.config framing policy', () => {
  it('the app catch-all allows only same-origin framing', async () => {
    const all = await rules()
    const catchAll = ruleFor(all, '/:path*')
    expect(catchAll).toBeDefined()
    expect(csp(catchAll)).toContain("frame-ancestors 'self'")
    expect(xfo(catchAll)).toContain('SAMEORIGIN')
  })

  it('locks all overlay routes by default so new sub-routes are not silently framable', async () => {
    const all = await rules()
    const base = ruleFor(all, '/overlay/:id*')
    expect(base).toBeDefined()
    expect(csp(base)).toContain("frame-ancestors 'none'")
    expect(xfo(base)).toContain('DENY')
  })

  it('makes the display-only widgets embeddable by any site', async () => {
    const all = await rules()
    for (const source of WIDGET_SOURCES) {
      const rule = ruleFor(all, source)
      expect(rule, `expected an embeddable rule for ${source}`).toBeDefined()
      // Full policy is preserved (last-wins replaces the whole CSP header), not
      // just the frame-ancestors directive.
      expect(csp(rule), source).toContain("default-src 'self'")
      expect(csp(rule), source).toContain('frame-ancestors *')
      // Must not hard-block via the legacy header on a route we are opening up.
      expect(xfo(rule), source).not.toContain('DENY')
    }
  })

  it('declares each widget override after the /overlay lockdown so it wins', async () => {
    const all = await rules()
    const lockdown = indexOfSource(all, '/overlay/:id*')
    expect(lockdown).toBeGreaterThanOrEqual(0)
    for (const source of WIDGET_SOURCES) {
      expect(indexOfSource(all, source), source).toBeGreaterThan(lockdown)
    }
  })

  it('keeps interactive/authenticated overlay routes frame-locked (no widget override)', async () => {
    const all = await rules()
    // participate (viewer login + points) and view (ProtectedRoute) must NOT be
    // in the embeddable set — they inherit the /overlay/:id* lockdown.
    expect(ruleFor(all, '/overlay/:id/participate')).toBeUndefined()
    expect(ruleFor(all, '/overlay/:id/view')).toBeUndefined()
  })

  it('allows Monaco editor blob workers so the CSS editor language services load (ADR-0040)', async () => {
    const all = await rules()
    // The overlay editor's self-hosted Monaco (/monaco/vs) runs its CSS
    // language services in same-origin blob: workers. Without an explicit
    // worker-src these fall back to script-src (no blob:) and are blocked, so
    // the editor loads but validation/autocomplete silently die. This must
    // hold on the app routes AND survive the last-wins full-CSP replacement on
    // the embeddable widget routes.
    expect(csp(ruleFor(all, '/:path*'))).toContain("worker-src 'self' blob:")
    for (const source of WIDGET_SOURCES) {
      expect(csp(ruleFor(all, source)), source).toContain("worker-src 'self' blob:")
    }
  })
})
