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
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';
import { sortMessageBadges } from '@/lib/badgeOrder';

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

const EXAMPLE_CUSTOM_CSS = `/* Example neon glass theme */
body {
  background: transparent !important;
  font-family: 'Space Grotesk', sans-serif !important;
}

.space-y-3 > div {
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
  const [mockForm, setMockForm] = useState<MockMessageFormState>(DEFAULT_MOCK_FORM);
  const [customCss, setCustomCss] = useState('');
  const [useCustomCss, setUseCustomCss] = useState(false);
  const [configLoaded, setConfigLoaded] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [configAlert, setConfigAlert] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

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

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const copyOverlayUrl = () => {
    const url = `${window.location.origin}/overlay/${params.id}`;
    navigator.clipboard.writeText(url);
    alert('Overlay URL copied to clipboard!\n\nAdd this as a Browser Source in OBS.');
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

  const handleMockInputChange = <K extends keyof MockMessageFormState>(field: K, value: MockMessageFormState[K]) => {
    setMockForm((prev) => ({
      ...prev,
      [field]: value
    }));
  };

  const handleAddMockMessage = () => {
    if (!mockForm.message.trim()) {
      return;
    }

    const now = new Date().toISOString();
    const mockMessage: ChatMessage = {
      id: `mock-${Date.now()}`,
      overlay_id: params.id,
      platform: mockForm.platform,
      channel_id: 'mock-channel',
      channel_name: 'Mock Channel',
      user: {
        id: 'mock-user',
        username: mockForm.username || mockForm.displayName.toLowerCase().replace(/\s+/g, ''),
        display_name: mockForm.displayName || mockForm.username || 'Mock Viewer',
        avatar_url: mockForm.avatarUrl || undefined,
        badges: [],
        color: mockForm.color || undefined
      },
      message: {
        text: mockForm.message,
        emotes: []
      },
      timestamp: now,
      metadata: { mock: true }
    };

    setMessages((prev) => [...prev, mockMessage].slice(-maxMessages));
    setMockForm((prev) => ({
      ...prev,
      message: ''
    }));
  };

  const handleAddSampleTranscript = () => {
    const seeded = SAMPLE_MOCK_MESSAGES.map((sample, index) => ({
      ...sample,
      overlay_id: params.id,
      id: `mock-seed-${Date.now()}-${index}`,
      timestamp: new Date(Date.now() + index * 500).toISOString(),
      metadata: { ...sample.metadata, mock: true }
    }));

    setMessages((prev) => [...prev, ...seeded].slice(-maxMessages));
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
          max_messages: maxMessages
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
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Preview Area (Main) */}
          <div className="lg:col-span-2">
            <div
              id="overlay-preview-root"
              className={`overlay-preview-root bg-black rounded-lg p-4 h-[700px] overflow-hidden relative ${
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
                    {messages.map((message) => (
                      <div
                        key={message.id}
                        className="flex gap-3 p-3 bg-gray-900/50 rounded-lg hover:bg-gray-900/80 transition-colors"
                      >
                        {/* Avatar */}
                        <Image
                          src={
                            message.user.avatar_url ||
                            `https://ui-avatars.com/api/?name=${encodeURIComponent(
                              message.user.display_name
                            )}&background=6b7280&color=fff&size=40`
                          }
                          alt={message.user.display_name}
                          width={40}
                          height={40}
                          className="w-10 h-10 rounded-full flex-shrink-0 object-cover"
                          onError={(e) => {
                            e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(
                              message.user.display_name
                            )}&background=6b7280&color=fff&size=40`;
                          }}
                        />

                        {/* Message Content */}
                        <div className="flex-1 min-w-0">
                          {/* User Info */}
                          <div className="flex items-center gap-2 mb-1 flex-wrap">
                            <span
                              className={`${getPlatformColor(message.platform)} text-xs font-bold`}
                            >
                              {message.platform.toUpperCase()}
                            </span>
                            <span
                              className="font-semibold text-white"
                              style={{
                                color: message.user.color || undefined
                              }}
                            >
                              {message.user.display_name}
                            </span>
                            {message.user.badges?.map((badge, index) => (
                              <Image
                                key={`${badge.name}-${index}`}
                                src={badge.icon_url}
                                alt={badge.name}
                                title={`${badge.name} (${badge.version})`}
                                width={16}
                                height={16}
                                className="w-4 h-4 inline-block object-contain"
                                onError={(e) => {
                                  e.currentTarget.style.display = 'none';
                                }}
                              />
                            ))}
                          </div>

                          {/* Message Text */}
                          <p
                            className="text-gray-200 break-words"
                            style={{ fontSize: `${fontSize}px` }}
                          >
                            {renderMessageContent(message)}
                          </p>

                          {/* Timestamp */}
                          <span className="text-xs text-gray-600 mt-1 block">
                            {new Date(message.timestamp).toLocaleTimeString()}
                          </span>
                        </div>
                      </div>
                    ))}
                    <div ref={messagesEndRef} />
                  </>
                )}
              </div>
            </div>
          </div>

          {/* Customization Panel (Sidebar) */}
          <div className="lg:col-span-1">
            <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 sticky top-6">
              <h2 className="text-lg font-semibold text-white mb-6">Customization</h2>

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
                  />
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
                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={handleAddMockMessage}
                        className="flex-1 bg-twitch hover:bg-purple-700 text-white text-sm font-semibold py-2 rounded-lg transition-colors disabled:opacity-60"
                        disabled={!mockForm.message.trim()}
                      >
                        Inject Message
                      </button>
                      <button
                        type="button"
                        onClick={handleAddSampleTranscript}
                        className="px-3 py-2 text-xs border border-gray-600 rounded-lg text-gray-200 hover:bg-gray-700"
                      >
                        Sample Set
                      </button>
                    </div>
                  </div>
                </div>

                {/* Custom CSS */}
                <div className="border border-gray-700 rounded-lg p-4 bg-gray-900/40">
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="text-sm font-semibold text-white">Custom CSS</h3>
                    <label className="flex items-center gap-2 text-xs text-gray-300">
                      <input
                        type="checkbox"
                        checked={useCustomCss}
                        onChange={(e) => setUseCustomCss(e.target.checked)}
                        className="accent-twitch"
                      />
                      Enable
                    </label>
                  </div>
                  <textarea
                    value={customCss}
                    onChange={(e) => setCustomCss(e.target.value)}
                    className="w-full bg-gray-900 border border-gray-700 rounded px-2 py-2 text-xs text-gray-100 h-32"
                    placeholder="Paste your OBS custom CSS here to preview it..."
                  />
                  <div className="flex gap-2 mt-2">
                    <button
                      type="button"
                      onClick={() => {
                        setCustomCss(EXAMPLE_CUSTOM_CSS.trim());
                        setUseCustomCss(true);
                      }}
                      className="flex-1 bg-gray-700 hover:bg-gray-600 text-white text-xs font-semibold py-2 rounded"
                    >
                      Load Example
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setCustomCss('');
                        setUseCustomCss(false);
                      }}
                      className="px-3 py-2 text-xs border border-gray-600 rounded text-gray-200 hover:bg-gray-700"
                    >
                      Reset
                    </button>
                  </div>
                  <p className="text-[11px] text-gray-500 mt-3">
                    Need inspiration? Explore <a href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes" target="_blank" rel="noreferrer" className="text-twitch hover:underline">theme docs</a> or paste your OBS CSS to preview in real time.
                  </p>
                </div>

                {/* Save Button */}
                <div className="space-y-3">
                  <button
                    onClick={handleSaveCustomization}
                    disabled={!configLoaded || isSavingConfig}
                    className="w-full bg-twitch hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold py-2 px-4 rounded-lg transition-colors mt-6"
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
              </div>

              {/* Stats */}
              <div className="mt-8 pt-6 border-t border-gray-700">
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
  );
}
