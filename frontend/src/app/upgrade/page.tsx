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
import { cn } from '@/lib/utils'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { PATREON_JOIN_URL } from '@/lib/constants'
import { getTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

const t = getTranslations()

export const metadata = {
  title: t('metadata.upgrade.title'),
  description: t('metadata.upgrade.description'),
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
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-3xl space-y-10 px-4 py-12">
        {/* Hero */}
        <header className="space-y-4 text-center">
          <span className="mono-label inline-flex items-center gap-1.5">
            <Sparkles className="h-3.5 w-3.5" />
            {t('marketing.upgrade.badge')}
          </span>
          <h1 className="text-3xl sm:text-4xl">{t('marketing.upgrade.title')}</h1>
          <p className="text-sub mx-auto max-w-xl">{t('marketing.upgrade.body')}</p>
          <div className="flex flex-col items-center justify-center gap-3 pt-2 sm:flex-row">
            <a
              href={PATREON_JOIN_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="lanes-btn"
            >
              {t('marketing.upgrade.subscribe')}
            </a>
            <Link href="/settings/premium" className="lanes-btn ghost">
              {t('marketing.upgrade.connectPatreon')}
            </Link>
          </div>
        </header>

        {/* Feature list */}
        <div className="lanes-panel divide-y divide-white/20 p-0">
          {features.map(({ icon: Icon, messageStem }) => (
            <div key={messageStem} className="flex items-start gap-4 p-5">
              <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center border-2 border-white text-white">
                <Icon className="h-5 w-5" />
              </span>
              <div className="space-y-1">
                <h2 className="text-base">{t(`marketing.upgrade.${messageStem}Title`)}</h2>
                <p className="text-sub text-sm">{t(`marketing.upgrade.${messageStem}Body`)}</p>
              </div>
            </div>
          ))}
        </div>

        {/* How it works */}
        <div className="lanes-panel space-y-4 p-6">
          <h2 className="text-lg">{t('marketing.upgrade.howItWorks')}</h2>
          <ol className="space-y-3 text-sm text-text-sub">
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-white" />
              <span>
                {interpolateElements(t('marketing.upgrade.step1'), {
                  patreon: (
                    <a
                      href={PATREON_JOIN_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium underline underline-offset-2"
                    >
                      {t('marketing.upgrade.step1Patreon')}
                    </a>
                  ),
                })}
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-white" />
              <span>
                {interpolateElements(t('marketing.upgrade.step2'), {
                  settings: (
                    <Link
                      href="/settings/premium"
                      className="font-medium underline underline-offset-2"
                    >
                      {t('marketing.upgrade.step2Settings')}
                    </Link>
                  ),
                })}
              </span>
            </li>
            <li className="flex gap-3">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-white" />
              <span>{t('marketing.upgrade.step3')}</span>
            </li>
          </ol>
        </div>

        <p className="text-center text-xs text-text-dim">
          {interpolateElements(t('marketing.upgrade.viewerFootnote'), {
            link: (
              <Link href="/settings/viewer/premium" className="underline underline-offset-2">
                {t('marketing.upgrade.viewerFootnoteLink')}
              </Link>
            ),
          })}
        </p>
      </main>
    </div>
  )
}
