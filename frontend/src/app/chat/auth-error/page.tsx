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
import { useTrackOnce } from '@/hooks/useTrackOnce'
import { bucketViewerAuthError, sanitizeViewerPlatform } from '@/lib/analytics-auth'
import { useTranslations } from '@/lib/i18n'

// Not copy: decoration above text that states the failure in words. U+274C.
const FAILURE_GLYPH = '❌'

// Backend `platform` query values that have a display name in the catalog.
// Anything else falls back to a platform-neutral label so the page never
// misnames the provider that failed (it used to hardcode "Twitch" for every
// platform).
const NAMEABLE_PLATFORMS = ['twitch', 'youtube', 'kick', 'tiktok', 'discord'] as const

function AuthErrorContent() {
  const t = useTranslations()
  const searchParams = useSearchParams()
  const error = searchParams.get('error') || t('auth.viewerError.defaultError')
  const platform = searchParams.get('platform') ?? ''
  // Resolve the label defensively. Indexing a plain object with an
  // attacker-supplied query value (e.g. ?platform=toString) can resolve to an
  // inherited Object prototype member, so match against the runtime list
  // instead — otherwise fall back to the neutral label.
  const nameable = NAMEABLE_PLATFORMS.find((known) => known === platform)
  const accountLabel = nameable
    ? t('auth.viewerError.accountNamed', { platform: t(`common.platforms.${nameable}`) })
    : t('auth.viewerError.accountNeutral')

  // Instrument the viewer/extension auth error rate + causes (previously untracked).
  // The raw `error` is bucketed to a non-PII enum before it leaves the browser.
  useTrackOnce('viewer_signin_failed', {
    platform: sanitizeViewerPlatform(platform),
    reason: bucketViewerAuthError(searchParams.get('error')),
  })

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg">
      <div className="max-w-md text-center">
        <div className="mb-6 text-6xl">{FAILURE_GLYPH}</div>
        <h1 className="mb-4 text-3xl font-bold text-text">{t('auth.viewerError.title')}</h1>
        <p className="mb-6 text-lg text-youtube">{error}</p>
        <p className="mb-8 text-text-sub">
          {t('auth.viewerError.body', { account: accountLabel })}
        </p>
        <div className="space-y-4">
          <Link
            href="/"
            className="block rounded-lg bg-twitch px-6 py-3 font-semibold text-bg transition-colors hover:bg-twitch/80"
          >
            {t('auth.viewerError.returnHome')}
          </Link>
          <button
            onClick={() => window.history.back()}
            className="block w-full rounded-lg border border-border bg-surface px-6 py-3 font-semibold text-text transition-colors hover:bg-surface-2"
          >
            {t('auth.viewerError.goBack')}
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
