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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

const push = vi.fn()

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/overlay/abc/view',
  useSearchParams: () => new URLSearchParams(),
}))

// Hydration completes immediately so ProtectedRoute evaluates auth synchronously.
vi.mock('@/hooks/useHydrated', () => ({ useHydrated: () => true }))

// Anonymous visitor: no token, no user, done loading.
vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: () => ({
    user: null,
    token: null,
    loading: false,
    init: vi.fn(),
  }),
}))

import { OverlayViewGuard } from '../OverlayViewGuard'

// ProtectedRoute renders InfinityLogo (an SVG animation) while gating; jsdom has
// no SVG geometry, so force a getTotalLength stub to let it mount.
;(SVGElement.prototype as unknown as { getTotalLength: () => number }).getTotalLength = () => 0

afterEach(() => {
  cleanup()
  push.mockClear()
})

describe('OverlayViewGuard', () => {
  it('redirects anonymous visitors home and does not render children', async () => {
    render(
      <OverlayViewGuard>
        <div data-testid="protected-child">secret monitor</div>
      </OverlayViewGuard>
    )

    await waitFor(() => expect(push).toHaveBeenCalledWith('/'))
    expect(screen.queryByTestId('protected-child')).not.toBeInTheDocument()
  })
})
