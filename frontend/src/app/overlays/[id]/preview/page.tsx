/**
 * Overlay Preview Page
 *
 * Real-time preview of the overlay with live chat messages via WebSocket.
 *
 * Features:
 * - WebSocket connection to API Gateway
 * - Real-time message rendering
 * - Platform identification (Twitch, YouTube, etc.)
 * - User badges and colors
 * - Emote display
 * - Auto-scroll to latest messages
 * - Connection status indicator
 * - Copy OBS URL button
 * - Customization panel
 *
 * This is a Client Component because it:
 * - Uses WebSocket (browser API)
 * - Manages real-time state
 * - Handles user interactions
 */

'use client';

import Image from 'next/image';
import { useEffect, useState, useRef, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { WebSocketClient } from '@/lib/api/websocket';
import { overlaysApi } from '@/lib/api/overlays';
import type { ChatMessage } from '@/lib/types/message';
import type { ChatSource } from '@/lib/types/overlay';
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';
import { sortMessageBadges } from '@/lib/badgeOrder';
import dynamic from 'next/dynamic';
import '@/styles/events.css';

// Dynamically import Monaco Editor to avoid SSR issues
const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div className="h-[300px] bg-gray-900 border border-gray-700 rounded-lg flex items-center justify-center">
      <div className="text-gray-400 text-sm">Loading editor...</div>
    </div>
  )
});

// Dynamically import Theme Marketplace Modal
const ThemeMarketplaceModal = dynamic(
  () => import('@/components/theme-marketplace/ThemeMarketplaceModal'),
  { ssr: false }
);

type MockMessageFormState = {
  platform: ChatMessage['platform'];
  displayName: string;
  username: string;
  avatarUrl: string;
  message: string;
  color: string;
};

const DEFAULT_MOCK_FORM: MockMessageFormState = {
  platform: 'twitch',
  displayName: 'Overlay Fan',
  username: 'overlayfan',
  avatarUrl: '',
  message: 'This overlay looks great! PogChamp',
  color: '#9146ff'
};

const SAMPLE_MOCK_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'sample-user-1',
      username: 'retro_mod',
      display_name: 'RetroMod',
      avatar_url: 'https://i.pravatar.cc/100?img=13',
      badges: [],
      color: '#fbbf24'
    },
    message: {
      text: 'Welcome to the overlay preview! PogChamp',
      emotes: []
    },
    metadata: { mock: true }
  },
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'sample-user-2',
      username: 'cybercritic',
      display_name: 'CyberCritic',
      avatar_url: 'https://i.pravatar.cc/100?img=32',
      badges: [],
      color: '#f87171'
    },
    message: {
      text: 'Picked up the neon CSS preset and it SLAPS 🔥',
      emotes: []
    },
    metadata: { mock: true }
  },
  {
    platform: 'kick',
    channel_id: 'sample-kick',
    channel_name: 'Sample Kick',
    user: {
      id: 'sample-user-3',
      username: 'emote_master',
      display_name: 'EmoteMaster',
      avatar_url: 'https://i.pravatar.cc/100?img=56',
      badges: [],
      color: '#4ade80'
    },
    message: {
      text: 'Drop your favorite emotes in chat 😎',
      emotes: []
    },
    metadata: { mock: true }
  }
];

const SAMPLE_EVENT_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  // High-tier Twitch subscription
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-1',
      username: 'generousviewer',
      display_name: 'GenerousViewer',
      avatar_url: 'https://i.pravatar.cc/100?img=45',
      badges: [],
      color: '#ff6b6b'
    },
    message: {
      text: 'Love the stream! Keep it up!',
      emotes: []
    },
    event: {
      type: 'subscription',
      tier: 'high',
      duration: 30,
      is_update: false,
      metadata: {
        sub_tier: '1000',
        months: 1,
        streak: 1
      }
    },
    metadata: { mock: true, event: true }
  },
  // High-tier YouTube Super Chat
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'event-user-2',
      username: 'superfan',
      display_name: 'SuperFan',
      avatar_url: 'https://i.pravatar.cc/100?img=67',
      badges: [],
      color: '#e91e63'
    },
    message: {
      text: 'Amazing content! Thanks for all you do!',
      emotes: []
    },
    event: {
      type: 'super_chat',
      tier: 'high',
      value: {
        amount: 50,
        currency: 'USD',
        display_text: '$50.00'
      },
      duration: 60,
      is_update: false,
      metadata: {}
    },
    metadata: { mock: true, event: true }
  },
  // High-tier Twitch raid
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-3',
      username: 'bigstreamer',
      display_name: 'BigStreamer',
      avatar_url: 'https://i.pravatar.cc/100?img=23',
      badges: [],
      color: '#9146ff'
    },
    message: {
      text: 'is raiding with 2,500 viewers!',
      emotes: []
    },
    event: {
      type: 'raid',
      tier: 'high',
      duration: 40,
      is_update: false,
      metadata: {
        viewer_count: 2500
      }
    },
    metadata: { mock: true, event: true }
  },
  // Medium-tier gift subscription
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-4',
      username: 'kindperson',
      display_name: 'KindPerson',
      avatar_url: 'https://i.pravatar.cc/100?img=89',
      badges: [],
      color: '#f59e0b'
    },
    message: {
      text: 'gifted 5 subs to the community!',
      emotes: []
    },
    event: {
      type: 'gift_subscription',
      tier: 'medium',
      duration: 20,
      is_update: false,
      metadata: {
        gift_count: 5,
        sub_tier: '1000'
      }
    },
    metadata: { mock: true, event: true }
  },
  // Medium-tier TikTok gift
  {
    platform: 'tiktok',
    channel_id: 'sample-tiktok',
    channel_name: 'Sample TikTok',
    user: {
      id: 'event-user-5',
      username: 'tiktokfan',
      display_name: 'TikTokFan',
      avatar_url: 'https://i.pravatar.cc/100?img=34',
      badges: [],
      color: '#00f2ea'
    },
    message: {
      text: 'sent a Rose (1 diamonds)',
      emotes: []
    },
    event: {
      type: 'gift',
      tier: 'medium',
      duration: 15,
      is_update: false,
      metadata: {
        gift_name: 'Rose',
        diamonds: 1,
        gift_count: 1
      }
    },
    metadata: { mock: true, event: true }
  },
  // Low-tier TikTok likes (aggregated)
  {
    platform: 'tiktok',
    channel_id: 'sample-tiktok',
    channel_name: 'Sample TikTok',
    user: {
      id: 'event-user-6',
      username: 'liker123',
      display_name: 'Liker123',
      avatar_url: 'https://i.pravatar.cc/100?img=78',
      badges: [],
      color: '#10b981'
    },
    message: {
      text: 'sent 47 likes',
      emotes: []
    },
    event: {
      type: 'like_aggregate',
      tier: 'low',
      duration: 10,
      is_update: false,
      aggregation_id: 'sample-agg-1',
      metadata: {
        like_count: 47
      }
    },
    metadata: { mock: true, event: true }
  },
  // Low-tier TikTok follow
  {
    platform: 'tiktok',
    channel_id: 'sample-tiktok',
    channel_name: 'Sample TikTok',
    user: {
      id: 'event-user-7',
      username: 'newfollower',
      display_name: 'NewFollower',
      avatar_url: 'https://i.pravatar.cc/100?img=91',
      badges: [],
      color: '#8b5cf6'
    },
    message: {
      text: 'followed',
      emotes: []
    },
    event: {
      type: 'follow',
      tier: 'low',
      duration: 10,
      is_update: false,
      metadata: {}
    },
    metadata: { mock: true, event: true }
  },
  // Medium-tier bits
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-8',
      username: 'cheerleader',
      display_name: 'CheerLeader',
      avatar_url: 'https://i.pravatar.cc/100?img=55',
      badges: [],
      color: '#06b6d4'
    },
    message: {
      text: 'Cheer100 Love the vibes!',
      emotes: []
    },
    event: {
      type: 'bits',
      tier: 'medium',
      duration: 15,
      is_update: false,
      metadata: {
        bits: 100
      }
    },
    metadata: { mock: true, event: true }
  }
];

const EXAMPLE_CUSTOM_CSS = `/* Example neon glass theme */
body {
  background: transparent !important;
  font-family: 'Space Grotesk', sans-serif !important;
}

/* Target only message containers */
.space-y-3 > div.bg-gray-900\\/90 {
  background: rgba(74, 29, 150, 0.45) !important;
  border: 1px solid rgba(236, 72, 153, 0.5) !important;
  border-radius: 16px !important;
  padding: 1.25rem !important;
  backdrop-filter: blur(18px) saturate(180%) !important;
  box-shadow: 0 25px 45px rgba(0, 0, 0, 0.35) !important;
}

.text-xs.font-semibold.uppercase {
  background: rgba(236, 72, 153, 0.2) !important;
  color: #f472b6 !important;
  padding: 0.15rem 0.6rem !important;
  border-radius: 999px !important;
  letter-spacing: 0.15em !important;
}

.text-white.break-words {
  font-size: 18px !important;
  color: #fff1f2 !important;
  text-shadow: 0 0 12px rgba(236, 72, 153, 0.65) !important;
}
`;

const isMockMessage = (message: ChatMessage): boolean => {
  const data = message.metadata as { mock?: boolean };
  return Boolean(data?.mock);
};

const scopeCustomCss = (css: string, scopeSelector: string, bodySelector: string): string => {
  if (!css.trim()) {
    return '';
  }

  const replaceBody = css
    .replace(/:root/gi, scopeSelector)
    .replace(/\bbody\b/gi, bodySelector);

  return replaceBody.replace(/(^|}|{)\s*([^@}{]+)\s*{/g, (match, prefix, selectorGroup) => {
    const trimmed = selectorGroup.trim();
    if (!trimmed) {
      return match;
    }

    const isKeyframeStep =
      ['from', 'to'].includes(trimmed.toLowerCase()) || /^\d+\.?\d*%$/i.test(trimmed);
    if (isKeyframeStep) {
      return `${prefix} ${trimmed} {`;
    }

    const scopedSelectors = trimmed
      .split(',')
      .map((selector: string) => {
        const sel = selector.trim();
        if (!sel || sel.startsWith(scopeSelector) || sel.startsWith(bodySelector)) {
          return sel;
        }
        return `${scopeSelector} ${sel}`;
      })
      .filter(Boolean)
      .join(', ');

    return `${prefix} ${scopedSelectors} {`;
  });
};

export default function OverlayPreviewPage({ params }: { params: { id: string } }) {
  const router = useRouter();
  const { token } = useAuthStore();

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [connected, setConnected] = useState(false);
  const [maxMessages, setMaxMessages] = useState(50);
  const [fontSize, setFontSize] = useState(16);
  const [messageDuration, setMessageDuration] = useState(15);
  const [disableMessageFade, setDisableMessageFade] = useState(false);
  const [mockForm, setMockForm] = useState<MockMessageFormState>(DEFAULT_MOCK_FORM);
  const [customCss, setCustomCss] = useState('');
  const [useCustomCss, setUseCustomCss] = useState(false);
  const [configLoaded, setConfigLoaded] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [configAlert, setConfigAlert] = useState<{ type: 'success' | 'error'; message: string } | null>(null);
  const [sources, setSources] = useState<ChatSource[]>([]);
  const [showThemeMarketplace, setShowThemeMarketplace] = useState(false);
  const [platformBadgePosition, setPlatformBadgePosition] = useState<'before' | 'after'>('before');
  const [platformBadgeStyle, setPlatformBadgeStyle] = useState<'text' | 'icon'>('text');
  const [showPlatformBadge, setShowPlatformBadge] = useState(true);

  const wsClientRef = useRef<WebSocketClient | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const scopedPreviewCss = useMemo(() => {
    if (!useCustomCss || !customCss.trim()) {
      return '';
    }
    return scopeCustomCss(customCss, '#overlay-preview-root', '#overlay-preview-root .overlay-preview-body');
  }, [customCss, useCustomCss]);

  // Fetch overlay config for customization defaults
  useEffect(() => {
    if (!token) {
      return;
    }

    const loadConfig = async () => {
      try {
        const config = await overlaysApi.getConfig(params.id);
        const display = config.display_settings || {};

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

        const css = config.custom_css || '';
        setCustomCss(css);
        setUseCustomCss(Boolean(css.trim().length));
      } catch (error) {
        console.warn('Failed to load overlay config', error);
      } finally {
        setConfigLoaded(true);
      }
    };

    loadConfig();
  }, [params.id, token]);

  // Load overlay sources for determining mock targets
  useEffect(() => {
    const loadSources = async () => {
      try {
        const loadedSources = await overlaysApi.getSources(params.id);
        setSources(loadedSources);
      } catch (error) {
        console.error('[Preview] Failed to load sources:', error);
      }
    };

    loadSources();
  }, [params.id]);

  // Initialize WebSocket connection
  useEffect(() => {
    if (!token) {
      router.push('/');
      return;
    }

    // Create WebSocket client
    const wsClient = new WebSocketClient();
    wsClientRef.current = wsClient;

    // Connect to overlay WebSocket
    wsClient.connect(params.id, token);

    // Listen for messages
    const unsubscribe = wsClient.onMessage(async (incoming) => {
      const message = sortMessageBadges(await resolveTwitchBadgeIcons(incoming));
      setMessages((prev) => [...prev, message].slice(-maxMessages));
      setConnected(true);
    });

    // Check connection status periodically
    const interval = setInterval(() => {
      setConnected(wsClient.isConnected());
    }, 1000);

    // Cleanup on unmount
    return () => {
      unsubscribe();
      wsClient.disconnect();
      clearInterval(interval);
    };
  }, [params.id, token, maxMessages, router]);

  // Trim message buffer when maxMessages changes
  useEffect(() => {
    setMessages((prev) => (prev.length > maxMessages ? prev.slice(-maxMessages) : prev));
  }, [maxMessages]);

  // Automatically send sample messages on initial load
  useEffect(() => {
    if (!token || !configLoaded) {
      return;
    }

    // Send sample messages after a short delay to ensure WebSocket is connected
    const timer = setTimeout(() => {
      void handleAddSampleTranscript();
    }, 1500);

    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [configLoaded, token]);

  // Auto-scroll to bottom when new messages arrive (disabled for preview to avoid annoying scroll jumps)
  // useEffect(() => {
  //   messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  // }, [messages]);

  const copyOverlayUrl = () => {
    const url = `${window.location.origin}/overlay/${params.id}`;
    navigator.clipboard.writeText(url);
    alert('Overlay URL copied to clipboard!\n\nAdd this as a Browser Source in OBS.');
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

  const handleMockInputChange = <K extends keyof MockMessageFormState>(field: K, value: MockMessageFormState[K]) => {
    setMockForm((prev) => ({
      ...prev,
      [field]: value
    }));
  };

  const resolveMockTarget = (requestedPlatform?: ChatMessage['platform']) => {
    const preferred =
      sources.find((source) =>
        requestedPlatform ? source.platform === requestedPlatform : true
      ) ?? sources[0];

    if (!preferred) {
      return {
        platform: requestedPlatform || 'twitch',
        channel_id: 'mock-channel',
        channel_name: 'Mock Channel'
      };
    }

    return {
      platform: requestedPlatform || (preferred.platform as ChatMessage['platform']),
      channel_id: preferred.channel_id || 'mock-channel',
      channel_name: preferred.channel_name || preferred.channel_id || 'Mock Channel'
    };
  };

  const handleAddMockMessage = async () => {
    if (!mockForm.message.trim()) {
      return;
    }

    const target = resolveMockTarget(mockForm.platform);

    try {
      await overlaysApi.sendMockMessage(params.id, {
        platform: target.platform,
        channel_id: target.channel_id,
        channel_name: target.channel_name,
        text: mockForm.message,
        username:
          mockForm.username ||
          mockForm.displayName.toLowerCase().replace(/\s+/g, '') ||
          'mockuser',
        display_name: mockForm.displayName || mockForm.username || 'Mock Viewer',
        avatar_url: mockForm.avatarUrl || undefined,
        color: mockForm.color || undefined,
        metadata: { mock: true, source: 'preview-form' }
      });

      setMockForm((prev) => ({
        ...prev,
        message: ''
      }));
    } catch (error) {
      console.error('[Preview] Failed to send mock message:', error);
      alert('Failed to send mock message. Check console for details.');
    }
  };

  const handleAddSampleTranscript = async () => {
    for (const [index, sample] of SAMPLE_MOCK_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform);

      try {
        await overlaysApi.sendMockMessage(params.id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          metadata: {
            ...(sample.metadata || {}),
            mock: true,
            preset: true,
            order: index
          }
        });
      } catch (error) {
        console.error('[Preview] Failed to send sample message:', error);
        alert('Failed to send sample messages. Check console for details.');
        break;
      }
    }
  };

  const handleAddSampleEvents = async () => {
    for (const [index, sample] of SAMPLE_EVENT_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform);

      try {
        await overlaysApi.sendMockMessage(params.id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          event: sample.event,
          metadata: {
            ...(sample.metadata || {}),
            mock: true,
            preset: true,
            order: index
          }
        });

        // Add delay between events so they don't all arrive at once
        await new Promise(resolve => setTimeout(resolve, 800));
      } catch (error) {
        console.error('[Preview] Failed to send sample event:', error);
        alert('Failed to send sample events. Check console for details.');
        break;
      }
    }
  };

  const handleClearMockMessages = () => {
    setMessages((prev) => prev.filter((message) => !isMockMessage(message)));
  };

  const handleSaveCustomization = async () => {
    setIsSavingConfig(true);
    setConfigAlert(null);

    try {
      await overlaysApi.updateConfig(params.id, {
        display_settings: {
          font_size: fontSize,
          message_duration: messageDuration,
          max_messages: maxMessages,
          disable_message_fade: disableMessageFade,
          platform_badge_position: platformBadgePosition,
          platform_badge_style: platformBadgeStyle,
          show_platform_badge: showPlatformBadge
        },
        custom_css: useCustomCss ? customCss : ''
      });

      setConfigAlert({ type: 'success', message: 'Customization saved!' });
    } catch (error) {
      console.error('Failed to save overlay config', error);
      setConfigAlert({ type: 'error', message: 'Failed to save customization' });
    } finally {
      setIsSavingConfig(false);
      setTimeout(() => setConfigAlert(null), 5000);
    }
  };

  return (
    <div className="min-h-screen bg-gray-900">
      {useCustomCss && scopedPreviewCss && (
        <style
          key={scopedPreviewCss}
          id="overlay-preview-custom-css"
          dangerouslySetInnerHTML={{ __html: scopedPreviewCss }}
        />
      )}
      {/* Header */}
      <div className="bg-gray-800 border-b border-gray-700 px-4 py-3">
        <div className="container mx-auto flex justify-between items-center">
          <div className="flex items-center gap-4">
            <button
              onClick={() => router.push(`/overlays/${params.id}`)}
              className="text-gray-400 hover:text-white transition-colors"
            >
              ← Back
            </button>
            <h1 className="text-xl font-semibold text-white">Overlay Preview</h1>
            <span
              className={`px-2 py-1 rounded text-xs ${
                connected ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
              }`}
            >
              {connected ? '● Connected' : '● Disconnected'}
            </span>
          </div>

          <button
            onClick={copyOverlayUrl}
            className="bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-4 rounded-lg transition-colors text-sm"
          >
            📋 Copy OBS URL
          </button>
        </div>
      </div>

      <div className="container mx-auto px-4 py-6">
        {/* Top Row: Preview and Customization */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-6">
          {/* Preview Area (Main) */}
          <div className="lg:col-span-2">
            <div
              id="overlay-preview-root"
              className={`overlay-preview-root bg-black rounded-lg p-4 h-[800px] overflow-hidden relative ${
                useCustomCss ? 'overlay-preview' : ''
              }`}
            >
              <div
                className="overlay-preview-body h-full overflow-y-auto space-y-3"
                style={{
                  scrollbarWidth: 'thin',
                  scrollbarColor: '#374151 transparent'
                }}
              >
                {messages.length === 0 ? (
                  <div className="flex items-center justify-center h-full">
                    <div className="text-center text-gray-600">
                      <svg
                        className="w-16 h-16 mx-auto mb-4"
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
                      <p className="text-lg font-medium mb-2">Waiting for messages...</p>
                      <p className="text-sm">Messages will appear here when chat is active</p>
                    </div>
                  </div>
                ) : (
                  <>
                    {messages.map((message) => {
                      const isEvent = message.event != null;
                      const eventTierClass = isEvent ? `event-tier-${message.event?.tier}` : '';
                      const eventTypeClass = isEvent ? `event-type-${message.event?.type}` : '';

                      return (
                      <div
                        key={message.id}
                        data-platform={message.platform}
                        data-event-type={isEvent ? message.event?.type : undefined}
                        className={
                          isEvent
                            ? `event-message ${eventTierClass} ${eventTypeClass}`
                            : 'backdrop-blur-sm rounded-lg p-3 shadow-lg bg-gray-900/90'
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
                                className="w-10 h-10 rounded-full object-cover"
                                onError={(e) => {
                                  e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(
                                    message.user.display_name
                                  )}&background=6b7280&color=fff&size=40`;
                                }}
                              />
                            ) : (
                              <div className="w-10 h-10 rounded-full bg-gray-700 flex items-center justify-center text-white font-semibold">
                                {message.user.display_name?.slice(0, 2).toUpperCase() || '?'}
                              </div>
                            )}
                          </div>

                          {/* Message Content */}
                          <div className="flex-1 min-w-0">
                            {/* User Info */}
                            <div className="flex items-center gap-2 mb-1 flex-wrap">
                              {/* Platform badge before username */}
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

                              {/* Badges before username when position is 'before' */}
                              {platformBadgePosition === 'before' && message.user.badges && message.user.badges.length > 0 && (
                                <div className="flex gap-1">
                                  {message.user.badges.map((badge, index) => (
                                    <Image
                                      key={`${badge.name}-${index}`}
                                      src={badge.icon_url}
                                      alt={badge.name}
                                      title={`${badge.name} (${badge.version})`}
                                      width={16}
                                      height={16}
                                      className="w-4 h-4 object-contain"
                                      onError={(e) => {
                                        e.currentTarget.style.display = 'none';
                                      }}
                                    />
                                  ))}
                                </div>
                              )}

                              {/* Username */}
                              <span
                                className="font-semibold text-sm"
                                style={{
                                  color: message.user.color || '#FFFFFF'
                                }}
                              >
                                {message.user.display_name}
                              </span>

                              {/* Platform badge after username */}
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

                              {/* Badges after username when position is 'after' */}
                              {platformBadgePosition === 'after' && message.user.badges && message.user.badges.length > 0 && (
                                <div className="flex gap-1">
                                  {message.user.badges.map((badge, index) => (
                                    <Image
                                      key={`${badge.name}-${index}`}
                                      src={badge.icon_url}
                                      alt={badge.name}
                                      title={`${badge.name} (${badge.version})`}
                                      width={16}
                                      height={16}
                                      className="w-4 h-4 object-contain"
                                      onError={(e) => {
                                        e.currentTarget.style.display = 'none';
                                      }}
                                    />
                                  ))}
                                </div>
                              )}
                            </div>

                            {/* Message Text or Event Content */}
                            <div
                              className="text-white break-words"
                              style={{ fontSize: `${fontSize}px` }}
                            >
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
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Customization Panel (Sidebar) */}
          <div className="lg:col-span-1">
            <div className="bg-gray-800 rounded-lg border border-gray-700 h-[800px] overflow-y-auto flex flex-col">
              <div className="p-6 flex-shrink-0">
                <h2 className="text-lg font-semibold text-white mb-6">Customization</h2>
              </div>
              <div className="px-6 pb-6 flex-1 overflow-y-auto"
                style={{
                  scrollbarWidth: 'thin',
                  scrollbarColor: '#374151 transparent'
                }}
              >

              <div className="space-y-6">
                {/* Font Size */}
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Font Size: <span className="text-twitch">{fontSize}px</span>
                  </label>
                  <input
                    type="range"
                    min="12"
                    max="32"
                    value={fontSize}
                    onChange={(e) => setFontSize(parseInt(e.target.value))}
                    className="w-full accent-twitch"
                  />
                </div>

                {/* Max Messages */}
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Max Messages: <span className="text-twitch">{maxMessages}</span>
                  </label>
                  <input
                    type="range"
                    min="10"
                    max="100"
                    value={maxMessages}
                    onChange={(e) => setMaxMessages(parseInt(e.target.value))}
                    className="w-full accent-twitch"
                  />
                </div>

                {/* Message Duration */}
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">
                    Message Duration: <span className="text-twitch">{messageDuration}s</span>
                  </label>
                  <input
                    type="range"
                    min="5"
                    max="60"
                    value={messageDuration}
                    onChange={(e) => setMessageDuration(parseInt(e.target.value))}
                    className="w-full accent-twitch"
                    disabled={disableMessageFade}
                  />
                </div>

                {/* Disable Message Fade Toggle */}
                <div>
                  <label className="flex items-center gap-2 text-sm text-gray-300">
                    <input
                      type="checkbox"
                      checked={disableMessageFade}
                      onChange={(e) => setDisableMessageFade(e.target.checked)}
                      className="accent-twitch"
                    />
                    Disable Message Fade Out
                  </label>
                  <p className="text-xs text-gray-400 mt-1 ml-6">
                    When enabled, messages will not automatically fade out and will remain visible until max messages is reached
                  </p>
                </div>

                {/* Platform Badge Settings */}
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-3">
                    Platform Badge
                  </label>
                  <div className="space-y-3">
                    <div>
                      <label className="flex items-center gap-2 text-sm text-gray-300 mb-3">
                        <input
                          type="checkbox"
                          checked={showPlatformBadge}
                          onChange={(e) => setShowPlatformBadge(e.target.checked)}
                          className="accent-twitch"
                        />
                        Show Platform Badge
                      </label>
                    </div>
                    <div className={!showPlatformBadge ? 'opacity-50 pointer-events-none' : ''}>
                      <label className="block text-xs text-gray-400 mb-2">Position</label>
                      <div className="flex gap-3">
                        <label className="flex items-center gap-2 text-gray-300 cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgePosition"
                            value="before"
                            checked={platformBadgePosition === 'before'}
                            onChange={(e) => setPlatformBadgePosition(e.target.value as 'before')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Before username
                        </label>
                        <label className="flex items-center gap-2 text-gray-300 cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgePosition"
                            value="after"
                            checked={platformBadgePosition === 'after'}
                            onChange={(e) => setPlatformBadgePosition(e.target.value as 'after')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          After username
                        </label>
                      </div>
                    </div>
                    <div className={!showPlatformBadge ? 'opacity-50 pointer-events-none' : ''}>
                      <label className="block text-xs text-gray-400 mb-2">Style</label>
                      <div className="flex gap-3">
                        <label className="flex items-center gap-2 text-gray-300 cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgeStyle"
                            value="text"
                            checked={platformBadgeStyle === 'text'}
                            onChange={(e) => setPlatformBadgeStyle(e.target.value as 'text')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Text (TWITCH)
                        </label>
                        <label className="flex items-center gap-2 text-gray-300 cursor-pointer">
                          <input
                            type="radio"
                            name="platformBadgeStyle"
                            value="icon"
                            checked={platformBadgeStyle === 'icon'}
                            onChange={(e) => setPlatformBadgeStyle(e.target.value as 'icon')}
                            className="accent-twitch"
                            disabled={!showPlatformBadge}
                          />
                          Icon (logo)
                        </label>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Emote Providers */}
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-3">
                    Emote Providers
                  </label>
                  <div className="space-y-2">
                    <label className="flex items-center gap-2 text-gray-300">
                      <input type="checkbox" defaultChecked className="accent-twitch" />
                      7TV
                    </label>
                    <label className="flex items-center gap-2 text-gray-300">
                      <input type="checkbox" defaultChecked className="accent-twitch" />
                      BetterTTV
                    </label>
                    <label className="flex items-center gap-2 text-gray-300">
                      <input type="checkbox" defaultChecked className="accent-twitch" />
                      FrankerFaceZ
                    </label>
                  </div>
                </div>

                {/* Mock Messages */}
                <div className="border border-gray-700 rounded-lg p-4 bg-gray-900/40">
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-sm font-semibold text-white">Mock Messages</h3>
                    <button
                      type="button"
                      onClick={handleClearMockMessages}
                      className="text-xs text-gray-400 hover:text-white"
                    >
                      Clear
                    </button>
                  </div>
                  <div className="space-y-3">
                    <div>
                      <label className="block text-xs text-gray-400 mb-1">Platform</label>
                      <select
                        value={mockForm.platform}
                        onChange={(e) => handleMockInputChange('platform', e.target.value as MockMessageFormState['platform'])}
                        className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm text-white"
                      >
                        <option value="twitch">Twitch</option>
                        <option value="youtube">YouTube</option>
                        <option value="kick">Kick</option>
                        <option value="tiktok">TikTok</option>
                      </select>
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="block text-xs text-gray-400 mb-1">Display Name</label>
                        <input
                          type="text"
                          value={mockForm.displayName}
                          onChange={(e) => handleMockInputChange('displayName', e.target.value)}
                          className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-gray-400 mb-1">Username</label>
                        <input
                          type="text"
                          value={mockForm.username}
                          onChange={(e) => handleMockInputChange('username', e.target.value)}
                          className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm text-white"
                        />
                      </div>
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="block text-xs text-gray-400 mb-1">Avatar URL (optional)</label>
                        <input
                          type="text"
                          value={mockForm.avatarUrl}
                          onChange={(e) => handleMockInputChange('avatarUrl', e.target.value)}
                          className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm text-white"
                          placeholder="https://..."
                        />
                      </div>
                      <div>
                        <label className="block text-xs text-gray-400 mb-1">Name Color</label>
                        <input
                          type="color"
                          value={mockForm.color}
                          onChange={(e) => handleMockInputChange('color', e.target.value)}
                          className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm"
                        />
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs text-gray-400 mb-1">Message</label>
                      <textarea
                        value={mockForm.message}
                        onChange={(e) => handleMockInputChange('message', e.target.value)}
                        className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-sm text-white h-20"
                        placeholder="Type something fun..."
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <button
                        type="button"
                        onClick={() => void handleAddMockMessage()}
                        className="w-full bg-twitch hover:bg-purple-700 text-white text-sm font-semibold py-2 rounded-lg transition-colors disabled:opacity-60"
                        disabled={!mockForm.message.trim()}
                      >
                        Inject Message
                      </button>
                      <div className="flex gap-2">
                        <button
                          type="button"
                          onClick={() => void handleAddSampleTranscript()}
                          className="flex-1 px-3 py-2 text-xs border border-gray-600 rounded-lg text-gray-200 hover:bg-gray-700"
                        >
                          💬 Sample Chat
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleAddSampleEvents()}
                          className="flex-1 px-3 py-2 text-xs border border-yellow-600 rounded-lg text-yellow-200 hover:bg-yellow-900/30"
                        >
                          ⭐ Sample Events
                        </button>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Save Button */}
                <div className="space-y-3">
                  <button
                    onClick={handleSaveCustomization}
                    disabled={!configLoaded || isSavingConfig}
                    className="w-full bg-twitch hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold py-2 px-4 rounded-lg transition-colors"
                  >
                    {isSavingConfig ? 'Saving...' : 'Save Configuration'}
                  </button>
                  {configAlert && (
                    <p
                      className={`text-sm ${
                        configAlert.type === 'success' ? 'text-green-400' : 'text-red-400'
                      }`}
                    >
                      {configAlert.message}
                    </p>
                  )}
                </div>

                {/* Stats */}
                <div className="mt-6 pt-6 border-t border-gray-700">
                  <h3 className="text-sm font-medium text-gray-400 mb-3">Statistics</h3>
                  <div className="space-y-2 text-sm">
                    <div className="flex justify-between text-gray-300">
                      <span>Messages:</span>
                      <span className="text-white font-medium">{messages.length}</span>
                    </div>
                    <div className="flex justify-between text-gray-300">
                      <span>Status:</span>
                      <span className={connected ? 'text-green-400' : 'text-red-400'}>
                        {connected ? 'Connected' : 'Disconnected'}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
              </div>
            </div>
          </div>
        </div>

        {/* Bottom Row: Full-Width CSS Editor */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <h2 className="text-lg font-semibold text-white">Custom CSS Editor</h2>
              <label className="flex items-center gap-2 text-sm text-gray-300">
                <input
                  type="checkbox"
                  checked={useCustomCss}
                  onChange={(e) => setUseCustomCss(e.target.checked)}
                  className="accent-twitch"
                />
                Enable Custom CSS
              </label>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => setShowThemeMarketplace(true)}
                className="bg-purple-600 hover:bg-purple-700 text-white text-sm font-semibold px-4 py-2 rounded-lg transition-colors flex items-center gap-2"
              >
                <svg
                  className="w-4 h-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01"
                  />
                </svg>
                Browse Themes
              </button>
              <button
                type="button"
                onClick={() => {
                  setCustomCss('');
                  setUseCustomCss(false);
                }}
                className="px-4 py-2 text-sm border border-gray-600 rounded-lg text-gray-200 hover:bg-gray-700 transition-colors"
              >
                Reset
              </button>
            </div>
          </div>

          <MonacoCSSEditor
            value={customCss}
            onChange={setCustomCss}
            height="400px"
            placeholder="/* Enter your custom CSS here */"
          />

          <p className="text-sm text-gray-400 mt-4">
            Need inspiration? Explore{' '}
            <a
              href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes"
              target="_blank"
              rel="noreferrer"
              className="text-twitch hover:underline"
            >
              theme docs
            </a>{' '}
            or paste your OBS CSS to preview in real time.
          </p>
        </div>
      </div>

      {/* Theme Marketplace Modal */}
      <ThemeMarketplaceModal
        isOpen={showThemeMarketplace}
        onClose={() => setShowThemeMarketplace(false)}
        onApplyTheme={(css) => {
          setCustomCss(css);
          setUseCustomCss(true);
          setShowThemeMarketplace(false);
        }}
      />
    </div>
  );
}
