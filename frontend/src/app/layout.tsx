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
import { Inter, Geist } from 'next/font/google';
import './globals.css';
import '@/styles/events.css';
import CookieBanner from '@/components/CookieBanner';
import ImpersonationBanner from '@/components/ImpersonationBanner';
import { cn } from "@/lib/utils";

const geist = Geist({subsets:['latin'],variable:'--font-sans'});

const inter = Inter({ subsets: ['latin'] });

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
    <html lang="en" className={cn("font-sans", geist.variable)}>
      <body className={inter.className}>
        <ImpersonationBanner />
        {children}
        <CookieBanner />
      </body>
    </html>
  );
}
