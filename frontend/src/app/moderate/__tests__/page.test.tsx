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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ModeratePage from '@/app/moderate/page'
import type { Delegation } from '@/lib/types/moderation'

// vi.hoisted, because vi.mock's factory is lifted above the imports and would otherwise
// reference these before initialization.
const api = vi.hoisted(() => ({ listDelegations: vi.fn() }))
const nav = vi.hoisted(() => ({ params: new URLSearchParams() }))

vi.mock('@/lib/api/moderation', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/moderation')>('@/lib/api/moderation')
  return { ...actual, moderationApi: api }
})

vi.mock('next/navigation', () => ({
  useSearchParams: () => nav.params,
  usePathname: () => '/moderate',
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))

// The page is behind ProtectedRoute, which pulls in the auth store and its network init.
// The delegation list is what is under test, so the guard is stubbed to a pass-through.
vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const OVERLAY = 'aaaaaaaa-1111-1111-1111-111111111111'

function delegation(over: Partial<Delegation> = {}): Delegation {
  return {
    grant_id: 'grant-1',
    overlay_id: OVERLAY,
    overlay_name: 'Main overlay',
    owner_display_name: 'SomeStreamer',
    status: 'active',
    actions: ['delete', 'timeout'],
    platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
    available: true,
    ...over,
  }
}

async function renderPage(delegations: Delegation[]) {
  api.listDelegations.mockResolvedValue({ delegations })
  render(<ModeratePage />)
  await waitFor(() => expect(api.listDelegations).toHaveBeenCalled())
}

beforeEach(() => {
  vi.clearAllMocks()
  nav.params = new URLSearchParams()
})
afterEach(cleanup)

describe('channels you moderate', () => {
  // The link to the overlay is the entire reason this page exists: /api/v1/overlays is
  // owner-filtered, so without it an accepted grant has no route into the overlay.
  it('links each channel to its monitor', async () => {
    await renderPage([delegation()])

    const link = await screen.findByRole('link', { name: /open chat monitor/i })
    expect(link).toHaveAttribute('href', `/overlay/${OVERLAY}/view`)
  })

  it('names the streamer the channel belongs to', async () => {
    await renderPage([delegation()])

    expect(await screen.findByText('Main overlay')).toBeInTheDocument()
    expect(screen.getByText(/for SomeStreamer/)).toBeInTheDocument()
  })

  it('tells a moderator with no channels how they get one', async () => {
    await renderPage([])

    expect(await screen.findByText(/no channels yet/i)).toBeInTheDocument()
  })

  // A failed load must offer a retry rather than looking like "you moderate nothing",
  // which would send the moderator to ask the streamer about a revocation that never
  // happened.
  it('offers a retry instead of an empty state when the load fails', async () => {
    api.listDelegations.mockRejectedValue(new Error('network'))
    render(<ModeratePage />)

    expect(await screen.findByRole('button', { name: /try again/i })).toBeInTheDocument()
    expect(screen.queryByText(/no channels yet/i)).not.toBeInTheDocument()
  })

  // Entitlement is the STREAMER's. Linking a volunteer to /upgrade would sell them a plan
  // that would not fix anything even if they bought it.
  it('blames the streamer plan for an unavailable channel and offers no upgrade link', async () => {
    await renderPage([delegation({ available: false })])

    expect(
      await screen.findByText(/SomeStreamer's plan does not include moderation/)
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /upgrade/i })).not.toBeInTheDocument()
  })

  // A suspended grant that simply vanished would be indistinguishable from a revocation,
  // leaving the moderator with nothing to ask about.
  it('keeps a suspended channel visible and explains it', async () => {
    await renderPage([delegation({ status: 'suspended' })])

    expect(await screen.findByText(/paused after 90 days/i)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /open chat monitor/i })).toBeInTheDocument()
  })

  it('says so when the streamer has enabled no platform yet', async () => {
    await renderPage([delegation({ platforms: [] })])

    expect(await screen.findByText(/no platforms turned on yet/i)).toBeInTheDocument()
  })

  // A disabled leg is not a delegated platform, so it must not be advertised as one.
  it('shows only the platforms whose leg is enabled', async () => {
    await renderPage([
      delegation({
        platforms: [
          { platform: 'twitch', enabled: true, verification: 'unverified' },
          { platform: 'kick', enabled: false, verification: 'unverified' },
        ],
      }),
    ])

    expect(await screen.findByText('TWITCH')).toBeInTheDocument()
    expect(screen.queryByText('KICK')).not.toBeInTheDocument()
  })

  it('confirms a completed platform connection from the consent redirect', async () => {
    nav.params = new URLSearchParams('connected=twitch')
    await renderPage([delegation()])

    expect(await screen.findByText(/Twitch connected/)).toBeInTheDocument()
  })

  it('explains a failed platform connection from the consent redirect', async () => {
    nav.params = new URLSearchParams('error=credential_store_failed&platform=twitch')
    await renderPage([delegation()])

    expect(await screen.findByText(/did not complete/i)).toBeInTheDocument()
  })
})
