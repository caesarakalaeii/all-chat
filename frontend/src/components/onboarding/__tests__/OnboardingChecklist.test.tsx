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
import { render, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/lib/analytics', () => ({ trackEvent: vi.fn() }))
vi.mock('@/lib/api/auth', () => ({ authApi: { updateOnboarding: vi.fn() } }))
vi.mock('@/lib/stores/auth-store', () => ({
  useAuthStore: { getState: () => ({ user: { id: 'user-1' }, init: vi.fn() }) },
}))

import { trackEvent } from '@/lib/analytics'
import { useOnboardingStore } from '@/lib/stores/onboarding-store'
import { OnboardingChecklist } from '@/components/onboarding/OnboardingChecklist'

const mockTrack = vi.mocked(trackEvent)

/** Only the `onboarding_step_viewed` events, in order, with their step id. */
function viewedSteps(): string[] {
  return mockTrack.mock.calls
    .filter(([event]) => event === 'onboarding_step_viewed')
    .map(([, data]) => (data as { step: string }).step)
}

beforeEach(() => {
  localStorage.clear()
  useOnboardingStore.setState({
    status: 'inactive',
    trigger: null,
    activeOverlayId: null,
    sessionSteps: { obsCopied: false, extrasDone: false },
    minimized: false,
    reportedSteps: [],
  })
  mockTrack.mockClear()
})

afterEach(() => cleanup())

describe('OnboardingChecklist step-viewed tracking', () => {
  it('reports nothing while the guide is inactive', () => {
    render(<OnboardingChecklist surface="dashboard" />)
    expect(viewedSteps()).toEqual([])
  })

  it('reports the active step once when the guide opens', () => {
    useOnboardingStore.setState({ status: 'active' })
    render(<OnboardingChecklist surface="dashboard" />)
    expect(viewedSteps()).toEqual(['create_overlay'])
    expect(mockTrack).toHaveBeenCalledWith('onboarding_step_viewed', {
      step: 'create_overlay',
      index: 0,
    })
  })

  it('does not re-report the same active step across rerenders', () => {
    useOnboardingStore.setState({ status: 'active' })
    const { rerender } = render(<OnboardingChecklist surface="dashboard" />)
    rerender(<OnboardingChecklist surface="dashboard" />)
    rerender(<OnboardingChecklist surface="dashboard" />)
    expect(viewedSteps()).toEqual(['create_overlay'])
  })

  it('reports the next step once the previous one completes', () => {
    useOnboardingStore.setState({ status: 'active' })
    const { rerender } = render(<OnboardingChecklist surface="dashboard" overlayCount={0} />)
    expect(viewedSteps()).toEqual(['create_overlay'])
    rerender(<OnboardingChecklist surface="dashboard" overlayCount={1} />)
    expect(viewedSteps()).toEqual(['create_overlay', 'connect_source'])
  })

  it('reports no further step once every core step is done', () => {
    useOnboardingStore.setState({
      status: 'active',
      sessionSteps: { obsCopied: true, extrasDone: false },
    })
    render(<OnboardingChecklist surface="editor" sourceCount={1} themeId="neon" overlayCount={1} />)
    expect(viewedSteps()).toEqual([])
  })
})

describe('OnboardingChecklist derived completions', () => {
  it('reports every already-done step to the store exactly once', () => {
    useOnboardingStore.setState({ status: 'active' })
    const { rerender } = render(
      <OnboardingChecklist surface="editor" sourceCount={1} themeId="neon" overlayCount={1} />
    )
    rerender(
      <OnboardingChecklist surface="editor" sourceCount={1} themeId="neon" overlayCount={1} />
    )
    expect(useOnboardingStore.getState().reportedSteps).toEqual([
      'create_overlay',
      'connect_source',
      'choose_theme',
    ])
  })

  it('renders the progress summary from the derived steps', () => {
    useOnboardingStore.setState({ status: 'active' })
    const { getByText } = render(
      <OnboardingChecklist surface="dashboard" overlayCount={1} sourceCount={1} />
    )
    expect(getByText('2 of 4 steps done')).toBeInTheDocument()
  })
})
