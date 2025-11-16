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

import { useEffect, useState, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { WebSocketClient } from '@/lib/api/websocket';
import type { ChatMessage } from '@/lib/types/message';
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';

export default function OverlayPreviewPage({ params }: { params: { id: string } }) {
  const router = useRouter();
  const { token } = useAuthStore();

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [connected, setConnected] = useState(false);
  const [maxMessages, setMaxMessages] = useState(50);

  const wsClientRef = useRef<WebSocketClient | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

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
      const message = await resolveTwitchBadgeIcons(incoming);
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

  return (
    <div className="min-h-screen bg-gray-900">
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
            <div className="bg-black rounded-lg p-4 h-[700px] overflow-hidden relative">
              <div
                className="h-full overflow-y-auto space-y-3"
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
                        <img
                          src={message.user.avatar_url || `https://ui-avatars.com/api/?name=${encodeURIComponent(message.user.display_name)}&background=6b7280&color=fff&size=40`}
                          alt={message.user.display_name}
                          className="w-10 h-10 rounded-full flex-shrink-0"
                          onError={(e) => {
                            // Fallback to generated avatar if image fails to load
                            e.currentTarget.src = `https://ui-avatars.com/api/?name=${encodeURIComponent(message.user.display_name)}&background=6b7280&color=fff&size=40`;
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
                              <img
                                key={`${badge.name}-${index}`}
                                src={badge.icon_url}
                                alt={badge.name}
                                title={`${badge.name} (${badge.version})`}
                                className="w-4 h-4 inline-block"
                                onError={(e) => {
                                  // Fallback to text badge if icon fails to load
                                  e.currentTarget.style.display = 'none';
                                }}
                              />
                            ))}
                          </div>

                          {/* Message Text */}
                          <p className="text-gray-200 break-words">{renderMessageContent(message)}</p>

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
                    Font Size: <span className="text-twitch">16px</span>
                  </label>
                  <input
                    type="range"
                    min="12"
                    max="32"
                    defaultValue="16"
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
                    Message Duration: <span className="text-twitch">15s</span>
                  </label>
                  <input
                    type="range"
                    min="5"
                    max="60"
                    defaultValue="15"
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

                {/* Save Button */}
                <button className="w-full bg-twitch hover:bg-purple-700 text-white font-semibold py-2 px-4 rounded-lg transition-colors mt-6">
                  Save Configuration
                </button>
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
