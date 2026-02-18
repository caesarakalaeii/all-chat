/**
 * OBS Overlay Page (Unauthenticated)
 *
 * Clean overlay view for OBS Browser Source - no authentication required.
 * Displays only chat messages without any UI chrome.
 *
 * Features:
 * - WebSocket connection without authentication
 * - Real-time message rendering
 * - Platform identification (Twitch, YouTube, etc.)
 * - User badges, avatars, and colors
 * - Emote display
 * - Auto-scroll to latest messages
 * - Transparent background for OBS
 *
 * This is a Client Component because it:
 * - Uses WebSocket (browser API)
 * - Manages real-time state
 */

'use client';

import Image from 'next/image';
import { use, useEffect, useState, useRef } from 'react';
import type { ChatMessage, EventTier, PlatformStatus, DeletionMetadata } from '@/lib/types/message';
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';
import { sortMessageBadges } from '@/lib/badgeOrder';
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators';
import '@/styles/events.css';

export default function OBSOverlayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [maxMessages, setMaxMessages] = useState(50);
  const [fontSize, setFontSize] = useState(16);
  const [messageDuration, setMessageDuration] = useState(15);
  const [disableMessageFade, setDisableMessageFade] = useState(false);
  const [customCss, setCustomCss] = useState('');
  const [activePlatforms, setActivePlatforms] = useState<Set<string>>(new Set());
  const [platformStatuses, setPlatformStatuses] = useState<Map<string, PlatformStatus>>(new Map());
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [forceReconnect, setForceReconnect] = useState(0);
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before');
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text');
  const [showPlatformBadge, setShowPlatformBadge] = useState(true);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Load overlay display configuration (public endpoint)
  useEffect(() => {
    const loadConfig = async () => {
      try {
        const response = await fetch(`/api/v1/overlays/public/${id}/config`);
        if (!response.ok) {
          throw new Error('failed to load config');
        }

        const data = await response.json();
        const display = data.display_settings || {};

        if (typeof display.max_messages === 'number') {
          setMaxMessages(display.max_messages);
        }
        if (typeof display.font_size === 'number') {
          setFontSize(display.font_size);
        }
        if (typeof display.message_duration === 'number') {
          setMessageDuration(display.message_duration);
        }
        if (typeof display.disable_message_fade === 'boolean') {
          setDisableMessageFade(display.disable_message_fade);
        }
        if (display.platform_badge_position === 'before' || display.platform_badge_position === 'after') {
          setPlatformBadgePosition(display.platform_badge_position);
        }
        if (display.platform_badge_style === 'text' || display.platform_badge_style === 'icon') {
          setPlatformBadgeStyle(display.platform_badge_style);
        }
        if (typeof display.show_platform_badge === 'boolean') {
          setShowPlatformBadge(display.show_platform_badge);
        }

        setCustomCss(typeof data.custom_css === 'string' ? data.custom_css : '');

        // Load active platforms from sources
        if (Array.isArray(data.sources)) {
          const active = new Set<string>();
          data.sources.forEach((source: { platform: string; is_active: boolean }) => {
            if (source.is_active) {
              active.add(source.platform);
            }
          });
          setActivePlatforms(active);
        }
      } catch (error) {
        console.warn('[OBS Overlay] Failed to load config', error);
      }
    };

    loadConfig();

    // Refresh source status periodically (every 30 seconds)
    const interval = setInterval(loadConfig, 30000);
    return () => clearInterval(interval);
  }, [id]);

  // Initialize WebSocket connection (no auth required)
  useEffect(() => {
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/overlay/${id}`;

    console.log('[OBS Overlay] Connecting to:', wsUrl, reconnectAttempts ? `(attempt ${reconnectAttempts + 1})` : '');

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('[OBS Overlay] Connected');
      // Reset reconnection attempts on successful connection
      setReconnectAttempts(0);
    };

    ws.onmessage = async (event) => {
      try {
        const envelope = JSON.parse(event.data);

        console.log('[OBS Overlay] Received message:', envelope);

        // Handle deletion events FIRST (before regular messages)
        if (envelope.type === 'chat_message' && envelope.data?.event?.type === 'message_deletion') {
          const deletion = envelope.data.event.metadata as DeletionMetadata;

          // Update state based on deletion type
          setMessages((prev) => {
            switch (deletion.deletion_type) {
              case 'single':
                // Remove specific message by internal UUID
                const targetId = deletion.target_uuid;
                if (!targetId) {
                  console.warn('[Deletion] Single deletion missing target_uuid');
                  return prev;
                }
                console.debug('[Deletion] Removing single message:', targetId);
                return prev.filter((m) => m.id !== targetId);

              case 'batch':
                // Remove all messages from specific user (timeout/ban)
                const targetUserId = deletion.target_user_id;
                if (!targetUserId) {
                  console.warn('[Deletion] Batch deletion missing target_user_id');
                  return prev;
                }
                console.debug('[Deletion] Removing all messages from user:', targetUserId, deletion.target_username);
                return prev.filter((m) => m.user.id !== targetUserId);

              case 'clear':
                // Remove all messages (full chat clear)
                console.debug('[Deletion] Clearing all messages');
                return [];

              default:
                console.warn('[Deletion] Unknown deletion type:', deletion.deletion_type);
                return prev;
            }
          });

          return; // Don't process as regular message
        }

        // Handle chat messages and events
        if (envelope.type === 'chat_message' && envelope.data && !envelope.data.event) {
          let message: ChatMessage = envelope.data;
          message = await resolveTwitchBadgeIcons(message);
          message = sortMessageBadges(message);

          setMessages((prev) => {
            const newMessages = [...prev, message];
            return newMessages.slice(-maxMessages);
          });
        }

        // Handle message updates (TikTok like aggregates)
        if (envelope.type === 'message_update' && envelope.data) {
          let updatedMessage: ChatMessage = envelope.data;
          updatedMessage = await resolveTwitchBadgeIcons(updatedMessage);
          updatedMessage = sortMessageBadges(updatedMessage);

          setMessages((prev) => {
            // Find existing message by aggregation_id
            const aggregationId = updatedMessage.event?.aggregation_id;
            if (!aggregationId) {
              // No aggregation ID, treat as new message
              const newMessages = [...prev, updatedMessage];
              return newMessages.slice(-maxMessages);
            }

            const index = prev.findIndex(
              (m) => m.event?.aggregation_id === aggregationId
            );

            if (index === -1) {
              // Original message already faded away, treat as new
              const newMessages = [...prev, updatedMessage];
              return newMessages.slice(-maxMessages);
            }

            // Update existing message in place
            const updated = [...prev];
            updated[index] = updatedMessage;
            return updated;
          });
        }

        // Handle platform status updates
        if (envelope.type === 'platform_status' && envelope.data) {
          const statusData = envelope.data as PlatformStatus;
          setPlatformStatuses((prev) => {
            const next = new Map(prev);
            next.set(statusData.platform, statusData);
            return next;
          });
        }
      } catch (error) {
        console.error('[OBS Overlay] Failed to parse message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('[OBS Overlay] WebSocket error:', error);
    };

    ws.onclose = (event) => {
      console.log('[OBS Overlay] Disconnected:', event.code, event.reason);

      // Calculate exponential backoff with jitter
      const baseDelay = 1000; // Start at 1 second
      const maxDelay = 30000; // Cap at 30 seconds
      const jitter = Math.random() * 1000; // 0-1s jitter to prevent thundering herd
      const delay = Math.min(baseDelay * Math.pow(1.5, reconnectAttempts), maxDelay) + jitter;

      console.log(`[OBS Overlay] Reconnecting in ${Math.round(delay)}ms (attempt ${reconnectAttempts + 1})`);

      // Schedule reconnection
      reconnectTimeoutRef.current = setTimeout(() => {
        setReconnectAttempts(prev => prev + 1);
        setForceReconnect(Date.now()); // Trigger reconnection via effect dependency
      }, delay);
    };

    // Cleanup on unmount
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close();
      }
    };
  }, [id, maxMessages, forceReconnect]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Auto-remove old messages based on duration (if fade is enabled)
  // Events have tier-based durations, chat uses configured duration
  useEffect(() => {
    if (messages.length === 0 || disableMessageFade) return;

    const firstMessage = messages[0];

    // Determine display duration
    let duration = messageDuration; // Default from settings

    if (firstMessage.event) {
      // Event: use event-specific duration or tier-based default
      duration = firstMessage.event.duration || getTierDuration(firstMessage.event.tier);
    }

    const timer = setTimeout(() => {
      setMessages((prev) => prev.slice(1));
    }, duration * 1000);

    return () => clearTimeout(timer);
  }, [messages, messageDuration, disableMessageFade]);

  // Helper function to get default duration based on event tier
  const getTierDuration = (tier: EventTier): number => {
    switch (tier) {
      case 'high':
        return 30;
      case 'medium':
        return 15;
      case 'low':
        return 8;
      default:
        return 15;
    }
  };

  // Helper function to render event-specific content
  const renderEventContent = (message: ChatMessage): React.ReactNode => {
    const event = message.event!;

    // Event icon based on type
    const getEventIcon = () => {
      switch (event.type) {
        case 'subscription':
        case 'resubscription':
        case 'gift_subscription':
        case 'kick_subscription':
        case 'new_sponsor':
          return '⭐';
        case 'bits':
          return '💎';
        case 'raid':
          return '🚀';
        case 'channel_points':
          return '🎁';
        case 'super_chat':
          return '💰';
        case 'super_sticker':
          return '🎨';
        case 'gift':
          return '🎁';
        case 'follow':
          return '❤️';
        case 'like_aggregate':
          return '👍';
        case 'share':
          return '🔗';
        case 'member_milestone':
          return '🎂';
        case 'membership_gift':
          return '🎁';
        case 'token_expiration_warning':
          return '⚠️';
        default:
          return '✨';
      }
    };

    // Event title based on type
    const getEventTitle = () => {
      switch (event.type) {
        case 'subscription':
          return 'New Subscriber!';
        case 'resubscription':
          return 'Resubscribed!';
        case 'gift_subscription':
          return 'Gift Subscription!';
        case 'mystery_gift':
          return 'Mystery Gift Bomb!';
        case 'bits':
          return 'Bits Cheered!';
        case 'raid':
          return 'Raid Incoming!';
        case 'channel_points':
          return 'Channel Points Redeemed!';
        case 'super_chat':
          return 'Super Chat!';
        case 'super_sticker':
          return 'Super Sticker!';
        case 'new_sponsor':
          return 'New Member!';
        case 'member_milestone':
          return 'Member Milestone!';
        case 'membership_gift':
          return 'Membership Gift!';
        case 'gift':
          return 'Gift Received!';
        case 'follow':
          return 'New Follower!';
        case 'like_aggregate':
          return 'Likes!';
        case 'share':
          return 'Stream Shared!';
        case 'token_expiration_warning':
          const platform = (event.metadata?.platform as string) || 'Platform';
          return `${platform.charAt(0).toUpperCase() + platform.slice(1)} Authentication Error`;
        default:
          return 'Event!';
      }
    };

    return (
      <div className="event-content">
        <div className="flex items-center gap-3 mb-1">
          <span className="text-4xl event-icon leading-none">{getEventIcon()}</span>
          <div className="flex-1">
            <div className="text-lg font-bold event-title text-white">{getEventTitle()}</div>
            <div className="text-sm font-semibold event-user" style={{ color: message.user?.color || '#FFFFFF' }}>
              {message.user?.display_name || message.user?.username}
            </div>
          </div>
          {event.value && (
            <div className="text-2xl font-bold event-value text-yellow-300">
              {event.value.display_text}
            </div>
          )}
        </div>
        {message.message.text && (
          <div className="text-sm event-message-text text-gray-200 ml-14">
            {message.message.text}
          </div>
        )}
        {event.type === 'token_expiration_warning' && (
          <div className="text-sm event-warning-message text-orange-200 ml-14 mt-2 space-y-1">
            <div className="font-semibold">
              {event.metadata?.failure_reason === 'expired'
                ? 'OAuth token has expired'
                : 'Failed to refresh OAuth token'}
              {event.metadata?.username ? ` for ${String(event.metadata.username)}` : ''}
            </div>
            <div className="text-xs text-orange-300">
              {'→ Please reconnect your account in Settings → Connections'}
            </div>
          </div>
        )}
        {event.metadata && Object.keys(event.metadata).length > 0 && (
          <div className="text-xs event-metadata text-gray-400 mt-1 ml-14">
            {(event.metadata as any).viewer_count && `${(event.metadata as any).viewer_count.toLocaleString()} viewers`}
            {(event.metadata as any).months && `${(event.metadata as any).months} months`}
            {(event.metadata as any).streak && ` • ${(event.metadata as any).streak} month streak`}
            {(event.metadata as any).gift_count && `${(event.metadata as any).gift_count} gifts`}
            {(event.metadata as any).bits && `${(event.metadata as any).bits} bits`}
            {(event.metadata as any).like_count && `${(event.metadata as any).like_count} likes`}
            {(event.metadata as any).diamonds && `${(event.metadata as any).diamonds} diamonds`}
          </div>
        )}
      </div>
    );
  };

  const getPlatformColor = (platform: string): string => {
    switch (platform) {
      case 'twitch':
        return 'text-purple-400';
      case 'youtube':
        return 'text-red-400';
      case 'kick':
        return 'text-green-400';
      default:
        return 'text-gray-400';
    }
  };

  // Platform icon components
  const PlatformIcon = ({ platform }: { platform: string }) => {
    const iconClass = "inline-block w-4 h-4";

    switch (platform) {
      case 'twitch':
        return (
          <svg viewBox="0 0 24 24" className={iconClass}>
            <path fill="#9146FF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
          </svg>
        );
      case 'youtube':
        return (
          <svg viewBox="0 0 24 24" className={iconClass}>
            <path fill="#FF0000" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
          </svg>
        );
      case 'kick':
        return (
          <svg viewBox="0 0 24 24" className={iconClass} style={{ imageRendering: 'pixelated' }}>
            <text x="12" y="18" fontSize="20" fontWeight="bold" fill="#00E701" textAnchor="middle" fontFamily="monospace">K</text>
          </svg>
        );
      case 'tiktok':
        return (
          <svg viewBox="0 0 24 24" className={iconClass}>
            <path fill="#000000" d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"/>
          </svg>
        );
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen w-full p-4 bg-transparent">
      {/* Hide scrollbars and ensure transparent background */}
      <style dangerouslySetInnerHTML={{ __html: `
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
      ` }} />
      {customCss.trim().length > 0 && (
        <style dangerouslySetInnerHTML={{ __html: customCss }} />
      )}

      {/* Platform Status Indicators */}
      <PlatformStatusIndicators activePlatforms={activePlatforms} platformStatuses={platformStatuses} />

      <div className="space-y-3">
        {messages.map((message, index) => {
          const isSharedChat = message.metadata?.is_shared_chat === true;
          const isEvent = message.event != null;
          const eventTierClass = isEvent ? `event-tier-${message.event?.tier}` : '';
          const eventTypeClass = isEvent ? `event-type-${message.event?.type}` : '';

          return (
          <div
            key={`${message.id}-${index}`}
            data-message-id={message.id}
            data-platform={message.platform}
            data-event-type={isEvent ? message.event?.type : undefined}
            className={
              isEvent
                ? `event-message ${eventTierClass} ${eventTypeClass}`
                : `backdrop-blur-sm rounded-lg p-3 shadow-lg animate-in slide-in-from-bottom-2 duration-300 chat-message ${
                    isSharedChat
                      ? 'bg-purple-900/40 border-2 border-purple-500/50'
                      : 'bg-gray-900/90'
                  }`
            }
          >
            <div className="flex items-start gap-3">
              {/* Avatar */}
              <div className="flex-shrink-0">
                {message.user?.avatar_url ? (
                  <Image
                    src={message.user.avatar_url}
                    alt={message.user.username}
                    width={40}
                    height={40}
                    className="w-10 h-10 rounded-full object-cover"
                    onError={(e) => {
                      e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(
                        message.user.display_name || message.user.username
                      )}&background=6b7280&color=fff&size=40`;
                    }}
                  />
                ) : (
                  <div className="w-10 h-10 rounded-full bg-gray-700 flex items-center justify-center text-white font-semibold">
                    {message.user?.username?.slice(0, 2).toUpperCase() || '?'}
                  </div>
                )}
              </div>

              {/* Message Content */}
              <div className="flex-1 min-w-0">
                {/* Username and Platform */}
                <div className="flex items-center gap-2 mb-1 flex-wrap">
                  {/* Platform badge - render based on position and style settings */}
                  {showPlatformBadge && platformBadgePosition === 'before' && (
                    platformBadgeStyle === 'icon' ? (
                      <span className="platform-badge platform-badge-icon flex items-center" title={message.platform}>
                        <PlatformIcon platform={message.platform} />
                      </span>
                    ) : (
                      <span className={`platform-badge platform-badge-text text-xs font-semibold uppercase ${getPlatformColor(message.platform)}`}>
                        {message.platform}
                      </span>
                    )
                  )}

                  {/* Regular Badges (before username when position is 'before') */}
                  {platformBadgePosition === 'before' && message.user?.badges && message.user.badges.length > 0 && (
                    <div className="flex gap-1">
                      {message.user.badges.map((badge, idx) => (
                        <Image
                          key={idx}
                          src={badge.icon_url}
                          alt={badge.name}
                          width={16}
                          height={16}
                          className="w-4 h-4 object-contain"
                          title={`${badge.name} (receiving channel)`}
                        />
                      ))}
                    </div>
                  )}

                  {/* Username */}
                  <span
                    className="font-semibold text-sm"
                    style={{ color: message.user?.color || '#FFFFFF' }}
                  >
                    {message.user?.display_name || message.user?.username}
                  </span>

                  {/* Platform badge after username (original position) */}
                  {showPlatformBadge && platformBadgePosition === 'after' && (
                    platformBadgeStyle === 'icon' ? (
                      <span className="platform-badge platform-badge-icon flex items-center" title={message.platform}>
                        <PlatformIcon platform={message.platform} />
                      </span>
                    ) : (
                      <span className={`platform-badge platform-badge-text text-xs font-semibold uppercase ${getPlatformColor(message.platform)}`}>
                        {message.platform}
                      </span>
                    )
                  )}

                  {/* Shared Chat Indicator */}
                  {isSharedChat && (
                    <span className="text-xs font-semibold uppercase px-1.5 py-0.5 rounded bg-purple-600/80 text-purple-100 border border-purple-400/50">
                      Shared Chat
                    </span>
                  )}

                  {/* Regular Badges (after username when position is 'after') */}
                  {platformBadgePosition === 'after' && message.user?.badges && message.user.badges.length > 0 && (
                    <div className="flex gap-1">
                      {message.user.badges.map((badge, idx) => (
                        <Image
                          key={idx}
                          src={badge.icon_url}
                          alt={badge.name}
                          width={16}
                          height={16}
                          className="w-4 h-4 object-contain"
                          title={`${badge.name} (receiving channel)`}
                        />
                      ))}
                    </div>
                  )}

                  {/* Source Channel Badges (for shared chat) */}
                  {isSharedChat && message.user?.source_badges && message.user.source_badges.length > 0 && (
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
                            className="w-4 h-4 object-contain ring-1 ring-purple-400/50 rounded-sm"
                            title={`${badge.name} (source channel)`}
                          />
                        ))}
                      </div>
                    </>
                  )}
                </div>

                {/* Message Text with Emotes (or Event Content) */}
                <div className="text-white break-words" style={{ fontSize: `${fontSize}px` }}>
                  {message.event ? renderEventContent(message) : renderMessageContent(message)}
                </div>

                {/* Timestamp */}
                <div className="text-xs text-gray-500 mt-1">
                  {new Date(message.timestamp).toLocaleTimeString()}
                </div>
              </div>
            </div>
          </div>
        )})}
        <div ref={messagesEndRef} />
      </div>
    </div>
  );
}
