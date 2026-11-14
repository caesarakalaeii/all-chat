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
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { YouTubeVerticalCompanionToggle } from '@/components/overlay/YouTubeVerticalCompanionToggle'
import { overlaysApi } from '@/lib/api/overlays'
import { getTranslations } from '@/lib/i18n'
import type { ChatSource } from '@/lib/types/overlay'

vi.mock('@/lib/api/overlays', () => ({
  overlaysApi: { updateSourceConfig: vi.fn() },
}))

const t = getTranslations()
const updateSourceConfig = vi.mocked(overlaysApi.updateSourceConfig)

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function makeSource(config: Record<string, unknown>): ChatSource {
  return {
    id: 'src-1',
    overlay_id: 'ovl-1',
    platform: 'youtube',
    channel_id: 'UCabc',
    config,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    is_active: true,
  }
}

const mergeSwitch = () =>
  screen.getByRole('switch', { name: t('overlayEditor.verticalCompanion.label') })
const unmarkedSwitch = () =>
  screen.queryByRole('switch', { name: t('overlayEditor.verticalCompanion.unmarkedLabel') })

describe('YouTubeVerticalCompanionToggle', () => {
  it('is on by default, with the unmarked-title switch off', () => {
    render(
      <YouTubeVerticalCompanionToggle source={makeSource({})} overlayId="ovl-1" onSaved={vi.fn()} />
    )
    expect(mergeSwitch()).toBeChecked()
    expect(unmarkedSwitch()).not.toBeChecked()
  })

  it('switching it off stores "off" merged into the existing config', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    render(
      <YouTubeVerticalCompanionToggle
        source={makeSource({ stream_select: 'most_viewers' })}
        overlayId="ovl-1"
        onSaved={vi.fn()}
      />
    )
    fireEvent.click(mergeSwitch())
    await waitFor(() =>
      expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
        stream_select: 'most_viewers',
        vertical_companion: 'off',
      })
    )
    await waitFor(() => expect(unmarkedSwitch()).toBeNull())
  })

  it('the unmarked-title switch stores "on", and switching it back drops the key', async () => {
    updateSourceConfig.mockResolvedValue(undefined)
    const { rerender } = render(
      <YouTubeVerticalCompanionToggle source={makeSource({})} overlayId="ovl-1" onSaved={vi.fn()} />
    )
    fireEvent.click(unmarkedSwitch()!)
    await waitFor(() =>
      expect(updateSourceConfig).toHaveBeenCalledWith('ovl-1', 'src-1', {
        vertical_companion: 'on',
      })
    )

    rerender(
      <YouTubeVerticalCompanionToggle
        source={makeSource({ vertical_companion: 'on' })}
        overlayId="ovl-1"
        onSaved={vi.fn()}
      />
    )
    fireEvent.click(unmarkedSwitch()!)
    await waitFor(() => expect(updateSourceConfig).toHaveBeenLastCalledWith('ovl-1', 'src-1', {}))
  })

  it('multi-stream strategies show a note instead of the switches', () => {
    render(
      <YouTubeVerticalCompanionToggle
        source={makeSource({ stream_select: 'all' })}
        overlayId="ovl-1"
        onSaved={vi.fn()}
      />
    )
    expect(
      screen.getByText(t('overlayEditor.verticalCompanion.multiStreamNote'))
    ).toBeInTheDocument()
    expect(screen.queryByRole('switch')).toBeNull()
  })
})
