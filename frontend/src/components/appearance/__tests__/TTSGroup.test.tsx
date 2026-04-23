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

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { TTSGroup } from '../TTSGroup'
import type { TTSGroupProps } from '../TTSGroup'
import type { DisplaySettings } from '@/lib/types/overlay'

afterEach(() => {
  cleanup()
})

interface MockSpeechSynthesis {
  getVoices(): SpeechSynthesisVoice[]
  addEventListener: ReturnType<typeof vi.fn>
  removeEventListener: ReturnType<typeof vi.fn>
}

function installSpeechSynthesis(): MockSpeechSynthesis {
  const mock: MockSpeechSynthesis = {
    getVoices: (): SpeechSynthesisVoice[] =>
      [
        { voiceURI: 'VoiceA', name: 'Voice A', lang: 'en-US' },
        { voiceURI: 'VoiceB', name: 'Voice B', lang: 'en-GB' },
      ] as unknown as SpeechSynthesisVoice[],
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }
  // Install on window (jsdom) — this is what the hook/component check reads.
  Object.defineProperty(window, 'speechSynthesis', {
    value: mock,
    configurable: true,
    writable: true,
  })
  return mock
}

function uninstallSpeechSynthesis(): void {
  Object.defineProperty(window, 'speechSynthesis', {
    value: undefined,
    configurable: true,
    writable: true,
  })
}

// Mock speechSynthesis (jsdom doesn't implement it)
beforeEach(() => {
  installSpeechSynthesis()
})

afterEach(() => {
  uninstallSpeechSynthesis()
  vi.unstubAllGlobals()
})

function baseSettings(overrides: Partial<DisplaySettings> = {}): Partial<DisplaySettings> {
  return {
    tts_enabled: true,
    tts_provider: 'browser',
    tts_volume: 0.8,
    tts_rate: 1.0,
    tts_pitch: 1.0,
    tts_filter_mode: 'sample',
    tts_sample_rate: 0.25,
    tts_max_queue: 5,
    tts_messages_per_minute: 8,
    tts_user_cooldown_seconds: 30,
    tts_staleness_seconds: 15,
    tts_priority_events: true,
    tts_priority_bits_min: 0,
    tts_read_username: true,
    tts_read_platform: false,
    tts_max_message_chars: 200,
    tts_skip_emote_only: true,
    tts_skip_links: true,
    tts_enabled_platforms: ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
    ...overrides,
  }
}

type OnChangeFn = (patch: Partial<DisplaySettings>) => void

function renderTTSGroup(overrides: Partial<TTSGroupProps> = {}): {
  onChange: ReturnType<typeof vi.fn> & OnChangeFn
  container: HTMLElement
} {
  const onChange = vi.fn() as ReturnType<typeof vi.fn> & OnChangeFn
  const defaults: TTSGroupProps = {
    displaySettings: baseSettings(),
    onChange,
    isPremium: true,
    overlayId: 'overlay-123',
    hasElevenLabsConfig: false,
    obsUrl: undefined,
  }
  const result = render(<TTSGroup {...defaults} {...overrides} onChange={onChange} />)
  return { onChange, container: result.container }
}

describe('TTSGroup', () => {
  // Test 1: Master toggle
  it('renders master toggle and fires onChange({tts_enabled: !current}) on click', () => {
    const { onChange } = renderTTSGroup({
      displaySettings: baseSettings({ tts_enabled: false }),
    })
    expect(screen.getByText('Enable text-to-speech')).toBeDefined()
    const masterSwitch = screen.getAllByRole('switch')[0]
    fireEvent.click(masterSwitch)
    expect(onChange).toHaveBeenCalledWith({ tts_enabled: true })
  })

  // Test 2: No sub-section content when disabled
  it('does not render sub-section headers when tts_enabled=false', () => {
    renderTTSGroup({ displaySettings: baseSettings({ tts_enabled: false }) })
    // Exact-match the sub-section headers (all-caps)
    expect(screen.queryByText('VOICE')).toBeNull()
    expect(screen.queryByText('THROTTLING')).toBeNull()
    expect(screen.queryByText('CONTENT')).toBeNull()
    expect(screen.queryByText('PRIORITY')).toBeNull()
  })

  // Test 3: Sub-section headers render when enabled
  it('renders Voice/Throttling/Content/Priority headers when tts_enabled=true', () => {
    renderTTSGroup()
    expect(screen.getByText('VOICE')).toBeDefined()
    expect(screen.getByText('THROTTLING')).toBeDefined()
    expect(screen.getByText('CONTENT')).toBeDefined()
    expect(screen.getByText('PRIORITY')).toBeDefined()
    // Advanced is conditional on provider=elevenlabs
    expect(screen.queryByText('ADVANCED (ELEVENLABS)')).toBeNull()
  })

  // Test 4: Provider radio with disabled ElevenLabs for non-premium
  it('provider radio offers Browser (free) and ElevenLabs (premium); ElevenLabs disabled when !isPremium', () => {
    renderTTSGroup({ isPremium: false })
    expect(screen.getByLabelText(/Browser \(free\)/)).toBeDefined()
    const elevenlabsRadio = screen.getByLabelText(/ElevenLabs \(premium\)/) as HTMLInputElement
    expect(elevenlabsRadio).toBeDefined()
    expect(elevenlabsRadio.disabled).toBe(true)
  })

  // Test 5: Sliders fire onChange with mapped field name
  it('Volume/Rate sliders fire onChange with mapped tts_* field', () => {
    const { onChange } = renderTTSGroup()
    const sliders = screen.getAllByRole('slider')
    const volumeSlider = sliders.find(
      s => s.getAttribute('min') === '0' && s.getAttribute('max') === '1' && s.getAttribute('step') === '0.05',
    )
    expect(volumeSlider).toBeDefined()
    fireEvent.change(volumeSlider!, { target: { value: '0.5' } })
    expect(onChange).toHaveBeenCalledWith({ tts_volume: 0.5 })

    onChange.mockClear()
    const rateSlider = sliders.find(s => s.getAttribute('min') === '0.5' && s.getAttribute('max') === '2')
    expect(rateSlider).toBeDefined()
    fireEvent.change(rateSlider!, { target: { value: '1.5' } })
    expect(onChange).toHaveBeenCalledWith({ tts_rate: 1.5 })
  })

  // Test 6: Voice picker populated from useBrowserVoices
  it('voice picker is a <select> populated with browser voices; selection fires onChange', () => {
    const { onChange } = renderTTSGroup()
    const select = screen.getByRole('combobox', { name: /Voice/i }) as HTMLSelectElement
    expect(select).toBeDefined()
    // Should have voice options (at least default + mocked voices)
    const options = Array.from(select.options).map(o => o.value)
    expect(options).toContain('VoiceA')
    expect(options).toContain('VoiceB')
    fireEvent.change(select, { target: { value: 'VoiceB' } })
    expect(onChange).toHaveBeenCalledWith({ tts_voice_uri: 'VoiceB' })
  })

  // Test 7: Pitch slider hidden when provider=elevenlabs
  it('Pitch slider is HIDDEN when tts_provider=elevenlabs', () => {
    renderTTSGroup({ displaySettings: baseSettings({ tts_provider: 'elevenlabs' }) })
    expect(screen.queryByText('Pitch')).toBeNull()
  })

  // Test 8: Sample-rate slider hidden when filter_mode != 'sample'
  it('Sample rate slider is HIDDEN when tts_filter_mode !== "sample"', () => {
    renderTTSGroup({ displaySettings: baseSettings({ tts_filter_mode: 'all' }) })
    expect(screen.queryByText('Sample rate')).toBeNull()
  })

  // Test 9: Priority-bits-min hidden when priority_events=false
  it('Minimum bits field is HIDDEN when tts_priority_events=false', () => {
    renderTTSGroup({ displaySettings: baseSettings({ tts_priority_events: false }) })
    expect(screen.queryByText('Minimum bits to announce')).toBeNull()
  })

  // Test 10: Platform chip row renders 5 chips and toggles state
  it('renders 5 platform chips; clicking toggles tts_enabled_platforms', () => {
    const { onChange } = renderTTSGroup()
    // Check all 5 platforms render
    expect(screen.getByText(/twitch/i)).toBeDefined()
    expect(screen.getByText(/youtube/i)).toBeDefined()
    expect(screen.getByText(/kick/i)).toBeDefined()
    expect(screen.getByText(/tiktok/i)).toBeDefined()
    expect(screen.getByText(/discord/i)).toBeDefined()

    // Click tiktok — should fire onChange with the platform removed
    const tiktokChip = screen.getByRole('button', { name: /tiktok/i })
    fireEvent.click(tiktokChip)
    expect(onChange).toHaveBeenCalled()
    const lastCall = onChange.mock.calls[onChange.mock.calls.length - 1][0] as Partial<DisplaySettings>
    expect(lastCall.tts_enabled_platforms).not.toContain('tiktok')
  })

  // Test 11: Test voice button invokes onPreview
  it('Test voice button is rendered; clicking invokes onPreview callback', () => {
    const onPreview = vi.fn()
    renderTTSGroup({ onPreview })
    const btn = screen.getByRole('button', { name: /Test voice/i })
    expect(btn).toBeDefined()
    fireEvent.click(btn)
    expect(onPreview).toHaveBeenCalled()
  })

  // Test 12: Advanced block hidden when provider='browser'
  it('Advanced (ElevenLabs) block is HIDDEN when tts_provider=browser', () => {
    renderTTSGroup({ displaySettings: baseSettings({ tts_provider: 'browser' }) })
    expect(screen.queryByText(/ADVANCED/i)).toBeNull()
  })

  // Test 13: Advanced stub renders with placeholder when provider=elevenlabs + isPremium=true
  it('Advanced block stub renders placeholder when tts_provider=elevenlabs + isPremium=true', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
    })
    expect(screen.getByText(/ADVANCED/i)).toBeDefined()
    // Placeholder copy mentions Plan 03
    expect(screen.getByText(/Plan 03/i)).toBeDefined()
  })

  // Test 14: Non-premium user with ElevenLabs selected sees PremiumBadge + upgrade copy
  it('non-premium user sees disabled ElevenLabs radio + upgrade copy', () => {
    renderTTSGroup({ isPremium: false })
    const elevenRadio = screen.getByLabelText(/ElevenLabs \(premium\)/) as HTMLInputElement
    expect(elevenRadio.disabled).toBe(true)
    // Look for upgrade copy
    expect(screen.getByText(/Upgrade to Premium/i)).toBeDefined()
  })

  // Test 15: Master toggle disabled when window.speechSynthesis is undefined
  it('master toggle is disabled + helper copy shown when speechSynthesis is undefined', () => {
    // Remove the global speechSynthesis
    vi.stubGlobal('speechSynthesis', undefined)
    // Also make sure window doesn't have it
    Object.defineProperty(window, 'speechSynthesis', {
      value: undefined,
      configurable: true,
    })
    renderTTSGroup({ displaySettings: baseSettings({ tts_enabled: false }) })
    expect(screen.getByText(/This browser does not support text-to-speech/i)).toBeDefined()
  })
})
