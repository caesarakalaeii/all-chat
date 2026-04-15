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

'use client'

import { useEffect, useState, useRef } from 'react'
import { useParams } from 'next/navigation'
import Link from 'next/link'
import clsx from 'clsx'
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store'
import { useAuthStore } from '@/lib/stores/auth-store'
import { viewerApi } from '@/lib/api/viewer'
import { apiClient } from '@/lib/api/client'
import type { StreamerInfo, SendMessageRequest } from '@/lib/types/viewer'
import type { ChatMessage } from '@/lib/types/message'
import { parseApiError, parseFetchError } from '@/lib/errorParser'
import type { ChatError } from '@/lib/types/errors'
import { ChatErrorType } from '@/lib/types/errors'
import ErrorDisplay from '@/components/ErrorDisplay'

export default function ViewerChatPage() {
  const params = useParams()
  const streamerUsername = params.streamer as string

  const { viewerInfo, viewerToken, loading, setStreamer, viewerLogout } = useViewerAuthStore()
  const { user: streamerUser, token: streamerToken } = useAuthStore()

  // Detect if the logged-in streamer is viewing their own chat page
  const isOwnChat = !!(streamerUser && streamerToken &&
    streamerUser.username.toLowerCase() === streamerUsername.toLowerCase())

  const [streamerInfo, setStreamerInfo] = useState<StreamerInfo | null>(null)
  const [loadingStreamer, setLoadingStreamer] = useState(true)
  const [message, setMessage] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState<ChatError | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)
  const [chatMessages, setChatMessages] = useState<ChatMessage[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const messagesEndRef = useRef<HTMLDivElement | null>(null)

  // Initialize viewer auth store
  useEffect(() => {
    useViewerAuthStore.getState().init()
  }, [])

  // Set current streamer
  useEffect(() => {
    if (streamerUsername) {
      setStreamer(streamerUsername)
      if (typeof window !== 'undefined') {
        localStorage.setItem('viewer_streamer', streamerUsername)
      }
    }
  }, [streamerUsername, setStreamer])

  // Fetch streamer info
  useEffect(() => {
    async function fetchStreamerInfo() {
      try {
        setLoadingStreamer(true)
        const info = await viewerApi.getStreamerInfo(streamerUsername)
        setStreamerInfo(info)
      } catch (err) {
        console.error('Failed to fetch streamer info:', err)
        setLoadError('Streamer not found or has no active platforms')
      } finally {
        setLoadingStreamer(false)
      }
    }

    if (streamerUsername) {
      fetchStreamerInfo()
    }
  }, [streamerUsername])

  // WebSocket connection for live chat display (optional auth via token query param)
  useEffect(() => {
    if (!streamerUsername) return

    // Use viewer-specific WebSocket endpoint (does NOT trigger YouTube polling)
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const tokenParam = viewerToken ? `?token=${viewerToken}` : ''
    const wsUrl = `${protocol}//${window.location.host}/ws/chat/${streamerUsername}${tokenParam}`
    console.log('[Viewer Chat] Connecting to:', wsUrl)

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      console.log('[Viewer Chat] WebSocket connected')
    }

    ws.onmessage = (event) => {
      try {
        const envelope = JSON.parse(event.data)
        if (envelope.type === 'chat_message' && envelope.data) {
          const message: ChatMessage = envelope.data
          setChatMessages((prev) => {
            // Prevent duplicate messages (check if message ID already exists)
            if (message.id && prev.some((m) => m.id === message.id)) {
              return prev
            }
            return [...prev, message].slice(-100) // Keep last 100 messages
          })
        }
      } catch (error) {
        console.error('[Viewer Chat] Failed to parse message:', error)
      }
    }

    ws.onerror = (error) => {
      console.error('[Viewer Chat] WebSocket error:', error)
    }

    ws.onclose = () => {
      console.log('[Viewer Chat] Disconnected, will reconnect...')
    }

    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close()
      }
    }
  }, [streamerUsername, viewerToken])

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [chatMessages])

  const handleLogin = async (platform: 'twitch' | 'youtube') => {
    try {
      setError(null)
      const authUrl = await viewerApi.getLoginUrl(platform, streamerUsername)
      window.location.href = authUrl
    } catch (err) {
      console.error('Login failed:', err)
      setError({
        type: ChatErrorType.NETWORK_ERROR,
        message: 'Failed to initiate login',
        userMessage: 'Failed to initiate login. Please check your connection and try again.',
        actionableSteps: ['Check your internet connection', 'Try again in a moment'],
      })
    }
  }

  const handleSendMessage = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!message.trim()) return

    // Must have either viewer auth or streamer auth (own chat)
    if (!viewerToken && !isOwnChat) return

    try {
      setSending(true)
      setError(null)
      setSuccess(null)

      let response
      if (isOwnChat) {
        // Streamer sending in their own chat — use streamer endpoint with their stored OAuth tokens
        const activePlatform = streamerInfo?.platforms?.find(p => p.is_active)
        response = await viewerApi.sendStreamerMessage({
          message: message.trim(),
          platform: activePlatform?.platform || streamerUser!.auth_provider || 'twitch',
        })
      } else {
        // Viewer sending — use viewer endpoint
        const request: SendMessageRequest = {
          streamer_username: streamerUsername,
          message: message.trim(),
          platform: viewerInfo?.platform || 'twitch',
        }
        response = await viewerApi.sendMessage(request)
      }

      if (response.success) {
        setSuccess('Message sent successfully!')
        setMessage('')
        setTimeout(() => setSuccess(null), 3000)
      }
    } catch (err: any) {
      console.error('Failed to send message:', err)

      let parsedError: ChatError
      if (err.response && err.data) {
        parsedError = parseApiError(err.response, err.data)
      } else {
        parsedError = parseFetchError(err)
      }

      setError(parsedError)
    } finally {
      setSending(false)
    }
  }

  if (loading || loadingStreamer) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg">
        <div className="text-xl text-text">Loading...</div>
      </div>
    )
  }

  if (loadError && !streamerInfo) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg">
        <div className="text-center">
          <div className="mb-4 text-xl text-youtube">{loadError}</div>
          <Link href="/" className="text-twitch hover:underline">
            Return to Home
          </Link>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-bg">
      {/* Header */}
      <nav className="border-b border-border bg-surface">
        <div className="container mx-auto flex items-center justify-between px-4 py-4">
          <Link href="/" className="text-2xl font-bold text-text">
            All-Chat
          </Link>
          <div className="flex items-center gap-4">
            {viewerInfo ? (
              <>
                <span className="text-text-sub">
                  Logged in as{' '}
                  <span className="font-semibold text-text">{viewerInfo.username}</span>
                </span>
                <button
                  onClick={viewerLogout}
                  className="text-text-sub transition-colors hover:text-text"
                >
                  Logout
                </button>
              </>
            ) : (
              <div className="flex gap-2">
                <button
                  onClick={() => handleLogin('twitch')}
                  className="rounded-lg bg-twitch px-4 py-2 text-white transition-colors hover:bg-twitch/80"
                >
                  Twitch
                </button>
                <button
                  onClick={() => handleLogin('youtube')}
                  className="rounded-lg bg-youtube px-4 py-2 text-white transition-colors hover:bg-youtube/80"
                >
                  YouTube
                </button>
              </div>
            )}
          </div>
        </div>
      </nav>

      {/* Main Content */}
      <div className="container mx-auto px-4 py-8">
        {/* Streamer Info */}
        <div className="mb-6 rounded-xl border border-border bg-surface p-6">
          <h1 className="mb-4 text-3xl font-bold text-text">
            Chat with {streamerInfo?.display_name || streamerUsername}
          </h1>

          {streamerInfo && streamerInfo.platforms.length > 0 ? (
            <div className="mb-4">
              <h2 className="mb-2 text-lg font-semibold text-text-sub">Active Platforms:</h2>
              <div className="flex gap-3">
                {streamerInfo.platforms.map((platform) => (
                  <div
                    key={platform.platform}
                    className="flex items-center gap-2 rounded-lg bg-surface-2 px-4 py-2"
                  >
                    <span className="font-medium text-text capitalize">{platform.platform}</span>
                    <span className="text-text-dim">&middot;</span>
                    <span className="text-text-sub">{platform.channel_name}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-text-sub">No active platforms found for this streamer.</div>
          )}
        </div>

        {/* Live Chat Display */}
        {streamerInfo && (
          <div className="mb-6 rounded-xl border border-border bg-surface p-6">
            <h2 className="mb-4 text-xl font-semibold text-text">Live Chat</h2>
            <div className="h-96 overflow-y-auto rounded-lg bg-bg p-4">
              {chatMessages.length === 0 ? (
                <div className="py-8 text-center text-text-dim">
                  No messages yet. Chat will appear here when streamer is live.
                </div>
              ) : (
                <div className="space-y-3">
                  {chatMessages.map((msg) => (
                    <div
                      key={msg.id || `${msg.timestamp}-${msg.user.username}`}
                      className="flex gap-3"
                    >
                      <div className="flex-shrink-0">
                        {msg.user.avatar_url && (
                          // eslint-disable-next-line @next/next/no-img-element
                          <img
                            src={msg.user.avatar_url}
                            alt={msg.user.username}
                            className="h-8 w-8 rounded-full"
                          />
                        )}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-baseline gap-2">
                          <span
                            className="font-semibold"
                            style={{ color: msg.user.color || '#FFFFFF' }}
                          >
                            {msg.user.display_name || msg.user.username}
                          </span>
                          <span className="text-xs text-text-dim uppercase">{msg.platform}</span>
                        </div>
                        <div className="break-words text-text">{msg.message.text}</div>
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
        {(viewerInfo || isOwnChat) ? (
          <div className="rounded-xl border border-border bg-surface p-6">
            <h2 className="mb-4 text-xl font-semibold text-text">
              {isOwnChat ? 'Send a Message (as Streamer)' : 'Send a Message'}
            </h2>

            {error && (
              <ErrorDisplay
                error={error}
                onRetry={() => {
                  // Clear error and allow retry
                  setError(null)
                }}
                onDismiss={() => setError(null)}
                className="mb-4"
              />
            )}

            {success && (
              <div className="mb-4 rounded-lg border border-kick/30 bg-kick/10 px-4 py-3 text-kick">
                {success}
              </div>
            )}

            <form onSubmit={handleSendMessage} className="space-y-4">
              <div>
                <textarea
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  placeholder="Type your message here..."
                  className="w-full resize-none rounded-lg border border-border bg-bg px-4 py-3 text-text focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                  rows={4}
                  maxLength={500}
                  disabled={sending}
                />
                <div className="mt-1 text-right text-sm text-text-dim">
                  {message.length}/500 characters
                </div>
              </div>

              <button
                type="submit"
                disabled={!message.trim() || sending}
                className="rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-colors hover:bg-twitch/80 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {sending ? 'Sending...' : 'Send Message'}
              </button>
            </form>

            {!isOwnChat && (
              <div className="mt-6 text-sm text-text-sub">
                <p className="mb-2 font-semibold text-text">Rate Limits:</p>
                <ul className="list-inside list-disc space-y-1">
                  <li>20 messages per minute</li>
                  <li>100 messages per hour</li>
                </ul>
              </div>
            )}
          </div>
        ) : (
          <div className="rounded-xl border border-border bg-surface p-6 text-center">
            <p className="mb-4 text-text-sub">Please log in to send messages</p>
            <div className="flex justify-center gap-3">
              <button
                onClick={() => handleLogin('twitch')}
                className="rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-colors hover:bg-twitch/80"
              >
                Login with Twitch
              </button>
              <button
                onClick={() => handleLogin('youtube')}
                className="rounded-lg bg-youtube px-6 py-3 font-semibold text-white transition-colors hover:bg-youtube/80"
              >
                Login with YouTube
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
