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

import { Button } from '@/components/ui/button'
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
      <div className="final welcome">
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
    <div className="final" id="get-started">
      <h2>
        {t('marketing.final.line1')}
        <br />
        {t('marketing.final.line2')}
        <br />
        <span className="accent">{t('marketing.final.line3')}</span>
      </h2>

      {/* Sign-in buttons — the same three providers and brand colours as the
          old hero, so returning OAuth users see a familiar login row. */}
      <div className="flex flex-wrap justify-center gap-3">
        <Button
          onClick={onTwitchLogin}
          size="lg"
          className="gap-2.5 px-6 py-3"
          aria-label={t('marketing.lanes.signInWith', {
            platform: t('common.platforms.twitch'),
          })}
        >
          <svg
            className="h-5 w-5 shrink-0"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <path
              fill="currentColor"
              d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
            />
          </svg>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.twitch') })}
        </Button>

        {/* YouTube — exact brand red #FF0000; dark label for WCAG AA (white on
            #FF0000 is ~4.0:1), official white-on-red icon kept (logo exemption) */}
        <Button
          onClick={onYouTubeLogin}
          size="lg"
          className="gap-2.5 px-6 py-3 text-bg"
          style={
            {
              backgroundColor: '#FF0000',
              '--tw-ring-color': '#FF0000',
            } as React.CSSProperties
          }
          aria-label={t('marketing.lanes.signInWith', {
            platform: t('common.platforms.youtube'),
          })}
        >
          {/* Official YouTube icon — white play button on brand red */}
          <svg
            className="h-5 w-5 shrink-0"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <path
              fill="#FFFFFF"
              d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
            />
          </svg>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.youtube') })}
        </Button>

        {/* Kick — brand green, dark text + official block-K logo */}
        <Button
          onClick={onKickLogin}
          size="lg"
          className="gap-2.5 px-6 py-3 text-bg"
          style={{ backgroundColor: 'var(--color-kick)' }}
          aria-label={t('marketing.lanes.signInWith', {
            platform: t('common.platforms.kick'),
          })}
        >
          <svg
            className="h-5 w-5 shrink-0"
            viewBox="0 0 512 512"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <path
              fill="currentColor"
              d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
            />
          </svg>
          {t('marketing.lanes.signInWith', { platform: t('common.platforms.kick') })}
        </Button>
      </div>

      <p className="micro">{t('marketing.final.micro')}</p>
    </div>
  )
}
