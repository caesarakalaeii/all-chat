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
 * OBS Overlay Page (Unauthenticated)
 *
 * Clean overlay view for OBS Browser Source - no authentication required.
 * Displays only chat messages without any UI chrome.
 *
 * Features:
 * - Realtime stream via the shared useOverlayStream hook (no auth)
 * - Real-time message rendering
 * - Platform identification (Twitch, YouTube, etc.)
 * - User badges, avatars, and colors
 * - Emote display
 * - Auto-scroll to latest messages
 * - Transparent background for OBS
 *
 * This is a Client Component because it:
 * - Uses WebSocket (browser API) via useOverlayStream
 * - Manages real-time state
 */

'use client'

import Image from 'next/image'
import { use, useEffect, useMemo, useState, useRef, useCallback } from 'react'
import clsx from 'clsx'
import { toastManager } from '@/lib/toast'
import type { ChatMessage, EventTier, DeletionMetadata } from '@/lib/types/message'
import { renderMessageContent } from '@/lib/renderMessage'
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators'
import { useOverlayStream } from '@/hooks/useOverlayStream'
import { buildGradientCSS } from '@/lib/utils/gradient'
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css'
import { getBundledTheme } from '@/lib/theme-marketplace/bundled-themes'
import { rewriteThemeFontImports } from '@/lib/theme-marketplace/font-proxy'
import { chatBubbleStyle, overlayContainerStyle } from '@/lib/utils/visual-inline-styles'
import { isDisplayVisible } from '@/lib/utils/displayVisibility'
import {
  DEFAULT_FEED_ANCHOR,
  orderMessages,
  parseFeedAnchor,
  resolveFeedAnchorLayout,
  shouldAutoScroll,
  type FeedAnchor,
} from '@/lib/utils/feedAnchor'
import {
  isMessageAnimation,
  MESSAGE_ANIMATION_CLASS,
  type MessageAnimation,
  type VisualSettings,
} from '@/lib/types/visual-settings'
import { shouldFilterMessage } from '@/lib/utils/filterMessage'
import type { FilterSettings } from '@/lib/types/overlay'
import { createSoundPlayer } from '@/lib/utils/soundPlayer'
import type { SoundPlayer, SoundSettings } from '@/lib/utils/soundPlayer'
import { createTTSPlayer } from '@/lib/utils/ttsPlayer'
import type { TTSPlayer, TTSSettings } from '@/lib/utils/ttsPlayer'

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

import { UserAvatar } from '@/components/UserAvatar'
import { AllChatBadge } from '@/components/AllChatBadge'
import { PremiumBadge } from '@/components/PremiumBadge'
import { EventContent } from '@/components/overlay/EventContent'
import { MessageAttachments } from '@/components/overlay/MessageAttachments'
import { resolveUsernameColor } from '@/lib/utils/usernameColor'
import '@/styles/events.css'

// Default display duration (seconds) for an event based on its tier. Pure
// helper hoisted to module scope so the fade effect can reference it safely.
function getTierDuration(tier: EventTier): number {
  switch (tier) {
    case 'high':
      return 30
    case 'medium':
      return 15
    case 'low':
      return 8
    default:
      return 15
  }
}

export default function OBSOverlayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [maxMessages, setMaxMessages] = useState(50)
  const [fontSize, setFontSize] = useState(16)
  const [messageDuration, setMessageDuration] = useState(15)
  const [disableMessageFade, setDisableMessageFade] = useState(false)
  const [customCss, setCustomCss] = useState('')
  // Theme CSS resolved from the build bundle by config.theme_id, so a theme fix
  // ships with a deploy instead of being frozen per-overlay. Legacy overlays
  // (no theme_id) leave this empty and still render via custom_css.
  const [themeCss, setThemeCss] = useState('')
  const [visualSettingsCss, setVisualSettingsCss] = useState('')
  // Body font-size for message text. Prefer the visual-customizer `fontSize`
  // (e.g. "18px") when set, otherwise fall back to the legacy display-settings
  // `font_size` (number, applied as px). Applied inline rather than via CSS var
  // so it doesn't get clobbered by the layered visual-customizer rules.
  const [messageFontSizeCss, setMessageFontSizeCss] = useState('')
  // Background fills (overlay container + chat bubbles), shadow and max-width.
  // Applied inline ONLY when set so they don't clobber the per-variant Tailwind
  // defaults (slate/purple bubbles, transparent overlay) — see visual-inline-styles.
  const [containerStyle, setContainerStyle] = useState<React.CSSProperties>({})
  const [bubbleStyle, setBubbleStyle] = useState<React.CSSProperties>({})
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before')
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text')
  // Entry animation for new chat bubbles; null keeps the default fade + slide-up
  const [messageAnimation, setMessageAnimation] = useState<MessageAnimation | null>(null)
  const [showPlatformBadge, setShowPlatformBadge] = useState(true)
  const [showPlatformIndicators, setShowPlatformIndicators] = useState(true)
  // Visibility toggles whose CSS rules are scoped to `.overlay-preview-body`
  // (preview/embed only). The live OBS overlay lacks that scope hook, so it
  // must honor these in React — otherwise disabled elements still render here.
  const [showAvatars, setShowAvatars] = useState(true)
  const [showBadges, setShowBadges] = useState(true)
  const [showTimestamps, setShowTimestamps] = useState(true)
  const [showUsername, setShowUsername] = useState(true)
  const [invertMessageOrder, setInvertMessageOrder] = useState(false)
  // Which canvas edge the stack rests on. Orthogonal to invertMessageOrder:
  // that picks which end of the LIST is newest, this picks which EDGE the list
  // is glued to. Defaults to 'top', matching every overlay saved before #728.
  const [feedAnchor, setFeedAnchor] = useState<FeedAnchor>(DEFAULT_FEED_ANCHOR)
  const [showPronouns, setShowPronouns] = useState(true) // D-07: default on
  const [pronounPosition, setPronounPosition] = useState<'before' | 'after'>('after') // default after
  const [pronounColor, setPronounColor] = useState('#7B68EE') // default medium slate blue

  const [filterSettings, setFilterSettings] = useState<FilterSettings>({})
  const filterSettingsRef = useRef<FilterSettings>({})

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

  const messagesEndRef = useRef<HTMLDivElement>(null)
  // The full-canvas wrapper. Measured (layout height) by the auto-scroll effect.
  const feedWrapperRef = useRef<HTMLDivElement>(null)
  // Single source of truth for the two orthogonal feed-layout axes.
  const feedLayout = useMemo(
    () => resolveFeedAnchorLayout(feedAnchor, invertMessageOrder),
    [feedAnchor, invertMessageOrder]
  )
  // Keep a ref so the stream callbacks always see the latest value without
  // maxMessages needing to be a dependency.
  const maxMessagesRef = useRef<number>(50)

  // Keep filterSettingsRef in sync so the onChat callback always reads the latest value
  useEffect(() => {
    filterSettingsRef.current = filterSettings
  }, [filterSettings])

  // Phase 12: Destroy sound player on unmount
  useEffect(() => {
    return () => {
      soundPlayerRef.current?.destroy()
      soundPlayerRef.current = null
    }
  }, [])

  // Phase 13: ElevenLabs session fallback callback (D-38)
  const handleTTSFallback = useCallback(() => {
    if (ttsFallbackToastShownRef.current) return
    ttsFallbackToastShownRef.current = true
    toastManager.add({ title: 'ElevenLabs unavailable — using browser voice.' })
  }, [])

  // Phase 13: Destroy TTS player on unmount
  useEffect(() => {
    return () => {
      ttsPlayerRef.current?.destroy()
      ttsPlayerRef.current = null
    }
  }, [])

  // --- Stream callbacks ----------------------------------------------------
  // useOverlayStream owns the connection, replay, dedup and enrichment; the
  // overlay applies its own filter → sound → TTS → append+fade policy here.

  const onChat = useCallback((message: ChatMessage) => {
    // Phase 11: apply filter settings before adding to render queue (D-01, D-02)
    if (shouldFilterMessage(message, filterSettingsRef.current)) return
    // Phase 12: play notification sound for messages that pass the filter (D-05)
    soundPlayerRef.current?.play()
    // Phase 13: speak the message via TTS (D-41, D-42 — independent of sound; both fire on non-filtered)
    ttsPlayerRef.current?.speak(message)
    setMessages((prev) =>
      // Ignore a redelivery of a message already on screen. The render key is
      // `message.id`, so a duplicate id would be a duplicate React key (and the
      // deletion path already assumes ids are unique — it removes every row
      // matching `target_uuid`). A WebSocket reconnect that replays the tail is
      // the realistic source.
      prev.some((m) => m.id === message.id)
        ? prev
        : [...prev, message].slice(-maxMessagesRef.current)
    )
  }, [])

  const onMessageUpdate = useCallback((updatedMessage: ChatMessage) => {
    setMessages((prev) => {
      // Find existing message by aggregation_id (TikTok like aggregates), then
      // by id — an update that carries a live row's id has to REPLACE that row.
      // Appending it would put the same id on screen twice, and the render key
      // is the id (see onChat).
      const aggregationId = updatedMessage.event?.aggregation_id
      const index = aggregationId
        ? prev.findIndex((m) => m.event?.aggregation_id === aggregationId)
        : prev.findIndex((m) => m.id === updatedMessage.id)
      if (index === -1) {
        // Original message already faded away, treat as new
        return [...prev, updatedMessage].slice(-maxMessagesRef.current)
      }
      // Update existing message in place
      const updated = [...prev]
      updated[index] = updatedMessage
      return updated
    })
  }, [])

  const onDeletion = useCallback((deletion: DeletionMetadata) => {
    setMessages((prev) => {
      switch (deletion.deletion_type) {
        case 'single':
          // Remove specific message by internal UUID
          if (!deletion.target_uuid) return prev
          return prev.filter((m) => m.id !== deletion.target_uuid)
        case 'batch':
          // Remove all messages from specific user (timeout/ban)
          if (!deletion.target_user_id) return prev
          return prev.filter((m) => m.user.id !== deletion.target_user_id)
        case 'clear':
          // Remove all messages (full chat clear)
          return []
        default:
          return prev
      }
    })
  }, [])

  // Render-only "disconnect-protection" mode (?passive=true): a 24/7 OBS instance
  // stays connected for rendering but does NOT assert overlay demand, so it never
  // starts YouTube discovery (and the 1h give-up) while the streamer is offline.
  // Discovery is triggered on demand from the chat monitor when they go live.
  const [passive] = useState(
    () =>
      typeof window !== 'undefined' &&
      new URLSearchParams(window.location.search).get('passive') === 'true'
  )

  const { config, sources, activeChannels, channelStatuses } = useOverlayStream(id, {
    onChat,
    onMessageUpdate,
    onDeletion,
    passive,
  })

  // Interpret the public config into display state + sound/TTS hydration. Runs
  // on first load and on every 30s refresh (config is a fresh object each time).
  useEffect(() => {
    /* eslint-disable react-hooks/set-state-in-effect --
       Syncing display state from asynchronously-loaded public config; this is the
       canonical "sync with external data" effect (see useHydrated.ts for the same
       disable). It mirrors the overlay's original async loadConfig behavior. */
    if (!config) return

    const display = config.display_settings || {}

    if (typeof display.max_messages === 'number') {
      setMaxMessages(display.max_messages)
      maxMessagesRef.current = display.max_messages
    }
    if (typeof display.font_size === 'number') {
      setFontSize(display.font_size)
    }
    if (typeof display.message_duration === 'number') {
      setMessageDuration(display.message_duration)
    }
    if (typeof display.disable_message_fade === 'boolean') {
      setDisableMessageFade(display.disable_message_fade)
    }
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
    setInvertMessageOrder(display.invert_message_order === true)
    setFeedAnchor(parseFeedAnchor(display.feed_anchor))

    // Phase 9: Pronoun settings from display_settings
    if (typeof display.show_pronouns === 'boolean') {
      setShowPronouns(display.show_pronouns)
    }
    if (display.pronoun_position === 'before' || display.pronoun_position === 'after') {
      setPronounPosition(display.pronoun_position)
    }
    if (typeof display.pronoun_color === 'string' && display.pronoun_color) {
      setPronounColor(display.pronoun_color)
    }

    setCustomCss(typeof config.custom_css === 'string' ? config.custom_css : '')
    setThemeCss(
      typeof config.theme_id === 'string' && config.theme_id
        ? rewriteThemeFontImports(getBundledTheme(config.theme_id)?.css ?? '')
        : ''
    )

    if (config.visual_settings && typeof config.visual_settings === 'object') {
      const vs = config.visual_settings as Partial<VisualSettings>
      setVisualSettingsCss(visualSettingsToCss(vs))
      // Body font-size override from the visual customizer (see state decl).
      setMessageFontSizeCss(typeof vs.fontSize === 'string' && vs.fontSize ? vs.fontSize : '')
      // Background fills / shadow / max-width (see state decl).
      setContainerStyle(overlayContainerStyle(vs))
      setBubbleStyle(chatBubbleStyle(vs))
      for (const key of ['fontFamily', 'usernameFontFamily', 'timestampFontFamily'] as const) {
        if (typeof vs[key] === 'string') ensureGoogleFontLoaded(vs[key]!)
      }
      // Override display_settings with visual_settings if present
      if (vs.showPlatformBadge !== undefined) {
        setShowPlatformBadge(vs.showPlatformBadge !== 'none')
      }
      if (vs.platformBadgePosition !== undefined) {
        setPlatformBadgePosition(vs.platformBadgePosition)
      }
      if (vs.platformBadgeStyle !== undefined) {
        setPlatformBadgeStyle(vs.platformBadgeStyle)
      }
      // Unconditional so clearing the setting restores the default on reload
      setMessageAnimation(isMessageAnimation(vs.messageAnimation) ? vs.messageAnimation : null)
      if (vs.showPlatformIndicators !== undefined) {
        setShowPlatformIndicators(vs.showPlatformIndicators !== 'none')
      }
      // Visibility toggles the live overlay must interpret itself (see state decls)
      if (vs.showAvatars !== undefined) {
        setShowAvatars(isDisplayVisible(vs.showAvatars))
      }
      if (vs.showBadges !== undefined) {
        setShowBadges(isDisplayVisible(vs.showBadges))
      }
      if (vs.showTimestamps !== undefined) {
        setShowTimestamps(isDisplayVisible(vs.showTimestamps))
      }
      if (vs.showUsername !== undefined) {
        setShowUsername(isDisplayVisible(vs.showUsername))
      }
      // Phase 9: Pronoun visual_settings overrides
      if (vs.showPronouns !== undefined) {
        setShowPronouns(vs.showPronouns !== 'none')
      }
      if (vs.pronounPosition !== undefined) {
        setPronounPosition(vs.pronounPosition)
      }
      if (vs.pronounColor !== undefined) {
        setPronounColor(vs.pronounColor)
      }
    }

    // Phase 11: Load filter settings
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
        typeof display.tts_messages_per_minute === 'number' ? display.tts_messages_per_minute : 8,
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
        ? display.tts_enabled_platforms.filter((p: unknown): p is string => typeof p === 'string')
        : ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
    }

    // Phase 13 Plan 03: For the ElevenLabs branch, hydrate the runtime fetch
    // endpoint + tts_token JWT (read from the URL query string produced by
    // Plan 02's rotate-token / GET /tts-config). Voice-fallback contract:
    // when `voiceId` is empty, Plan 02's HandleTTS substitutes cfg.VoiceID
    // from the saved overlay_tts_configs row (no DB round-trip here).
    if (ttsLoaded.provider === 'elevenlabs') {
      const urlParams = new URLSearchParams(window.location.search)
      const tokenFromUrl = urlParams.get('tts_token') ?? undefined
      ttsLoaded.ttsEndpoint = `/api/v1/overlays/${id}/tts`
      ttsLoaded.ttsToken = tokenFromUrl
      ttsLoaded.voiceId = ''
    }

    ttsSettingsRef.current = ttsLoaded
    if (ttsPlayerRef.current) {
      ttsPlayerRef.current.updateSettings(ttsLoaded)
    } else {
      ttsPlayerRef.current = createTTSPlayer(ttsLoaded, handleTTSFallback)
    }
    /* eslint-enable react-hooks/set-state-in-effect */
  }, [config, id, handleTTSFallback])

  // Auto-scroll to bottom when new messages arrive.
  //
  // A short bottom-anchored feed has nothing to scroll, so skip the call. Once
  // it overflows, the `mt-auto` has collapsed to 0 and the feed is a plain
  // scrolling list again — so scroll, exactly as before.
  useEffect(() => {
    // "Overflows" has to mean "the page scrolls", which is why this measures the
    // wrapper and not the list. The wrapper is `min-h-screen p-4`, so the canvas
    // the list actually gets is the viewport MINUS 32px of padding, and the
    // wrapper's own box grows with the list once it passes that. Comparing the
    // list's height against a bare `window.innerHeight` therefore reported "no
    // overflow" for a band of ~32px in which the page did scroll — and a
    // bottom-anchored feed then skipped the scroll and let `body`'s
    // `overflow: hidden` clip the newest row until the next message arrived.
    //
    // `offsetHeight` (LAYOUT height), not `documentElement.scrollHeight`
    // (SCROLLABLE height): an entry animation translates the new row outside the
    // list — `msg-anim-bounce` starts at `translateY(110%)`, the default
    // `slide-in-from-bottom-2` at 8px — and transformed descendants count toward
    // scrollable overflow. On a bottom-anchored feed, whose list ends flush with
    // the canvas, that briefly made a feed that fits report an overflow, so this
    // effect smooth-scrolled the page and let it snap back when the animation
    // ended: a visible jitter on every single message. Layout height ignores
    // transforms, which is exactly the question being asked here.
    const wrapper = feedWrapperRef.current
    const overflows = wrapper != null && wrapper.offsetHeight > window.innerHeight
    if (!shouldAutoScroll(feedLayout, overflows)) return
    // `block: 'nearest'` (as on the embed preview) scrolls the minimum needed to
    // reveal the sentinel instead of always slamming it to the viewport's top
    // edge, which keeps a transient mid-animation measurement from moving a feed
    // that is already showing its newest row.
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [messages, feedLayout])

  // Auto-remove old messages based on duration (if fade is enabled)
  // Events have tier-based durations, chat uses configured duration
  useEffect(() => {
    if (messages.length === 0 || disableMessageFade) return

    const firstMessage = messages[0]

    // Determine display duration
    let duration = messageDuration // Default from settings

    if (firstMessage.event) {
      // Event: use event-specific duration or tier-based default
      duration = firstMessage.event.duration || getTierDuration(firstMessage.event.tier)
    }

    const timer = setTimeout(() => {
      setMessages((prev) => prev.slice(1))
    }, duration * 1000)

    return () => clearTimeout(timer)
  }, [messages, messageDuration, disableMessageFade])

  // Helper function to render event-specific content
  const renderEventContent = (message: ChatMessage): React.ReactNode => (
    <EventContent message={message} />
  )

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

  // Platform icon components
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

  // Platform badge — single platform, or a combined badge when a message carries
  // multiple (a streamer's "send to all" echo collapsed into one message). Honors
  // the configured style (icon | text); for length ≤ 1 it renders exactly as before.
  const PlatformBadge = ({ message }: { message: ChatMessage }) => {
    const multi = Array.isArray(message.platforms) && message.platforms.length > 1
    const platforms = multi ? (message.platforms as string[]) : [message.platform]
    const title = platforms.join(', ')

    if (platformBadgeStyle === 'icon') {
      return (
        <span
          className="platform-badge platform-badge-icon flex items-center gap-0.5"
          title={title}
        >
          {platforms.map((p) => (
            <PlatformIcon key={p} platform={p} />
          ))}
        </span>
      )
    }
    // Text style: join multiple platforms with '+', colored by the primary platform.
    return (
      <span
        className={clsx(
          'platform-badge platform-badge-text text-xs font-semibold uppercase',
          getPlatformColor(message.platform)
        )}
      >
        {platforms.join('+')}
      </span>
    )
  }

  return (
    /*
     * feedAnchor 'bottom' makes this wrapper a flex column so the message
     * list's `mt-auto` has free space to absorb. The list must stay its only
     * IN-FLOW child for that to hold: the <style> blocks below are
     * `display: none` per the UA stylesheet and PlatformStatusIndicators
     * defaults to variant='fixed' (position: fixed), so neither becomes a flex
     * item. Adding an in-flow sibling here would split the free space and
     * un-anchor the feed.
     */
    <div
      ref={feedWrapperRef}
      className={clsx('min-h-screen w-full bg-transparent p-4', feedLayout.wrapperClass)}
      data-feed-anchor={feedLayout.dataAnchor}
      data-feed-order={feedLayout.dataOrder}
    >
      {/* Hide scrollbars and ensure transparent background */}
      <style
        dangerouslySetInnerHTML={{
          __html: `
        body {
          overflow: hidden !important;
          background: transparent !important;
        }
        body::-webkit-scrollbar {
          display: none !important;
        }
        * {
          scrollbar-width: none !important;
          -ms-overflow-style: none !important;
        }
      `,
        }}
      />
      {/*
       * SECURITY (audit L33): the following <style> blocks use
       * dangerouslySetInnerHTML for owner-authored CSS (visual settings, theme,
       * and raw custom_css) on a public overlay page. A CSP `style-src`
       * directive must constrain the blast radius (frontend-hardening agent
       * adds CSP via next.config.js headers). Do not remove this note until
       * CSP is in place.
       */}
      {visualSettingsCss.length > 0 && (
        <style dangerouslySetInnerHTML={{ __html: visualSettingsCss }} />
      )}
      {/* Bundled theme CSS first, then the user's raw custom_css overrides it. */}
      {themeCss.length > 0 && <style dangerouslySetInnerHTML={{ __html: themeCss }} />}
      {customCss.trim().length > 0 && <style dangerouslySetInnerHTML={{ __html: customCss }} />}

      {/* Platform Status Indicators */}
      {showPlatformIndicators && (
        <PlatformStatusIndicators
          configuredSources={sources}
          activeChannels={activeChannels}
          channelStatuses={channelStatuses}
        />
      )}

      {/* text-shadow inherits to every text node below, which is how the setting
          reaches nodes no rule names (pronoun pill, shared-chat tag, event body).
          It is NOT what makes the setting stick: this is a normal inline style,
          and every bundled theme declares `text-shadow: … !important` on the
          message text, which beats it. The authoritative delivery is the
          `!important` rule visualSettingsToCss emits inside
          `@layer visual-customizer` (see OVERRIDE_RULES). Gradient usernames
          force the shadow off locally with an important inline style, which
          outranks both. */}
      {/* `mt-auto` (feedAnchor 'bottom') sits on the list itself, never on its
          children: `.overlay-live-body > * + *` in events.css is `!important`
          inside a cascade layer and would beat any child-level rule. */}
      <div
        className={clsx(
          'overlay-live-body space-y-3',
          feedLayout.bodyClass,
          messageAnimation && 'msg-anim-container'
        )}
        style={{ textShadow: 'var(--chat-text-shadow, none)', ...containerStyle }}
      >
        {feedLayout.sentinelPosition === 'start' && (
          <div ref={messagesEndRef} className="scroll-anchor" />
        )}
        {orderMessages(messages, invertMessageOrder).map((message) => {
          const isSharedChat = message.metadata?.is_shared_chat === true
          const isEvent = message.event != null
          const eventTierClass = isEvent ? `event-tier-${message.event?.tier}` : ''
          const eventTypeClass = isEvent ? `event-type-${message.event?.type}` : ''

          return (
            /* The key must be POSITION-INDEPENDENT. It used to be
               `${message.id}-${index}`, and every path that shifts an index —
               `invert_message_order` (a prepend moves every row), the
               `max_messages` cap, the fade timer's `slice(1)` — changed every
               key at once, so React unmounted and remounted the whole feed and
               every row replayed its entry animation on every new message.
               `onChat` keeps ids unique, which is what makes the bare id safe. */
            <div
              key={message.id}
              data-message-id={message.id}
              data-platform={message.platform}
              data-event-type={isEvent ? message.event?.type : undefined}
              data-username={message.user?.username}
              className={clsx(
                isEvent
                  ? ['event-message', eventTierClass, eventTypeClass]
                  : [
                      'chat-message rounded-lg p-3 shadow-lg backdrop-blur-sm',
                      messageAnimation
                        ? MESSAGE_ANIMATION_CLASS[messageAnimation]
                        : feedLayout.defaultEntryAnimationClass,
                      isSharedChat
                        ? 'border-2 border-purple-500/50 bg-purple-900/40'
                        : 'bg-slate-900/90',
                    ]
              )}
              style={isEvent ? undefined : bubbleStyle}
            >
              <div className="flex items-start gap-3">
                {/* Avatar */}
                {showAvatars && (
                  <div className="flex-shrink-0" style={{ overflow: 'visible' }}>
                    <UserAvatar
                      avatarUrl={message.user?.avatar_url}
                      frameUrl={message.user?.avatar_frame_url}
                      flairUrl={message.user?.avatar_flair_url}
                      size={40}
                      displayName={message.user?.display_name}
                    />
                  </div>
                )}

                {/* Message Content */}
                <div className="min-w-0 flex-1">
                  {/* Username and Platform */}
                  <div className="mb-1 flex flex-wrap items-center gap-2">
                    {/* Platform badge - render based on position and style settings */}
                    {showPlatformBadge && platformBadgePosition === 'before' && (
                      <PlatformBadge message={message} />
                    )}

                    {/* Regular Badges (before username when position is 'before') */}
                    {showBadges &&
                      platformBadgePosition === 'before' &&
                      message.user?.badges &&
                      message.user.badges.length > 0 && (
                        <div className="flex items-center gap-1">
                          {message.user.badges.map((badge, idx) =>
                            badge.name === 'allchat' ? (
                              <AllChatBadge key={idx} size={18} title={badge.name} />
                            ) : badge.name === 'allchat-premium' ? (
                              <PremiumBadge key={idx} size={18} title={badge.name} />
                            ) : badge.icon_url ? (
                              <Image
                                key={idx}
                                src={badge.icon_url}
                                alt={badge.name}
                                width={18}
                                height={18}
                                className="h-[1em] w-auto object-contain"
                                title={badge.name}
                              />
                            ) : (
                              <span
                                key={idx}
                                className="rounded bg-slate-700 px-1 py-0.5 text-xs leading-none text-slate-300"
                                title={badge.name}
                              >
                                {badge.name}
                              </span>
                            )
                          )}
                        </div>
                      )}

                    {/* Phase 9: Pronoun pill - before username */}
                    {showPronouns && message.user?.pronouns && pronounPosition === 'before' && (
                      <span
                        className="inline-flex items-center rounded-full px-2 py-1 text-[11px] leading-none font-semibold text-white"
                        style={{ backgroundColor: pronounColor }}
                      >
                        {message.user.pronouns}
                      </span>
                    )}

                    {/* Username */}
                    {showUsername &&
                      (message.user?.name_gradient ? (
                        <span
                          ref={(el) => {
                            if (el) {
                              // Clean drop shadow behind the gradient fill instead of a
                              // -webkit-text-stroke that muddied the colours (ADR-0044).
                              el.style.setProperty('-webkit-text-stroke', '0', 'important')
                              el.style.setProperty(
                                'text-shadow',
                                '0 1px 2px rgba(0,0,0,0.65), 0 0 2px rgba(0,0,0,0.5)',
                                'important'
                              )
                              el.style.setProperty('color', 'transparent', 'important')
                              el.style.setProperty(
                                '-webkit-text-fill-color',
                                'transparent',
                                'important'
                              )
                              el.style.setProperty('background-clip', 'text', 'important')
                              el.style.setProperty('-webkit-background-clip', 'text', 'important')
                            }
                          }}
                          className="chat-username username-gradient bg-clip-text text-sm font-semibold text-transparent"
                          style={{ backgroundImage: buildGradientCSS(message.user.name_gradient) }}
                        >
                          {message.user?.display_name || message.user?.username}
                        </span>
                      ) : (
                        <span
                          className="chat-username text-sm font-semibold"
                          style={{
                            color: resolveUsernameColor(message.user),
                          }}
                        >
                          {message.user?.display_name || message.user?.username}
                        </span>
                      ))}

                    {/* Phase 9: Pronoun pill - after username */}
                    {showPronouns && message.user?.pronouns && pronounPosition === 'after' && (
                      <span
                        className="inline-flex items-center rounded-full px-2 py-1 text-[11px] leading-none font-semibold text-white"
                        style={{ backgroundColor: pronounColor }}
                      >
                        {message.user.pronouns}
                      </span>
                    )}

                    {/* Platform badge after username (original position) */}
                    {showPlatformBadge && platformBadgePosition === 'after' && (
                      <PlatformBadge message={message} />
                    )}

                    {/* Shared Chat Indicator */}
                    {isSharedChat && (
                      <span className="rounded border border-purple-400/50 bg-purple-600/80 px-1.5 py-0.5 text-xs font-semibold text-purple-100 uppercase">
                        Shared Chat
                      </span>
                    )}

                    {/* Regular Badges (after username when position is 'after') */}
                    {showBadges &&
                      platformBadgePosition === 'after' &&
                      message.user?.badges &&
                      message.user.badges.length > 0 && (
                        <div className="flex items-center gap-1">
                          {message.user.badges.map((badge, idx) =>
                            badge.name === 'allchat' ? (
                              <AllChatBadge key={idx} size={18} title={badge.name} />
                            ) : badge.name === 'allchat-premium' ? (
                              <PremiumBadge key={idx} size={18} title={badge.name} />
                            ) : badge.icon_url ? (
                              <Image
                                key={idx}
                                src={badge.icon_url}
                                alt={badge.name}
                                width={18}
                                height={18}
                                className="h-[1em] w-auto object-contain"
                                title={badge.name}
                              />
                            ) : (
                              <span
                                key={idx}
                                className="rounded bg-slate-700 px-1 py-0.5 text-xs leading-none text-slate-300"
                                title={badge.name}
                              >
                                {badge.name}
                              </span>
                            )
                          )}
                        </div>
                      )}

                    {/* Source Channel Badges (for shared chat) */}
                    {isSharedChat &&
                      message.user?.source_badges &&
                      message.user.source_badges.length > 0 && (
                        <>
                          <span className="text-xs text-purple-300">|</span>
                          <div className="flex gap-1">
                            {message.user.source_badges.map((badge, idx) => (
                              <Image
                                key={`source-${idx}`}
                                src={badge.icon_url}
                                alt={badge.name}
                                width={16}
                                height={16}
                                className="h-4 w-4 rounded-sm object-contain ring-1 ring-purple-400/50"
                                title={`${badge.name} (source channel)`}
                              />
                            ))}
                          </div>
                        </>
                      )}
                  </div>

                  {/* Message Text with Emotes (or Event Content) */}
                  <div
                    className="break-words text-white"
                    style={{ fontSize: messageFontSizeCss || `${fontSize}px` }}
                  >
                    {message.event ? renderEventContent(message) : renderMessageContent(message)}
                  </div>

                  {!message.event && <MessageAttachments message={message} />}

                  {/* Timestamp */}
                  {showTimestamps && (
                    <div className="mt-1 text-xs text-slate-500">
                      {new Date(message.timestamp).toLocaleTimeString()}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )
        })}
        {feedLayout.sentinelPosition === 'end' && (
          <div ref={messagesEndRef} className="scroll-anchor" />
        )}
      </div>
    </div>
  )
}
