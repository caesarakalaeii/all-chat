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
    <div className="flex min-h-screen items-center justify-center bg-bg">
      <div className="max-w-md text-center">
        <div className="mb-6 text-6xl">&#10060;</div>
        <h1 className="mb-4 text-3xl font-bold text-text">Authentication Failed</h1>
        <p className="mb-6 text-lg text-youtube">{error}</p>
        <p className="mb-8 text-text-sub">
          There was an error authenticating with your Twitch account. Please try again or contact
          support if the problem persists.
        </p>
        <div className="space-y-4">
          <Link
            href="/"
            className="block rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-colors hover:bg-twitch/80"
          >
            Return to Home
          </Link>
          <button
            onClick={() => window.history.back()}
            className="block w-full rounded-lg border border-border bg-surface px-6 py-3 font-semibold text-text transition-colors hover:bg-surface-2"
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
        <div className="flex min-h-screen items-center justify-center bg-bg">
          <div className="h-16 w-16 animate-spin rounded-full border-b-2 border-twitch"></div>
        </div>
      }
    >
      <AuthErrorContent />
    </Suspense>
  )
}
