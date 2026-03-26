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

import type { Metadata } from 'next'
import { Barlow, DM_Mono } from 'next/font/google'
import './globals.css'
import '@/styles/events.css'
import CookieBanner from '@/components/CookieBanner'
import ImpersonationBanner from '@/components/ImpersonationBanner'
import { ToastProvider } from '@/components/ui/toast'
import { cn } from '@/lib/utils'

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
    default: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    template: '%s | All-Chat',
  },
  description:
    'Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single real-time overlay for OBS. Supports 7TV, BTTV, and FFZ emotes. Free and open source.',
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
    title: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    description:
      'Aggregate chat from Twitch, YouTube, Kick, and TikTok into one real-time overlay. 7TV, BTTV, and FFZ emote support included.',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'All-Chat - Multi-Platform Chat Aggregation for Streamers',
    description:
      'Aggregate chat from Twitch, YouTube, Kick, and TikTok into one real-time overlay. 7TV, BTTV, and FFZ emote support included.',
  },
  robots: {
    index: true,
    follow: true,
  },
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={cn(barlow.variable, dmMono.variable)}>
      <body>
        <ToastProvider>
          <ImpersonationBanner />
          {children}
          <CookieBanner />
        </ToastProvider>
      </body>
    </html>
  )
}
