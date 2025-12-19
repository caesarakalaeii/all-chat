/**
 * Viewer Chat Page
 *
 * Allows viewers to send messages to a streamer's chat through All-Chat.
 * Viewers authenticate with their own platform account (Twitch initially).
 *
 * Features:
 * - View aggregated chat from all streamer platforms
 * - Login with viewer's Twitch account
 * - Send messages to streamer's Twitch chat
 * - Rate limiting (20/min, 100/hour)
 * - Display streamer's active platforms
 *
 * Route: /chat/[streamer]
 */

'use client';

import { useEffect, useState, useRef } from 'react';
import { useParams } from 'next/navigation';
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store';
import { viewerApi } from '@/lib/api/viewer';
import { apiClient } from '@/lib/api/client';
import type { StreamerInfo, SendMessageRequest } from '@/lib/types/viewer';
import type { ChatMessage } from '@/lib/types/message';

export default function ViewerChatPage() {
  const params = useParams();
  const streamerUsername = params.streamer as string;

  const { viewerInfo, viewerToken, loading, setStreamer, viewerLogout } = useViewerAuthStore();

  const [streamerInfo, setStreamerInfo] = useState<StreamerInfo | null>(null);
  const [loadingStreamer, setLoadingStreamer] = useState(true);
  const [message, setMessage] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([]);
  const [overlayId, setOverlayId] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  // Initialize viewer auth store
  useEffect(() => {
    useViewerAuthStore.getState().init();
  }, []);

  // Set current streamer
  useEffect(() => {
    if (streamerUsername) {
      setStreamer(streamerUsername);
      if (typeof window !== 'undefined') {
        localStorage.setItem('viewer_streamer', streamerUsername);
      }
    }
  }, [streamerUsername, setStreamer]);

  // Fetch streamer info and overlay ID
  useEffect(() => {
    async function fetchStreamerInfo() {
      try {
        setLoadingStreamer(true);
        const info = await viewerApi.getStreamerInfo(streamerUsername);
        setStreamerInfo(info);

        // Set overlay ID from streamer info response for WebSocket connection
        if (info.overlay_id) {
          setOverlayId(info.overlay_id);
        }
      } catch (err) {
        console.error('Failed to fetch streamer info:', err);
        setError('Streamer not found or has no active platforms');
      } finally {
        setLoadingStreamer(false);
      }
    }

    if (streamerUsername) {
      fetchStreamerInfo();
    }
  }, [streamerUsername]);

  // WebSocket connection for live chat display (no auth required)
  useEffect(() => {
    if (!overlayId) return;

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/overlay/${overlayId}`;
    console.log('[Viewer Chat] Connecting to:', wsUrl);

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('[Viewer Chat] WebSocket connected');
    };

    ws.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data);
        if (envelope.type === 'chat_message' && envelope.data) {
          const message: ChatMessage = envelope.data;
          setChatMessages((prev) => {
            // Prevent duplicate messages (check if message ID already exists)
            if (message.id && prev.some(m => m.id === message.id)) {
              return prev;
            }
            return [...prev, message].slice(-100); // Keep last 100 messages
          });
        }
      } catch (error) {
        console.error('[Viewer Chat] Failed to parse message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('[Viewer Chat] WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('[Viewer Chat] Disconnected, will reconnect...');
    };

    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
  }, [overlayId]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);

  const handleLogin = async () => {
    try {
      setError(null);
      const authUrl = await viewerApi.getLoginUrl('twitch', streamerUsername);
      // Redirect to Twitch OAuth
      window.location.href = authUrl;
    } catch (err) {
      console.error('Login failed:', err);
      setError('Failed to initiate login. Please try again.');
    }
  };

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!message.trim() || !viewerToken) return;

    try {
      setSending(true);
      setError(null);
      setSuccess(null);

      const request: SendMessageRequest = {
        streamer_username: streamerUsername,
        message: message.trim(),
        platform: 'twitch'
      };

      const response = await viewerApi.sendMessage(request);

      if (response.success) {
        setSuccess('Message sent successfully!');
        setMessage('');
        // Clear success message after 3 seconds
        setTimeout(() => setSuccess(null), 3000);
      }
    } catch (err: any) {
      console.error('Failed to send message:', err);
      if (err.message.includes('rate limit')) {
        setError('Rate limit exceeded. Please wait before sending more messages.');
      } else {
        setError(err.message || 'Failed to send message. Please try again.');
      }
    } finally {
      setSending(false);
    }
  };

  if (loading || loadingStreamer) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="text-white text-xl">Loading...</div>
      </div>
    );
  }

  if (error && !streamerInfo) {
    return (
      <div className="min-h-screen bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <div className="text-red-500 text-xl mb-4">{error}</div>
          <a href="/" className="text-blue-400 hover:text-blue-300">
            Return to Home
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-900">
      {/* Header */}
      <nav className="bg-gray-800 border-b border-gray-700">
        <div className="container mx-auto px-4 py-4 flex justify-between items-center">
          <a href="/" className="text-2xl font-bold text-white">
            All-Chat
          </a>
          <div className="flex items-center gap-4">
            {viewerInfo ? (
              <>
                <span className="text-gray-400">
                  Logged in as <span className="text-white font-semibold">{viewerInfo.username}</span>
                </span>
                <button
                  onClick={viewerLogout}
                  className="text-gray-400 hover:text-white transition-colors"
                >
                  Logout
                </button>
              </>
            ) : (
              <button
                onClick={handleLogin}
                className="bg-purple-600 hover:bg-purple-700 text-white px-4 py-2 rounded-lg transition-colors"
              >
                Login with Twitch
              </button>
            )}
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        {/* Streamer Info */}
        <div className="bg-gray-800 rounded-lg p-6 mb-6">
          <h1 className="text-3xl font-bold text-white mb-4">
            Chat with {streamerInfo?.display_name || streamerUsername}
          </h1>

          {streamerInfo && streamerInfo.platforms.length > 0 ? (
            <div className="mb-4">
              <h2 className="text-lg font-semibold text-gray-300 mb-2">Active Platforms:</h2>
              <div className="flex gap-3">
                {streamerInfo.platforms.map((platform) => (
                  <div
                    key={platform.platform}
                    className="bg-gray-700 px-4 py-2 rounded-lg flex items-center gap-2"
                  >
                    <span className="text-white font-medium capitalize">{platform.platform}</span>
                    <span className="text-gray-400">•</span>
                    <span className="text-gray-400">{platform.channel_name}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-gray-400">
              No active platforms found for this streamer.
            </div>
          )}
        </div>

        {/* Live Chat Display */}
        {overlayId && (
          <div className="bg-gray-800 rounded-lg p-6 mb-6">
            <h2 className="text-xl font-semibold text-white mb-4">Live Chat</h2>
            <div className="bg-gray-900 rounded-lg p-4 h-96 overflow-y-auto">
              {chatMessages.length === 0 ? (
                <div className="text-gray-500 text-center py-8">
                  No messages yet. Chat will appear here when streamer is live.
                </div>
              ) : (
                <div className="space-y-3">
                  {chatMessages.map((msg) => (
                    <div key={msg.id || `${msg.timestamp}-${msg.user.username}`} className="flex gap-3">
                      <div className="flex-shrink-0">
                        {msg.user.avatar_url && (
                          <img
                            src={msg.user.avatar_url}
                            alt={msg.user.username}
                            className="w-8 h-8 rounded-full"
                          />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-baseline gap-2">
                          <span
                            className="font-semibold"
                            style={{ color: msg.user.color || '#FFFFFF' }}
                          >
                            {msg.user.display_name || msg.user.username}
                          </span>
                          <span className="text-xs text-gray-500 uppercase">
                            {msg.platform}
                          </span>
                        </div>
                        <div className="text-gray-200 break-words">
                          {msg.message.text}
                        </div>
                      </div>
                    </div>
                  ))}
                  <div ref={messagesEndRef} />
                </div>
              )}
            </div>
          </div>
        )}

        {/* Message Input Section */}
        {viewerInfo ? (
          <div className="bg-gray-800 rounded-lg p-6">
            <h2 className="text-xl font-semibold text-white mb-4">Send a Message</h2>

            {error && (
              <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-lg mb-4">
                {error}
              </div>
            )}

            {success && (
              <div className="bg-green-900/50 border border-green-500 text-green-200 px-4 py-3 rounded-lg mb-4">
                {success}
              </div>
            )}

            <form onSubmit={handleSendMessage} className="space-y-4">
              <div>
                <textarea
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  placeholder="Type your message here..."
                  className="w-full bg-gray-700 text-white rounded-lg px-4 py-3 focus:outline-none focus:ring-2 focus:ring-purple-500 resize-none"
                  rows={4}
                  maxLength={500}
                  disabled={sending}
                />
                <div className="text-right text-gray-400 text-sm mt-1">
                  {message.length}/500 characters
                </div>
              </div>

              <button
                type="submit"
                disabled={!message.trim() || sending}
                className="bg-purple-600 hover:bg-purple-700 disabled:bg-gray-600 disabled:cursor-not-allowed text-white px-6 py-3 rounded-lg font-semibold transition-colors"
              >
                {sending ? 'Sending...' : 'Send Message'}
              </button>
            </form>

            <div className="mt-6 text-gray-400 text-sm">
              <p className="font-semibold mb-2">Rate Limits:</p>
              <ul className="list-disc list-inside space-y-1">
                <li>20 messages per minute</li>
                <li>100 messages per hour</li>
              </ul>
            </div>
          </div>
        ) : (
          <div className="bg-gray-800 rounded-lg p-6 text-center">
            <p className="text-gray-300 mb-4">
              Please log in with your Twitch account to send messages
            </p>
            <button
              onClick={handleLogin}
              className="bg-purple-600 hover:bg-purple-700 text-white px-6 py-3 rounded-lg font-semibold transition-colors inline-block"
            >
              Login with Twitch
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
