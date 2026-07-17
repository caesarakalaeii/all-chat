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

import { describe, it, expect, vi, beforeEach } from 'vitest'

// Node test env has no localStorage — stub the minimal surface the store
// touches (same approach as the CollapsibleSection tests).
const localStorageStub = (() => {
  let data: Record<string, string> = {}
  return {
    getItem: (key: string) => data[key] ?? null,
    setItem: (key: string, value: string) => {
      data[key] = value
    },
    removeItem: (key: string) => {
      delete data[key]
    },
    clear: () => {
      data = {}
    },
  }
})()
vi.stubGlobal('localStorage', localStorageStub)

vi.mock('@/lib/api/auth', () => ({
  authApi: {
    updateOnboarding: vi
      .fn()
      .mockResolvedValue({ onboarding_completed_at: '2026-07-17T00:00:00Z' }),
  },
}))
vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: {
    getState: () => ({
      user: { id: 'user-1' },
      init: vi.fn().mockResolvedValue(undefined),
    }),
  },
}))
vi.mock('@/lib/analytics', () => ({ trackEvent: vi.fn() }))

import { deriveSteps, useOnboardingStore } from '../onboarding-store'
import { authApi } from '@/lib/api/auth'
import { trackEvent } from '@/lib/analytics'

describe('deriveSteps', () => {
  const base = { overlayCount: 0, sourceCount: 0, themeId: null, obsCopied: false }

  it('fresh user: only create_overlay is active', () => {
    const steps = deriveSteps(base)
    expect(steps.map((s) => [s.id, s.done, s.active])).toEqual([
      ['create_overlay', false, true],
      ['connect_source', false, false],
      ['choose_theme', false, false],
      ['copy_obs', false, false],
    ])
  })

  it('overlay without sources: connect_source becomes active', () => {
    const steps = deriveSteps({ ...base, overlayCount: 1 })
    expect(steps.find((s) => s.active)?.id).toBe('connect_source')
    expect(steps[0].done).toBe(true)
  })

  it('sources without theme: choose_theme becomes active', () => {
    const steps = deriveSteps({ ...base, overlayCount: 1, sourceCount: 2 })
    expect(steps.find((s) => s.active)?.id).toBe('choose_theme')
  })

  it('theme picked: copy_obs becomes active; empty theme string counts as unpicked', () => {
    expect(
      deriveSteps({ ...base, overlayCount: 1, sourceCount: 1, themeId: 'minimal-theme' }).find(
        (s) => s.active
      )?.id
    ).toBe('copy_obs')
    expect(
      deriveSteps({ ...base, overlayCount: 1, sourceCount: 1, themeId: '' }).find((s) => s.active)
        ?.id
    ).toBe('choose_theme')
  })

  it('all done: no active step remains', () => {
    const steps = deriveSteps({ overlayCount: 1, sourceCount: 1, themeId: 't', obsCopied: true })
    expect(steps.every((s) => s.done)).toBe(true)
    expect(steps.some((s) => s.active)).toBe(false)
  })

  it('regression (overlay deleted mid-flow) re-activates the earlier step', () => {
    const steps = deriveSteps({ overlayCount: 0, sourceCount: 0, themeId: 't', obsCopied: true })
    expect(steps.find((s) => s.active)?.id).toBe('create_overlay')
  })
})

describe('useOnboardingStore', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    useOnboardingStore.setState({
      status: 'inactive',
      trigger: null,
      activeOverlayId: null,
      sessionSteps: { obsCopied: false, extrasDone: false },
      minimized: false,
      reportedSteps: [],
    })
  })

  it('start() activates once and reports the trigger', () => {
    useOnboardingStore.getState().start('auto')
    useOnboardingStore.getState().start('auto') // no double-fire
    expect(useOnboardingStore.getState().status).toBe('active')
    expect(trackEvent).toHaveBeenCalledTimes(1)
    expect(trackEvent).toHaveBeenCalledWith('onboarding_started', { trigger: 'auto' })
  })

  it('markObsCopied persists per-user and reports completion once', () => {
    useOnboardingStore.getState().start('auto')
    useOnboardingStore.getState().markObsCopied()
    useOnboardingStore.getState().markObsCopied()
    expect(useOnboardingStore.getState().sessionSteps.obsCopied).toBe(true)
    expect(JSON.parse(localStorage.getItem('onboarding-v1:user-1') ?? '{}').obsCopied).toBe(true)
    const completions = vi
      .mocked(trackEvent)
      .mock.calls.filter(([name]) => name === 'onboarding_step_completed')
    expect(completions).toEqual([['onboarding_step_completed', { step: 'copy_obs' }]])
  })

  it('start() after a round-trip restores session steps from localStorage', () => {
    localStorage.setItem('onboarding-v1:user-1', JSON.stringify({ obsCopied: true }))
    useOnboardingStore.getState().start('auto')
    expect(useOnboardingStore.getState().sessionSteps.obsCopied).toBe(true)
  })

  it('finish() persists completed=true and deactivates', async () => {
    useOnboardingStore.getState().start('settings')
    await useOnboardingStore.getState().finish()
    expect(useOnboardingStore.getState().status).toBe('inactive')
    expect(authApi.updateOnboarding).toHaveBeenCalledWith(true)
    expect(trackEvent).toHaveBeenCalledWith('onboarding_finished')
  })

  it('dismiss() records the abandoned step and persists', async () => {
    useOnboardingStore.getState().start('auto')
    await useOnboardingStore.getState().dismiss('connect_source')
    expect(trackEvent).toHaveBeenCalledWith('onboarding_dismissed', { step: 'connect_source' })
    expect(authApi.updateOnboarding).toHaveBeenCalledWith(true)
  })

  it('finish() retries persistence once on failure', async () => {
    vi.mocked(authApi.updateOnboarding)
      .mockRejectedValueOnce(new Error('network'))
      .mockResolvedValueOnce({ onboarding_completed_at: 'x' })
    useOnboardingStore.getState().start('auto')
    await useOnboardingStore.getState().finish()
    expect(authApi.updateOnboarding).toHaveBeenCalledTimes(2)
  })
})
