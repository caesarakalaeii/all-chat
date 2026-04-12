import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createSoundPlayer } from '../soundPlayer'
import type { SoundSettings } from '../soundPlayer'

// Mock HTMLAudioElement globally
const mockPlay = vi.fn().mockResolvedValue(undefined)
const mockPause = vi.fn()
const mockAudioInstances: Array<{
  src: string
  volume: number
  currentTime: number
  play: typeof mockPlay
  pause: typeof mockPause
  onerror: (() => void) | null
}> = []

// Use a class-based mock so `new Audio()` works correctly
class MockAudio {
  src = ''
  volume = 1
  currentTime = 0
  play = mockPlay
  pause = mockPause
  onerror: (() => void) | null = null

  constructor() {
    mockAudioInstances.push(this as unknown as (typeof mockAudioInstances)[0])
  }
}

vi.stubGlobal('Audio', MockAudio)

describe('createSoundPlayer', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockPlay.mockClear()
    mockPause.mockClear()
    mockAudioInstances.length = 0
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // Test 1: createSoundPlayer returns object with play, updateSettings, destroy methods
  it('returns an object with play, updateSettings, and destroy methods', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    expect(typeof player.play).toBe('function')
    expect(typeof player.updateSettings).toBe('function')
    expect(typeof player.destroy).toBe('function')
  })

  // Test 2: play() is a no-op when enabled=false
  it('play() does not trigger Audio.play() when enabled=false', () => {
    const settings: SoundSettings = {
      enabled: false,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    player.play()
    expect(mockPlay).not.toHaveBeenCalled()
  })

  // Test 3: play() triggers Audio.play() when enabled=true
  it('play() triggers Audio.play() when enabled=true', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    player.play()
    expect(mockPlay).toHaveBeenCalledTimes(1)
  })

  // Test 4: play() respects cooldown — second call within cooldownMs does not trigger Audio.play()
  it('play() respects cooldown — second call within cooldownMs does not trigger Audio.play()', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    player.play()
    expect(mockPlay).toHaveBeenCalledTimes(1)
    // Call immediately again — within cooldown
    player.play()
    expect(mockPlay).toHaveBeenCalledTimes(1)
  })

  // Test 5: play() triggers Audio.play() after cooldown expires
  it('play() triggers Audio.play() after cooldown expires', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    player.play()
    expect(mockPlay).toHaveBeenCalledTimes(1)
    // Advance time past cooldown
    vi.advanceTimersByTime(501)
    player.play()
    expect(mockPlay).toHaveBeenCalledTimes(2)
  })

  // Test 6: play() uses customUrl when provided
  it('play() sets el.src to customUrl when provided', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
      customUrl: 'https://example.com/mysound.mp3',
    }
    const player = createSoundPlayer(settings)
    player.play()
    // At least one audio instance should have src set to customUrl
    const usedInstance = mockAudioInstances.find(
      i => i.src === 'https://example.com/mysound.mp3'
    )
    expect(usedInstance).toBeDefined()
  })

  // Test 7: play() uses preset URL /sounds/{preset}.mp3 when no customUrl
  it('play() uses preset URL /sounds/{preset}.mp3 when no customUrl', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'ping',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    player.play()
    const usedInstance = mockAudioInstances.find(i => i.src === '/sounds/ping.mp3')
    expect(usedInstance).toBeDefined()
  })

  // Test 8: updateSettings() changes volume on pool elements
  it('updateSettings() updates volume on all pool elements', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    // Pool is created on instantiation, so instances are available
    const instancesBefore = mockAudioInstances.length
    expect(instancesBefore).toBeGreaterThan(0)

    player.updateSettings({ ...settings, volume: 0.9 })
    // All pool elements should have volume updated
    mockAudioInstances.slice(0, instancesBefore).forEach(i => {
      expect(i.volume).toBe(0.9)
    })
  })

  // Test 9: destroy() pauses all pool elements and clears src
  it('destroy() pauses all pool elements and clears src', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    const poolInstances = [...mockAudioInstances]
    player.destroy()
    poolInstances.forEach(i => {
      expect(mockPause).toHaveBeenCalled()
      expect(i.src).toBe('')
    })
  })

  // Test 10: play() does not re-assign el.src when URL has not changed
  it('play() does not re-assign el.src when URL has not changed (Pitfall 2 avoidance)', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'chime',
      volume: 0.5,
      cooldownMs: 500,
    }
    const player = createSoundPlayer(settings)
    // First play — sets src
    player.play()
    const firstInstance = mockAudioInstances[mockAudioInstances.length - 1]
    const srcAfterFirst = firstInstance.src

    // Advance past cooldown so second play() is allowed
    vi.advanceTimersByTime(501)

    // Track src assignments by intercepting the property
    let srcAssignmentCount = 0
    const originalSrc = firstInstance.src
    Object.defineProperty(firstInstance, 'src', {
      get: () => originalSrc,
      set: (val: string) => {
        if (val === srcAfterFirst) srcAssignmentCount++
      },
      configurable: true,
    })

    // Second play with same URL — should not re-assign src
    player.play()
    expect(srcAssignmentCount).toBe(0)
  })

  // Test 11: onerror on custom URL falls back to preset URL (D-08)
  it('onerror on custom URL falls back to preset URL', () => {
    const settings: SoundSettings = {
      enabled: true,
      preset: 'pop',
      volume: 0.5,
      cooldownMs: 500,
      customUrl: 'https://example.com/broken.mp3',
    }
    const player = createSoundPlayer(settings)
    player.play()

    // Find the instance that was used (has customUrl as src)
    const usedInstance = mockAudioInstances.find(
      i => i.src === 'https://example.com/broken.mp3'
    )
    expect(usedInstance).toBeDefined()

    // Trigger onerror
    expect(usedInstance!.onerror).toBeTypeOf('function')
    usedInstance!.onerror!()

    // After onerror, src should fall back to preset URL
    expect(usedInstance!.src).toBe('/sounds/pop.mp3')
  })
})
