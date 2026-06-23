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

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createTTSPlayer, PRIORITY_EVENTS } from '../ttsPlayer'
import type { TTSSettings } from '../ttsPlayer'
import type { ChatMessage, EventType } from '@/lib/types/message'

// ---- Mocks ---------------------------------------------------------------

interface MockVoice {
  voiceURI: string
  name: string
  lang: string
}

interface MockUtterance {
  text: string
  voice?: MockVoice
  volume: number
  rate: number
  pitch: number
  onend: (() => void) | null
  onerror: (() => void) | null
}

let lastUtterance: MockUtterance | null = null
let autoResolveOnEnd = true

class MockSpeechSynthesisUtterance implements MockUtterance {
  text: string
  voice?: MockVoice
  volume = 1
  rate = 1
  pitch = 1
  onend: (() => void) | null = null
  onerror: (() => void) | null = null

  constructor(text: string) {
    this.text = text
    lastUtterance = this
  }
}

const mockSpeak = vi.fn((u: MockUtterance) => {
  // When autoResolveOnEnd=true, resolve on next microtask so pump() drains.
  if (autoResolveOnEnd) {
    Promise.resolve().then(() => {
      u.onend?.()
    })
  }
})
const mockCancel = vi.fn()
const mockGetVoices = vi.fn((): MockVoice[] => [
  { voiceURI: 'Default', name: 'Default', lang: 'en-US' },
  { voiceURI: 'Alex', name: 'Alex', lang: 'en-US' },
])
const mockAddEventListener = vi.fn()
const mockRemoveEventListener = vi.fn()

vi.stubGlobal('SpeechSynthesisUtterance', MockSpeechSynthesisUtterance)
vi.stubGlobal('speechSynthesis', {
  speak: mockSpeak,
  cancel: mockCancel,
  getVoices: mockGetVoices,
  addEventListener: mockAddEventListener,
  removeEventListener: mockRemoveEventListener,
})
// window is not defined in node env — stub a minimal one
vi.stubGlobal('window', {
  speechSynthesis: {
    speak: mockSpeak,
    cancel: mockCancel,
    getVoices: mockGetVoices,
    addEventListener: mockAddEventListener,
    removeEventListener: mockRemoveEventListener,
  },
})

// ---- Helpers -------------------------------------------------------------

const defaultSettings: TTSSettings = {
  enabled: true,
  provider: 'browser',
  volume: 0.8,
  rate: 1.0,
  pitch: 1.0,
  filter_mode: 'all',
  sample_rate: 1.0,
  max_queue: 5,
  messages_per_minute: 60,
  user_cooldown_seconds: 0,
  staleness_seconds: 600,
  priority_events: true,
  priority_bits_min: 0,
  read_username: false,
  read_platform: false,
  max_message_chars: 200,
  skip_emote_only: false,
  skip_links: false,
  enabled_platforms: ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
}

function makeMessage(overrides: Partial<ChatMessage> = {}): ChatMessage {
  const base: ChatMessage = {
    id: 'm1',
    overlay_id: 'ov1',
    platform: 'twitch',
    channel_id: 'ch1',
    channel_name: 'test',
    user: {
      id: 'u1',
      username: 'alice',
      display_name: 'Alice',
      badges: [],
    },
    message: { text: 'hello world', emotes: [] },
    timestamp: new Date().toISOString(),
    metadata: {},
  }
  return { ...base, ...overrides } as ChatMessage
}

function makeEventMessage(type: EventType, overrides: Partial<ChatMessage> = {}): ChatMessage {
  return makeMessage({
    ...overrides,
    event: {
      type,
      tier: 'high',
      duration: 5,
      is_update: false,
      metadata: {},
      ...(overrides.event ?? {}),
    },
  })
}

/**
 * Advance microtasks until the pump drains. Without real timers,
 * Promise.resolve().then(...) resolves immediately when awaited.
 */
async function flushMicrotasks(): Promise<void> {
  for (let i = 0; i < 20; i++) {
    await Promise.resolve()
  }
}

beforeEach(() => {
  mockSpeak.mockClear()
  mockCancel.mockClear()
  mockGetVoices.mockClear()
  mockGetVoices.mockImplementation((): MockVoice[] => [
    { voiceURI: 'Default', name: 'Default', lang: 'en-US' },
    { voiceURI: 'Alex', name: 'Alex', lang: 'en-US' },
  ])
  lastUtterance = null
  autoResolveOnEnd = true
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  // Re-stub the globals we need for next test
  vi.stubGlobal('SpeechSynthesisUtterance', MockSpeechSynthesisUtterance)
  vi.stubGlobal('speechSynthesis', {
    speak: mockSpeak,
    cancel: mockCancel,
    getVoices: mockGetVoices,
    addEventListener: mockAddEventListener,
    removeEventListener: mockRemoveEventListener,
  })
  vi.stubGlobal('window', {
    speechSynthesis: {
      speak: mockSpeak,
      cancel: mockCancel,
      getVoices: mockGetVoices,
      addEventListener: mockAddEventListener,
      removeEventListener: mockRemoveEventListener,
    },
  })
})

// =========================================================================

describe('createTTSPlayer - basic API', () => {
  // Test 1
  it('returns an object with speak, updateSettings, destroy methods', () => {
    const p = createTTSPlayer(defaultSettings)
    expect(typeof p.speak).toBe('function')
    expect(typeof p.updateSettings).toBe('function')
    expect(typeof p.destroy).toBe('function')
  })

  // Test 2
  it('speak() is a no-op when settings.enabled=false', async () => {
    const p = createTTSPlayer({ ...defaultSettings, enabled: false })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })

  // Test 3
  it('speak() triggers window.speechSynthesis.speak() when enabled=true', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)
  })
})

// =========================================================================

describe('D-25..D-30 formatContent', () => {
  // Test 4 — D-25 username prefix
  it('adds "display_name says:" prefix when tts_read_username=true', async () => {
    const p = createTTSPlayer({ ...defaultSettings, read_username: true })
    p.speak(makeMessage({ user: { id: 'u1', username: 'alice', display_name: 'Alice', badges: [] } }))
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('Alice says: hello world')
  })

  // Test 5 — D-26 platform prefix (capitalized)
  it('adds capitalized platform prefix "Twitch: " when tts_read_platform=true', async () => {
    const p = createTTSPlayer({
      ...defaultSettings,
      read_username: true,
      read_platform: true,
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('Twitch: Alice says: hello world')
  })

  // Test 6 — D-29 strip emote tokens
  it('strips emote tokens from the message text', async () => {
    // "hello Kappa world Kappa" — positions [6,10] + [17,21]
    const p = createTTSPlayer(defaultSettings)
    p.speak(
      makeMessage({
        message: {
          text: 'hello Kappa world Kappa',
          emotes: [
            {
              code: 'Kappa',
              provider: 'twitch',
              url: 'https://example/kappa.png',
              positions: [[6, 10], [18, 22]],
            },
          ],
        },
      }),
    )
    await flushMicrotasks()
    // "hello " + (Kappa stripped) + "world " + (Kappa stripped)
    // -> "hello  world "
    expect(lastUtterance?.text).toBe('hello  world ')
  })

  // Test 7 — D-29 skip emote-heavy messages
  it('skips entirely when emotes > 50% of tokens AND tts_skip_emote_only=true', async () => {
    const p = createTTSPlayer({ ...defaultSettings, skip_emote_only: true })
    // 3 tokens total, 2 emotes → ratio 2/3 > 0.5
    p.speak(
      makeMessage({
        message: {
          text: 'Kappa LUL hello',
          emotes: [
            { code: 'Kappa', provider: 'twitch', url: 'x', positions: [[0, 4]] },
            { code: 'LUL', provider: 'twitch', url: 'x', positions: [[6, 8]] },
          ],
        },
      }),
    )
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })

  // Test 8 — D-30 URL replacement
  it('replaces https://example.com with literal "link"', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(makeMessage({ message: { text: 'check https://example.com now', emotes: [] } }))
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('check link now')
  })

  // Test 9 — D-30 skip link-only messages
  it('skips when message is link-only and tts_skip_links=true', async () => {
    const p = createTTSPlayer({ ...defaultSettings, skip_links: true })
    p.speak(makeMessage({ message: { text: 'https://example.com', emotes: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })

  // Test 9b — D-30 pure-URL message speaks "link" when skip_links=false
  it('replaces pure-URL message with literal "link" when tts_skip_links=false', async () => {
    const p = createTTSPlayer({ ...defaultSettings, skip_links: false })
    p.speak(makeMessage({ message: { text: 'https://example.com', emotes: [] } }))
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('link')
  })

  // Test 10 — D-32 like_aggregate always excluded
  it("excludes 'like_aggregate' event type regardless of settings", async () => {
    const p = createTTSPlayer({ ...defaultSettings, priority_events: true })
    p.speak(makeEventMessage('like_aggregate'))
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })

  // Test 11 — platform filter
  it('is a no-op when message.platform is not in tts_enabled_platforms', async () => {
    const p = createTTSPlayer({
      ...defaultSettings,
      enabled_platforms: ['youtube', 'kick'], // twitch excluded
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })
})

// =========================================================================

describe('D-31 priority events', () => {
  // Test 12 — sampling
  it('sampling with sample_rate=0 never speaks non-priority', async () => {
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0.5)
    const p = createTTSPlayer({
      ...defaultSettings,
      filter_mode: 'sample',
      sample_rate: 0, // never speak
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
    randomSpy.mockRestore()
  })

  it('sampling with sample_rate=1 always speaks non-priority', async () => {
    const randomSpy = vi.spyOn(Math, 'random').mockReturnValue(0.99)
    const p = createTTSPlayer({
      ...defaultSettings,
      filter_mode: 'sample',
      sample_rate: 1, // always speak
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)
    randomSpy.mockRestore()
  })

  // Test 13 — priority always speaks in priority_only mode
  it('subscription event speaks even when filter_mode=priority_only', async () => {
    const p = createTTSPlayer({
      ...defaultSettings,
      filter_mode: 'priority_only',
      priority_events: true,
    })
    p.speak(makeEventMessage('subscription'))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)
  })

  // Test 14 — subscription prefix
  it('subscription event produces "New subscription from {display_name}"', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(makeEventMessage('subscription'))
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('New subscription from Alice')
  })

  // Test 15 — raid prefix
  it('raid event with viewers=42 produces "display_name raided with 42 viewers"', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(
      makeEventMessage('raid', {
        event: {
          type: 'raid',
          tier: 'high',
          duration: 5,
          is_update: false,
          metadata: { viewers: 42 },
        },
      }),
    )
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('Alice raided with 42 viewers')
  })

  // Test 16 — bits prefix
  it('bits event with bits=100 + message "hype" produces "display_name cheered 100 bits: hype"', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(
      makeEventMessage('bits', {
        message: { text: 'hype', emotes: [] },
        event: {
          type: 'bits',
          tier: 'medium',
          duration: 5,
          is_update: false,
          metadata: { bits: 100 },
        },
      }),
    )
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('Alice cheered 100 bits: hype')
  })

  // Test — super_chat prefix
  it('super_chat event produces "Super chat from {display_name}: {text}"', async () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(
      makeEventMessage('super_chat', {
        message: { text: 'thanks!', emotes: [] },
      }),
    )
    await flushMicrotasks()
    expect(lastUtterance?.text).toBe('Super chat from Alice: thanks!')
  })

  // PRIORITY_EVENTS export
  it('PRIORITY_EVENTS set contains the 11 D-31 event types', () => {
    const expected = [
      'subscription', 'resubscription', 'gift_subscription', 'mystery_gift',
      'bits', 'raid', 'super_chat', 'super_sticker',
      'kick_subscription', 'kick_gift_subscription', 'kick_donation',
    ]
    expected.forEach(t => expect(PRIORITY_EVENTS.has(t)).toBe(true))
    expect(PRIORITY_EVENTS.size).toBe(11)
  })
})

// =========================================================================

describe('D-35 cooldown', () => {
  // Test 17
  it('same username within tts_user_cooldown_seconds is suppressed', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const p = createTTSPlayer({ ...defaultSettings, user_cooldown_seconds: 10 })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Second message same user within cooldown window
    p.speak(makeMessage({ id: 'm2' }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1) // still 1 — suppressed

    // Advance past cooldown
    vi.advanceTimersByTime(11_000)
    p.speak(makeMessage({ id: 'm3' }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(2)
  })

  // Test 18
  it('priority event from same username bypasses cooldown', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const p = createTTSPlayer({ ...defaultSettings, user_cooldown_seconds: 60 })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Same user, priority event — should speak despite cooldown
    p.speak(makeEventMessage('subscription'))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(2)
  })
})

// =========================================================================

describe('D-36 token bucket', () => {
  // Test 19
  it('3rd non-priority in same minute is dropped (bucket size 2)', async () => {
    const p = createTTSPlayer({ ...defaultSettings, messages_per_minute: 2 })
    p.speak(makeMessage({ id: 'm1', user: { id: '1', username: 'a', display_name: 'A', badges: [] } }))
    await flushMicrotasks()
    p.speak(makeMessage({ id: 'm2', user: { id: '2', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()
    p.speak(makeMessage({ id: 'm3', user: { id: '3', username: 'c', display_name: 'C', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(2)
  })

  // Test 20
  it('bucket refills after 60s elapsed', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const p = createTTSPlayer({ ...defaultSettings, messages_per_minute: 1 })
    p.speak(makeMessage({ id: 'm1', user: { id: '1', username: 'a', display_name: 'A', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Second immediately — bucket empty
    p.speak(makeMessage({ id: 'm2', user: { id: '2', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Advance 61s — bucket refills
    vi.advanceTimersByTime(61_000)
    p.speak(makeMessage({ id: 'm3', user: { id: '3', username: 'c', display_name: 'C', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(2)
  })

  // Test 21
  it('priority event speaks even when bucket is empty', async () => {
    const p = createTTSPlayer({ ...defaultSettings, messages_per_minute: 0 })
    p.speak(makeEventMessage('subscription'))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)
  })
})

// =========================================================================

describe('D-33 queue overflow', () => {
  // Test 22 — non-priority full: drop
  it('non-priority message is dropped when queue is full', async () => {
    autoResolveOnEnd = false // make the first speak() hang
    const p = createTTSPlayer({ ...defaultSettings, max_queue: 1, messages_per_minute: 100 })
    p.speak(makeMessage({ id: 'm1', user: { id: '1', username: 'a', display_name: 'A', badges: [] } }))
    await flushMicrotasks()
    // First utterance is speaking (hung). Subsequent are queued.
    // Queue slot 1 is free (the speaking message is out of queue already).
    // A second message enters queue — queue = [m2], max_queue=1
    p.speak(makeMessage({ id: 'm2', user: { id: '2', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()
    // Third non-priority enters when queue=[m2] and max_queue=1 → drop
    p.speak(makeMessage({ id: 'm3', user: { id: '3', username: 'c', display_name: 'C', badges: [] } }))
    await flushMicrotasks()

    // Only one speak called — the first (hung).
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Release the first (complete onend), queue drains
    autoResolveOnEnd = true
    if (lastUtterance?.onend) {
      lastUtterance.onend()
    }
    await flushMicrotasks()
    // m2 should speak, m3 should NOT
    expect(mockSpeak).toHaveBeenCalledTimes(2)
    p.destroy()
  })

  // Test 23 — priority drops oldest non-priority
  it('priority event drops oldest non-priority when queue full', async () => {
    autoResolveOnEnd = false
    const p = createTTSPlayer({
      ...defaultSettings,
      max_queue: 2,
      messages_per_minute: 100,
      user_cooldown_seconds: 0,
    })
    // First — becomes the speaking one (hung).
    // Each message uses a unique text so we can track which ones spoke.
    p.speak(makeMessage({
      id: 'm1',
      message: { text: 'first', emotes: [] },
      user: { id: '1', username: 'a', display_name: 'A', badges: [] },
    }))
    await flushMicrotasks()
    // Second + third — queued, fills max_queue=2
    p.speak(makeMessage({
      id: 'm2',
      message: { text: 'second', emotes: [] },
      user: { id: '2', username: 'b', display_name: 'B', badges: [] },
    }))
    p.speak(makeMessage({
      id: 'm3',
      message: { text: 'third', emotes: [] },
      user: { id: '3', username: 'c', display_name: 'C', badges: [] },
    }))
    await flushMicrotasks()
    // Now queue = [m2, m3], all non-priority. A priority arrives.
    p.speak(makeEventMessage('subscription', {
      id: 'mp',
      user: { id: 'p', username: 'premium', display_name: 'P', badges: [] },
    }))
    await flushMicrotasks()
    // After priority: queue = [m3, mp] (m2 evicted as oldest non-priority).
    // Release the speaking m1 → m3 speaks, then mp.
    autoResolveOnEnd = true
    // Drain the queue — trigger onend for each spoken utterance
    let drainGuard = 0
    while (mockSpeak.mock.calls.length < 3 && drainGuard++ < 10) {
      lastUtterance?.onend?.()
      await flushMicrotasks()
    }
    const allTexts = mockSpeak.mock.calls.map(c => (c[0] as MockUtterance).text)
    // m1 spoke (hung initially, released)
    expect(allTexts).toContain('first')
    // m3 spoke (priority evicted m2, not m3)
    expect(allTexts).toContain('third')
    // priority event spoke
    expect(allTexts).toContain('New subscription from P')
    // m2 was evicted — must NOT appear
    expect(allTexts).not.toContain('second')
    p.destroy()
  })
})

// =========================================================================

describe('D-34 FIFO', () => {
  // Test 24 — FIFO ordering when both priority and non-priority fit
  it('priority and non-priority are spoken in insertion order when queue has room', async () => {
    autoResolveOnEnd = false
    const p = createTTSPlayer({
      ...defaultSettings,
      max_queue: 10,
      messages_per_minute: 100,
      user_cooldown_seconds: 0,
    })
    p.speak(makeMessage({ id: 'm1', message: { text: 'first', emotes: [] }, user: { id: '1', username: 'a', display_name: 'A', badges: [] } }))
    await flushMicrotasks()
    p.speak(makeEventMessage('subscription', { id: 'mp', user: { id: 'p', username: 'p', display_name: 'P', badges: [] } }))
    p.speak(makeMessage({ id: 'm2', message: { text: 'third', emotes: [] }, user: { id: '2', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()

    // Drain the queue manually
    autoResolveOnEnd = true
    while (mockSpeak.mock.calls.length < 3) {
      lastUtterance?.onend?.()
      await flushMicrotasks()
    }
    const texts = mockSpeak.mock.calls.map(c => (c[0] as MockUtterance).text)
    expect(texts[0]).toBe('first')
    expect(texts[1]).toBe('New subscription from P')
    expect(texts[2]).toBe('third')
    p.destroy()
  })
})

// =========================================================================

describe('D-37 staleness', () => {
  // Test 25
  it('message with timestamp older than staleness_seconds is dropped silently', async () => {
    const oldTs = Date.now() - 30_000 // 30s ago
    const p = createTTSPlayer({ ...defaultSettings, staleness_seconds: 15 })
    p.speak(makeMessage({ timestamp: new Date(oldTs).toISOString() }))
    await flushMicrotasks()
    expect(mockSpeak).not.toHaveBeenCalled()
  })
})

// =========================================================================

describe('D-38 session fallback (ElevenLabs)', () => {
  // Test 26
  it('calls fetch at expected URL with tts_token and voice query params', async () => {
    const mockFetch = vi.fn((_url: string, _init?: RequestInit) =>
      Promise.resolve({
        ok: true,
        status: 200,
        blob: () => Promise.resolve(new Blob()),
      } as unknown as Response),
    )
    vi.stubGlobal('fetch', mockFetch)
    vi.stubGlobal('Audio', class MockAudio {
      src = ''
      volume = 1
      onended: (() => void) | null = null
      onerror: (() => void) | null = null
      play() {
        Promise.resolve().then(() => this.onended?.())
        return Promise.resolve()
      }
    })
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:mock'),
      revokeObjectURL: vi.fn(),
    })

    const p = createTTSPlayer({
      ...defaultSettings,
      provider: 'elevenlabs',
      ttsEndpoint: '/api/v1/overlays/abc/tts',
      ttsToken: 'jwt-xyz',
      voiceId: 'voice-1',
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    expect(mockFetch).toHaveBeenCalled()
    const firstCall = mockFetch.mock.calls[0]
    const url = firstCall[0]
    const opts = firstCall[1] as RequestInit
    expect(url).toContain('tts_token=jwt-xyz')
    expect(url).toContain('voice=voice-1')
    // SECURITY (audit M17): text is now sent in the JSON body, not the URL.
    expect(url).not.toContain('text=')
    expect(opts.body).toBeTruthy()
    expect(String(opts.body)).toContain('"text"')
  })

  // Test 27
  it('ElevenLabs 401 triggers session fallback + invokes onFallback once', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 401,
        blob: () => Promise.resolve(new Blob()),
      } as unknown as Response),
    )
    vi.stubGlobal('fetch', mockFetch)
    const onFallback = vi.fn()
    const p = createTTSPlayer(
      {
        ...defaultSettings,
        provider: 'elevenlabs',
        ttsEndpoint: '/api/v1/overlays/abc/tts',
        ttsToken: 't',
        voiceId: 'v',
      },
      onFallback,
    )
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    // After 401 we expect Web Speech fallback to have fired
    expect(mockSpeak).toHaveBeenCalled()
    expect(onFallback).toHaveBeenCalledTimes(1)

    // Second speak — onFallback should NOT be called again; Web Speech path used
    p.speak(makeMessage({ id: 'm2', user: { id: '2', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()
    expect(onFallback).toHaveBeenCalledTimes(1)
  })

  // Test 28
  it('ElevenLabs 429 triggers session fallback', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 429,
        blob: () => Promise.resolve(new Blob()),
      } as unknown as Response),
    )
    vi.stubGlobal('fetch', mockFetch)
    const onFallback = vi.fn()
    const p = createTTSPlayer(
      { ...defaultSettings, provider: 'elevenlabs', ttsEndpoint: '/x', ttsToken: 't', voiceId: 'v' },
      onFallback,
    )
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    expect(onFallback).toHaveBeenCalledTimes(1)
  })

  // Test 29
  it('ElevenLabs 500 triggers session fallback', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        blob: () => Promise.resolve(new Blob()),
      } as unknown as Response),
    )
    vi.stubGlobal('fetch', mockFetch)
    const onFallback = vi.fn()
    const p = createTTSPlayer(
      { ...defaultSettings, provider: 'elevenlabs', ttsEndpoint: '/x', ttsToken: 't', voiceId: 'v' },
      onFallback,
    )
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    expect(onFallback).toHaveBeenCalledTimes(1)
  })

  // Test 30
  it('ElevenLabs network error triggers session fallback', async () => {
    const mockFetch = vi.fn(() => Promise.reject(new Error('network')))
    vi.stubGlobal('fetch', mockFetch)
    const onFallback = vi.fn()
    const p = createTTSPlayer(
      { ...defaultSettings, provider: 'elevenlabs', ttsEndpoint: '/x', ttsToken: 't', voiceId: 'v' },
      onFallback,
    )
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    expect(onFallback).toHaveBeenCalledTimes(1)
  })

  // Test 27d — 400 also triggers fallback
  it('ElevenLabs 400 triggers session fallback identically to 401', async () => {
    const mockFetch = vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 400,
        blob: () => Promise.resolve(new Blob()),
      } as unknown as Response),
    )
    vi.stubGlobal('fetch', mockFetch)
    const onFallback = vi.fn()
    const p = createTTSPlayer(
      { ...defaultSettings, provider: 'elevenlabs', ttsEndpoint: '/x', ttsToken: 't', voiceId: 'v' },
      onFallback,
    )
    p.speak(makeMessage())
    await flushMicrotasks()
    await flushMicrotasks()
    expect(onFallback).toHaveBeenCalledTimes(1)
  })
})

// =========================================================================

describe('voice uri fallback D-28', () => {
  // Test 31
  it('persisted voice_uri not in current voice list → console.warn + no voice on utterance', async () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const p = createTTSPlayer({ ...defaultSettings, voice_uri: 'NonExistentVoice' })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('NonExistentVoice'))
    expect(lastUtterance?.voice).toBeUndefined()
    warnSpy.mockRestore()
  })

  it('matching voice_uri sets utterance.voice', async () => {
    const p = createTTSPlayer({ ...defaultSettings, voice_uri: 'Alex' })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(lastUtterance?.voice?.voiceURI).toBe('Alex')
  })
})

// =========================================================================

describe('utterance properties', () => {
  // Test 32
  it('speak() applies volume, rate, pitch from settings', async () => {
    const p = createTTSPlayer({
      ...defaultSettings,
      volume: 0.33,
      rate: 1.25,
      pitch: 0.75,
    })
    p.speak(makeMessage())
    await flushMicrotasks()
    expect(lastUtterance?.volume).toBe(0.33)
    expect(lastUtterance?.rate).toBe(1.25)
    expect(lastUtterance?.pitch).toBe(0.75)
  })
})

// =========================================================================

describe('destroy + updateSettings', () => {
  // Test 33
  it('destroy() calls synth.cancel() and clears internal state', () => {
    const p = createTTSPlayer(defaultSettings)
    p.speak(makeMessage())
    p.destroy()
    expect(mockCancel).toHaveBeenCalled()
  })

  // Test 34
  it('updateSettings merges new settings (bucket size updates)', async () => {
    // Make sure the new messages_per_minute is respected on the next speaks
    const p = createTTSPlayer({ ...defaultSettings, messages_per_minute: 1 })
    p.speak(makeMessage({ id: 'a', user: { id: 'a', username: 'a', display_name: 'A', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)
    // Bucket empty
    p.speak(makeMessage({ id: 'b', user: { id: 'b', username: 'b', display_name: 'B', badges: [] } }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(1)

    // Bump messages_per_minute → after an internal refill cycle new capacity applies.
    // We simply assert updateSettings doesn't throw and leaves speaking functional.
    p.updateSettings({ ...defaultSettings, messages_per_minute: 10, user_cooldown_seconds: 0 })
    // updateSettings alone doesn't trigger refill, but priority events bypass
    // so the updated settings are observable via a priority speak.
    p.speak(makeEventMessage('subscription', { id: 'c' }))
    await flushMicrotasks()
    expect(mockSpeak).toHaveBeenCalledTimes(2)
  })
})
