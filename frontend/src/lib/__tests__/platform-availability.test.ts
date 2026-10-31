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
import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'

import { usePlatformAvailable } from '@/lib/platform-availability'
import { useAuthStore } from '@/lib/stores/auth-store'
import type { User } from '@/lib/types/auth'

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.setState({ user: null })
})

function signIn(flags: Partial<User>) {
  useAuthStore.setState({ user: { is_beta_tester: false, is_ambassador: false, ...flags } as User })
}

describe('usePlatformAvailable', () => {
  it('hides Altstream outside the beta host from users without early access', () => {
    signIn({})
    const { result } = renderHook(() => usePlatformAvailable())
    expect(result.current('altstream')).toBe(false)
    expect(result.current('twitch')).toBe(true)
  })

  it('offers Altstream to beta testers and ambassadors on any host', () => {
    signIn({ is_beta_tester: true })
    expect(renderHook(() => usePlatformAvailable()).result.current('altstream')).toBe(true)
    signIn({ is_ambassador: true })
    expect(renderHook(() => usePlatformAvailable()).result.current('altstream')).toBe(true)
  })

  it('keeps beta-only platforms off allch.at even for early-access users', () => {
    signIn({ is_beta_tester: true })
    expect(renderHook(() => usePlatformAvailable()).result.current('picarto')).toBe(false)
  })

  it('offers Altstream on beta.allch.at', () => {
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      hostname: 'beta.allch.at',
    })
    const { result } = renderHook(() => usePlatformAvailable())
    expect(result.current('altstream')).toBe(true)
  })
})
