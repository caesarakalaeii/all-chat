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
 * Pins the 60px sticky-nav height across all three navs.
 *
 * The height is not free to change and, more to the point, is not free to be
 * respelled. Three places hard-code 60px against it and none of them go through
 * a Tailwind spacing token:
 *
 *   - src/components/SplitView.tsx        `h-[calc(100vh-60px)]`
 *   - src/app/overlays/[id]/page.tsx      `h-[calc(100vh-60px)]` (twice)
 *   - src/app/globals.css                 `scroll-padding-top`, commented
 *                                         "AppNav h-60px"
 *
 * `tailwindcss/no-unnecessary-arbitrary-value` suggests rewriting `h-[60px]` as
 * `h-15`, which compiles to `calc(var(--spacing) * 15)` = 3.75rem. That is the
 * same 60px only at the default root font size; at any other it de-synchronises
 * the navs from the three `calc(100vh-60px)` consumers above, and the content
 * pane either overflows the viewport or leaves a gap under the nav.
 *
 * So: if someone runs that autofix, this test fails. That is its whole job.
 * Converting the navs is only safe together with the coupled calc() sites, and
 * this test is the place to record that it was a decision, not an oversight.
 */

// @vitest-environment jsdom
import { describe, expect, it, vi, afterEach } from 'vitest'
import React from 'react'
import { cleanup, render } from '@testing-library/react'

import { AdminNav } from '@/components/AdminNav'
import { AppNav } from '@/components/AppNav'
import { HomeHeader } from '@/components/HomeHeader'

vi.mock('next/navigation', () => ({
  usePathname: () => '/dashboard',
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: () => ({ user: null, loading: false, logout: vi.fn() }),
}))

vi.mock('@/lib/stores/viewer-auth-store', () => ({
  useViewerAuthStore: () => ({ viewerToken: null, viewerLogout: vi.fn() }),
}))

// All three navs render InfinityLogo, whose stroke animation measures its own SVG
// path on mount; jsdom implements no SVG geometry. Same stub as ChatRow.test.tsx.
;(SVGElement.prototype as unknown as { getTotalLength: () => number }).getTotalLength = () => 0

afterEach(cleanup)

describe('sticky nav height', () => {
  it.each([
    ['AppNav', <AppNav key="app" />],
    ['AdminNav', <AdminNav key="admin" />],
    ['HomeHeader', <HomeHeader key="home" />],
  ])('%s sizes its bar with h-[60px], not the rem-relative h-15', (_name, element) => {
    const { container } = render(element)
    const bar = container.querySelector('.h-\\[60px\\]')
    expect(bar).not.toBeNull()
    expect(container.querySelector('.h-15')).toBeNull()
  })
})
