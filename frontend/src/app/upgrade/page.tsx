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

import { Check, MessagesSquare, Radio, ShieldCheck, Sparkles, Users, Volume2 } from 'lucide-react'
import Link from 'next/link'
import { AppNav } from '@/components/AppNav'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PATREON_JOIN_URL } from '@/lib/constants'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

const t = getTranslations()

export const metadata = {
  title: 'Upgrade to Premium | All-Chat',
  description:
    'Back All-Chat on Patreon to unlock premium features: moderate from your overlay, ElevenLabs TTS, YouTube stream selection, shared chat, and viewer flairs.',
  alternates: { canonical: '/upgrade' },
}

interface PremiumFeature {
  icon: React.ComponentType<{ className?: string }>
  /** Names this row's pair of `marketing.upgrade.*Title` / `*Body` leaves. */
  messageStem: string
}

// Keep in sync with the onboarding extras tour in
// components/onboarding/OnboardingChecklist.tsx — its copy mirrors this list
// (see CLAUDE.md "Shipping a Feature").
//
// `as const satisfies` rather than a plain annotation: an annotation widens the
// stems to string, and a typo would then resolve to a missing key at runtime
// instead of failing tsc at the call site.
const features = [
  { icon: ShieldCheck, messageStem: 'moderation' },
  { icon: Users, messageStem: 'moderators' },
  { icon: Volume2, messageStem: 'tts' },
  { icon: Radio, messageStem: 'streamSelection' },
  { icon: MessagesSquare, messageStem: 'sharedChat' },
  { icon: Sparkles, messageStem: 'flairs' },
] as const satisfies ReadonlyArray<PremiumFeature>

export default function UpgradePage() {
  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-3xl space-y-10 px-4 py-12">
        {/* Hero */}
        <header className="space-y-4 text-center">
          <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub">
            <Sparkles className="h-3.5 w-3.5 text-twitch" />
            {t('marketing.upgrade.badge')}
          </span>
          <h1 className="text-3xl font-bold text-text sm:text-4xl">
            {t('marketing.upgrade.title')}
          </h1>
          <p className="mx-auto max-w-xl text-text-sub">{t('marketing.upgrade.body')}</p>
          <div className="flex flex-col items-center justify-center gap-3 pt-2 sm:flex-row">
            <Button
              variant="gradient"
              size="lg"
              render={
                // The label lives on the anchor itself so the link has
                // screen-reader-accessible content (jsx-a11y/anchor-has-content);
                // Base UI renders the anchor with the Button's styling.
                <a href={PATREON_JOIN_URL} target="_blank" rel="noopener noreferrer">
                  {t('marketing.upgrade.subscribe')}
                </a>
              }
            />
            <Button variant="outline" size="lg" render={<Link href="/settings/premium" />}>
              {t('marketing.upgrade.connectPatreon')}
            </Button>
          </div>
        </header>

        {/* Feature list */}
        <Card className="divide-y divide-border p-0">
          {features.map(({ icon: Icon, messageStem }) => (
            <div key={messageStem} className="flex items-start gap-4 p-5">
              <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-twitch">
                <Icon className="h-5 w-5" />
              </span>
              <div className="space-y-1">
                <h2 className="font-semibold text-text">
                  {t(`marketing.upgrade.${messageStem}Title`)}
                </h2>
                <p className="text-sm text-text-sub">{t(`marketing.upgrade.${messageStem}Body`)}</p>
              </div>
            </div>
          ))}
        </Card>

        {/* How it works */}
        <Card className="space-y-4 p-6">
          <h2 className="text-lg font-semibold text-text">{t('marketing.upgrade.howItWorks')}</h2>
          <ol className="space-y-3 text-sm text-text-sub">
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>
                {interpolateElements(t('marketing.upgrade.step1'), {
                  patreon: (
                    <a
                      href={PATREON_JOIN_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium text-twitch underline underline-offset-2"
                    >
                      {t('marketing.upgrade.step1Patreon')}
                    </a>
                  ),
                })}
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>
                {interpolateElements(t('marketing.upgrade.step2'), {
                  settings: (
                    <Link
                      href="/settings/premium"
                      className="font-medium text-twitch underline underline-offset-2"
                    >
                      {t('marketing.upgrade.step2Settings')}
                    </Link>
                  ),
                })}
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-twitch" />
              <span>{t('marketing.upgrade.step3')}</span>
            </li>
          </ol>
        </Card>

        <p className="text-center text-xs text-text-dim">
          {interpolateElements(t('marketing.upgrade.viewerFootnote'), {
            link: (
              <Link
                href="/settings/viewer/premium"
                className="text-text-sub underline underline-offset-2"
              >
                {t('marketing.upgrade.viewerFootnoteLink')}
              </Link>
            ),
          })}
        </p>
      </main>
    </div>
  )
}
