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
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { EngagementControls } from '../EngagementControls'
import type { Prediction } from '@/lib/types/engagement'

vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))
vi.mock('@/lib/api/engagement', () => ({
  engagementApi: {
    createPoll: vi.fn(),
    closePoll: vi.fn(),
    createPrediction: vi.fn(),
    lockPrediction: vi.fn(),
    resolvePrediction: vi.fn(),
    cancelPrediction: vi.fn(),
    getTwitchMirrorConsentUrl: vi.fn(),
  },
}))
vi.mock('@/lib/hooks/useEngagementLive', () => ({ useEngagementLive: vi.fn() }))

import { engagementApi } from '@/lib/api/engagement'

function lockedPrediction(id: string, title: string): Prediction {
  return {
    id,
    source: 'allchat',
    title,
    state: 'LOCKED',
    outcomes: [
      { id: `${id}-yes`, idx: 0, label: 'Yes', total_points: 10, entrants: 1 },
      { id: `${id}-no`, idx: 1, label: 'No', total_points: 5, entrants: 1 },
    ],
    created_at: '2026-01-01T00:00:00Z',
  }
}

/** What the next poll of /active-prediction returns; swapped to stage a round handoff. */
let servedPrediction: Prediction | null = null

beforeEach(() => {
  vi.clearAllMocks()
  servedPrediction = lockedPrediction('round-a', 'Round A')
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL) => {
      if (String(input).endsWith('/active-prediction') && servedPrediction) {
        return new Response(JSON.stringify(servedPrediction), { status: 200 })
      }
      return new Response(null, { status: 404 })
    })
  )
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

/** Mounts the controls and waits for the first refresh to land round A. */
async function renderWithRoundA() {
  render(<EngagementControls overlayId="ov-1" />)
  await waitFor(() => expect(screen.getByText('Round A')).toBeInTheDocument())
}

/**
 * Serves a different round and waits for the component's 3s refresh to pick it up —
 * the "round changed underneath the owner" case the reset exists for.
 */
async function serveAndAwait(prediction: Prediction) {
  servedPrediction = prediction
  await waitFor(() => expect(screen.getByText(prediction.title)).toBeInTheDocument(), {
    timeout: 6000,
  })
}

describe('EngagementControls prediction round handoff', () => {
  it('arms the payout button once a winning outcome is picked', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: Yes' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pay out "Yes"' })).toBeEnabled())
  })

  it('drops the winner selection when the active round changes underneath', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: Yes' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pay out "Yes"' })).toBeEnabled())

    await serveAndAwait(lockedPrediction('round-b', 'Round B'))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Resolve' })).toBeDisabled())
    expect(screen.getByRole('radio', { name: 'Winning outcome: Yes' })).not.toBeChecked()
  })

  it('resolves with an outcome id from the new round, never the previous one', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: Yes' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pay out "Yes"' })).toBeEnabled())

    await serveAndAwait(lockedPrediction('round-b', 'Round B'))

    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: No' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pay out "No"' })).toBeEnabled())
    fireEvent.click(screen.getByRole('button', { name: 'Pay out "No"' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Pay out "No" — final?' }))

    await waitFor(() =>
      expect(engagementApi.resolvePrediction).toHaveBeenCalledWith('ov-1', 'round-b', 'round-b-no')
    )
  })

  it('disarms an armed payout confirmation when the round changes', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: Yes' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Pay out "Yes"' }))
    await screen.findByRole('button', { name: 'Pay out "Yes" — final?' })

    await serveAndAwait(lockedPrediction('round-b', 'Round B'))

    await waitFor(() => expect(screen.getByRole('button', { name: 'Resolve' })).toBeInTheDocument())
    expect(engagementApi.resolvePrediction).not.toHaveBeenCalled()
  })

  it('disarms an armed cancel confirmation when the round changes', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('button', { name: 'Cancel & refund' }))
    await screen.findByRole('button', { name: 'Really refund all wagers?' })

    await serveAndAwait(lockedPrediction('round-b', 'Round B'))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Cancel & refund' })).toBeInTheDocument()
    )
    expect(screen.queryByRole('button', { name: 'Really refund all wagers?' })).toBeNull()
  })

  it('keeps the winner selection while the same round is refreshed', async () => {
    await renderWithRoundA()
    fireEvent.click(screen.getByRole('radio', { name: 'Winning outcome: Yes' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pay out "Yes"' })).toBeEnabled())

    // Same id, fresher tallies: a refresh, not a handoff.
    const refreshed = lockedPrediction('round-a', 'Round A')
    refreshed.outcomes[0].total_points = 99
    servedPrediction = refreshed
    await waitFor(() => expect(screen.getByText(/99 pts/)).toBeInTheDocument(), { timeout: 6000 })

    expect(screen.getByRole('button', { name: 'Pay out "Yes"' })).toBeEnabled()
  })
})
