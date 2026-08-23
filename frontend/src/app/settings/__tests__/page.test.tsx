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
 * Settings page tests.
 *
 * The load-bearing ones are the Discord connect-callback assertions and the
 * guild-fetch call count. The page reads `?discord=connected`, toasts, strips the
 * marker from the URL and re-reads the guild list; the fetch itself must fire once
 * per mount and must not loop. Both behaviours live in effects, so a change to how
 * those effects are wired can silently drop the toast, leave the marker in the URL,
 * or turn one request into an endless stream of them — none of which shows up as a
 * type error.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SettingsPage from '@/app/settings/page'
import type { DiscordGuild } from '@/lib/api/discord'

// vi.hoisted, because vi.mock's factory is lifted above the imports.
const discordApi = vi.hoisted(() => ({
  getGuilds: vi.fn(),
  getDiscordIdentity: vi.fn(),
}))

vi.mock('@/lib/api/discord', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api/discord')>('@/lib/api/discord')
  return { ...actual, ...discordApi }
})

const nav = vi.hoisted(() => ({
  params: new URLSearchParams(),
  replace: vi.fn(),
  push: vi.fn(),
}))

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: nav.push, replace: nav.replace, prefetch: vi.fn() }),
  usePathname: () => '/settings',
  useSearchParams: () => nav.params,
}))

const toast = vi.hoisted(() => ({ add: vi.fn() }))
vi.mock('@/lib/toast', () => ({ toastManager: { add: toast.add } }))

vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))
// The ambassador card fetches its own showcase state and is not under test here.
vi.mock('@/components/settings/AmbassadorSettingsCard', () => ({
  AmbassadorSettingsCard: () => null,
}))

// Both stores are read with selectors, so the mock has to apply the selector to a
// plain state object rather than return a fixed value.
const stores = vi.hoisted(() => ({
  auth: {
    user: { id: 'u1', username: 'streamer', display_name: 'Streamer' },
    logout: vi.fn(),
    init: vi.fn(),
  } as Record<string, unknown>,
  onboarding: { start: vi.fn() } as Record<string, unknown>,
}))

vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: (selector: (state: Record<string, unknown>) => unknown) => selector(stores.auth),
}))
vi.mock('@/lib/stores/onboarding-store', () => ({
  useOnboardingStore: (selector: (state: Record<string, unknown>) => unknown) =>
    selector(stores.onboarding),
}))

function guild(over: Partial<DiscordGuild> = {}): DiscordGuild {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    user_id: 'u1',
    guild_id: '999',
    guild_name: 'Test Server',
    guild_icon: null,
    connected_at: '2026-01-05T10:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  nav.params = new URLSearchParams()
  discordApi.getGuilds.mockResolvedValue([guild()])
  discordApi.getDiscordIdentity.mockResolvedValue({ linked: false })
})

afterEach(cleanup)

describe('Settings page — Discord connect callback', () => {
  it('toasts, strips the marker from the URL and reads the guilds when ?discord=connected', async () => {
    nav.params = new URLSearchParams('discord=connected')

    render(<SettingsPage />)

    await waitFor(() => {
      expect(toast.add).toHaveBeenCalledWith({
        title: 'Discord server connected!',
        type: 'success',
      })
    })
    // Exactly once each: a callback effect that re-runs on every render would
    // toast repeatedly and fight the router for the URL.
    expect(toast.add).toHaveBeenCalledTimes(1)
    expect(nav.replace).toHaveBeenCalledTimes(1)
    expect(nav.replace).toHaveBeenCalledWith('/settings')
    // Only that the list is read, not how many times: the marker being stripped is
    // itself a reason to read it again, and with a mocked router the strip never
    // reaches useSearchParams, so a count here would pin the mock rather than the
    // page. The plain-mount test below is where the no-loop guarantee lives.
    expect(discordApi.getGuilds).toHaveBeenCalled()
  })

  it('does none of the three when there is no discord marker', async () => {
    render(<SettingsPage />)

    // The guild list still loads — that is the plain-mount fetch, asserted below.
    await screen.findByText('Test Server')

    expect(toast.add).not.toHaveBeenCalledWith(
      expect.objectContaining({ title: 'Discord server connected!' })
    )
    expect(nav.replace).not.toHaveBeenCalled()
  })
})

describe('Settings page — guild fetch', () => {
  it('reads the guilds exactly once on a plain mount and does not loop on re-render', async () => {
    const { rerender } = render(<SettingsPage />)
    await screen.findByText('Test Server')

    rerender(<SettingsPage />)
    // A dependency array that changes identity every render turns this fetch into a
    // loop. Give any extra render a chance to schedule one before counting.
    await new Promise((resolve) => setTimeout(resolve, 50))

    expect(discordApi.getGuilds).toHaveBeenCalledTimes(1)
  })
})
