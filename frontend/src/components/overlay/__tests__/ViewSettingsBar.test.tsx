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
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ViewSettingsBar } from '@/components/overlay/ViewSettingsBar'
import { DEFAULT_VIEW_PREFS, type MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'

afterEach(() => cleanup())

function setup(overrides: Partial<MonitorViewPrefs> = {}, onTest?: () => void) {
  const prefs: MonitorViewPrefs = { ...DEFAULT_VIEW_PREFS, ...overrides }
  const onChange = vi.fn()
  const onTestActivitySound = onTest ?? vi.fn()
  render(
    <ViewSettingsBar prefs={prefs} onChange={onChange} onTestActivitySound={onTestActivitySound} />
  )
  // The controls live in a popover; open it via the gear button.
  fireEvent.click(screen.getByRole('button', { name: 'Display settings' }))
  return { onChange, onTestActivitySound }
}

describe('ViewSettingsBar activity sound', () => {
  it('renders the activity-sound toggle and the separation note', () => {
    setup()
    expect(screen.getByRole('switch', { name: 'Sound on new activity' })).toBeInTheDocument()
    expect(screen.getByText(/separate from your overlay's on-stream notification sounds/i)).toBeInTheDocument()
  })

  it('enabling the toggle calls onChange with activitySoundEnabled: true', () => {
    const { onChange } = setup({ activitySoundEnabled: false })
    fireEvent.click(screen.getByRole('switch', { name: 'Sound on new activity' }))
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ activitySoundEnabled: true }))
  })

  it('hides sound/volume/test controls while the activity sound is off', () => {
    setup({ activitySoundEnabled: false })
    expect(screen.queryByLabelText('Sound')).toBeNull()
    expect(screen.queryByText('Volume')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Test sound' })).toBeNull()
  })

  it('shows sound/volume/test controls when the activity sound is on', () => {
    setup({ activitySoundEnabled: true, activitySoundPreset: 'pop', activitySoundVolume: 0.4 })
    expect(screen.getByLabelText('Sound')).toBeInTheDocument()
    expect(screen.getByText('Volume')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Test sound' })).toBeInTheDocument()
  })

  it('changing the preset calls onChange with the new preset', () => {
    const { onChange } = setup({ activitySoundEnabled: true })
    fireEvent.change(screen.getByLabelText('Sound'), { target: { value: 'pop' } })
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ activitySoundPreset: 'pop' }))
  })

  it('moving the volume slider calls onChange with the new volume', () => {
    const { onChange } = setup({ activitySoundEnabled: true, activitySoundVolume: 0.5 })
    fireEvent.change(screen.getByRole('slider'), { target: { value: '0.3' } })
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ activitySoundVolume: 0.3 }))
  })

  it('clicking Test triggers the preview callback', () => {
    const onTest = vi.fn()
    setup({ activitySoundEnabled: true }, onTest)
    fireEvent.click(screen.getByRole('button', { name: 'Test sound' }))
    expect(onTest).toHaveBeenCalledTimes(1)
  })
})
