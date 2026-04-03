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
import type { ChatMessage, EventTier, NameGradient, PlatformStatus, DeletionMetadata } from '@/lib/types/message';
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';
import { sortMessageBadges } from '@/lib/badgeOrder';
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators';
import { buildGradientCSS } from '@/lib/utils/gradient';
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css';
import type { VisualSettings } from '@/lib/types/visual-settings';

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

import { UserAvatar } from '@/components/UserAvatar';
import { AllChatBadge } from '@/components/AllChatBadge';
import { PremiumBadge } from '@/components/PremiumBadge';
import type { SourceInfo } from '@/components/PlatformStatusIndicators';
import '@/styles/events.css';

export default function OBSOverlayPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [maxMessages, setMaxMessages] = useState(50);
  const [fontSize, setFontSize] = useState(16);
  const [messageDuration, setMessageDuration] = useState(15);
  const [disableMessageFade, setDisableMessageFade] = useState(false);
  const [customCss, setCustomCss] = useState('');
  const [visualSettingsCss, setVisualSettingsCss] = useState('');
  const [configuredSources, setConfiguredSources] = useState<Map<string, SourceInfo>>(new Map()); // channel_id -> SourceInfo
  const [activeChannels, setActiveChannels] = useState<Set<string>>(new Set()); // Set of channel_ids
  const [channelStatuses, setChannelStatuses] = useState<Map<string, PlatformStatus>>(new Map()); // channel_id -> PlatformStatus
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [forceReconnect, setForceReconnect] = useState(0);
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before');
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text');
  const [showPlatformBadge, setShowPlatformBadge] = useState(true);
  const [showPlatformIndicators, setShowPlatformIndicators] = useState(true);
  const [invertMessageOrder, setInvertMessageOrder] = useState(false);
  const [showPronouns, setShowPronouns] = useState(true);          // D-07: default on
  const [pronounPosition, setPronounPosition] = useState<'before' | 'after'>('after');  // default after
  const [pronounColor, setPronounColor] = useState('#7B68EE');     // default medium slate blue

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const lastSeenTimestampRef = useRef<number>(0);

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
        setInvertMessageOrder(display.invert_message_order === true);

        // Phase 9: Pronoun settings from display_settings
        if (typeof display.show_pronouns === 'boolean') {
          setShowPronouns(display.show_pronouns);
        }
        if (display.pronoun_position === 'before' || display.pronoun_position === 'after') {
          setPronounPosition(display.pronoun_position);
        }
        if (typeof display.pronoun_color === 'string' && display.pronoun_color) {
          setPronounColor(display.pronoun_color);
        }

        setCustomCss(typeof data.custom_css === 'string' ? data.custom_css : '');

        if (data.visual_settings && typeof data.visual_settings === 'object') {
          const vs = data.visual_settings as Partial<VisualSettings>;
          setVisualSettingsCss(visualSettingsToCss(vs));
          for (const key of ['fontFamily', 'usernameFontFamily', 'timestampFontFamily'] as const) {
            if (typeof vs[key] === 'string') ensureGoogleFontLoaded(vs[key]!);
          }
          // Override display_settings with visual_settings if present
          if (vs.showPlatformBadge !== undefined) {
            setShowPlatformBadge(vs.showPlatformBadge !== 'none');
          }
          if (vs.platformBadgePosition !== undefined) {
            setPlatformBadgePosition(vs.platformBadgePosition);
          }
          if (vs.platformBadgeStyle !== undefined) {
            setPlatformBadgeStyle(vs.platformBadgeStyle);
          }
          if (vs.showPlatformIndicators !== undefined) {
            setShowPlatformIndicators(vs.showPlatformIndicators !== 'none');
          }
          // Phase 9: Pronoun visual_settings overrides
          if (vs.showPronouns !== undefined) {
            setShowPronouns(vs.showPronouns !== 'none');
          }
          if (vs.pronounPosition !== undefined) {
            setPronounPosition(vs.pronounPosition);
          }
          if (vs.pronounColor !== undefined) {
            setPronounColor(vs.pronounColor);
          }
        }

        // Load configured sources (channel_id -> SourceInfo)
        // Note: activeChannels is only set by live platform_status messages (status === 'connected')
        // so that the indicator reflects actual live connection state, not DB is_active flag.
        if (Array.isArray(data.sources)) {
          const sources = new Map<string, SourceInfo>();
          data.sources.forEach((source: { platform: string; channel_id: string; channel_name?: string; is_active: boolean }) => {
            sources.set(source.channel_id, {
              platform: source.platform,
              channelId: source.channel_id,
              channelName: source.channel_name || source.channel_id,
            });
          });
          setConfiguredSources(sources);
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

    // Load last seen timestamp from localStorage (survives page reload)
    const storageKey = `ws_last_seen_${id}`;
    const storedTimestamp = localStorage.getItem(storageKey);
    if (storedTimestamp) {
      lastSeenTimestampRef.current = parseInt(storedTimestamp, 10);
    }

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('[OBS Overlay] Connected');
      // Reset reconnection attempts on successful connection
      setReconnectAttempts(0);

      // Request replay if reconnecting (not first connect)
      if (lastSeenTimestampRef.current > 0) {
        const replayRequest = {
          type: 'replay_request',
          data: {
            since: lastSeenTimestampRef.current,
          },
          timestamp: new Date().toISOString(),
        };
        ws.send(JSON.stringify(replayRequest));
        console.log('[OBS Overlay] Requested deletion replay since:', new Date(lastSeenTimestampRef.current));
      }
    };

    ws.onmessage = async (event) => {
      try {
        const envelope = JSON.parse(event.data);

        console.log('[OBS Overlay] Received message:', envelope);

        // Handle replay response (batch of missed deletions)
        if (envelope.type === 'replay_response') {
          const deletions = envelope.data as DeletionMetadata[];
          console.log(`[OBS Overlay] Replaying ${deletions.length} missed deletions`);

          // Apply each deletion
          deletions.forEach((deletion) => {
            setMessages((prev) => {
              switch (deletion.deletion_type) {
                case 'single':
                  const targetId = deletion.target_uuid;
                  if (!targetId) return prev;
                  return prev.filter((m) => m.id !== targetId);

                case 'batch':
                  const targetUserId = deletion.target_user_id;
                  if (!targetUserId) return prev;
                  return prev.filter((m) => m.user.id !== targetUserId);

                case 'clear':
                  return [];

                default:
                  return prev;
              }
            });
          });

          return; // Don't process further
        }

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

          // Update last seen timestamp AFTER processing deletion
          lastSeenTimestampRef.current = Date.now();
          localStorage.setItem(`ws_last_seen_${id}`, String(lastSeenTimestampRef.current));

          return; // Don't process as regular message
        }

        // Handle chat messages and events
        if (envelope.type === 'chat_message' && envelope.data) {
          let message: ChatMessage = envelope.data;
          message = await resolveTwitchBadgeIcons(message);
          message = sortMessageBadges(message);
          if (message.user?.name_gradient && typeof message.user.name_gradient === 'string') {
            message.user.name_gradient = JSON.parse(message.user.name_gradient as unknown as string) as NameGradient;
          }

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
          if (updatedMessage.user?.name_gradient && typeof updatedMessage.user.name_gradient === 'string') {
            updatedMessage.user.name_gradient = JSON.parse(updatedMessage.user.name_gradient as unknown as string) as NameGradient;
          }

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

        // Handle platform status updates - only for channels configured on this overlay
        if (envelope.type === 'platform_status' && envelope.data) {
          const statusData = envelope.data as PlatformStatus;
          const channelId = statusData.channel_id || statusData.platform;
          // Reject status for channels not configured on this overlay.
          // configuredSources is populated from the overlay's sources; a missing entry means
          // the channel is not used here — not that the filter hasn't loaded yet.
          // An empty configuredSources map (size === 0) means config hasn't arrived; in that
          // transient window we fall through and accept all statuses to avoid a blank display.
          const isConfigured = configuredSources.size === 0 || configuredSources.has(channelId);
          if (isConfigured) {
            // Update activeChannels based on connection state
            if (statusData.status === 'connected') {
              setActiveChannels((prev) => {
                if (prev.has(channelId)) return prev;
                const next = new Set(prev);
                next.add(channelId);
                return next;
              });
            } else if (statusData.status === 'offline') {
              setActiveChannels((prev) => {
                if (!prev.has(channelId)) return prev;
                const next = new Set(prev);
                next.delete(channelId);
                return next;
              });
            }
            setChannelStatuses((prev) => {
              const next = new Map(prev);
              // Don't overwrite connected with reconnecting from a different channel
              const existing = prev.get(channelId);
              if (existing?.status === 'connected' && statusData.status === 'reconnecting') {
                return prev;
              }
              next.set(channelId, statusData);
              return next;
            });
          }
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
        case 'source_permission_error':
          return '🔒';
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
        case 'source_permission_error':
          return 'Bot Missing Channel Permission';
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
            <div className="text-sm font-semibold event-user" style={{ color: message.user?.color || 'var(--chat-username-color, #FFFFFF)' }}>
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
        {event.type === 'source_permission_error' && (
          <div className="text-sm event-warning-message text-red-200 ml-14 mt-2 space-y-1">
            <div className="font-semibold">
              {`Channel ${String(event.metadata?.channel_id || '')} is not accessible`}
            </div>
            <div className="text-xs text-red-300">
              {'→ Grant the bot "View Channel" permission in your Discord server settings'}
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
      {visualSettingsCss.length > 0 && (
        <style dangerouslySetInnerHTML={{ __html: visualSettingsCss }} />
      )}
      {customCss.trim().length > 0 && (
        <style dangerouslySetInnerHTML={{ __html: customCss }} />
      )}

      {/* Platform Status Indicators */}
      {showPlatformIndicators && (
        <PlatformStatusIndicators configuredSources={configuredSources} activeChannels={activeChannels} channelStatuses={channelStatuses} />
      )}

      <div className="space-y-3">
        {invertMessageOrder && <div ref={messagesEndRef} className="scroll-anchor" />}
        {(invertMessageOrder ? [...messages].reverse() : messages).map((message, index) => {
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
            data-username={message.user?.username}
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
              <div className="flex-shrink-0" style={{ overflow: 'visible' }}>
                <UserAvatar
                  avatarUrl={message.user?.avatar_url}
                  frameUrl={message.user?.avatar_frame_url}
                  flairUrl={message.user?.avatar_flair_url}
                  size={40}
                  displayName={message.user?.display_name}
                />
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
                    <div className="flex gap-1 items-center">
                      {message.user.badges.map((badge, idx) => (
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
                          <span key={idx} className="text-xs px-1 py-0.5 rounded bg-gray-700 text-gray-300 leading-none" title={badge.name}>
                            {badge.name}
                          </span>
                        )
                      ))}
                    </div>
                  )}

                  {/* Phase 9: Pronoun pill - before username */}
                  {showPronouns && message.user?.pronouns && pronounPosition === 'before' && (
                    <span
                      className="inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white"
                      style={{ backgroundColor: pronounColor }}
                    >
                      {message.user.pronouns}
                    </span>
                  )}

                  {/* Username */}
                  {message.user?.name_gradient ? (
                    <span
                      ref={(el) => {
                        if (el) {
                          el.style.setProperty('text-shadow', 'none', 'important')
                          el.style.setProperty('-webkit-text-stroke', '0.5px rgba(0,0,0,0.5)', 'important')
                          el.style.setProperty('color', 'transparent', 'important')
                          el.style.setProperty('-webkit-text-fill-color', 'transparent', 'important')
                          el.style.setProperty('background-clip', 'text', 'important')
                          el.style.setProperty('-webkit-background-clip', 'text', 'important')
                        }
                      }}
                      className="font-semibold text-sm bg-clip-text text-transparent username-gradient"
                      style={{ backgroundImage: buildGradientCSS(message.user.name_gradient) }}
                    >
                      {message.user?.display_name || message.user?.username}
                    </span>
                  ) : (
                    <span
                      className="font-semibold text-sm"
                      style={{ color: message.user?.color || 'var(--chat-username-color, #FFFFFF)' }}
                    >
                      {message.user?.display_name || message.user?.username}
                    </span>
                  )}

                  {/* Phase 9: Pronoun pill - after username */}
                  {showPronouns && message.user?.pronouns && pronounPosition === 'after' && (
                    <span
                      className="inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white"
                      style={{ backgroundColor: pronounColor }}
                    >
                      {message.user.pronouns}
                    </span>
                  )}

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
                    <div className="flex gap-1 items-center">
                      {message.user.badges.map((badge, idx) => (
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
                          <span key={idx} className="text-xs px-1 py-0.5 rounded bg-gray-700 text-gray-300 leading-none" title={badge.name}>
                            {badge.name}
                          </span>
                        )
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
        {!invertMessageOrder && <div ref={messagesEndRef} className="scroll-anchor" />}
      </div>
    </div>
  );
}
