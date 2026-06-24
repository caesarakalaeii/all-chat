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

import { Check, MessagesSquare, Radio, ShieldCheck, Sparkles, Volume2 } from 'lucide-react'
import Link from 'next/link'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { buttonVariants } from '@/components/ui/button'
import { PATREON_JOIN_URL } from '@/lib/constants'

export const metadata = {
  title: 'Upgrade to Premium | All-Chat',
  description:
    'Back All-Chat on Patreon to unlock premium features: moderate from your overlay, ElevenLabs TTS, YouTube stream selection, shared chat, and viewer flairs.',
  alternates: { canonical: '/upgrade' },
}

interface PremiumFeature {
  icon: React.ComponentType<{ className?: string }>
  title: string
  description: string
}

const features: PremiumFeature[] = [
  {
    icon: ShieldCheck,
    title: 'Moderate from your overlay',
    description:
      'Delete, timeout, and ban across Twitch, YouTube, Kick, and Discord straight from the monitor view — no second dashboard.',
  },
  {
    icon: Volume2,
    title: 'ElevenLabs text-to-speech',
    description:
      'Read chat aloud with high-quality ElevenLabs voices, with full control over priority and pronunciation.',
  },
  {
    icon: Radio,
    title: 'YouTube stream selection',
    description:
      'Pick exactly which YouTube broadcast an overlay listens to instead of relying on auto-detection.',
  },
  {
    icon: MessagesSquare,
    title: 'Shared chat',
    description: 'Combine several channels into one shared conversation across your overlays.',
  },
  {
    icon: Sparkles,
    title: 'Viewer flairs',
    description:
      'Stand out in any chat you appear in with premium cosmetics like animated name gradients.',
  },
]

export default function UpgradePage() {
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main className="mx-auto max-w-3xl space-y-10 px-4 py-12">
        {/* Hero */}
        <header className="space-y-4 text-center">
          <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub">
            <Sparkles className="h-3.5 w-3.5 text-twitch" />
            All-Chat Premium
          </span>
          <h1 className="text-3xl font-bold text-text sm:text-4xl">
            Unlock the full power of your overlay
          </h1>
          <p className="mx-auto max-w-xl text-text-sub">
            Premium is funded entirely through Patreon — it keeps All-Chat running and unlocks the
            features that make multistream moderation effortless. Back the project once, and premium
            applies automatically to your account.
          </p>
          <div className="flex flex-col items-center justify-center gap-3 pt-2 sm:flex-row">
            <a
              href={PATREON_JOIN_URL}
              target="_blank"
              rel="noopener noreferrer"
              className={buttonVariants({ variant: 'gradient', size: 'lg' })}
            >
              Subscribe on Patreon
            </a>
            <Link
              href="/settings/premium"
              className={buttonVariants({ variant: 'outline', size: 'lg' })}
            >
              Already a patron? Connect Patreon
            </Link>
          </div>
        </header>

        {/* Feature list */}
        <Card className="divide-y divide-border p-0">
          {features.map(({ icon: Icon, title, description }) => (
            <div key={title} className="flex items-start gap-4 p-5">
              <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-twitch">
                <Icon className="h-5 w-5" />
              </span>
              <div className="space-y-1">
                <h2 className="font-semibold text-text">{title}</h2>
                <p className="text-sm text-text-sub">{description}</p>
              </div>
            </div>
          ))}
        </Card>

        {/* How it works */}
        <Card className="space-y-4 p-6">
          <h2 className="text-lg font-semibold text-text">How it works</h2>
          <ol className="space-y-3 text-sm text-text-sub">
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>
                Back All-Chat on{' '}
                <a
                  href={PATREON_JOIN_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-twitch hover:underline"
                >
                  Patreon
                </a>{' '}
                at the premium tier.
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>
                Connect your Patreon account on the{' '}
                <Link href="/settings/premium" className="font-medium text-twitch hover:underline">
                  Premium settings
                </Link>{' '}
                page.
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>Premium unlocks automatically — no codes, no waiting.</span>
            </li>
          </ol>
        </Card>

        <p className="text-center text-xs text-text-dim">
          Just want viewer cosmetics?{' '}
          <Link href="/settings/viewer/premium" className="text-text-sub hover:underline">
            See viewer premium
          </Link>
          .
        </p>
      </main>
    </div>
  )
}
