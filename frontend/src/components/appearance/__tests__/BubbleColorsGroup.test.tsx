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
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import type { VisualSettings } from '@/lib/types/visual-settings'
import { MAX_BUBBLE_PALETTE } from '@/lib/utils/visual-settings-to-css'
import { BubbleColorsGroup } from '../BubbleColorsGroup'

afterEach(() => {
  cleanup()
})

const renderGroup = (
  visualSettings: Partial<VisualSettings>,
  isPremium = true
): { onChange: ReturnType<typeof vi.fn> } => {
  const onChange = vi.fn()
  render(
    <BubbleColorsGroup visualSettings={visualSettings} onChange={onChange} isPremium={isPremium} />
  )
  return { onChange }
}

describe('BubbleColorsGroup', () => {
  it('renders a colour control per platform', () => {
    renderGroup({})
    for (const platform of ['Twitch', 'YouTube', 'Kick', 'TikTok', 'Discord']) {
      expect(screen.getByText(platform)).toBeDefined()
    }
  })

  it('reports a patch when a platform bubble colour is reset', () => {
    const { onChange } = renderGroup({ twitchBubbleBg: '#2a1b3d' })
    fireEvent.click(screen.getByLabelText(/reset twitch bubble colour/i))
    expect(onChange).toHaveBeenCalledWith({ twitchBubbleBg: undefined })
  })

  it('appends a palette entry, keeping the existing ones', () => {
    const { onChange } = renderGroup({ bubblePalette: ['#111111', '#222222'] })
    fireEvent.click(screen.getByText(/add colour/i))

    const patch = onChange.mock.calls[0][0] as Partial<VisualSettings>
    expect(patch.bubblePalette).toHaveLength(3)
    expect(patch.bubblePalette?.slice(0, 2)).toEqual(['#111111', '#222222'])
  })

  it('removes a palette entry by index', () => {
    const { onChange } = renderGroup({ bubblePalette: ['#111111', '#222222', '#333333'] })
    fireEvent.click(screen.getByLabelText(/remove colour 2/i))
    expect(onChange).toHaveBeenCalledWith({ bubblePalette: ['#111111', '#333333'] })
  })

  /** An empty list must clear the setting, not persist `[]`. */
  it('unsets the palette when the last entry is removed', () => {
    const { onChange } = renderGroup({ bubblePalette: ['#111111'] })
    fireEvent.click(screen.getByLabelText(/remove colour 1/i))
    expect(onChange).toHaveBeenCalledWith({ bubblePalette: undefined })
  })

  it('stops offering more entries at the maximum', () => {
    renderGroup({
      bubblePalette: Array.from({ length: MAX_BUBBLE_PALETTE }, (_, i) => `#00000${i}`),
    })
    expect(screen.queryByText(/add colour/i)).toBeNull()
  })

  it('says a single colour does nothing yet', () => {
    renderGroup({ bubblePalette: ['#111111'] })
    expect(screen.getByText(/add a second to start cycling/i)).toBeDefined()
  })

  it('locks every control behind premium and says so', () => {
    renderGroup({ bubblePalette: ['#111111', '#222222'] }, false)

    // The notice interleaves an upsell link, so match the paragraph's full text.
    expect(screen.getByText(/different bubble colours per platform/i)).toBeDefined()
    expect(screen.getByLabelText(/remove colour 1/i)).toHaveProperty('disabled', true)
    expect(screen.getByText(/add colour/i)).toHaveProperty('disabled', true)
  })
})
