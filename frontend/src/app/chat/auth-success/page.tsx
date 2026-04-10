/**
 * Viewer OAuth Success Page
 *
 * Handles successful OAuth authentication for viewers.
 * The backend redirects here with a JWT token as a query parameter.
 *
 * Flow:
 * 1. Extract token from URL (?token=xxx&streamer=yyy)
 * 2. Store viewer token in localStorage
 * 3. Fetch viewer info from API
 * 4. Redirect to chat page with streamer
 *
 * Route: /chat/auth-success
 */

'use client'

import { Suspense, useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store'
import { viewerApi } from '@/lib/api/viewer'

function AuthSuccessContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setViewerToken, setViewerInfo } = useViewerAuthStore()

  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const handleSuccess = async () => {
      const code = searchParams.get('code')
      const streamer = searchParams.get('streamer')

      if (!code) {
        setError('No authentication code received')
        setLoading(false)

        // Notify opener (extension) about error
        if (window.opener) {
          window.opener.postMessage(
            {
              type: 'ALLCHAT_AUTH_ERROR',
              error: 'No authentication code received',
            },
            '*'
          )
        }
        return
      }

      // Exchange short-lived code for JWT token
      let token: string
      try {
        const resp = await fetch('/api/v1/auth/viewer/token/exchange', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code }),
        })
        if (!resp.ok) {
          throw new Error(`Exchange failed: ${resp.status}`)
        }
        const data = await resp.json()
        token = data.token
      } catch (err) {
        console.error('Token exchange failed:', err)
        setError('Authentication failed. The code may have expired — please try again.')
        setLoading(false)
        return
      }

      // Check if this was opened from an extension popup
      const isExtensionPopup = window.opener && !window.opener.closed

      if (isExtensionPopup) {
        // Extension flow: post message to opener and close
        console.log('[AllChat Auth] Posting token to extension opener')
        window.opener.postMessage(
          {
            type: 'ALLCHAT_AUTH_SUCCESS',
            token,
            streamer,
          },
          '*'
        )

        // Show success message briefly before closing
        setLoading(false)
        setTimeout(() => {
          window.close()
        }, 1000)
        return
      }

      // Web app flow: store token and redirect
      setViewerToken(token)

      try {
        // Fetch viewer info
        const viewerInfo = await viewerApi.getMe()
        setViewerInfo(viewerInfo)

        // Explicit redirect_to takes highest priority (e.g. /settings/viewer)
        const redirectTo = searchParams.get('redirect_to')
        if (redirectTo && redirectTo.startsWith('/')) {
          router.push(redirectTo)
          return
        }

        // Fall back to streamer chat page
        const redirectStreamer = streamer || localStorage.getItem('viewer_streamer')
        if (redirectStreamer) {
          router.push(`/chat/${redirectStreamer}`)
        } else {
          router.push('/')
        }
      } catch (err) {
        console.error('Failed to fetch viewer info:', err)
        setError('Failed to complete authentication. Please try again.')
        setLoading(false)
      }
    }

    handleSuccess()
  }, [searchParams, setViewerToken, setViewerInfo, router])

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg">
      {loading ? (
        <div className="text-center">
          <div className="mx-auto mb-4 h-16 w-16 animate-spin rounded-full border-b-2 border-twitch"></div>
          <p className="text-lg text-text">Completing authentication...</p>
          <p className="mt-2 text-sm text-text-sub">Please wait</p>
        </div>
      ) : !error && window.opener ? (
        <div className="text-center">
          <div className="mb-4 text-6xl">&#10003;</div>
          <p className="mb-4 text-lg text-kick">Authentication successful!</p>
          <p className="text-sm text-text-sub">You can close this window</p>
        </div>
      ) : error ? (
        <div className="text-center">
          <div className="mb-4 text-6xl">&#9888;&#65039;</div>
          <p className="mb-4 text-lg text-youtube">{error}</p>
          <Link
            href="/"
            className="inline-block rounded-lg bg-twitch px-6 py-2 font-semibold text-white transition-colors hover:bg-twitch/80"
          >
            Return to Home
          </Link>
        </div>
      ) : null}
    </div>
  )
}

export default function AuthSuccessPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-bg">
          <div className="h-16 w-16 animate-spin rounded-full border-b-2 border-twitch"></div>
        </div>
      }
    >
      <AuthSuccessContent />
    </Suspense>
  )
}
