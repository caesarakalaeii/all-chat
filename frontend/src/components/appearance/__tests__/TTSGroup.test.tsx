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
import { render, screen, fireEvent, cleanup, waitFor, act } from '@testing-library/react'

// Mock react-hot-toast BEFORE importing the component under test. The mock must
// expose .success/.error on the default export so TTSGroup's `toast.success(...)` /
// `toast.error(...)` calls resolve through the vi.fn stubs.
//
// vi.hoisted runs before vi.mock's factory, which itself runs before `import`
// statements — this is the supported pattern for accessing shared mock fns.
const toastMocks = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
}))
vi.mock('react-hot-toast', () => {
  const fn = vi.fn()
  return {
    default: Object.assign(fn, toastMocks),
    __esModule: true,
  }
})

import { TTSGroup } from '../TTSGroup'
import type { TTSGroupProps, ElevenLabsVoice } from '../TTSGroup'
import type { DisplaySettings } from '@/lib/types/overlay'

/**
 * Wait for the ElevenLabs voice <select> to render an <option> with the given
 * label, then return the select element. Used by tests that exercise the new
 * typed-key → preview-voices auto-load flow (replaces the old on-focus loader).
 */
async function waitForVoiceOption(name: string): Promise<HTMLSelectElement> {
  const select = screen.getByLabelText(/ElevenLabs voice/i) as HTMLSelectElement
  await waitFor(
    () => {
      const opts = Array.from(select.options).map((o) => o.textContent ?? '')
      expect(opts).toContain(name)
    },
    { timeout: 2000 },
  )
  return select
}

afterEach(() => {
  cleanup()
  toastMocks.success.mockClear()
  toastMocks.error.mockClear()
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

// Mock clipboard.writeText so Copy OBS URL tests can assert on it.
const clipboardWriteMock = vi.fn().mockResolvedValue(undefined)
Object.defineProperty(global.navigator, 'clipboard', {
  value: { writeText: clipboardWriteMock },
  writable: true,
  configurable: true,
})
beforeEach(() => {
  clipboardWriteMock.mockClear()
  clipboardWriteMock.mockResolvedValue(undefined)
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
    expect(screen.queryByText(/ADVANCED \(ELEVENLABS\)/)).toBeNull()
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

  // Test 13: Advanced block renders when provider=elevenlabs + isPremium=true
  it('Advanced block renders heading when tts_provider=elevenlabs + isPremium=true', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
    })
    expect(screen.getByText(/ADVANCED \(ELEVENLABS\)/)).toBeDefined()
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

  // ========================================================================
  // Plan 03 — ElevenLabs Advanced block tests (A1..A20)
  // ========================================================================

  // Test A1: API-key input + Save button rendered with helper copy when no saved key
  it('A1: premium + elevenlabs + no saved key renders password input with Save key button', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    expect(input).toBeDefined()
    expect(input.type).toBe('password')
    expect(
      screen.getByText(/Your key is encrypted server-side and never returned\./)
    ).toBeDefined()
    expect(screen.getByRole('button', { name: /^Save key$/ })).toBeDefined()
  })

  // Test A2: Clicking Save-key fires onSaveKey with key+voice
  it('A2: clicking Save key fires onSaveKey(key, voiceId)', async () => {
    const onSaveKey = vi.fn().mockResolvedValue(undefined)
    const onPreviewVoices = vi
      .fn<(apiKey: string) => Promise<ElevenLabsVoice[]>>()
      .mockResolvedValue([{ voice_id: 'v-alpha', name: 'Alpha' }])
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onSaveKey,
      onPreviewVoices,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-real-key' } })
    // Wait for the debounced preview load + select a voice (new precondition).
    const select = await waitForVoiceOption('Alpha')
    fireEvent.change(select, { target: { value: 'v-alpha' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    expect(onSaveKey).toHaveBeenCalledWith('sk-real-key', 'v-alpha')
  })

  // Test A3: After successful save, input clears
  it('A3: after onSaveKey resolves, the API-key input clears to empty', async () => {
    const onSaveKey = vi.fn().mockResolvedValue(undefined)
    const onPreviewVoices = vi
      .fn<(apiKey: string) => Promise<ElevenLabsVoice[]>>()
      .mockResolvedValue([{ voice_id: 'v-alpha', name: 'Alpha' }])
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onSaveKey,
      onPreviewVoices,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-secret' } })
    const select = await waitForVoiceOption('Alpha')
    fireEvent.change(select, { target: { value: 'v-alpha' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    await waitFor(() => expect(input.value).toBe(''))
  })

  // Test A4: Save-key error surfaces inline error
  it('A4: Save-key error leaves input intact and shows "Could not save. Try again."', async () => {
    const onSaveKey = vi.fn().mockRejectedValue(new Error('boom'))
    const onPreviewVoices = vi
      .fn<(apiKey: string) => Promise<ElevenLabsVoice[]>>()
      .mockResolvedValue([{ voice_id: 'v-alpha', name: 'Alpha' }])
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onSaveKey,
      onPreviewVoices,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-bad-typed' } })
    const select = await waitForVoiceOption('Alpha')
    fireEvent.change(select, { target: { value: 'v-alpha' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    await waitFor(() => {
      expect(screen.getByText(/Could not save\. Try again\./)).toBeDefined()
    })
    expect(input.value).toBe('sk-bad-typed')
  })

  // Test A5: Saved state shows Test/Remove + saved helper copy
  it('A5: hasElevenLabsConfig=true shows saved-key helper + Test key + Remove key + OBS URL panel', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
    })
    expect(
      screen.getByText(/Key saved and encrypted\. Click Test key to verify\./)
    ).toBeDefined()
    expect(screen.getByRole('button', { name: /^Test key$/ })).toBeDefined()
    expect(screen.getByRole('button', { name: /^Remove key$/ })).toBeDefined()
    expect(screen.getByRole('button', { name: /^Copy OBS URL$/ })).toBeDefined()
  })

  // Test A6: Clicking Test-key flips button label to "Testing…" and disables
  it('A6: clicking Test key disables button + shows "Testing…" while onTestKey pending', async () => {
    let resolveFn: (v: { ok: boolean }) => void = () => {}
    const onTestKey = vi.fn(
      () =>
        new Promise<{ ok: boolean; errorCode?: number }>((r) => {
          resolveFn = r
        }),
    )
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      onTestKey,
    })
    const btn = screen.getByRole('button', { name: /^Test key$/ }) as HTMLButtonElement
    fireEvent.click(btn)
    await waitFor(() => {
      const btn2 = screen.getByRole('button', { name: /^Testing…$/ }) as HTMLButtonElement
      expect(btn2).toBeDefined()
      expect(btn2.disabled).toBe(true)
    })
    await act(async () => {
      resolveFn({ ok: true })
    })
  })

  // Test A7: Successful test-key renders quota with N,/M, format + percent
  it('A7: onTestKey success {remaining:8432, limit:10000} renders "8,432 / 10,000 characters this month (84%)"', async () => {
    const onTestKey = vi
      .fn()
      .mockResolvedValue({ ok: true, charactersRemaining: 8432, charactersLimit: 10000 })
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      onTestKey,
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Test key$/ }))
    })
    await waitFor(() => {
      expect(
        screen.getByText(/8,432 \/ 10,000 characters this month \(84%\)/),
      ).toBeDefined()
    })
  })

  // Test A8: 401 error shows "Invalid API key" toast
  it('A8: onTestKey {ok:false, errorCode:401} fires toast.error("Invalid API key")', async () => {
    const onTestKey = vi.fn().mockResolvedValue({ ok: false, errorCode: 401 })
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      onTestKey,
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Test key$/ }))
    })
    await waitFor(() => {
      expect(toastMocks.error).toHaveBeenCalledWith('Invalid API key')
    })
  })

  // Test A9: 429 -> rate-limited toast
  it('A9: onTestKey {ok:false, errorCode:429} fires toast.error("Rate-limited — try again in a minute")', async () => {
    const onTestKey = vi.fn().mockResolvedValue({ ok: false, errorCode: 429 })
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      onTestKey,
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Test key$/ }))
    })
    await waitFor(() => {
      expect(toastMocks.error).toHaveBeenCalledWith('Rate-limited — try again in a minute')
    })
  })

  // Test A10: 5xx -> service unavailable
  it('A10: onTestKey {ok:false, errorCode:500} fires toast.error("ElevenLabs service unavailable")', async () => {
    const onTestKey = vi.fn().mockResolvedValue({ ok: false, errorCode: 500 })
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      onTestKey,
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Test key$/ }))
    })
    await waitFor(() => {
      expect(toastMocks.error).toHaveBeenCalledWith('ElevenLabs service unavailable')
    })
  })

  // Test A11: obsUrl prop -> read-only input + Copy button
  it('A11: obsUrl prop renders read-only <input readOnly> + Copy button', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
    })
    const input = screen.getByLabelText(/OBS URL — copy and paste into OBS browser source/i) as HTMLInputElement
    expect(input).toBeDefined()
    expect(input.readOnly).toBe(true)
    expect(input.value).toBe('https://allch.at/overlay/abc?tts_token=xyz')
    expect(screen.getByRole('button', { name: /^Copy OBS URL$/ })).toBeDefined()
  })

  // Test A12: Clicking Copy OBS URL -> clipboard.writeText + toast
  it('A12: clicking Copy OBS URL writes URL to clipboard and shows success toast', async () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Copy OBS URL$/ }))
    })
    await waitFor(() => {
      expect(clipboardWriteMock).toHaveBeenCalledWith(
        'https://allch.at/overlay/abc?tts_token=xyz',
      )
      expect(toastMocks.success).toHaveBeenCalledWith('OBS URL copied.')
    })
  })

  // Test A13: Regenerate URL -> opens alertdialog
  it('A13: clicking Regenerate URL opens role=alertdialog confirmation modal', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
    })
    fireEvent.click(screen.getByRole('button', { name: /^Regenerate URL$/ }))
    expect(screen.getByRole('alertdialog')).toBeDefined()
    expect(screen.getByText(/This invalidates the current OBS URL/)).toBeDefined()
  })

  // Test A14: Cancel closes dialog without calling onRotateToken
  it('A14: clicking Cancel in the dialog closes it and does NOT fire onRotateToken', () => {
    const onRotateToken = vi.fn()
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
      onRotateToken,
    })
    fireEvent.click(screen.getByRole('button', { name: /^Regenerate URL$/ }))
    fireEvent.click(screen.getByRole('button', { name: /^Cancel$/ }))
    expect(onRotateToken).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })

  // Test A15: Confirm -> fires onRotateToken + writes new URL + toast
  it('A15: Confirm-Regenerate fires onRotateToken, copies new URL, shows "New OBS URL copied to clipboard." toast', async () => {
    const onRotateToken = vi.fn().mockResolvedValue({
      obsUrl: 'https://allch.at/overlay/abc?tts_token=NEW',
    })
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=OLD',
      onRotateToken,
    })
    fireEvent.click(screen.getByRole('button', { name: /^Regenerate URL$/ }))
    const dialog = screen.getByRole('alertdialog')
    // The destructive confirm button is the second Regenerate URL button, inside the dialog.
    const confirmBtn = dialog.querySelector('button.bg-red-500\\/10') as HTMLButtonElement
    expect(confirmBtn).not.toBeNull()
    await act(async () => {
      fireEvent.click(confirmBtn)
    })
    await waitFor(() => {
      expect(onRotateToken).toHaveBeenCalled()
      expect(clipboardWriteMock).toHaveBeenCalledWith(
        'https://allch.at/overlay/abc?tts_token=NEW',
      )
      expect(toastMocks.success).toHaveBeenCalledWith('New OBS URL copied to clipboard.')
    })
  })

  // Test A16: Remove key first click arms -> label "Confirm remove"; second click -> onRemoveKey
  it('A16: Remove-key first click arms (label->"Confirm remove"); second click within 3s fires onRemoveKey', async () => {
    const onRemoveKey = vi.fn().mockResolvedValue(undefined)
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
      onRemoveKey,
    })
    const removeBtn = screen.getByRole('button', { name: /^Remove key$/ }) as HTMLButtonElement
    fireEvent.click(removeBtn)
    // Label changes to "Confirm remove"
    const armed = await screen.findByRole('button', { name: /^Confirm remove$/ })
    expect(armed).toBeDefined()
    // Second click -> onRemoveKey
    await act(async () => {
      fireEvent.click(armed)
    })
    await waitFor(() => {
      expect(onRemoveKey).toHaveBeenCalled()
    })
  })

  // Test A17: After 3s with no second click, button reverts to "Remove key"
  it('A17: after 3s without second click, button reverts to "Remove key"', async () => {
    vi.useFakeTimers()
    try {
      const onRemoveKey = vi.fn().mockResolvedValue(undefined)
      renderTTSGroup({
        displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
        isPremium: true,
        hasElevenLabsConfig: true,
        obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
        onRemoveKey,
      })
      const removeBtn = screen.getByRole('button', { name: /^Remove key$/ })
      fireEvent.click(removeBtn)
      expect(screen.getByRole('button', { name: /^Confirm remove$/ })).toBeDefined()
      await act(async () => {
        vi.advanceTimersByTime(3001)
      })
      expect(screen.getByRole('button', { name: /^Remove key$/ })).toBeDefined()
      expect(screen.queryByRole('button', { name: /^Confirm remove$/ })).toBeNull()
      expect(onRemoveKey).not.toHaveBeenCalled()
    } finally {
      vi.useRealTimers()
    }
  })

  // Test A18: Saved-key flow auto-loads voices via onFetchVoices
  it('A18: hasSavedKey=true auto-loads voices via onFetchVoices on mount', async () => {
    const onFetchVoices = vi.fn().mockResolvedValue([
      { voice_id: 'v-alpha', name: 'Alpha' },
      { voice_id: 'v-bravo', name: 'Bravo' },
    ])
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: true,
      obsUrl: 'https://allch.at/overlay/abc?tts_token=xyz',
      onFetchVoices,
    })
    const select = screen.getByLabelText(/ElevenLabs voice/i) as HTMLSelectElement
    await waitFor(() => {
      expect(onFetchVoices).toHaveBeenCalledTimes(1)
      const optionTexts = Array.from(select.options).map((o) => o.textContent ?? '')
      expect(optionTexts).toContain('Alpha')
      expect(optionTexts).toContain('Bravo')
    })
  })

  // Test A19: Typed (unsaved) key drives onPreviewVoices and the picked voice
  // flows into the Save-key call. End-to-end exercise of the chicken-and-egg
  // bugfix.
  it('A19: typed key auto-fetches voices via onPreviewVoices; selection flows to onSaveKey', async () => {
    const onPreviewVoices = vi
      .fn<(apiKey: string) => Promise<ElevenLabsVoice[]>>()
      .mockResolvedValue([
        { voice_id: 'v-alpha', name: 'Alpha' },
        { voice_id: 'v-bravo', name: 'Bravo' },
      ])
    const onSaveKey = vi.fn().mockResolvedValue(undefined)
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onPreviewVoices,
      onSaveKey,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-new-key' } })
    const select = await waitForVoiceOption('Bravo')
    expect(onPreviewVoices).toHaveBeenCalledWith('sk-new-key')
    fireEvent.change(select, { target: { value: 'v-bravo' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    expect(onSaveKey).toHaveBeenCalledWith('sk-new-key', 'v-bravo')
  })

  // Test A19b: Save-key without a picked voice surfaces inline guidance
  // instead of failing server-side with "voice_id is required".
  it('A19b: Save-key with no picked voice shows "Pick a voice before saving." and does NOT call onSaveKey', async () => {
    const onSaveKey = vi.fn().mockResolvedValue(undefined)
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onSaveKey,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-typed-but-no-voice' } })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    expect(screen.getByText(/Pick a voice before saving\./)).toBeDefined()
    expect(onSaveKey).not.toHaveBeenCalled()
  })

  // Test A19c: API key input is rendered ABOVE the voice picker (placement
  // requirement — bug report: "the placement of the api key field is also
  // not good. please place it directly under the selector so it's
  // immediately visible, not after scrolling a bunch").
  it('A19c: API key input precedes the voice picker in DOM order', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i)
    const select = screen.getByLabelText(/ElevenLabs voice/i)
    // compareDocumentPosition: bit 0x04 = DOCUMENT_POSITION_FOLLOWING (select follows input)
    expect(input.compareDocumentPosition(select) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  // Test A19d: ADVANCED (ELEVENLABS) block renders BEFORE the THROTTLING
  // header — i.e. directly under the provider radio, not at the bottom of
  // the panel. Defends against the regression that the user reported.
  it('A19d: Advanced (ElevenLabs) block precedes the THROTTLING header in DOM order', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
    })
    const advancedHeader = screen.getByText(/ADVANCED \(ELEVENLABS\)/)
    const throttlingHeader = screen.getByText('THROTTLING')
    expect(
      advancedHeader.compareDocumentPosition(throttlingHeader) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()
  })

  // Test A19e: Voices auto-select the first option when the list loads with
  // no prior selection. Without this, Save fails with "Pick a voice before
  // saving." (the second symptom the user reported).
  it('A19e: typed key → voices load → first voice auto-selected → Save uses it', async () => {
    const onPreviewVoices = vi
      .fn<(apiKey: string) => Promise<ElevenLabsVoice[]>>()
      .mockResolvedValue([
        { voice_id: 'v-first', name: 'First' },
        { voice_id: 'v-second', name: 'Second' },
      ])
    const onSaveKey = vi.fn().mockResolvedValue(undefined)
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: true,
      hasElevenLabsConfig: false,
      onPreviewVoices,
      onSaveKey,
    })
    const input = screen.getByLabelText(/ElevenLabs API key/i) as HTMLInputElement
    fireEvent.change(input, { target: { value: 'sk-typed-key' } })
    const select = await waitForVoiceOption('First')
    // Auto-select kicks in: select.value should equal the first voice without
    // any explicit fireEvent.change(select, …).
    await waitFor(() => expect(select.value).toBe('v-first'))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^Save key$/ }))
    })
    expect(onSaveKey).toHaveBeenCalledWith('sk-typed-key', 'v-first')
  })

  // Test A20: Non-premium user sees disabled inputs + PremiumBadge overlay + upgrade copy
  it('A20: non-premium user sees upgrade copy and disabled API-key input under the Advanced block', () => {
    renderTTSGroup({
      displaySettings: baseSettings({ tts_provider: 'elevenlabs' }),
      isPremium: false,
      hasElevenLabsConfig: false,
    })
    // The upgrade-to-premium copy renders (overlay message)
    const upgradeMatches = screen.getAllByText(/Upgrade to Premium to use ElevenLabs voices\./)
    expect(upgradeMatches.length).toBeGreaterThan(0)
    // API-key input exists but is disabled
    const input = screen.queryByLabelText(/ElevenLabs API key/i) as HTMLInputElement | null
    if (input) {
      expect(input.disabled).toBe(true)
    }
  })
})
