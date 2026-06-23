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
 * OAuth Callback Page
 *
 * Handles the OAuth callback from Twitch.
 * The backend redirects here with a JWT token in the URL query parameter.
 *
 * Flow:
 * 1. Extract token from URL (?token=xxx)
 * 2. Store token in localStorage
 * 3. Fetch user info from API
 * 4. Redirect to dashboard
 *
 * This is a Client Component because it:
 * - Uses useEffect for side effects
 * - Accesses URL parameters
 * - Uses localStorage
 * - Handles redirects
 */

'use client'

import { Suspense, useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { useAuthStore } from '@/lib/stores/auth-store'
import { authApi } from '@/lib/api/auth'
import { inMemoryTokens } from '@/lib/auth/in-memory-store'
import { trackEvent } from '@/lib/analytics'
import { InfinityLogo } from '@/components/InfinityLogo'

function AuthCallbackContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setToken, setUser } = useAuthStore()

  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const handleCallback = async () => {
      // Get token from URL fragment (#access_token=xxx)
      const hash = window.location.hash.substring(1) // Remove #
      const params = new URLSearchParams(hash)
      const token = params.get('access_token')
      const refreshToken = params.get('refresh_token')

      if (!token) {
        trackEvent('signin_failed', { reason: 'no_token' })
        setError('No authentication token received')
        setLoading(false)
        return
      }

      // Store token
      setToken(token)
      // SECURITY (audit H3): refresh token is stored in-memory ONLY, never in
      // localStorage, so XSS cannot read it from storage. It is lost on tab
      // close/refresh — the user must re-authenticate (acceptable tradeoff).
      // Full httpOnly-cookie migration is the longer-term fix (deferred).
      if (refreshToken) {
        inMemoryTokens.setRefreshToken(refreshToken)
      }

      try {
        // Fetch user info
        const user = await authApi.getMe()
        setUser(user)

        // Check for redirect_to parameter (used when adding sources / enabling
        // moderation via OAuth). `moderation_enabled` marks an opt-in moderation
        // re-consent (ADR-0017) that returns to the overlay monitor, not settings.
        const redirectTo = params.get('redirect_to')
        const sourceAdded = params.get('source_added')
        const moderationEnabled = params.get('moderation_enabled')

        // Distinct funnel steps: a completion marker means an OAuth source-add or
        // moderation re-consent finished, rather than a fresh sign-in.
        if (moderationEnabled) {
          trackEvent('moderation_enabled', { via: 'oauth', platform: moderationEnabled })
        } else if (sourceAdded) {
          trackEvent('source_added', { via: 'oauth' })
        } else {
          trackEvent('signin_completed', { platform: user.auth_provider ?? 'unknown' })
        }

        if (redirectTo) {
          // SECURITY (audit M12): validate redirect_to to prevent open redirect.
          // Must be a relative path (starts with '/') and not protocol-relative
          // ("//evil.com"). Mirrors the viewer flow at chat/auth-success/page.tsx.
          const isSafeRedirect = redirectTo.startsWith('/') && !redirectTo.startsWith('//')
          // Source-add returns to overlay settings with a confirmation marker; the
          // moderation monitor reflects the new scope on its own capabilities fetch,
          // so it just navigates back cleanly.
          const redirectURL = sourceAdded ? `${redirectTo}?source_added=${sourceAdded}` : redirectTo
          if (isSafeRedirect) {
            router.push(redirectURL)
          } else {
            console.warn('[AllChat] Blocked unsafe redirect_to:', redirectTo)
            router.push('/dashboard')
          }
        } else {
          // Default: redirect to dashboard
          router.push('/dashboard')
        }
      } catch (err) {
        console.error('Authentication failed:', err)
        trackEvent('signin_failed', { reason: 'me_fetch_failed' })
        setError('Authentication failed. Please try again.')
        setLoading(false)
      }
    }

    handleCallback()
  }, [searchParams, setToken, setUser, router])

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg">
      {loading ? (
        <div className="flex flex-col items-center gap-4">
          <InfinityLogo size={64} />
          <p className="text-sm text-text-sub">Authenticating...</p>
        </div>
      ) : error ? (
        <div className="text-center">
          <div className="mb-4 text-5xl">⚠️</div>
          <p className="mb-4 text-lg text-youtube">{error}</p>
          <Link
            href="/"
            className="inline-block rounded-lg bg-twitch px-6 py-2 font-semibold text-white transition-opacity hover:opacity-90"
          >
            Return to Home
          </Link>
        </div>
      ) : null}
    </div>
  )
}

export default function AuthCallbackPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-bg">
          <InfinityLogo size={64} />
        </div>
      }
    >
      <AuthCallbackContent />
    </Suspense>
  )
}
