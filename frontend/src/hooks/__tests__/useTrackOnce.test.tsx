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
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'

vi.mock('@/lib/analytics', () => ({ trackEvent: vi.fn() }))
import { trackEvent } from '@/lib/analytics'
import { useTrackOnce } from '../useTrackOnce'

const mockTrack = vi.mocked(trackEvent)

beforeEach(() => mockTrack.mockClear())

describe('useTrackOnce', () => {
  it('fires once on mount when enabled defaults to true', () => {
    renderHook(() => useTrackOnce('editor_opened'))
    expect(mockTrack).toHaveBeenCalledTimes(1)
    expect(mockTrack).toHaveBeenCalledWith('editor_opened', undefined)
  })

  it('does not fire while disabled, then fires once when enabled flips true', () => {
    const { rerender } = renderHook(({ on }) => useTrackOnce('preview_rendered', undefined, on), {
      initialProps: { on: false },
    })
    expect(mockTrack).not.toHaveBeenCalled()
    rerender({ on: true })
    expect(mockTrack).toHaveBeenCalledTimes(1)
  })

  it('fires only once across many rerenders', () => {
    const { rerender } = renderHook(({ on }) => useTrackOnce('preview_rendered', undefined, on), {
      initialProps: { on: true },
    })
    rerender({ on: true })
    rerender({ on: true })
    expect(mockTrack).toHaveBeenCalledTimes(1)
  })

  it('passes event data through to trackEvent', () => {
    renderHook(() => useTrackOnce('source_configured', { platform: 'tiktok' }))
    expect(mockTrack).toHaveBeenCalledWith('source_configured', { platform: 'tiktok' })
  })
})
