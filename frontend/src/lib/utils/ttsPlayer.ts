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

/**
 * ttsPlayer.ts
 *
 * Client-side Text-to-Speech pipeline for chat messages.
 * Implements decisions D-24..D-42 from Phase 13 context.
 *
 * Features:
 *  - Queue with FIFO ordering and priority-event eviction (D-33, D-34)
 *  - Content formatting: username prefix, platform prefix, emote stripping,
 *    URL → "link" replacement, event-specific prefixes (D-25..D-31)
 *  - Per-user cooldown (D-35) — priority bypasses
 *  - Token-bucket rate limiter (D-36) — priority bypasses
 *  - Staleness check at dequeue (D-37)
 *  - Session-wide ElevenLabs → Web Speech fallback on any error (D-38)
 *  - Web Speech voice-URI persistence with console.warn fallback (D-28)
 */

import type { ChatMessage, EventType } from '@/lib/types/message'

export interface TTSSettings {
  enabled: boolean
  provider: 'browser' | 'elevenlabs'
  volume: number
  voice_uri?: string
  rate: number
  pitch: number
  filter_mode: 'all' | 'sample' | 'priority_only'
  sample_rate: number
  max_queue: number
  messages_per_minute: number
  user_cooldown_seconds: number
  staleness_seconds: number
  priority_events: boolean
  priority_bits_min: number
  read_username: boolean
  read_platform: boolean
  max_message_chars: number
  skip_emote_only: boolean
  skip_links: boolean
  enabled_platforms: string[]
  // ElevenLabs runtime (optional; populated when provider === 'elevenlabs')
  ttsEndpoint?: string
  ttsToken?: string
  voiceId?: string
}

export interface TTSPlayer {
  speak(message: ChatMessage): void
  updateSettings(settings: TTSSettings): void
  destroy(): void
}

/**
 * PRIORITY_EVENTS — the 11 event types that bypass sampling, cooldown,
 * and rate limiting (D-31). TikTok `like_aggregate` is explicitly NOT in
 * this set and is always excluded separately (D-32).
 */
export const PRIORITY_EVENTS: ReadonlySet<string> = new Set<string>([
  'subscription',
  'resubscription',
  'gift_subscription',
  'mystery_gift',
  'bits',
  'raid',
  'super_chat',
  'super_sticker',
  'kick_subscription',
  'kick_gift_subscription',
  'kick_donation',
])

const URL_REGEX = /https?:\/\/\S+/gi

interface QueueItem {
  message: ChatMessage
  priority: boolean
  enqueuedAt: number
}

function capitalizePlatform(p: string): string {
  if (!p) return p
  return p.charAt(0).toUpperCase() + p.slice(1)
}

/**
 * Safe accessor for event metadata numbers (bits, viewers, etc.) which
 * live in ChatMessage.event.metadata (Record<string, unknown>).
 */
function eventNumber(msg: ChatMessage, key: string): number {
  const raw = msg.event?.metadata?.[key]
  if (typeof raw === 'number' && Number.isFinite(raw)) return raw
  if (typeof raw === 'string') {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) return parsed
  }
  return 0
}

/**
 * createTTSPlayer constructs a TTSPlayer with the given initial settings.
 *
 * @param initialSettings - initial TTSSettings (usually loaded from display_settings)
 * @param onFallback - called once per session when ElevenLabs first fails (D-38)
 */
export function createTTSPlayer(
  initialSettings: TTSSettings,
  onFallback?: () => void,
): TTSPlayer {
  let settings: TTSSettings = { ...initialSettings }
  const queue: QueueItem[] = []
  const cooldowns = new Map<string, number>()
  let bucketTokens = settings.messages_per_minute
  let bucketLastRefill = Date.now()
  let sessionFallback = false
  let speaking = false

  const synth =
    typeof window !== 'undefined' && window.speechSynthesis ? window.speechSynthesis : null

  function isPriority(msg: ChatMessage): boolean {
    if (!settings.priority_events) return false
    const t = msg.event?.type
    if (!t) return false
    if (!PRIORITY_EVENTS.has(t as string)) return false
    if (t === 'bits') {
      const n = eventNumber(msg, 'bits')
      if (n < settings.priority_bits_min) return false
    }
    return true
  }

  function refillBucket(): void {
    const now = Date.now()
    if (now - bucketLastRefill >= 60_000) {
      bucketTokens = settings.messages_per_minute
      bucketLastRefill = now
    }
  }

  /**
   * Strip all emote positions from the message text. Returns the stripped
   * text plus the ratio of emote-tokens to total whitespace-delimited tokens
   * in the ORIGINAL message text (for emote-heavy skip detection).
   */
  function stripEmotes(msg: ChatMessage): { text: string; emoteRatio: number } {
    const text = msg.message?.text ?? ''
    const emotes = msg.message?.emotes ?? []
    if (emotes.length === 0) {
      return { text, emoteRatio: 0 }
    }
    const ranges: Array<[number, number]> = []
    for (const e of emotes) {
      for (const pos of e.positions ?? []) {
        if (Array.isArray(pos) && pos.length === 2 &&
            typeof pos[0] === 'number' && typeof pos[1] === 'number') {
          ranges.push([pos[0], pos[1]])
        }
      }
    }
    if (ranges.length === 0) {
      return { text, emoteRatio: 0 }
    }
    ranges.sort((a, b) => a[0] - b[0])
    let out = ''
    let cursor = 0
    for (const [start, end] of ranges) {
      if (start > cursor) out += text.slice(cursor, start)
      cursor = end + 1
    }
    if (cursor < text.length) out += text.slice(cursor)

    const origTokenCount = (text.match(/\S+/g) ?? []).length || 1
    const emoteTokenCount = ranges.length
    return { text: out, emoteRatio: emoteTokenCount / origTokenCount }
  }

  /**
   * Format the message into its spoken text, or return null to skip.
   * Applies D-25/26 prefixes, D-29 emote stripping, D-30 URL replacement,
   * D-31 event-specific prefixes, and the max_message_chars cap.
   */
  function formatContent(msg: ChatMessage): string | null {
    const eventType = msg.event?.type as EventType | undefined
    const display = msg.user?.display_name || msg.user?.username || 'Someone'
    const platformPrefix = settings.read_platform
      ? `${capitalizePlatform(msg.platform)}: `
      : ''
    const rawText = msg.message?.text ?? ''

    if (eventType && PRIORITY_EVENTS.has(eventType)) {
      switch (eventType) {
        case 'subscription':
        case 'kick_subscription':
          return `${platformPrefix}New subscription from ${display}`
        case 'resubscription':
          return `${platformPrefix}${display} resubscribed`
        case 'gift_subscription':
        case 'kick_gift_subscription':
        case 'mystery_gift':
          return `${platformPrefix}${display} gifted a subscription`
        case 'bits': {
          const bits = eventNumber(msg, 'bits')
          return `${platformPrefix}${display} cheered ${bits} bits${rawText ? `: ${rawText}` : ''}`
        }
        case 'raid': {
          const viewers = eventNumber(msg, 'viewers')
          return `${platformPrefix}${display} raided with ${viewers} viewers`
        }
        case 'super_chat':
        case 'super_sticker':
          return `${platformPrefix}Super chat from ${display}${rawText ? `: ${rawText}` : ''}`
        case 'kick_donation':
          return `${platformPrefix}Donation from ${display}${rawText ? `: ${rawText}` : ''}`
      }
    }

    // Regular chat message
    const stripped = stripEmotes(msg)
    let text = stripped.text

    // D-29 emote-heavy skip
    if (settings.skip_emote_only && stripped.emoteRatio > 0.5) {
      return null
    }

    // D-30 URL handling: replace URLs with literal "link".
    // skip_links applies when the message, after removing URLs entirely, is
    // only whitespace/punctuation (i.e. the message had no meaningful content
    // besides the URL). In that case we skip rather than say "link" alone.
    const hasUrl = URL_REGEX.test(text)
    URL_REGEX.lastIndex = 0 // reset stateful regex
    if (settings.skip_links && hasUrl) {
      const withoutUrls = text.replace(URL_REGEX, '')
      const onlyPunctOrWhitespace = /^[\s\p{P}]*$/u.test(withoutUrls)
      if (onlyPunctOrWhitespace) {
        return null
      }
    }
    text = text.replace(URL_REGEX, 'link')
    // Skip if nothing meaningful remains (e.g. pure-whitespace message)
    if (text.trim() === '') return null

    // Length cap (D-24 max_message_chars)
    if (text.length > settings.max_message_chars) {
      text = text.slice(0, settings.max_message_chars)
    }

    // D-25 username prefix / D-26 platform prefix
    const prefix = settings.read_username ? `${display} says: ` : ''
    return `${platformPrefix}${prefix}${text}`
  }

  function speak(message: ChatMessage): void {
    if (!settings.enabled) return
    if (!settings.enabled_platforms.includes(message.platform)) return
    if (message.event?.type === 'like_aggregate') return // D-32 always exclude

    const priority = isPriority(message)

    // D-31 sampling / priority gating
    if (!priority) {
      if (settings.filter_mode === 'priority_only') return
      if (settings.filter_mode === 'sample' && Math.random() >= settings.sample_rate) return
    }

    const uname = (message.user?.username ?? '').toLowerCase()

    // D-35 per-user cooldown (priority bypasses)
    if (!priority) {
      const last = cooldowns.get(uname) ?? 0
      if (Date.now() - last < settings.user_cooldown_seconds * 1000) return
    }

    // D-36 token bucket (priority bypasses)
    refillBucket()
    if (!priority) {
      if (bucketTokens <= 0) return
      bucketTokens--
    }

    cooldowns.set(uname, Date.now())

    // D-33 queue management
    if (queue.length >= settings.max_queue) {
      if (priority) {
        const idx = queue.findIndex((q) => !q.priority)
        if (idx >= 0) {
          queue.splice(idx, 1)
        } else {
          return // all priority already — no room
        }
      } else {
        return // non-priority full: drop
      }
    }

    queue.push({ message, priority, enqueuedAt: Date.now() })
    void pump()
  }

  async function pump(): Promise<void> {
    if (speaking) return
    const item = queue.shift()
    if (!item) return

    // D-37 staleness check at dequeue
    const ts =
      typeof item.message.timestamp === 'string'
        ? Date.parse(item.message.timestamp)
        : Number(item.message.timestamp)
    if (Number.isFinite(ts) && Date.now() - ts > settings.staleness_seconds * 1000) {
      void pump()
      return
    }

    const text = formatContent(item.message)
    if (!text) {
      void pump()
      return
    }

    speaking = true
    try {
      if (sessionFallback || settings.provider === 'browser') {
        await speakBrowser(text)
      } else {
        await speakElevenLabs(text)
      }
    } catch {
      // D-38: ElevenLabs failure → switch session to Web Speech
      if (!sessionFallback) {
        sessionFallback = true
        try {
          onFallback?.()
        } catch {
          /* swallow */
        }
      }
      try {
        await speakBrowser(text)
      } catch {
        /* swallow */
      }
    } finally {
      speaking = false
      void pump()
    }
  }

  function speakBrowser(text: string): Promise<void> {
    return new Promise((resolve) => {
      if (!synth || typeof SpeechSynthesisUtterance === 'undefined') {
        resolve()
        return
      }
      synth.cancel() // ensure no overlap (Pitfall 10)
      const u = new SpeechSynthesisUtterance(text)
      u.volume = settings.volume
      u.rate = settings.rate
      u.pitch = settings.pitch
      const voices = synth.getVoices() ?? []
      const match = settings.voice_uri
        ? voices.find((v) => v.voiceURI === settings.voice_uri)
        : undefined
      if (match) {
        u.voice = match
      } else if (settings.voice_uri) {
        console.warn(
          `[TTS] Voice '${settings.voice_uri}' not available in this browser — using default (D-28)`,
        )
      }
      u.onend = (): void => resolve()
      u.onerror = (): void => resolve()
      synth.speak(u)
    })
  }

  async function speakElevenLabs(text: string): Promise<void> {
    if (!settings.ttsEndpoint || !settings.ttsToken) {
      throw new Error('ElevenLabs endpoint or token not configured')
    }
    // SECURITY (audit M17): tts_token is sent via Authorization header and
    // voice via JSON body — no longer leaked into nginx/gateway access logs.
    const resp = await fetch(`${settings.ttsEndpoint}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${settings.ttsToken}`,
      },
      body: JSON.stringify({ text, voice: settings.voiceId ?? '' }),
    })
    if (!resp.ok) {
      throw new Error(`ElevenLabs proxy ${resp.status}`)
    }
    const blob = await resp.blob()
    const audio = new Audio(URL.createObjectURL(blob))
    audio.volume = settings.volume
    await new Promise<void>((resolve) => {
      const cleanup = (): void => {
        try {
          URL.revokeObjectURL(audio.src)
        } catch {
          /* swallow */
        }
        resolve()
      }
      audio.onended = cleanup
      audio.onerror = cleanup
      audio.play().catch(cleanup)
    })
  }

  function updateSettings(newSettings: TTSSettings): void {
    const oldPerMinute = settings.messages_per_minute
    settings = { ...newSettings }
    // Clamp bucket to new capacity so a shrunk bucket doesn't overflow.
    if (settings.messages_per_minute !== oldPerMinute) {
      bucketTokens = Math.min(bucketTokens, settings.messages_per_minute)
    }
  }

  function destroy(): void {
    try {
      synth?.cancel()
    } catch {
      /* swallow */
    }
    queue.length = 0
    cooldowns.clear()
    speaking = false
  }

  return { speak, updateSettings, destroy }
}
