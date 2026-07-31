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
import { isAllowedExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { trackEvent } from '@/lib/analytics'

/*
 * SECURITY (audit H4 + #10): postMessage targetOrigin must be an explicit
 * origin, never '*', so the viewer JWT is delivered only to a known-good
 * opener. The opener is NOT this app's origin — this popup is opened from the
 * browser extension's content script running on a platform page (twitch.tv /
 * youtube.com / kick.com), so it must target those origins. The H4 fix
 * (targetOrigin = window.location.origin = allch.at) silently dropped the
 * token because the opener is twitch.tv, breaking extension login. The browser
 * delivers a message only to the window whose origin matches, so iterating this
 * allowlist delivers exactly once to the real opener and to nobody else. Keep in
 * sync with the extension manifest content_scripts matches.
 */
const ALLOWED_OPENER_ORIGINS = [
  'https://www.twitch.tv',
  'https://www.youtube.com',
  'https://studio.youtube.com',
  'https://kick.com',
  ...(typeof window !== 'undefined' ? [window.location.origin] : []),
]

function postToOpener(opener: Window, payload: Record<string, unknown>) {
  for (const origin of ALLOWED_OPENER_ORIGINS) {
    opener.postMessage(payload, origin)
  }
}

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
        trackEvent('viewer_signin_failed', { reason: 'no_code' })
        setError('No authentication code received')
        setLoading(false)

        // Notify opener (extension) about error
        if (window.opener) {
          postToOpener(window.opener, {
            type: 'ALLCHAT_AUTH_ERROR',
            error: 'No authentication code received',
          })
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
        // Viewer auth funnel: completion = a token was actually returned. Guard
        // on truthiness so a 200-with-no-token isn't mis-recorded as a success
        // (it's a failed exchange). Together with the no_code branch above and the
        // catch below, both sides of the ~11% viewer error rate are measurable.
        if (token) {
          trackEvent('viewer_signin_completed')
        } else {
          trackEvent('viewer_signin_failed', { reason: 'exchange_failed' })
        }
      } catch (err) {
        console.error('Token exchange failed:', err)
        trackEvent('viewer_signin_failed', { reason: 'exchange_failed' })
        setError('Authentication failed. The code may have expired — please try again.')
        setLoading(false)
        return
      }

      // Check if this was opened from an extension popup
      const isExtensionPopup = window.opener && !window.opener.closed

      if (isExtensionPopup) {
        // Extension flow: post message to opener and close
        console.log('[AllChat Auth] Posting token to extension opener')
        postToOpener(window.opener, {
          type: 'ALLCHAT_AUTH_SUCCESS',
          token,
          streamer,
        })

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

        // Explicit redirect_to takes highest priority (e.g. /settings/viewer).
        // Validate via the shared allowlist (rejects protocol-relative //evil.com,
        // backslash /\evil.com, and off-allowlist hosts) — the bare startsWith('/')
        // check here was an open redirect (PR #478 review M2).
        const redirectTo = searchParams.get('redirect_to')
        if (redirectTo && isAllowedExternalRedirect(redirectTo)) {
          router.push(redirectTo)
          return
        }

        // No explicit target → land on the home page.
        router.push('/')
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
            className="inline-block rounded-lg bg-twitch px-6 py-2 font-semibold text-bg transition-colors hover:bg-twitch/80"
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
