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
        setError('No authentication token received')
        setLoading(false)
        return
      }

      // Store token
      setToken(token)
      if (refreshToken && typeof window !== 'undefined') {
        localStorage.setItem('refresh_token', refreshToken)
      }

      try {
        // Fetch user info
        const user = await authApi.getMe()
        setUser(user)

        // Check for redirect_to parameter (used when adding sources via OAuth)
        const redirectTo = params.get('redirect_to')
        const sourceAdded = params.get('source_added')

        if (redirectTo) {
          // Redirect to specific page (e.g., overlay page after adding source)
          const redirectURL = sourceAdded ? `${redirectTo}?source_added=${sourceAdded}` : redirectTo
          router.push(redirectURL)
        } else {
          // Default: redirect to dashboard
          router.push('/dashboard')
        }
      } catch (err) {
        console.error('Authentication failed:', err)
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
