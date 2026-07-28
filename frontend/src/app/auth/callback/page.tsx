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
 * Handles the OAuth callback redirect from the backend.
 * The backend stores tokens in Redis under a single-use code and redirects
 * here with `?code=<uuid>`. This page POSTs the code to `/auth/exchange` to
 * retrieve the JWT + refresh token, eliminating token exposure in the URL
 * fragment (audit M1).
 *
 * Flow:
 * 1. Read `code` from query string (?code=xxx)
 * 2. POST to /auth/exchange to get {access_token, refresh_token, ...}
 * 3. Store access token + in-memory refresh token
 * 4. Fetch user info + redirect to dashboard (or redirect_to)
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
import type { User } from '@/lib/types/auth'
import { trackEvent } from '@/lib/analytics'
import { resolveSigninPlatform, readAndClearSigninPlatform } from '@/lib/analytics-auth'
import { isAllowedExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { InfinityLogo } from '@/components/InfinityLogo'

function AuthCallbackContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { setUser } = useAuthStore()

  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const handleCallback = async () => {
      // Get short-lived auth code from query param (audit M1 — replaces URL
      // fragment token exposure with code+POST exchange).
      const code = searchParams.get('code')
      if (!code) {
        trackEvent('signin_failed', { reason: 'no_code' })
        setError('No authentication code received')
        setLoading(false)
        return
      }

      // Exchange the single-use code for the authenticated session. H3 cookie
      // auth: the server sets the httpOnly access cookie; the body carries the
      // user + redirect target (no tokens are exposed to JS).
      let user: User | null = null
      let redirectTo: string | null = null
      let sourceAdded: string | null = null
      let moderationEnabled: string | null = null
      try {
        const resp = await fetch('/api/v1/auth/exchange', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ code }),
        })
        if (!resp.ok) {
          throw new Error(`Exchange failed: ${resp.status}`)
        }
        const data = await resp.json()
        user = data.user ?? null
        redirectTo = data.redirect_to || null
        sourceAdded = data.source_added || null
        moderationEnabled = data.moderation_enabled || null
      } catch (err) {
        console.error('Token exchange failed:', err)
        trackEvent('signin_failed', { reason: 'exchange_failed' })
        setError('Authentication failed. The code may have expired — please try again.')
        setLoading(false)
        return
      }

      // SECURITY (audit H3): the access token lives only in an httpOnly cookie
      // set by the server; no token is stored in JS/localStorage. Login state is
      // established directly from the exchange response's user object.
      try {
        if (!user) {
          throw new Error('No user in exchange response')
        }
        setUser(user)

        // Distinct funnel steps: a completion marker means an OAuth source-add or
        // moderation re-consent finished, rather than a fresh sign-in.
        // Consume the stashed sign-in platform unconditionally (in every branch)
        // so an abandoned or reflow login can't leave stale sessionStorage that
        // later misattributes a fresh sign-in on the same tab.
        const signinPlatform = readAndClearSigninPlatform()
        if (moderationEnabled) {
          trackEvent('moderation_enabled', { via: 'oauth', platform: moderationEnabled })
        } else if (sourceAdded) {
          trackEvent('source_added', { via: 'oauth' })
        } else {
          // Attribute the platform reliably: the just-clicked platform (stashed
          // before the OAuth redirect) wins, then auth_provider, then inference
          // from the provider id fields. auth_provider alone was ~58% 'unknown'.
          trackEvent('signin_completed', { platform: resolveSigninPlatform(user, signinPlatform) })
        }

        if (redirectTo) {
          // SECURITY (audit M1/M12): validate redirect_to to prevent open redirect.
          // Uses the shared allowlist helper which also rejects backslash-based
          // bypasses (browsers normalize \ → /, so /\evil.com would navigate to
          // evil.com). Mirrors the viewer flow at chat/auth-success/page.tsx.
          const isSafeRedirect = isAllowedExternalRedirect(redirectTo)
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
  }, [searchParams, setUser, router])

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
            className="inline-block rounded-lg bg-twitch px-6 py-2 font-semibold text-bg transition-opacity hover:opacity-90"
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
