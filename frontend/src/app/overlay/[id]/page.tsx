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
import { useEffect, useState, useRef } from 'react';
import type { ChatMessage } from '@/lib/types/message';
import { renderMessageContent } from '@/lib/renderMessage';
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges';
import { sortMessageBadges } from '@/lib/badgeOrder';
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators';

export default function OBSOverlayPage({ params }: { params: { id: string } }) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [maxMessages, setMaxMessages] = useState(50);
  const [fontSize, setFontSize] = useState(16);
  const [messageDuration, setMessageDuration] = useState(15);
  const [disableMessageFade, setDisableMessageFade] = useState(false);
  const [customCss, setCustomCss] = useState('');
  const [activePlatforms, setActivePlatforms] = useState<Set<string>>(new Set());
  const [reconnectAttempts, setReconnectAttempts] = useState(0);
  const [forceReconnect, setForceReconnect] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Load overlay display configuration (public endpoint)
  useEffect(() => {
    const loadConfig = async () => {
      try {
        const response = await fetch(`/api/v1/overlays/public/${params.id}/config`);
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
  }, [params.id]);

  // Initialize WebSocket connection (no auth required)
  useEffect(() => {
    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/overlay/${params.id}`;

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

        // Only process chat messages, ignore connected/ping/pong/error
        if (envelope.type === 'chat_message' && envelope.data) {
          let message: ChatMessage = envelope.data;
          message = await resolveTwitchBadgeIcons(message);
          message = sortMessageBadges(message);

          setMessages((prev) => {
            const newMessages = [...prev, message];
            return newMessages.slice(-maxMessages);
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
  }, [params.id, maxMessages, forceReconnect]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Auto-remove old messages based on duration (if fade is enabled)
  useEffect(() => {
    if (messages.length === 0 || disableMessageFade) return;

    const timer = setTimeout(() => {
      setMessages((prev) => prev.slice(1));
    }, messageDuration * 1000);

    return () => clearTimeout(timer);
  }, [messages, messageDuration, disableMessageFade]);

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
      <PlatformStatusIndicators activePlatforms={activePlatforms} />

      <div className="space-y-3">
        {messages.map((message, index) => {
          const isSharedChat = message.metadata?.is_shared_chat === true;
          
          return (
          <div
            key={`${message.id}-${index}`}
            className={`backdrop-blur-sm rounded-lg p-3 shadow-lg animate-in slide-in-from-bottom-2 duration-300 ${
              isSharedChat 
                ? 'bg-purple-900/40 border-2 border-purple-500/50' 
                : 'bg-gray-900/90'
            }`}
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
                  <span className={`text-xs font-semibold uppercase ${getPlatformColor(message.platform)}`}>
                    {message.platform}
                  </span>
                  
                  {/* Shared Chat Indicator */}
                  {isSharedChat && (
                    <span className="text-xs font-semibold uppercase px-1.5 py-0.5 rounded bg-purple-600/80 text-purple-100 border border-purple-400/50">
                      Shared Chat
                    </span>
                  )}
                  
                  <span
                    className="font-semibold text-sm"
                    style={{ color: message.user?.color || '#FFFFFF' }}
                  >
                    {message.user?.display_name || message.user?.username}
                  </span>

                  {/* Regular Badges */}
                  {message.user?.badges && message.user.badges.length > 0 && (
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

                {/* Message Text with Emotes */}
                <div className="text-white break-words" style={{ fontSize: `${fontSize}px` }}>
                  {renderMessageContent(message)}
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
