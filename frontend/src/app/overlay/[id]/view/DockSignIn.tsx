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

'use client'

/**
 * Sign-in panel for the monitor rendered as an OBS/Streamlabs browser dock.
 *
 * A custom browser dock is a separate CEF profile from the streamer's real
 * browser, so the dashboard session is not there on first open. The dock IS a
 * browser though (ADR-0049 and ADR-0051 solve credentials for clients that are
 * not), so it signs in normally — the point of this panel is only that it fits
 * a ~320px chromeless column and explains why sign-in is being asked for
 * twice. Sign-in must happen in THIS window: the cookie it sets has to land in
 * the dock's own jar.
 *
 * The three platform buttons are deliberately text-only, unlike the homepage's
 * brand-coloured hero buttons — at dock width the brand artwork does not fit,
 * and this is a utility panel rather than a conversion surface.
 */

import { Button } from '@/components/ui/button'

import { InfinityLogo } from '@/components/InfinityLogo'
import {
  STREAMER_PLATFORMS,
  stashSigninPlatform,
  type StreamerPlatform,
} from '@/lib/analytics-auth'
import { trackEvent } from '@/lib/analytics'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'

/** Catalog key for a platform's display name, e.g. `common.platforms.twitch`. */
function platformName(t: TFunction, platform: StreamerPlatform): string {
  if (platform === 'youtube') return t('common.platforms.youtube')
  if (platform === 'kick') return t('common.platforms.kick')
  return t('common.platforms.twitch')
}

export function DockSignIn() {
  const t = useTranslations()

  // Same failure copy whether the request itself failed or came back without an
  // auth_url: from the streamer's side both are "that button did nothing".
  const reportFailure = (platform: StreamerPlatform) => {
    toastManager.add({
      title: t('viewerOverlay.dock.signInFailed'),
      description: t('viewerOverlay.dock.signInFailedBody', {
        platform: platformName(t, platform),
      }),
      type: 'error',
    })
  }

  // Same flow as the homepage's hero buttons: ask the backend for the provider
  // URL, then navigate this window to it so the session cookie lands here.
  const signIn = async (platform: StreamerPlatform) => {
    trackEvent('signin_started', { platform })
    stashSigninPlatform(platform)
    try {
      const response = await fetch(`/api/v1/auth/${platform}/login`)
      const data = await response.json()
      if (!data.auth_url) {
        reportFailure(platform)
        return
      }
      safeExternalRedirect(data.auth_url)
    } catch {
      reportFailure(platform)
    }
  }

  return (
    <div className="flex h-screen flex-col items-center justify-center gap-3 bg-bg px-4 text-center">
      <InfinityLogo size={40} />
      <p className="text-sm font-semibold text-text">
        {t('viewerOverlay.dock.productName')}
      </p>
      <p className="text-xs text-text-sub">{t('viewerOverlay.dock.signInExplanation')}</p>
      <div className="flex w-full max-w-56 flex-col gap-2">
        {STREAMER_PLATFORMS.map((platform) => (
          <Button key={platform} variant="outline" size="sm" onClick={() => void signIn(platform)}>
            {t('viewerOverlay.dock.signInWith', {
              platform: platformName(t, platform),
            })}
          </Button>
        ))}
      </div>
    </div>
  )
}
