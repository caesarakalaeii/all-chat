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
 * The Commands nav link (ADR-0079) is beta-only and streamer-only: hidden on
 * production hosts (lib/platform-availability.ts) and for viewer sessions,
 * which have no chat to command.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { AppNav } from '@/components/AppNav'

const auth = vi.hoisted(() => ({ user: null as null | { id: string } }))

vi.mock('next/navigation', () => ({
  usePathname: () => '/dashboard',
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))
vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: () => ({ user: auth.user, loading: false, logout: vi.fn() }),
}))
vi.mock('@/lib/stores/viewer-auth-store', () => ({
  useViewerAuthStore: () => ({ viewerToken: 'viewer', viewerLogout: vi.fn() }),
}))

// InfinityLogo measures its SVG path and reads matchMedia; jsdom has neither.
;(SVGElement.prototype as unknown as { getTotalLength: () => number }).getTotalLength = () => 0
;(window as unknown as { matchMedia: typeof window.matchMedia }).matchMedia = (query) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
})

function onHost(hostname: string) {
  vi.spyOn(window, 'location', 'get').mockReturnValue({ ...window.location, hostname })
}

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  auth.user = null
})

describe('AppNav Commands link', () => {
  it('shows for a streamer on beta.allch.at', () => {
    auth.user = { id: 'u1' }
    onHost('beta.allch.at')
    render(<AppNav />)
    expect(screen.getByRole('link', { name: 'Commands' })).toHaveAttribute('href', '/commands')
  })

  it('is hidden on production', () => {
    auth.user = { id: 'u1' }
    onHost('allch.at')
    render(<AppNav />)
    expect(screen.queryByRole('link', { name: 'Commands' })).not.toBeInTheDocument()
  })

  it('is hidden for a viewer session on beta', () => {
    onHost('beta.allch.at')
    render(<AppNav />)
    expect(screen.queryByRole('link', { name: 'Commands' })).not.toBeInTheDocument()
  })
})
