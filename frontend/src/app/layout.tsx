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
import { ToastProvider } from '@/components/ui/toast'
import { Toaster as HotToaster } from 'react-hot-toast'
import { cn } from '@/lib/utils'
import { DISCORD_INVITE_URL } from '@/lib/constants'

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
    default: 'All-Chat — Every chat. One overlay.',
    template: '%s | All-Chat',
  },
  description:
    'See all your Twitch, YouTube, Kick, and TikTok chat in one overlay. Drop it into OBS and go. 7TV, BTTV, and FFZ emotes built in. Free and open source.',
  keywords: [
    'twitch chat',
    'youtube chat',
    'kick chat',
    'tiktok chat',
    'chat overlay',
    'obs overlay',
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
  openGraph: {
    type: 'website',
    locale: 'en_US',
    url: 'https://allch.at',
    siteName: 'All-Chat',
    title: 'All-Chat — Every chat. One overlay.',
    description:
      'All your Twitch, YouTube, Kick, and TikTok chat in one overlay. 7TV, BTTV, and FFZ emotes built in.',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'All-Chat — Every chat. One overlay.',
    description:
      'All your Twitch, YouTube, Kick, and TikTok chat in one overlay. 7TV, BTTV, and FFZ emotes built in.',
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
// the initial HTML on every page. `sameAs` mirrors the links in the landing footer.
const organizationLd = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  name: 'All-Chat',
  url: 'https://allch.at',
  logo: 'https://allch.at/icon.svg',
  sameAs: ['https://github.com/caesarakalaeii/all-chat', DISCORD_INVITE_URL],
}

const webSiteLd = {
  '@context': 'https://schema.org',
  '@type': 'WebSite',
  name: 'All-Chat',
  url: 'https://allch.at',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={cn(barlow.variable, dmMono.variable)}>
      <body>
        {/* Skip link (WCAG 2.4.1): first focusable element on every page;
            visually hidden until keyboard-focused. Pages opt in by giving
            their <main> id="main-content" tabIndex={-1}. */}
        <a
          href="#main-content"
          className="sr-only z-[100] rounded-md border border-border-md bg-surface-2 px-4 py-2 text-sm font-medium text-text focus-visible:not-sr-only focus-visible:fixed focus-visible:top-4 focus-visible:left-4 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          Skip to main content
        </a>
        <JsonLd data={organizationLd} />
        <JsonLd data={webSiteLd} />
        <Analytics />
        <ToastProvider>
          <ImpersonationBanner />
          {children}
          <CookieBanner />
          <HotToaster
            position="top-right"
            toastOptions={{
              duration: 4000,
              error: { duration: 5000, style: { background: '#ef4444', color: '#fff' } },
              success: { style: { background: '#10b981', color: '#fff' } },
            }}
          />
        </ToastProvider>
      </body>
    </html>
  )
}
