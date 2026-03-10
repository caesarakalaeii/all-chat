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

import type { Metadata } from 'next';
import { Barlow, DM_Mono } from 'next/font/google';
import './globals.css';
import '@/styles/events.css';
import CookieBanner from '@/components/CookieBanner';
import ImpersonationBanner from '@/components/ImpersonationBanner';
import { cn } from "@/lib/utils";

const barlow = Barlow({
  subsets: ['latin'],
  weight: ['400', '500', '600', '700', '800'],
  variable: '--font-barlow',
  display: 'swap',
});

const dmMono = DM_Mono({
  subsets: ['latin'],
  weight: ['400', '500'],
  variable: '--font-dm-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'All-Chat - Multi-Platform Chat Aggregation',
  description: 'Aggregate chat from Twitch, YouTube, and more in one overlay for OBS',
  keywords: ['twitch', 'youtube', 'chat', 'overlay', 'streaming', 'obs']
};

export default function RootLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={cn(barlow.variable, dmMono.variable)}>
      <body>
        <ImpersonationBanner />
        {children}
        <CookieBanner />
      </body>
    </html>
  );
}
