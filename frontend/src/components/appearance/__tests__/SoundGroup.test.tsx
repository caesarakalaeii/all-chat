// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { SoundGroup } from '../SoundGroup'
import type { SoundGroupProps } from '../SoundGroup'
import type { DisplaySettings } from '@/lib/types/overlay'

afterEach(() => { cleanup() })

function renderSoundGroup(overrides: Partial<SoundGroupProps> = {}) {
  const defaultProps: SoundGroupProps = {
    displaySettings: {
      notification_sound_enabled: true,
      notification_sound_preset: 'chime',
      notification_sound_volume: 0.5,
      notification_sound_cooldown: 500,
    },
    onChange: vi.fn(),
    isPremium: false,
    ...overrides,
  }
  return { ...render(<SoundGroup {...defaultProps} />), onChange: defaultProps.onChange }
}

describe('SoundGroup', () => {
  it('renders "Enable notification sounds" toggle (ToggleSwitch)', () => {
    renderSoundGroup()
    expect(screen.getByText('Enable notification sounds')).toBeDefined()
    expect(screen.getByRole('switch')).toBeDefined()
  })

  it('toggle calls onChange with { notification_sound_enabled: true } when toggled on', () => {
    const { onChange } = renderSoundGroup({
      displaySettings: {
        notification_sound_enabled: false,
        notification_sound_preset: 'chime',
        notification_sound_volume: 0.5,
        notification_sound_cooldown: 500,
      },
    })
    fireEvent.click(screen.getByRole('switch'))
    expect(onChange).toHaveBeenCalledWith({ notification_sound_enabled: true })
  })

  it('toggle calls onChange with { notification_sound_enabled: false } when toggled off', () => {
    const { onChange } = renderSoundGroup({
      displaySettings: {
        notification_sound_enabled: true,
        notification_sound_preset: 'chime',
        notification_sound_volume: 0.5,
        notification_sound_cooldown: 500,
      },
    })
    fireEvent.click(screen.getByRole('switch'))
    expect(onChange).toHaveBeenCalledWith({ notification_sound_enabled: false })
  })

  it('renders preset selector with options for chime, pop, ping', () => {
    renderSoundGroup()
    const select = screen.getByLabelText('Sound preset')
    expect(select).toBeDefined()
    expect(screen.getByRole('option', { name: /chime/i })).toBeDefined()
    expect(screen.getByRole('option', { name: /pop/i })).toBeDefined()
    expect(screen.getByRole('option', { name: /ping/i })).toBeDefined()
  })

  it('selecting a preset calls onChange with { notification_sound_preset: "pop" }', () => {
    const { onChange } = renderSoundGroup()
    const select = screen.getByLabelText('Sound preset')
    fireEvent.change(select, { target: { value: 'pop' } })
    expect(onChange).toHaveBeenCalledWith({ notification_sound_preset: 'pop' })
  })

  it('renders Volume slider with value from displaySettings', () => {
    renderSoundGroup({ displaySettings: { notification_sound_enabled: true, notification_sound_preset: 'chime', notification_sound_volume: 0.7, notification_sound_cooldown: 500 } })
    expect(screen.getByText('Volume')).toBeDefined()
    const sliders = screen.getAllByRole('slider')
    const volumeSlider = sliders.find(s => s.getAttribute('min') === '0' && s.getAttribute('max') === '1')
    expect(volumeSlider).toBeDefined()
    expect(volumeSlider!.getAttribute('value')).toBe('0.7')
  })

  it('moving volume slider calls onChange with { notification_sound_volume: 0.7 }', () => {
    const { onChange } = renderSoundGroup()
    const sliders = screen.getAllByRole('slider')
    const volumeSlider = sliders.find(s => s.getAttribute('min') === '0' && s.getAttribute('max') === '1')!
    fireEvent.change(volumeSlider, { target: { value: '0.7' } })
    expect(onChange).toHaveBeenCalledWith({ notification_sound_volume: 0.7 })
  })

  it('renders Cooldown slider with value from displaySettings', () => {
    renderSoundGroup({ displaySettings: { notification_sound_enabled: true, notification_sound_preset: 'chime', notification_sound_volume: 0.5, notification_sound_cooldown: 1000 } })
    expect(screen.getByText('Cooldown')).toBeDefined()
    const sliders = screen.getAllByRole('slider')
    const cooldownSlider = sliders.find(s => s.getAttribute('min') === '100' && s.getAttribute('max') === '5000')
    expect(cooldownSlider).toBeDefined()
    expect(cooldownSlider!.getAttribute('value')).toBe('1000')
  })

  it('moving cooldown slider calls onChange with { notification_sound_cooldown: 1000 }', () => {
    const { onChange } = renderSoundGroup()
    const sliders = screen.getAllByRole('slider')
    const cooldownSlider = sliders.find(s => s.getAttribute('min') === '100' && s.getAttribute('max') === '5000')!
    fireEvent.change(cooldownSlider, { target: { value: '1000' } })
    expect(onChange).toHaveBeenCalledWith({ notification_sound_cooldown: 1000 })
  })

  it('when isPremium=false, custom URL input is disabled and PremiumBadge is rendered', () => {
    renderSoundGroup({ isPremium: false })
    const input = screen.getByLabelText('Custom sound URL') as HTMLInputElement
    expect(input).toBeDefined()
    expect(input.disabled).toBe(true)
    // PremiumBadge renders an SVG with aria-label="Premium badge"
    expect(screen.getByLabelText('Premium badge')).toBeDefined()
  })

  it('when isPremium=true, custom URL input is enabled and PremiumBadge is NOT rendered', () => {
    renderSoundGroup({ isPremium: true })
    const input = screen.getByLabelText('Custom sound URL') as HTMLInputElement
    expect(input).toBeDefined()
    expect(input.disabled).toBe(false)
    expect(screen.queryByLabelText('Premium badge')).toBeNull()
  })

  it('typing in custom URL input calls onChange with { notification_sound_url: "https://example.com/sound.mp3" }', () => {
    const { onChange } = renderSoundGroup({ isPremium: true })
    const input = screen.getByLabelText('Custom sound URL')
    fireEvent.change(input, { target: { value: 'https://example.com/sound.mp3' } })
    expect(onChange).toHaveBeenCalledWith({ notification_sound_url: 'https://example.com/sound.mp3' })
  })

  it('sound controls (preset, volume, cooldown) are hidden when notification_sound_enabled is false', () => {
    renderSoundGroup({
      displaySettings: {
        notification_sound_enabled: false,
        notification_sound_preset: 'chime',
        notification_sound_volume: 0.5,
        notification_sound_cooldown: 500,
      },
    })
    expect(screen.queryByLabelText('Sound preset')).toBeNull()
    expect(screen.queryByText('Volume')).toBeNull()
    expect(screen.queryByText('Cooldown')).toBeNull()
  })
})
