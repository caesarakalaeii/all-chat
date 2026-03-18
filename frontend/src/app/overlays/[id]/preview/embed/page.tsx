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
import { use, useEffect, useState, useRef, useMemo } from 'react'
import clsx from 'clsx'
import { useAuthStore } from '@/lib/stores/auth-store'
import { WebSocketClient } from '@/lib/api/websocket'
import { overlaysApi } from '@/lib/api/overlays'
import type { ChatMessage } from '@/lib/types/message'
import { renderMessageContent } from '@/lib/renderMessage'
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges'
import { sortMessageBadges } from '@/lib/badgeOrder'
import '@/styles/events.css'

// ---- Utilities (identical to preview/page.tsx) ----------------------------

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

// ---- Google Font loader ---------------------------------------------------

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
  const encodedName = encodeURIComponent(fontFamily)
  link.href = `https://fonts.googleapis.com/css2?family=${encodedName}:wght@400;600;700&display=swap`
  document.head.appendChild(link)
}

// ---- Page -----------------------------------------------------------------

export default function OverlayEmbedPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const { token } = useAuthStore()

  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [maxMessages, setMaxMessages] = useState(50)
  const [fontSize, setFontSize] = useState(16)
  const [customCss, setCustomCss] = useState('')
  const [useCustomCss, setUseCustomCss] = useState(false)
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before')
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text')
  const [showPlatformBadge, setShowPlatformBadge] = useState(true)

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
      if (event.data?.type !== 'VISUAL_CSS_UPDATE') return
      const css = event.data.css as string

      // Upsert the visual customizer style tag
      let styleEl = document.getElementById('visual-customizer-style') as HTMLStyleElement | null
      if (!styleEl) {
        styleEl = document.createElement('style')
        styleEl.id = 'visual-customizer-style'
        document.head.appendChild(styleEl)
      }
      styleEl.textContent = css

      // Load any Google Fonts referenced in the CSS
      const fontFamilyMatches = css.matchAll(/--chat-[^:]*font-family\s*:\s*([^;}"]+)/g)
      for (const match of fontFamilyMatches) {
        const fontFamily = match[1].trim().replace(/['"]/g, '')
        ensureGoogleFontLoaded(fontFamily)
      }
    }

    window.addEventListener('message', handleMessage)
    return () => {
      window.removeEventListener('message', handleMessage)
    }
  }, [])

  // Load overlay config
  useEffect(() => {
    if (!token) return

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

        const css = config.custom_css || ''
        setCustomCss(css)
        setUseCustomCss(Boolean(css.trim().length))
      } catch (error) {
        console.warn('[Embed] Failed to load overlay config', error)
      }
    }

    loadConfig()
  }, [id, token])

  // Initialize WebSocket connection
  useEffect(() => {
    // No auth redirect — just show empty transparent page if no token
    if (!token) return

    const wsClient = new WebSocketClient()
    wsClientRef.current = wsClient

    wsClient.connect(id, token)

    const unsubscribe = wsClient.onMessage(async (incoming) => {
      const message = sortMessageBadges(await resolveTwitchBadgeIcons(incoming))
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
  }, [id, token, maxMessages])

  // Trim buffer when maxMessages changes
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setMessages((prev) => (prev.length > maxMessages ? prev.slice(-maxMessages) : prev))
  }, [maxMessages])

  // Helper: render event-specific content (identical to preview/page.tsx)
  const renderEventContent = (message: ChatMessage): React.ReactNode => {
    const event = message.event!

    const getEventIcon = () => {
      switch (event.type) {
        case 'subscription':
        case 'resubscription':
        case 'gift_subscription':
        case 'kick_subscription':
        case 'new_sponsor':
          return '⭐'
        case 'bits':
          return '💎'
        case 'raid':
          return '🚀'
        case 'channel_points':
          return '🎁'
        case 'super_chat':
          return '💰'
        case 'super_sticker':
          return '🎨'
        case 'gift':
          return '🎁'
        case 'follow':
          return '❤️'
        case 'like_aggregate':
          return '👍'
        case 'share':
          return '🔗'
        case 'member_milestone':
          return '🎂'
        case 'membership_gift':
          return '🎁'
        default:
          return '✨'
      }
    }

    const getEventTitle = () => {
      switch (event.type) {
        case 'subscription':
          return 'New Subscriber!'
        case 'resubscription':
          return 'Resubscribed!'
        case 'gift_subscription':
          return 'Gift Subscription!'
        case 'mystery_gift':
          return 'Mystery Gift Bomb!'
        case 'bits':
          return 'Bits Cheered!'
        case 'raid':
          return 'Raid Incoming!'
        case 'channel_points':
          return 'Channel Points Redeemed!'
        case 'super_chat':
          return 'Super Chat!'
        case 'super_sticker':
          return 'Super Sticker!'
        case 'new_sponsor':
          return 'New Member!'
        case 'member_milestone':
          return 'Member Milestone!'
        case 'membership_gift':
          return 'Membership Gift!'
        case 'gift':
          return 'Gift Received!'
        case 'follow':
          return 'New Follower!'
        case 'like_aggregate':
          return 'Likes!'
        case 'share':
          return 'Stream Shared!'
        default:
          return 'Event!'
      }
    }

    return (
      <div className="event-content">
        <div className="mb-1 flex items-center gap-3">
          <span className="event-icon text-4xl leading-none">{getEventIcon()}</span>
          <div className="flex-1">
            <div className="event-title text-lg font-bold text-white">{getEventTitle()}</div>
            <div
              className="event-user text-sm font-semibold"
              style={{ color: message.user?.color || '#FFFFFF' }}
            >
              {message.user?.display_name || message.user?.username}
            </div>
          </div>
          {event.value && (
            <div className="event-value text-2xl font-bold text-yellow-300">
              {event.value.display_text}
            </div>
          )}
        </div>
        {message.message.text && (
          <div className="event-message-text ml-14 text-sm text-slate-200">
            {message.message.text}
          </div>
        )}
        {event.metadata &&
          Object.keys(event.metadata).length > 0 &&
          (() => {
            const m = event.metadata as Record<string, unknown>
            const parts: string[] = []
            if (m.viewer_count) parts.push(`${String(m.viewer_count)} viewers`)
            if (m.months) parts.push(`${String(m.months)} months`)
            if (m.streak) parts.push(`${String(m.streak)} month streak`)
            if (m.gift_count) parts.push(`${String(m.gift_count)} gifts`)
            if (m.bits) parts.push(`${String(m.bits)} bits`)
            if (m.like_count) parts.push(`${String(m.like_count)} likes`)
            if (m.diamonds) parts.push(`${String(m.diamonds)} diamonds`)
            return parts.length > 0 ? (
              <div className="event-metadata mt-1 ml-14 text-xs text-slate-400">
                {parts.join(' • ')}
              </div>
            ) : null
          })()}
      </div>
    )
  }

  return (
    <main className="min-h-screen bg-transparent">
      {useCustomCss && scopedPreviewCss && (
        <style
          key={scopedPreviewCss}
          id="overlay-preview-custom-css"
          dangerouslySetInnerHTML={{ __html: scopedPreviewCss }}
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
                                {message.user.badges.map((badge, index) => (
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
                                ))}
                              </div>
                            )}

                          {/* Username */}
                          <span
                            className="text-sm font-semibold"
                            style={{ color: message.user.color || '#FFFFFF' }}
                          >
                            {message.user.display_name}
                          </span>

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
                                {message.user.badges.map((badge, index) => (
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
                                ))}
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
                        <div className="mt-1 text-xs text-gray-500">
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
