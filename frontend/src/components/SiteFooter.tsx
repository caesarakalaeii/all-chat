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

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { trackEvent } from '@/lib/analytics'
import { useTranslations } from '@/lib/i18n'
import { spaceMono } from '@/lib/fonts'
import { cn } from '@/lib/utils'

/**
 * Routes that must not carry the footer: overlays and the editor's embed
 * preview are captured by OBS, so links there would end up on stream; the
 * /chat and /auth pages are OAuth popups and redirect hops; /dev holds test
 * harnesses. Everything else gets it, so the Impressum and privacy policy
 * stay one click away on every page (§5 DDG).
 */
export function showsSiteFooter(pathname: string): boolean {
  if (pathname === '/overlay' || pathname.startsWith('/overlay/')) return false
  if (/^\/overlays\/[^/]+\/preview(\/|$)/.test(pathname)) return false
  return !['/chat/', '/auth/', '/dev/'].some((prefix) => pathname.startsWith(prefix))
}

const linkClass = 'underline-offset-4 hover:text-text hover:underline'

/** Site-wide footer, mounted once by the root layout. */
export function SiteFooter() {
  const t = useTranslations()
  const pathname = usePathname()
  if (!showsSiteFooter(pathname)) return null

  return (
    // The lanes mono face is scoped to the page wrappers, which this footer
    // sits outside of, so it applies the font itself.
    <footer
      className={cn(
        spaceMono.className,
        'space-y-2 border-t border-border pt-12 pb-12 text-center text-sm text-text-sub'
      )}
    >
      <p>{t('marketing.footer.tagline')}</p>
      <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
        <a
          href="https://github.com/caesarakalaeii/all-chat"
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => trackEvent('outbound_click', { dest: 'github' })}
          className="flex items-center gap-1 underline-offset-4 hover:text-text hover:underline"
        >
          <svg
            className="h-3.5 w-3.5"
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
            aria-hidden="true"
          >
            <path
              fill="currentColor"
              d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"
            />
          </svg>
          {t('marketing.footer.github')}
        </a>
        <span aria-hidden="true">&bull;</span>
        <a
          href={DISCORD_INVITE_URL}
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => trackEvent('outbound_click', { dest: 'discord' })}
          className={linkClass}
        >
          {t('marketing.footer.discord')}
        </a>
        <span aria-hidden="true">&bull;</span>
        <Link href="/docs" className={linkClass}>
          {t('marketing.footer.docs')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/docs/api" className={linkClass}>
          {t('marketing.footer.api')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/legal/privacy" className={linkClass}>
          {t('marketing.footer.privacy')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/legal/terms" className={linkClass}>
          {t('marketing.footer.terms')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/legal/impressum" className={linkClass}>
          {t('marketing.footer.impressum')}
        </Link>
      </p>
      {/* pt-2: the links are 16px tall, so the two rows need 8px more than
          space-y-2 gives for every link to keep the 24px target spacing
          WCAG 2.5.8 asks for. */}
      <p className="flex flex-wrap items-center justify-center gap-3 pt-2 text-xs">
        <Link href="/obs-chat-overlay" className={linkClass}>
          {t('marketing.footer.obsOverlay')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/obs-chat-dock" className={linkClass}>
          {t('marketing.footer.obsDock')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/tiktok-live-chat-overlay" className={linkClass}>
          {t('marketing.footer.tiktokLiveChat')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/multistream-chat" className={linkClass}>
          {t('marketing.footer.multistreamChat')}
        </Link>
        <span aria-hidden="true">&bull;</span>
        <Link href="/compare" className={linkClass}>
          {t('marketing.footer.compare')}
        </Link>
      </p>
    </footer>
  )
}
