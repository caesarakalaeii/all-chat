/**
 * soundPlayer.ts
 *
 * Audio pool-based sound player with cooldown logic for chat notification sounds.
 *
 * Design:
 * - Pre-creates a small pool of HTMLAudioElement instances (pool size 3)
 * - Round-robin selection across pool elements for natural overlap support
 * - Cooldown timer prevents audio spam in high-traffic chats (D-05)
 * - Custom URL falls back to preset on load failure (D-08)
 * - Only reassigns el.src when URL changes (Pitfall 2 avoidance)
 */

export const PRESET_NAMES = ['chime', 'pop', 'ping'] as const
export type PresetName = (typeof PRESET_NAMES)[number]

export interface SoundSettings {
  enabled: boolean
  preset: string
  volume: number
  cooldownMs: number
  customUrl?: string
}

export interface SoundPlayer {
  play(): void
  updateSettings(settings: SoundSettings): void
  destroy(): void
}

const POOL_SIZE = 3

function resolveUrl(settings: SoundSettings): string {
  if (settings.customUrl) return settings.customUrl
  return `/sounds/${settings.preset}.mp3`
}

/**
 * createSoundPlayer creates a new sound player with an internal audio pool.
 * Returns a SoundPlayer object with play(), updateSettings(), and destroy().
 */
export function createSoundPlayer(initialSettings: SoundSettings): SoundPlayer {
  let settings: SoundSettings = { ...initialSettings }

  // Pre-create pool of HTMLAudioElement instances
  const pool: HTMLAudioElement[] = Array.from({ length: POOL_SIZE }, () => {
    const el = new Audio()
    el.volume = settings.volume
    return el
  })

  let poolIndex = 0
  let lastPlayedAt = 0

  function play(): void {
    if (!settings.enabled) return

    const now = Date.now()
    if (now - lastPlayedAt < settings.cooldownMs) return
    lastPlayedAt = now

    const el = pool[poolIndex % POOL_SIZE]
    poolIndex++

    const url = resolveUrl(settings)

    // Only reassign el.src when URL has changed (Pitfall 2 avoidance)
    if (el.src !== url) {
      el.src = url

      // D-08: on custom URL load failure, fall back to preset URL
      if (settings.customUrl) {
        el.onerror = () => {
          el.onerror = null
          el.src = `/sounds/${settings.preset}.mp3`
        }
      } else {
        el.onerror = null
      }
    }

    el.currentTime = 0
    el.play().catch(() => {
      // Autoplay may be blocked in non-OBS browsers without user gesture.
      // Fail silently — the next message will retry.
    })
  }

  function updateSettings(newSettings: SoundSettings): void {
    settings = { ...newSettings }
    pool.forEach(el => {
      el.volume = settings.volume
    })
  }

  function destroy(): void {
    pool.forEach(el => {
      el.pause()
      el.src = ''
    })
  }

  return { play, updateSettings, destroy }
}
