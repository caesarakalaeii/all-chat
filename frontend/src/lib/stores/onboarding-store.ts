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
 * Onboarding setup-guide store (retention initiative).
 *
 * The guide is a persistent CHECKLIST, not a modal wizard: connecting
 * Twitch/YouTube/Kick is a full-window OAuth redirect that would destroy
 * modal state, so the core steps are DERIVED from server data (overlay
 * exists → source exists → theme picked) and trivially survive navigation.
 * Only the two signals the server cannot observe (OBS link copied, extras
 * step done) live client-side, mirrored to localStorage so the OAuth
 * round-trip in the same browser doesn't repeat them.
 *
 * Server truth: users.onboarding_completed_at (NULL = guide shows).
 * finish()/dismiss() persist via PATCH /api/v1/auth/me/onboarding and then
 * refresh the auth store; the guide is re-armed from Settings with
 * start('settings') after the API clears the flag.
 */

import { create } from 'zustand'
import { authApi } from '../api/auth'
import { useAuthStore } from './auth-store'
import { trackEvent } from '../analytics'

export type OnboardingStepId = 'create_overlay' | 'connect_source' | 'choose_theme' | 'copy_obs'

export interface OnboardingStep {
  id: OnboardingStepId
  done: boolean
  /** The first not-done step is the active one; later steps wait for it. */
  active: boolean
}

export interface DeriveStepsInput {
  overlayCount: number
  sourceCount: number
  themeId: string | null
  obsCopied: boolean
}

/**
 * Pure derivation of the core checklist from observable state. Steps are
 * strictly ordered; regressions (e.g. the only overlay was deleted) simply
 * re-activate the earlier step.
 */
export function deriveSteps(input: DeriveStepsInput): OnboardingStep[] {
  const done: Record<OnboardingStepId, boolean> = {
    create_overlay: input.overlayCount > 0,
    connect_source: input.sourceCount > 0,
    choose_theme: input.themeId !== null && input.themeId !== '',
    copy_obs: input.obsCopied,
  }
  const order: OnboardingStepId[] = ['create_overlay', 'connect_source', 'choose_theme', 'copy_obs']
  const firstOpen = order.find((id) => !done[id])
  return order.map((id) => ({ id, done: done[id], active: id === firstOpen }))
}

interface SessionSteps {
  obsCopied: boolean
  extrasDone: boolean
}

const storageKey = (userId: string) => `onboarding-v1:${userId}`

function readSessionSteps(userId: string): SessionSteps {
  try {
    const raw = localStorage.getItem(storageKey(userId))
    if (!raw) return { obsCopied: false, extrasDone: false }
    return { obsCopied: false, extrasDone: false, ...(JSON.parse(raw) as Partial<SessionSteps>) }
  } catch {
    return { obsCopied: false, extrasDone: false }
  }
}

function writeSessionSteps(userId: string, steps: SessionSteps): void {
  try {
    localStorage.setItem(storageKey(userId), JSON.stringify(steps))
  } catch {
    // localStorage unavailable — the flow still works, steps just re-show.
  }
}

interface OnboardingStore {
  status: 'inactive' | 'active'
  trigger: 'auto' | 'settings' | null
  /** The overlay steps 2-4 bind to: last one visited in the editor. */
  activeOverlayId: string | null
  sessionSteps: SessionSteps
  minimized: boolean
  /** Analytics double-fire guard: step ids already reported as completed. */
  reportedSteps: OnboardingStepId[]

  start: (trigger: 'auto' | 'settings') => void
  minimize: (minimized: boolean) => void
  setActiveOverlay: (id: string) => void
  markObsCopied: () => void
  markExtrasDone: (skipped: boolean) => void
  reportStepCompleted: (step: OnboardingStepId) => void
  /** Dismiss = "don't show again" (restartable from Settings). */
  dismiss: (currentStep: OnboardingStepId | 'extras') => Promise<void>
  finish: () => Promise<void>
}

function currentUserId(): string {
  return useAuthStore.getState().user?.id ?? 'anonymous'
}

/**
 * Persist completed=true with one retry. The UI closes optimistically; if
 * both attempts fail the flag stays NULL server-side and the guide re-shows
 * on the next visit — annoying but never data-lossy.
 */
async function persistCompleted(): Promise<void> {
  try {
    await authApi.updateOnboarding(true)
  } catch {
    try {
      await authApi.updateOnboarding(true)
    } catch {
      return
    }
  }
  // Refresh /auth/me so onboarding_completed_at is current everywhere.
  await useAuthStore.getState().init()
}

export const useOnboardingStore = create<OnboardingStore>((set, get) => ({
  status: 'inactive',
  trigger: null,
  activeOverlayId: null,
  sessionSteps: { obsCopied: false, extrasDone: false },
  minimized: false,
  reportedSteps: [],

  start: (trigger) => {
    if (get().status === 'active') return
    set({
      status: 'active',
      trigger,
      minimized: false,
      reportedSteps: [],
      sessionSteps: readSessionSteps(currentUserId()),
    })
    trackEvent('onboarding_started', { trigger })
  },

  minimize: (minimized) => set({ minimized }),

  setActiveOverlay: (id) => set({ activeOverlayId: id }),

  markObsCopied: () => {
    const next = { ...get().sessionSteps, obsCopied: true }
    writeSessionSteps(currentUserId(), next)
    set({ sessionSteps: next })
    get().reportStepCompleted('copy_obs')
  },

  markExtrasDone: (skipped) => {
    const next = { ...get().sessionSteps, extrasDone: true }
    writeSessionSteps(currentUserId(), next)
    set({ sessionSteps: next })
    if (skipped) trackEvent('onboarding_step_skipped', { step: 'extras' })
    else trackEvent('onboarding_step_completed', { step: 'extras' })
  },

  reportStepCompleted: (step) => {
    if (get().status !== 'active' || get().reportedSteps.includes(step)) return
    set({ reportedSteps: [...get().reportedSteps, step] })
    trackEvent('onboarding_step_completed', { step })
  },

  dismiss: async (currentStep) => {
    trackEvent('onboarding_dismissed', { step: currentStep })
    set({ status: 'inactive', trigger: null })
    await persistCompleted()
  },

  finish: async () => {
    trackEvent('onboarding_finished')
    set({ status: 'inactive', trigger: null })
    await persistCompleted()
  },
}))
