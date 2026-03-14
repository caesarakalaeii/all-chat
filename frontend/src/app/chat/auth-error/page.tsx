/**
 * Viewer OAuth Error Page
 *
 * Displays error when viewer authentication fails.
 * The backend redirects here with an error parameter.
 *
 * Route: /chat/auth-error
 */

'use client'

import { Suspense } from 'react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'

function AuthErrorContent() {
  const searchParams = useSearchParams()
  const error = searchParams.get('error') || 'Authentication failed'

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-900">
      <div className="max-w-md text-center">
        <div className="mb-6 text-6xl">❌</div>
        <h1 className="mb-4 text-3xl font-bold text-white">Authentication Failed</h1>
        <p className="mb-6 text-lg text-red-400">{error}</p>
        <p className="mb-8 text-slate-400">
          There was an error authenticating with your Twitch account. Please try again or contact
          support if the problem persists.
        </p>
        <div className="space-y-4">
          <Link
            href="/"
            className="block rounded-lg bg-purple-600 px-6 py-3 font-semibold text-white transition-colors hover:bg-purple-700"
          >
            Return to Home
          </Link>
          <button
            onClick={() => window.history.back()}
            className="block w-full rounded-lg bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:bg-slate-600"
          >
            Go Back
          </button>
        </div>
      </div>
    </div>
  )
}

export default function AuthErrorPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-slate-900">
          <div className="h-16 w-16 animate-spin rounded-full border-b-2 border-purple-500"></div>
        </div>
      }
    >
      <AuthErrorContent />
    </Suspense>
  )
}
