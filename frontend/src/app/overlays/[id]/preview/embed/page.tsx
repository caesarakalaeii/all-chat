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
 * Overlay Embed Page
 *
 * Bare chat-only route for embedding in the SplitView iframe.
 * Renders the message stream with no navigation, header, or controls.
 *
 * Designed for:
 * - SplitView iframe preview inside the overlay editor
 * - Transparent background so OBS can key it (same as /overlay/[id])
 *
 * No auth redirect — if no token, shows an empty transparent page.
 */

'use client'

import Image from 'next/image'
import { use, useEffect, useState, useRef, useMemo, useCallback } from 'react'
import clsx from 'clsx'
import toast from 'react-hot-toast'
import { WebSocketClient } from '@/lib/api/websocket'
import { overlaysApi } from '@/lib/api/overlays'
import type { ChatMessage, NameGradient } from '@/lib/types/message'
import { buildGradientCSS } from '@/lib/utils/gradient'
import { renderMessageContent } from '@/lib/renderMessage'
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges'
import { sortMessageBadges } from '@/lib/badgeOrder'
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css'
import { getBundledTheme } from '@/lib/theme-marketplace/bundled-themes'
import { rewriteThemeFontImports } from '@/lib/theme-marketplace/font-proxy'
import { chatBubbleStyle, overlayContainerStyle } from '@/lib/utils/visual-inline-styles'
import { AllChatBadge } from '@/components/AllChatBadge'
import { PremiumBadge } from '@/components/PremiumBadge'
import { EventContent } from '@/components/overlay/EventContent'
import { shouldFilterMessage } from '@/lib/utils/filterMessage'
import type { FilterSettings } from '@/lib/types/overlay'
import { createSoundPlayer } from '@/lib/utils/soundPlayer'
import type { SoundPlayer, SoundSettings } from '@/lib/utils/soundPlayer'
import { createTTSPlayer } from '@/lib/utils/ttsPlayer'
import type { TTSPlayer, TTSSettings } from '@/lib/utils/ttsPlayer'
import '@/styles/events.css'

// ---- Utilities (identical to preview/page.tsx) ----------------------------

// NOTE (M11): scopeCustomCss prefixes selectors so owner-authored CSS is
// scoped to the preview root. It is NOT a full CSS sanitizer: `@import`,
// `url(...)`, `expression()`, or escaped-selector tricks could still escape.
// A complete CSS sanitiser is large and out of scope here; the blast radius
// is capped by the CSP `style-src` directive added in next.config.js (M10),
// which blocks external stylesheets and inline style injection vectors that
// would otherwise be reachable via url()/@import.
const scopeCustomCss = (css: string, scopeSelector: string, bodySelector: string): string => {
  if (!css.trim()) {
    return ''
  }

  const replaceBody = css.replace(/:root/gi, scopeSelector).replace(/\bbody\b/gi, bodySelector)

  return replaceBody.replace(/(^|}|{)\s*([^@}{]+)\s*{/g, (match, prefix, selectorGroup) => {
    const trimmed = selectorGroup.trim()
    if (!trimmed) {
      return match
    }

    const isKeyframeStep =
      ['from', 'to'].includes(trimmed.toLowerCase()) || /^\d+\.?\d*%$/i.test(trimmed)
    if (isKeyframeStep) {
      return `${prefix} ${trimmed} {`
    }

    const scopedSelectors = trimmed
      .split(',')
      .map((selector: string) => {
        const sel = selector.trim()
        if (!sel || sel.startsWith(scopeSelector) || sel.startsWith(bodySelector)) {
          return sel
        }
        return `${scopeSelector} ${sel}`
      })
      .filter(Boolean)
      .join(', ')

    return `${prefix} ${scopedSelectors} {`
  })
}

// ---- Platform helpers (identical to preview/page.tsx) ---------------------

const getPlatformColor = (platform: string): string => {
  switch (platform) {
    case 'twitch':
      return 'text-purple-400'
    case 'youtube':
      return 'text-red-400'
    case 'kick':
      return 'text-green-400'
    default:
      return 'text-slate-400'
  }
}

const PlatformIcon = ({ platform }: { platform: string }) => {
  const iconClass = 'inline-block w-4 h-4'

  switch (platform) {
    case 'twitch':
      return (
        <svg viewBox="0 0 24 24" className={iconClass}>
          <path
            fill="#9146FF"
            d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"
          />
        </svg>
      )
    case 'youtube':
      return (
        <svg viewBox="0 0 24 24" className={iconClass}>
          <path
            fill="#FF0000"
            d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
          />
        </svg>
      )
    case 'kick':
      return (
        <svg viewBox="0 0 24 24" className={iconClass} style={{ imageRendering: 'pixelated' }}>
          <text
            x="12"
            y="18"
            fontSize="20"
            fontWeight="bold"
            fill="#00E701"
            textAnchor="middle"
            fontFamily="monospace"
          >
            K
          </text>
        </svg>
      )
    case 'tiktok':
      return (
        <svg viewBox="0 0 24 24" className={iconClass}>
          <path
            fill="#000000"
            d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"
          />
        </svg>
      )
    default:
      return null
  }
}

// ---- Font loader ----------------------------------------------------------
// Fonts are proxied through /font-proxy/css so end-user IPs never reach Google
// (DSGVO / "Google Fonts Urteil" LG München 2022-01-20, Az. 3 O 17493/20).

const GOOGLE_FONT_NAMES = new Set([
  'Bebas Neue',
  'Oswald',
  'Rajdhani',
  'Barlow Condensed',
  'Exo 2',
  'Nunito',
  'Poppins',
  'Roboto',
  'Open Sans',
  'Montserrat',
])

function ensureGoogleFontLoaded(fontFamily: string): void {
  if (!GOOGLE_FONT_NAMES.has(fontFamily)) return
  const slug = fontFamily.replace(/\s+/g, '-').toLowerCase()
  if (document.getElementById('gfont-' + slug)) return
  const link = document.createElement('link')
  link.id = 'gfont-' + slug
  link.rel = 'stylesheet'
  const family = encodeURIComponent(`${fontFamily}:wght@400;600;700`)
  link.href = `/font-proxy/css?family=${family}`
  document.head.appendChild(link)
}

// ---- Page -----------------------------------------------------------------

export default function OverlayEmbedPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)

  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [maxMessages, setMaxMessages] = useState(50)
  const [fontSize, setFontSize] = useState(16)
  const [customCss, setCustomCss] = useState('')
  const [useCustomCss, setUseCustomCss] = useState(false)
  const [visualSettingsCss, setVisualSettingsCss] = useState('')
  // Background fills / shadow / max-width applied inline only when set (see
  // visual-inline-styles); keeps this preview in sync with the live overlay.
  const [containerStyle, setContainerStyle] = useState<React.CSSProperties>({})
  const [bubbleStyle, setBubbleStyle] = useState<React.CSSProperties>({})
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before')
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text')
  const [showPlatformBadge, setShowPlatformBadge] = useState(true)

  // Phase 11: Filter settings state
  const [filterSettings, setFilterSettings] = useState<FilterSettings>({})
  const filterSettingsRef = useRef<FilterSettings>({})

  // Phase 12: Sound player state
  const soundPlayerRef = useRef<SoundPlayer | null>(null)
  const soundSettingsRef = useRef<SoundSettings>({
    enabled: false,
    preset: 'chime',
    volume: 0.5,
    cooldownMs: 500,
  })

  // Phase 13: TTS player state (D-41, D-42)
  const ttsPlayerRef = useRef<TTSPlayer | null>(null)
  const ttsSettingsRef = useRef<TTSSettings>({
    enabled: false,
    provider: 'browser',
    volume: 0.8,
    rate: 1.0,
    pitch: 1.0,
    filter_mode: 'sample',
    sample_rate: 0.25,
    max_queue: 5,
    messages_per_minute: 8,
    user_cooldown_seconds: 30,
    staleness_seconds: 15,
    priority_events: true,
    priority_bits_min: 0,
    read_username: true,
    read_platform: false,
    max_message_chars: 200,
    skip_emote_only: true,
    skip_links: true,
    enabled_platforms: ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
  })
  const ttsFallbackToastShownRef = useRef(false)

  // Phase 13 Plan 03: cache ElevenLabs runtime params (endpoint / token / voice_id)
  // loaded once from GET /tts-config. The TTS_SETTINGS_UPDATE postMessage from the
  // editor only carries display_settings; these runtime fields must survive those
  // updates so the fetch path continues to work after the editor tweaks settings.
  const elevenLabsRuntimeRef = useRef<{
    ttsEndpoint?: string
    ttsToken?: string
    voiceId?: string
  }>({})

  // Phase 13: ElevenLabs session fallback callback (D-38)
  const handleTTSFallback = useCallback(() => {
    if (ttsFallbackToastShownRef.current) return
    ttsFallbackToastShownRef.current = true
    toast('ElevenLabs unavailable — using browser voice.')
  }, [])

  const wsClientRef = useRef<WebSocketClient | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  const scopedPreviewCss = useMemo(() => {
    if (!useCustomCss || !customCss.trim()) {
      return ''
    }
    return scopeCustomCss(
      customCss,
      '#overlay-preview-root',
      '#overlay-preview-root .overlay-preview-body'
    )
  }, [customCss, useCustomCss])

  // postMessage listener for live visual CSS updates from the editor
  useEffect(() => {
    const handleMessage = (event: MessageEvent) => {
      // M11: the editor that embeds this page is same-origin
      // (/overlays/:id editor iframes /overlays/:id/preview/embed). Reject
      // messages from any other origin so a malicious parent cannot inject
      // CSS / TTS / filter settings into the preview.
      if (event.origin !== window.location.origin) return
      // Live visual settings (CSS variables)
      if (event.data?.type === 'VISUAL_CSS_UPDATE') {
        const css = event.data.css as string
        setVisualSettingsCss(css)
        const fontFamilyMatches = css.matchAll(/--chat-[^:]*font-family\s*:\s*([^;}"]+)/g)
        for (const match of fontFamilyMatches) {
          ensureGoogleFontLoaded(match[1].trim().replace(/['"]/g, ''))
        }
        return
      }
      // Non-CSS visual settings (platform badge position/style, indicators)
      if (event.data?.type === 'VISUAL_SETTINGS_UPDATE') {
        const s = event.data.settings
        if (s.platformBadgePosition === 'before' || s.platformBadgePosition === 'after') {
          setPlatformBadgePosition(s.platformBadgePosition)
        }
        if (s.platformBadgeStyle === 'text' || s.platformBadgeStyle === 'icon') {
          setPlatformBadgeStyle(s.platformBadgeStyle)
        }
        if (s.showPlatformBadge !== undefined) {
          setShowPlatformBadge(s.showPlatformBadge !== 'none')
        }
        return
      }
      // Full theme CSS replacement (sent when user applies a new theme)
      if (event.data?.type === 'CUSTOM_CSS_UPDATE') {
        const css = event.data.css as string
        setCustomCss(css)
        setUseCustomCss(Boolean(css.trim().length))
        return
      }
      // Phase 11: Real-time filter settings from editor (D-07 WYSIWYG)
      if (event.data?.type === 'FILTER_SETTINGS_UPDATE') {
        const settings = event.data.filterSettings as FilterSettings
        setFilterSettings(settings)
        filterSettingsRef.current = settings
        return
      }
      // Phase 12: Real-time sound settings from editor
      if (event.data?.type === 'SOUND_SETTINGS_UPDATE') {
        const s = event.data.soundSettings as Partial<import('@/lib/types/overlay').DisplaySettings>
        const newSettings: SoundSettings = {
          enabled: s.notification_sound_enabled ?? soundSettingsRef.current.enabled,
          preset: s.notification_sound_preset ?? soundSettingsRef.current.preset,
          volume: s.notification_sound_volume ?? soundSettingsRef.current.volume,
          cooldownMs: s.notification_sound_cooldown ?? soundSettingsRef.current.cooldownMs,
          customUrl: s.notification_sound_url ?? soundSettingsRef.current.customUrl,
        }
        soundSettingsRef.current = newSettings
        if (soundPlayerRef.current) {
          soundPlayerRef.current.updateSettings(newSettings)
        } else {
          soundPlayerRef.current = createSoundPlayer(newSettings)
        }
        return
      }
      // Phase 13: Real-time TTS settings from editor (D-22)
      if (event.data?.type === 'TTS_SETTINGS_UPDATE') {
        const s = event.data.ttsSettings as Partial<import('@/lib/types/overlay').DisplaySettings>
        const prev = ttsSettingsRef.current
        const newSettings: TTSSettings = {
          enabled: s.tts_enabled ?? prev.enabled,
          provider: (s.tts_provider ?? prev.provider) as 'browser' | 'elevenlabs',
          volume: s.tts_volume ?? prev.volume,
          voice_uri: s.tts_voice_uri ?? prev.voice_uri,
          rate: s.tts_rate ?? prev.rate,
          pitch: s.tts_pitch ?? prev.pitch,
          filter_mode: (s.tts_filter_mode ?? prev.filter_mode) as
            'all' | 'sample' | 'priority_only',
          sample_rate: s.tts_sample_rate ?? prev.sample_rate,
          max_queue: s.tts_max_queue ?? prev.max_queue,
          messages_per_minute: s.tts_messages_per_minute ?? prev.messages_per_minute,
          user_cooldown_seconds: s.tts_user_cooldown_seconds ?? prev.user_cooldown_seconds,
          staleness_seconds: s.tts_staleness_seconds ?? prev.staleness_seconds,
          priority_events: s.tts_priority_events ?? prev.priority_events,
          priority_bits_min: s.tts_priority_bits_min ?? prev.priority_bits_min,
          read_username: s.tts_read_username ?? prev.read_username,
          read_platform: s.tts_read_platform ?? prev.read_platform,
          max_message_chars: s.tts_max_message_chars ?? prev.max_message_chars,
          skip_emote_only: s.tts_skip_emote_only ?? prev.skip_emote_only,
          skip_links: s.tts_skip_links ?? prev.skip_links,
          enabled_platforms: Array.isArray(s.tts_enabled_platforms)
            ? s.tts_enabled_platforms
            : prev.enabled_platforms,
          // Phase 13 Plan 03: preserve ElevenLabs runtime params across settings
          // updates. The editor only sends display_settings; the endpoint/token
          // comes from the one-shot /tts-config GET on mount.
          ttsEndpoint: elevenLabsRuntimeRef.current.ttsEndpoint ?? prev.ttsEndpoint,
          ttsToken: elevenLabsRuntimeRef.current.ttsToken ?? prev.ttsToken,
          voiceId: elevenLabsRuntimeRef.current.voiceId ?? prev.voiceId,
        }
        ttsSettingsRef.current = newSettings
        if (ttsPlayerRef.current) {
          ttsPlayerRef.current.updateSettings(newSettings)
        } else {
          ttsPlayerRef.current = createTTSPlayer(newSettings, handleTTSFallback)
        }
        return
      }
    }

    window.addEventListener('message', handleMessage)
    // Signal the editor that we're ready to receive visual CSS updates.
    // M11: target only our own origin instead of '*' so the ready signal is
    // not leaked cross-origin.
    window.parent.postMessage({ type: 'EMBED_READY' }, window.location.origin)
    return () => {
      window.removeEventListener('message', handleMessage)
    }
  }, [])

  // Load overlay config (H3 cookie auth: same-origin cookie + CookieToBearer).
  useEffect(() => {
    const loadConfig = async () => {
      try {
        const config = await overlaysApi.getConfig(id)
        const display = config.display_settings || {}

        if (typeof display.max_messages === 'number') setMaxMessages(display.max_messages)
        if (typeof display.font_size === 'number') setFontSize(display.font_size)
        if (
          display.platform_badge_position === 'before' ||
          display.platform_badge_position === 'after'
        ) {
          setPlatformBadgePosition(display.platform_badge_position)
        }
        if (display.platform_badge_style === 'text' || display.platform_badge_style === 'icon') {
          setPlatformBadgeStyle(display.platform_badge_style)
        }
        if (typeof display.show_platform_badge === 'boolean') {
          setShowPlatformBadge(display.show_platform_badge)
        }

        // Bundled theme CSS (resolved fresh from the build by theme_id) + the
        // user's raw custom_css overrides. Both are scoped to the preview root
        // by scopedPreviewCss, mirroring the live overlay's theme→custom order.
        const themeCss =
          typeof config.theme_id === 'string' && config.theme_id
            ? rewriteThemeFontImports(getBundledTheme(config.theme_id)?.css ?? '')
            : ''
        const css = [themeCss, config.custom_css || ''].filter((s) => s.trim().length).join('\n')
        setCustomCss(css)
        setUseCustomCss(Boolean(css.trim().length))

        // Apply saved visual settings (CSS variables) directly — no postMessage needed
        const vs = config.visual_settings ?? {}
        const vcCss = visualSettingsToCss(vs)
        setVisualSettingsCss(vcCss)
        setContainerStyle(overlayContainerStyle(vs))
        setBubbleStyle(chatBubbleStyle(vs))
        // Apply non-CSS visual settings
        if (vs.showPlatformBadge !== undefined) {
          setShowPlatformBadge(vs.showPlatformBadge !== 'none')
        }
        if (vs.platformBadgePosition === 'before' || vs.platformBadgePosition === 'after') {
          setPlatformBadgePosition(vs.platformBadgePosition)
        }
        if (vs.platformBadgeStyle === 'text' || vs.platformBadgeStyle === 'icon') {
          setPlatformBadgeStyle(vs.platformBadgeStyle)
        }
        // Phase 11: Load filter settings from config
        if (config.filter_settings) {
          setFilterSettings(config.filter_settings)
          filterSettingsRef.current = config.filter_settings
        }

        // Phase 12: Load sound settings from display_settings
        const soundEnabled = display.notification_sound_enabled === true
        const soundPreset =
          typeof display.notification_sound_preset === 'string'
            ? display.notification_sound_preset
            : 'chime'
        const soundVolume =
          typeof display.notification_sound_volume === 'number'
            ? display.notification_sound_volume
            : 0.5
        const soundCooldown =
          typeof display.notification_sound_cooldown === 'number'
            ? display.notification_sound_cooldown
            : 500
        const soundCustomUrl =
          typeof display.notification_sound_url === 'string'
            ? display.notification_sound_url || undefined
            : undefined

        const newSoundSettings: SoundSettings = {
          enabled: soundEnabled,
          preset: soundPreset,
          volume: soundVolume,
          cooldownMs: soundCooldown,
          customUrl: soundCustomUrl,
        }
        soundSettingsRef.current = newSoundSettings
        if (soundPlayerRef.current) {
          soundPlayerRef.current.updateSettings(newSoundSettings)
        } else {
          soundPlayerRef.current = createSoundPlayer(newSoundSettings)
        }

        // Phase 13: Load TTS settings from display_settings (D-24)
        const ttsLoaded: TTSSettings = {
          enabled: display.tts_enabled === true,
          provider: display.tts_provider === 'elevenlabs' ? 'elevenlabs' : 'browser',
          volume: typeof display.tts_volume === 'number' ? display.tts_volume : 0.8,
          voice_uri: typeof display.tts_voice_uri === 'string' ? display.tts_voice_uri : undefined,
          rate: typeof display.tts_rate === 'number' ? display.tts_rate : 1.0,
          pitch: typeof display.tts_pitch === 'number' ? display.tts_pitch : 1.0,
          filter_mode:
            display.tts_filter_mode === 'all' || display.tts_filter_mode === 'priority_only'
              ? display.tts_filter_mode
              : 'sample',
          sample_rate: typeof display.tts_sample_rate === 'number' ? display.tts_sample_rate : 0.25,
          max_queue: typeof display.tts_max_queue === 'number' ? display.tts_max_queue : 5,
          messages_per_minute:
            typeof display.tts_messages_per_minute === 'number'
              ? display.tts_messages_per_minute
              : 8,
          user_cooldown_seconds:
            typeof display.tts_user_cooldown_seconds === 'number'
              ? display.tts_user_cooldown_seconds
              : 30,
          staleness_seconds:
            typeof display.tts_staleness_seconds === 'number' ? display.tts_staleness_seconds : 15,
          priority_events: display.tts_priority_events !== false,
          priority_bits_min:
            typeof display.tts_priority_bits_min === 'number' ? display.tts_priority_bits_min : 0,
          read_username: display.tts_read_username !== false,
          read_platform: display.tts_read_platform === true,
          max_message_chars:
            typeof display.tts_max_message_chars === 'number' ? display.tts_max_message_chars : 200,
          skip_emote_only: display.tts_skip_emote_only !== false,
          skip_links: display.tts_skip_links !== false,
          enabled_platforms: Array.isArray(display.tts_enabled_platforms)
            ? display.tts_enabled_platforms.filter(
                (p: unknown): p is string => typeof p === 'string'
              )
            : ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
        }

        // Phase 13 Plan 03: For the ElevenLabs branch, hydrate the runtime fetch
        // endpoint + tts_token JWT. The editor preview is authed (user JWT in
        // localStorage), so we can call GET /tts-config to recover the obs_url
        // and extract the token query param — mirroring the live overlay's
        // URLSearchParams read.
        if (ttsLoaded.provider === 'elevenlabs') {
          try {
            const meta = await overlaysApi.getTTSConfig(id)
            if (meta.has_elevenlabs_config && meta.obs_url) {
              try {
                const u = new URL(meta.obs_url, window.location.origin)
                const t = u.searchParams.get('tts_token') ?? undefined
                elevenLabsRuntimeRef.current = {
                  ttsEndpoint: `/api/v1/overlays/${id}/tts`,
                  ttsToken: t,
                  voiceId: meta.voice_id ?? '',
                }
                ttsLoaded.ttsEndpoint = elevenLabsRuntimeRef.current.ttsEndpoint
                ttsLoaded.ttsToken = elevenLabsRuntimeRef.current.ttsToken
                ttsLoaded.voiceId = elevenLabsRuntimeRef.current.voiceId
              } catch {
                // Bad URL — silently skip; editor preview won't play ElevenLabs,
                // but browser voice still works.
              }
            }
          } catch {
            // Non-fatal — if the overlay has no key saved yet, the proxy isn't
            // reachable anyway and the player will fall back to browser voice.
          }
        }

        ttsSettingsRef.current = ttsLoaded
        if (ttsPlayerRef.current) {
          ttsPlayerRef.current.updateSettings(ttsLoaded)
        } else {
          ttsPlayerRef.current = createTTSPlayer(ttsLoaded, handleTTSFallback)
        }
      } catch (error) {
        console.warn('[Embed] Failed to load overlay config', error)
      }
    }

    loadConfig()
  }, [id, handleTTSFallback])

  // Initialize WebSocket connection. H3 cookie auth: the owner overlay WS
  // handshake is same-origin, so the browser sends the httpOnly access cookie
  // automatically and the gateway authenticates without a JS-readable token.
  useEffect(() => {
    const wsClient = new WebSocketClient()
    wsClientRef.current = wsClient

    wsClient.connect(id)

    const unsubscribe = wsClient.onMessage(async (incoming) => {
      // Parse gradient JSON string → object (message processor sends it as a string)
      if (incoming.user?.name_gradient && typeof incoming.user.name_gradient === 'string') {
        incoming.user.name_gradient = JSON.parse(
          incoming.user.name_gradient as unknown as string
        ) as NameGradient
      }
      const message = sortMessageBadges(await resolveTwitchBadgeIcons(incoming))

      // Phase 11: apply filter settings before adding to render queue (D-07 WYSIWYG)
      if (shouldFilterMessage(message, filterSettingsRef.current)) return

      // Phase 12: play notification sound for messages that pass the filter
      soundPlayerRef.current?.play()
      // Phase 13: speak the message via TTS (D-41, D-42 — independent of sound; both fire on non-filtered)
      ttsPlayerRef.current?.speak(message)

      setMessages((prev) => [...prev, message].slice(-maxMessages))
    })

    const interval = setInterval(() => {
      // keep connection alive check
    }, 1000)

    return () => {
      unsubscribe()
      wsClient.disconnect()
      clearInterval(interval)
    }
  }, [id, maxMessages])

  // Trim buffer when maxMessages changes
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMessages((prev) => (prev.length > maxMessages ? prev.slice(-maxMessages) : prev))
  }, [maxMessages])

  // Phase 12: Destroy sound player on unmount
  useEffect(() => {
    return () => {
      soundPlayerRef.current?.destroy()
      soundPlayerRef.current = null
    }
  }, [])

  // Phase 13: Destroy TTS player on unmount
  useEffect(() => {
    return () => {
      ttsPlayerRef.current?.destroy()
      ttsPlayerRef.current = null
    }
  }, [])

  // Helper: render event-specific content (identical to preview/page.tsx)
  const renderEventContent = (message: ChatMessage): React.ReactNode => (
    <EventContent message={message} />
  )

  return (
    <main id="main-content" tabIndex={-1} className="min-h-screen bg-transparent">
      {useCustomCss && scopedPreviewCss && (
        <style
          key={scopedPreviewCss}
          id="overlay-preview-custom-css"
          dangerouslySetInnerHTML={{ __html: scopedPreviewCss }}
        />
      )}
      {visualSettingsCss && (
        <style
          id="overlay-preview-visual-settings"
          dangerouslySetInnerHTML={{ __html: visualSettingsCss }}
        />
      )}

      <div
        id="overlay-preview-root"
        className={clsx(
          'overlay-preview-root relative h-screen overflow-hidden',
          useCustomCss && 'overlay-preview'
        )}
      >
        <div
          className="overlay-preview-body h-full space-y-3 overflow-y-auto p-4"
          style={{
            scrollbarWidth: 'thin',
            scrollbarColor: '#374151 transparent',
            ...containerStyle,
          }}
        >
          {messages.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <div className="text-center text-slate-600">
                <svg
                  className="mx-auto mb-4 h-16 w-16"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
                  />
                </svg>
                <p className="mb-2 text-lg font-medium">Waiting for messages...</p>
                <p className="text-sm">Messages will appear here when chat is active</p>
              </div>
            </div>
          ) : (
            <>
              {messages.map((message) => {
                const isEvent = message.event != null
                const eventTierClass = isEvent ? `event-tier-${message.event?.tier}` : ''
                const eventTypeClass = isEvent ? `event-type-${message.event?.type}` : ''

                return (
                  <div
                    key={message.id}
                    data-platform={message.platform}
                    data-event-type={isEvent ? message.event?.type : undefined}
                    className={
                      isEvent
                        ? clsx('event-message', eventTierClass, eventTypeClass)
                        : 'rounded-lg bg-slate-900/90 p-3 shadow-lg backdrop-blur-sm'
                    }
                    style={isEvent ? undefined : bubbleStyle}
                  >
                    <div className="flex items-start gap-3">
                      {/* Avatar */}
                      <div className="flex-shrink-0">
                        {message.user.avatar_url ? (
                          <Image
                            src={message.user.avatar_url}
                            alt={message.user.display_name}
                            width={40}
                            height={40}
                            className="h-10 w-10 rounded-full object-cover"
                            onError={(e) => {
                              e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(
                                message.user.display_name
                              )}&background=6b7280&color=fff&size=40`
                            }}
                          />
                        ) : (
                          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-slate-700 font-semibold text-white">
                            {message.user.display_name?.slice(0, 2).toUpperCase() || '?'}
                          </div>
                        )}
                      </div>

                      {/* Message Content */}
                      <div className="min-w-0 flex-1">
                        {/* User Info */}
                        <div className="mb-1 flex flex-wrap items-center gap-2">
                          {/* Platform badge before username */}
                          {showPlatformBadge &&
                            platformBadgePosition === 'before' &&
                            (platformBadgeStyle === 'icon' ? (
                              <span
                                className="platform-badge platform-badge-icon flex items-center"
                                title={message.platform}
                              >
                                <PlatformIcon platform={message.platform} />
                              </span>
                            ) : (
                              <span
                                className={clsx(
                                  'platform-badge platform-badge-text text-xs font-semibold uppercase',
                                  getPlatformColor(message.platform)
                                )}
                              >
                                {message.platform}
                              </span>
                            ))}

                          {/* Badges before username when position is 'before' */}
                          {platformBadgePosition === 'before' &&
                            message.user.badges &&
                            message.user.badges.length > 0 && (
                              <div className="flex gap-1">
                                {message.user.badges.map((badge, index) =>
                                  badge.name === 'allchat' ? (
                                    <AllChatBadge
                                      key={`${badge.name}-${index}`}
                                      size={16}
                                      title={badge.name}
                                    />
                                  ) : badge.name === 'allchat-premium' ? (
                                    <PremiumBadge
                                      key={`${badge.name}-${index}`}
                                      size={16}
                                      title={badge.name}
                                    />
                                  ) : badge.icon_url ? (
                                    <Image
                                      key={`${badge.name}-${index}`}
                                      src={badge.icon_url}
                                      alt={badge.name}
                                      title={`${badge.name} (${badge.version})`}
                                      width={16}
                                      height={16}
                                      className="h-4 w-4 object-contain"
                                      onError={(e) => {
                                        e.currentTarget.style.display = 'none'
                                      }}
                                    />
                                  ) : null
                                )}
                              </div>
                            )}

                          {/* Username */}
                          {message.user.name_gradient ? (
                            <span
                              ref={(el) => {
                                if (el) {
                                  el.style.setProperty('text-shadow', 'none', 'important')
                                  el.style.setProperty(
                                    '-webkit-text-stroke',
                                    '0.5px rgba(0,0,0,0.5)',
                                    'important'
                                  )
                                  el.style.setProperty('color', 'transparent', 'important')
                                  el.style.setProperty(
                                    '-webkit-text-fill-color',
                                    'transparent',
                                    'important'
                                  )
                                  el.style.setProperty('background-clip', 'text', 'important')
                                  el.style.setProperty(
                                    '-webkit-background-clip',
                                    'text',
                                    'important'
                                  )
                                }
                              }}
                              className="chat-username username-gradient bg-clip-text text-sm font-semibold text-transparent"
                              style={{
                                backgroundImage: buildGradientCSS(message.user.name_gradient),
                              }}
                            >
                              {message.user.display_name}
                            </span>
                          ) : (
                            <span
                              className="chat-username text-sm font-semibold"
                              style={{ color: message.user.color || '#FFFFFF' }}
                            >
                              {message.user.display_name}
                            </span>
                          )}

                          {/* Platform badge after username */}
                          {showPlatformBadge &&
                            platformBadgePosition === 'after' &&
                            (platformBadgeStyle === 'icon' ? (
                              <span
                                className="platform-badge platform-badge-icon flex items-center"
                                title={message.platform}
                              >
                                <PlatformIcon platform={message.platform} />
                              </span>
                            ) : (
                              <span
                                className={clsx(
                                  'platform-badge platform-badge-text text-xs font-semibold uppercase',
                                  getPlatformColor(message.platform)
                                )}
                              >
                                {message.platform}
                              </span>
                            ))}

                          {/* Badges after username when position is 'after' */}
                          {platformBadgePosition === 'after' &&
                            message.user.badges &&
                            message.user.badges.length > 0 && (
                              <div className="flex gap-1">
                                {message.user.badges.map((badge, index) =>
                                  badge.name === 'allchat' ? (
                                    <AllChatBadge
                                      key={`${badge.name}-${index}`}
                                      size={16}
                                      title={badge.name}
                                    />
                                  ) : badge.name === 'allchat-premium' ? (
                                    <PremiumBadge
                                      key={`${badge.name}-${index}`}
                                      size={16}
                                      title={badge.name}
                                    />
                                  ) : badge.icon_url ? (
                                    <Image
                                      key={`${badge.name}-${index}`}
                                      src={badge.icon_url}
                                      alt={badge.name}
                                      title={`${badge.name} (${badge.version})`}
                                      width={16}
                                      height={16}
                                      className="h-4 w-4 object-contain"
                                      onError={(e) => {
                                        e.currentTarget.style.display = 'none'
                                      }}
                                    />
                                  ) : null
                                )}
                              </div>
                            )}
                        </div>

                        {/* Message Text or Event Content */}
                        <div
                          className="break-words text-white"
                          style={{ fontSize: `${fontSize}px` }}
                        >
                          {message.event
                            ? renderEventContent(message)
                            : renderMessageContent(message)}
                        </div>

                        {/* Timestamp */}
                        <div className="mt-1 text-xs text-slate-500">
                          {new Date(message.timestamp).toLocaleTimeString()}
                        </div>
                      </div>
                    </div>
                  </div>
                )
              })}
              <div ref={messagesEndRef} />
            </>
          )}
        </div>
      </div>
    </main>
  )
}
