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
 * along with this program. If not, <https://www.gnu.org/licenses/>.
 */

/**
 * FinalSection — the closing band (mockup G). Logged-out visitors get the
 * three provider sign-in buttons (handlers stay in HomeClient, passed as
 * props) under `id="get-started"`, the anchor the hero CTA and nav target.
 * Logged-in visitors get the welcome-back branch: no re-pitch for people
 * already sold.
 */

'use client'

import { PlatformSignInButton } from '@/components/PlatformSignInButton'
import { useTranslations } from '@/lib/i18n'

export interface FinalSectionProps {
  /** Display name of the logged-in user; undefined when logged out. */
  userName?: string
  onTwitchLogin: () => void
  onYouTubeLogin: () => void
  onKickLogin: () => void
  onCta: () => void
}

export function FinalSection({
  userName,
  onTwitchLogin,
  onYouTubeLogin,
  onKickLogin,
  onCta,
}: FinalSectionProps) {
  const t = useTranslations()
  const isLoggedIn = userName !== undefined

  if (isLoggedIn) {
    return (
      <div className="final welcome" data-reveal>
        <h2>
          {t('marketing.final.welcomeBack', { name: userName })}
          <br />
          <span className="accent">{t('marketing.final.welcomeMicro')}</span>
        </h2>
        <button type="button" className="cta-go" onClick={onCta}>
          {t('marketing.lanes.backToDashboard')}
        </button>
      </div>
    )
  }

  return (
    <div className="final" id="get-started" data-reveal>
      <h2>
        {t('marketing.final.line1')}
        <br />
        {t('marketing.final.line2')}
        <br />
        <span className="accent">{t('marketing.final.line3')}</span>
      </h2>

      <div className="final-signins">
        <PlatformSignInButton platform="twitch" onClick={onTwitchLogin}>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.twitch') })}
        </PlatformSignInButton>
        <PlatformSignInButton platform="youtube" onClick={onYouTubeLogin}>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.youtube') })}
        </PlatformSignInButton>
        <PlatformSignInButton platform="kick" onClick={onKickLogin}>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.kick') })}
        </PlatformSignInButton>
      </div>

      <p className="micro">{t('marketing.final.micro')}</p>
    </div>
  )
}
