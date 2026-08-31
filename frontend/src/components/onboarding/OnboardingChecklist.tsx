'use client'

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
 * The first-run setup guide (retention initiative): a floating checklist that
 * walks a new streamer to "chat visible in OBS". It rides on DERIVED state
 * (see onboarding-store) so it survives the full-window OAuth round-trips of
 * source connection, and it points INTO the real editor sections rather than
 * duplicating their UI.
 *
 * Mounted on /dashboard and /overlays/[id]. Desktop: bottom-right card.
 * Mobile: bottom bar that expands. Dismissible with confirm (restartable
 * from Settings), minimizable, fully keyboard operable.
 */

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Check, ChevronDown, ChevronUp, Clipboard, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { VisuallyHidden } from '@/components/ui/visually-hidden'
import { ObsHelpContent } from './ObsHelpContent'
import { CreateOverlayDialog } from './CreateOverlayDialog'
import {
  deriveSteps,
  useOnboardingStore,
  type OnboardingStepId,
} from '@/lib/stores/onboarding-store'
import { trackEvent } from '@/lib/analytics'
import { DISCORD_INVITE_URL, PATREON_JOIN_URL } from '@/lib/constants'
import { cn } from '@/lib/utils'
import { useTranslations } from '@/lib/i18n'
import type { SpotlightSection } from '@/components/editor/sectionRegistry'

/**
 * Step id to `onboarding.steps.*` leaf. `as const satisfies` rather than a
 * plain annotation so a typo in a stem still fails `tsc` at the `t()` call.
 */
const STEP_MESSAGE_STEMS = {
  create_overlay: 'createOverlay',
  connect_source: 'connectSource',
  choose_theme: 'chooseTheme',
  copy_obs: 'copyObs',
} as const satisfies Record<OnboardingStepId, string>

const STEP_INDEX: Record<OnboardingStepId, number> = {
  create_overlay: 0,
  connect_source: 1,
  choose_theme: 2,
  copy_obs: 3,
}

export interface OnboardingChecklistProps {
  /** Where the checklist is mounted; step CTAs differ between the two. */
  surface: 'dashboard' | 'editor'
  /**
   * floating: fixed bottom-right card (dashboard — nothing sits under it).
   * inline: in-flow block (editor config panel — a floating card would
   * obscure controls beneath it, WCAG 2.5.8 / axe target-size).
   */
  variant?: 'floating' | 'inline'
  /** Editor only: number of sources on the active overlay. */
  sourceCount?: number
  /** Editor only: currently applied theme id (null = none yet). */
  themeId?: string | null
  /** Editor only: scroll to + force open an editor section. */
  onSpotlightSection?: (section: SpotlightSection) => void
  /** Dashboard only: overlays count from the overlay store. */
  overlayCount?: number
}

export function OnboardingChecklist({
  surface,
  variant = 'floating',
  sourceCount = 0,
  themeId = null,
  onSpotlightSection,
  overlayCount = 0,
}: OnboardingChecklistProps) {
  const t = useTranslations()
  const router = useRouter()
  const status = useOnboardingStore((s) => s.status)
  const activeOverlayId = useOnboardingStore((s) => s.activeOverlayId)
  const sessionSteps = useOnboardingStore((s) => s.sessionSteps)
  const minimized = useOnboardingStore((s) => s.minimized)
  const minimize = useOnboardingStore((s) => s.minimize)
  const markObsCopied = useOnboardingStore((s) => s.markObsCopied)
  const markExtrasDone = useOnboardingStore((s) => s.markExtrasDone)
  const reportStepCompleted = useOnboardingStore((s) => s.reportStepCompleted)
  const dismiss = useOnboardingStore((s) => s.dismiss)
  const finish = useOnboardingStore((s) => s.finish)

  const [createOpen, setCreateOpen] = useState(false)
  const [confirmDismiss, setConfirmDismiss] = useState(false)
  const [copied, setCopied] = useState(false)

  const steps = useMemo(
    () =>
      deriveSteps({
        overlayCount: surface === 'editor' ? Math.max(overlayCount, 1) : overlayCount,
        sourceCount,
        themeId,
        obsCopied: sessionSteps.obsCopied,
      }),
    [surface, overlayCount, sourceCount, themeId, sessionSteps.obsCopied]
  )

  const activeStep = steps.find((s) => s.active)
  const doneCount = steps.filter((s) => s.done).length
  const coreDone = doneCount === steps.length
  const showExtras = coreDone && !sessionSteps.extrasDone
  const showFinale = coreDone && sessionSteps.extrasDone

  // Report derived completions + the active step view (double-fire guarded
  // in the store / by the step index key).
  useEffect(() => {
    if (status !== 'active') return
    for (const step of steps) {
      if (step.done && step.id !== 'copy_obs') reportStepCompleted(step.id)
    }
  }, [status, steps, reportStepCompleted])

  // The last step reported as viewed, so re-renders that do not change the active step
  // stay silent. A ref rather than state: nothing renders from it, and setting state here
  // would be a cascading render for no visible change
  // (react-hooks/set-state-in-effect).
  const reportedViewRef = useRef<OnboardingStepId | null>(null)
  useEffect(() => {
    if (status !== 'active' || !activeStep || activeStep.id === reportedViewRef.current) return
    reportedViewRef.current = activeStep.id
    trackEvent('onboarding_step_viewed', {
      step: activeStep.id,
      index: STEP_INDEX[activeStep.id],
    })
  }, [status, activeStep])

  if (status !== 'active') return null

  async function handleCopyObs() {
    const overlayId = activeOverlayId
    if (!overlayId) return
    try {
      await navigator.clipboard.writeText(`${window.location.origin}/overlay/${overlayId}`)
      trackEvent('obs_url_copied', { surface: 'onboarding' })
      markObsCopied()
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      // Clipboard unavailable — the editor's own copy button is the fallback.
    }
  }

  function stepCta(stepId: OnboardingStepId) {
    switch (stepId) {
      case 'create_overlay':
        return (
          <Button size="sm" variant="gradient" onClick={() => setCreateOpen(true)}>
            {t('onboarding.checklist.create')}
          </Button>
        )
      case 'connect_source':
      case 'choose_theme': {
        const section = stepId === 'connect_source' ? 'sources' : 'theme'
        if (surface === 'editor' && onSpotlightSection) {
          return (
            <Button size="sm" variant="outline" onClick={() => onSpotlightSection(section)}>
              {t('onboarding.checklist.showMe')}
            </Button>
          )
        }
        return (
          <Button
            size="sm"
            variant="outline"
            disabled={!activeOverlayId}
            onClick={() => activeOverlayId && router.push(`/overlays/${activeOverlayId}`)}
          >
            {t('onboarding.checklist.openEditor')}
          </Button>
        )
      }
      case 'copy_obs':
        return (
          <Button size="sm" variant="outline" disabled={!activeOverlayId} onClick={handleCopyObs}>
            <Clipboard className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
            {copied ? t('onboarding.checklist.copied') : t('onboarding.checklist.copyLink')}
          </Button>
        )
    }
  }

  return (
    <section
      aria-label={t('onboarding.checklist.title')}
      className={cn(
        'border border-border-md bg-surface',
        variant === 'floating'
          ? 'fixed inset-x-0 bottom-0 z-40 rounded-t-xl shadow-xl sm:inset-x-auto sm:right-4 sm:bottom-4 sm:w-96 sm:rounded-xl'
          : 'rounded-xl'
      )}
    >
      <header className="flex items-center justify-between gap-2 border-b border-border p-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-text">{t('onboarding.checklist.title')}</p>
          <p className="text-xs text-text-sub" aria-live="polite">
            {coreDone
              ? t('onboarding.checklist.allDone')
              : t('onboarding.checklist.progress', {
                  done: doneCount,
                  total: steps.length,
                })}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Button
            type="button"
            onClick={() => minimize(!minimized)}
            aria-expanded={!minimized}
            aria-label={
              minimized ? t('onboarding.checklist.expand') : t('onboarding.checklist.minimize')
            }
            variant="ghost"
            size="icon-xs"
          >
            {minimized ? (
              <ChevronUp className="h-4 w-4" aria-hidden="true" />
            ) : (
              <ChevronDown className="h-4 w-4" aria-hidden="true" />
            )}
          </Button>
          <Button
            type="button"
            onClick={() => setConfirmDismiss(true)}
            aria-label={t('onboarding.checklist.dismiss')}
            variant="ghost"
            size="icon-xs"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      </header>

      {!minimized && (
        <div className="space-y-3 p-3">
          <ol className="space-y-2">
            {steps.map((step) => (
              <li key={step.id} className="flex items-center justify-between gap-2">
                <span className="flex min-w-0 items-center gap-2">
                  <span
                    aria-hidden="true"
                    className={cn(
                      'flex h-5 w-5 shrink-0 items-center justify-center rounded-full border text-xs',
                      step.done
                        ? 'border-kick/60 bg-kick/15 text-kick'
                        : 'border-border-md text-text-dim'
                    )}
                  >
                    {step.done ? <Check className="h-3 w-3" /> : STEP_INDEX[step.id] + 1}
                  </span>
                  <span
                    className={cn(
                      'truncate text-sm',
                      step.done ? 'text-text-dim line-through' : 'text-text'
                    )}
                  >
                    {t(`onboarding.steps.${STEP_MESSAGE_STEMS[step.id]}`)}
                    {step.done && (
                      <VisuallyHidden>{t('onboarding.checklist.stepDone')}</VisuallyHidden>
                    )}
                  </span>
                </span>
                {step.active && !step.done && stepCta(step.id)}
              </li>
            ))}
          </ol>

          {activeStep?.id === 'copy_obs' && !coreDone && (
            <div className="rounded-lg border border-border bg-surface-2 p-3">
              <p className="mb-1.5 text-xs font-semibold text-text">
                {t('onboarding.obs.heading')}
              </p>
              <ObsHelpContent />
            </div>
          )}

          {showExtras && (
            <div className="rounded-lg border border-border bg-surface-2 p-3">
              <p className="text-sm font-semibold text-text">{t('onboarding.extras.heading')}</p>
              {/* Feature names/claims mirror app/upgrade/page.tsx — keep in sync. */}
              <ul className="mt-2 space-y-2 text-sm">
                <li>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-text">{t('onboarding.extras.ttsTitle')}</span>
                    {surface === 'editor' && onSpotlightSection ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          trackEvent('cta_click', { cta: 'tts', location: 'onboarding-extras' })
                          onSpotlightSection('appearance')
                        }}
                      >
                        {t('onboarding.checklist.showMe')}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!activeOverlayId}
                        onClick={() => {
                          trackEvent('cta_click', { cta: 'tts', location: 'onboarding-extras' })
                          if (activeOverlayId) router.push(`/overlays/${activeOverlayId}`)
                        }}
                      >
                        {t('onboarding.checklist.showMe')}
                      </Button>
                    )}
                  </div>
                  <p className="text-xs text-text-sub">{t('onboarding.extras.ttsBody')}</p>
                </li>
                <li>
                  <span className="font-medium text-text">
                    {t('onboarding.extras.moderationTitle')}
                  </span>
                  <p className="text-xs text-text-sub">{t('onboarding.extras.moderationBody')}</p>
                </li>
                <li>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-text">
                      {t('onboarding.extras.moderatorsTitle')}
                    </span>
                    {surface === 'editor' && onSpotlightSection ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          trackEvent('cta_click', {
                            cta: 'moderators',
                            location: 'onboarding-extras',
                          })
                          onSpotlightSection('moderators')
                        }}
                      >
                        {t('onboarding.checklist.showMe')}
                      </Button>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={!activeOverlayId}
                        onClick={() => {
                          trackEvent('cta_click', {
                            cta: 'moderators',
                            location: 'onboarding-extras',
                          })
                          if (activeOverlayId) router.push(`/overlays/${activeOverlayId}`)
                        }}
                      >
                        {t('onboarding.checklist.showMe')}
                      </Button>
                    )}
                  </div>
                  <p className="text-xs text-text-sub">{t('onboarding.extras.moderatorsBody')}</p>
                </li>
                <li>
                  <span className="font-medium text-text">
                    {t('onboarding.extras.sharedChatTitle')}
                  </span>
                  <p className="text-xs text-text-sub">{t('onboarding.extras.sharedChatBody')}</p>
                </li>
                <li>
                  <span className="font-medium text-text">
                    {t('onboarding.extras.streamSelectionTitle')}
                  </span>
                  <p className="text-xs text-text-sub">
                    {t('onboarding.extras.streamSelectionBody')}
                  </p>
                </li>
                <li>
                  <span className="font-medium text-text">
                    {t('onboarding.extras.bubbleColorsTitle')}
                  </span>
                  {/* Listed here but NOT in app/upgrade/page.tsx, for the same reason as
                      Stream Deck buttons below: that list is specifically the PREMIUM
                      tour, and the bubble_colors gate is seeded free (migration 090)
                      because it is pure client-side CSS — no bandwidth, quota or send
                      path. If the gate is ever flipped to premium, move this entry into
                      the /upgrade list and keep the two in sync per the note above. */}
                  <p className="text-xs text-text-sub">{t('onboarding.extras.bubbleColorsBody')}</p>
                </li>
                <li>
                  <span className="font-medium text-text">
                    {t('onboarding.extras.flairsTitle')}
                  </span>
                  <p className="text-xs text-text-sub">{t('onboarding.extras.flairsBody')}</p>
                </li>
                <li>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium text-text">
                      {t('onboarding.extras.streamDeckTitle')}
                    </span>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        trackEvent('cta_click', {
                          cta: 'paired_devices',
                          location: 'onboarding-extras',
                        })
                        router.push('/settings/devices')
                      }}
                    >
                      {t('onboarding.checklist.showMe')}
                    </Button>
                  </div>
                  {/* Deliberately listed here rather than in app/upgrade/page.tsx: that
                      list is specifically the PREMIUM feature tour, and the
                      desktop_control_surfaces gate is seeded free (migration 089) because
                      the shipped plugin docs say both plugins are free. If the gate is ever
                      flipped to premium, move this entry into the /upgrade list and keep the
                      two in sync per the note above. */}
                  <p className="text-xs text-text-sub">{t('onboarding.extras.streamDeckBody')}</p>
                </li>
              </ul>
              <p className="mt-2 text-xs text-text-sub">
                <a
                  href={PATREON_JOIN_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={() =>
                    trackEvent('cta_click', { cta: 'premium', location: 'onboarding-extras' })
                  }
                  className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
                >
                  {t('onboarding.extras.seePremium')}
                </a>
              </p>
              <div className="mt-2 flex gap-2">
                <Button size="sm" variant="outline" onClick={() => markExtrasDone(false)}>
                  {t('onboarding.extras.done')}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => markExtrasDone(true)}>
                  {t('onboarding.extras.skip')}
                </Button>
              </div>
            </div>
          )}

          {showFinale && (
            <div className="rounded-lg border border-border bg-surface-2 p-3">
              <p className="text-sm font-semibold text-text">{t('onboarding.finale.title')}</p>
              <p className="mt-1 text-sm text-text-sub">{t('onboarding.finale.body')}</p>
              <div className="mt-2 flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  render={
                    <a
                      href={DISCORD_INVITE_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      onClick={() => trackEvent('onboarding_discord_clicked')}
                    >
                      {t('onboarding.finale.joinDiscord')}
                    </a>
                  }
                />
                <Button size="sm" variant="gradient" onClick={() => void finish()}>
                  {t('onboarding.finale.finish')}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

      <CreateOverlayDialog open={createOpen} onOpenChange={setCreateOpen} />

      <Dialog.Root open={confirmDismiss} onOpenChange={setConfirmDismiss}>
        <DialogContent size="sm">
          <DialogTitle>{t('onboarding.dismissDialog.title')}</DialogTitle>
          <DialogDescription>{t('onboarding.dismissDialog.body')}</DialogDescription>
          <div className="mt-4 flex justify-end gap-2">
            <Button variant="ghost" onClick={() => setConfirmDismiss(false)}>
              {t('onboarding.dismissDialog.keep')}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setConfirmDismiss(false)
                void dismiss(activeStep?.id ?? 'extras')
              }}
            >
              {t('onboarding.dismissDialog.hide')}
            </Button>
          </div>
        </DialogContent>
      </Dialog.Root>
    </section>
  )
}
