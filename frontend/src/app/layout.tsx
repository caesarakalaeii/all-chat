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
 * Root Layout
 *
 * This is the root layout for the entire Next.js application.
 * It wraps all pages and provides global styles and metadata.
 *
 * Features:
 * - Global CSS (TailwindCSS)
 * - Metadata configuration
 * - Font optimization
 */

import type { Metadata, Viewport } from 'next'
import { Barlow, DM_Mono } from 'next/font/google'
import './globals.css'
import '@/styles/events.css'
import Analytics from '@/components/Analytics'
import { JsonLd } from '@/components/JsonLd'
import CookieBanner from '@/components/CookieBanner'
import ImpersonationBanner from '@/components/ImpersonationBanner'
import { SiteFooter } from '@/components/SiteFooter'
import { LocaleSync } from '@/components/LocaleSync'
import { ThemeSync } from '@/components/ThemeSync'
import { THEME_INIT_SCRIPT } from '@/lib/theme'
import { ToastProvider } from '@/components/ui/toast'
import { DEFAULT_LOCALE, getTranslations } from '@/lib/i18n'

// Module scope, not inside the component: `metadata` is evaluated on the
// server before any component runs.
const t = getTranslations()
import { cn } from '@/lib/utils'
import {
  DISCORD_INVITE_URL,
  EXTENSION_CHROME_URL,
  EXTENSION_FIREFOX_URL,
  PATREON_PAGE_URL,
} from '@/lib/constants'

const barlow = Barlow({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700', '800'],
  variable: '--font-barlow',
  display: 'swap',
})

const dmMono = DM_Mono({
  subsets: ['latin'],
  weight: ['400', '500'],
  variable: '--font-dm-mono',
  display: 'swap',
})

export const metadata: Metadata = {
  metadataBase: new URL('https://allch.at'),
  title: {
    default: t('metadata.site.titleDefault'),
    template: t('metadata.site.titleTemplate'),
  },
  description: t('metadata.site.description'),
  keywords: [
    'twitch chat',
    'youtube chat',
    'kick chat',
    'tiktok chat',
    'discord chat',
    'chat overlay',
    'obs overlay',
    'obs chat overlay',
    'multichat',
    'streamelements alternative',
    'streaming',
    'multistream',
    'chat aggregator',
    '7tv',
    'bttv',
    'ffz',
    'emotes',
    'streamer tools',
    'live streaming',
    'multi-platform chat',
  ],
  // No `url` here: every route that does not override openGraph inherits this
  // object, and a fixed https://allch.at told crawlers each guide was the
  // homepage. Without it, embeds use the URL that was shared.
  openGraph: {
    type: 'website',
    locale: 'en_US',
    siteName: 'All-Chat',
    title: t('metadata.site.socialTitle'),
    description: t('metadata.site.socialDescription'),
  },
  twitter: {
    card: 'summary_large_image',
    title: t('metadata.site.socialTitle'),
    description: t('metadata.site.socialDescription'),
  },
  robots: {
    index: true,
    follow: true,
  },
}

export const viewport: Viewport = {
  themeColor: '#0f0f13',
}

// Site-wide structured data. Emitted from the (server) root layout so it lands in
// the initial HTML on every page. `sameAs` lists every profile that is
// unambiguously All-Chat, so search engines and LLMs can join them to this site.
const organizationLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  name: 'All-Chat',
  url: 'https://allch.at',
  logo: 'https://allch.at/icon.svg',
  description: t('metadata.site.description'),
  sameAs: [
    'https://github.com/caesarakalaeii/all-chat',
    DISCORD_INVITE_URL,
    PATREON_PAGE_URL,
    EXTENSION_CHROME_URL,
    EXTENSION_FIREFOX_URL,
  ],
}

const webSiteLd = {
  '@context': 'https://schema.org',
  '@type': 'WebSite',
  name: 'All-Chat',
  url: 'https://allch.at',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // The server always renders DEFAULT_LOCALE; LocaleSync moves `lang` to the
    // stored UI language after hydration (ADR-0066).
    // suppressHydrationWarning: the theme script below sets data-theme before
    // React hydrates, so <html> differs from the server HTML by design.
    <html
      lang={DEFAULT_LOCALE}
      className={cn(barlow.variable, dmMono.variable)}
      suppressHydrationWarning
    >
      <head>
        {/* Applies a stored light theme before the first paint (ADR-0087). */}
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
      </head>
      <body>
        <LocaleSync />
        <ThemeSync />
        {/* Skip link (WCAG 2.4.1): first focusable element on every page;
            visually hidden until keyboard-focused. Pages opt in by giving
            their <main> id="main-content" tabIndex={-1}. */}
        <a
          href="#main-content"
          className="sr-only z-100 rounded-md border border-border-md bg-surface-2 px-4 py-2 text-sm font-medium text-text focus-visible:not-sr-only focus-visible:fixed focus-visible:top-4 focus-visible:left-4 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          {t('a11y.skipToMainContent')}
        </a>
        <JsonLd data={organizationLd} />
        <JsonLd data={webSiteLd} />
        <Analytics />
        <ToastProvider>
          <ImpersonationBanner />
          {children}
          <SiteFooter />
          <CookieBanner />
        </ToastProvider>
      </body>
    </html>
  )
}
