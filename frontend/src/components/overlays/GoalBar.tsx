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

'use client'

/**
 * The goal overlay renderer (issue #955, ADR-0072): a progress bar toward one
 * follower, subscriber, bits or Super Chat goal. Initial state comes from the
 * public goal route; `type: "goal"` frames (polls, counted events, owner
 * saves) keep it live. OBS-transparent: only the bar paints.
 */

import { useCallback, useState } from 'react'

import { formatNumber, useTranslations } from '@/lib/i18n'
import type { GoalFrame, GoalState } from '@/lib/types/goal'

import { applyGoalFrame, goalPercent } from './goalList'

/**
 * The goal's state, the onGoal callback for useOverlayStream, and seed for
 * the initial public read (usePublicSeed).
 */
export function useGoal() {
  const [state, setState] = useState<GoalState | null>(null)

  // A frame that arrived before the read is newer than it: keep the frame.
  const seed = useCallback((loaded: GoalState) => setState((prev) => prev ?? loaded), [])

  const onGoal = useCallback((frame: GoalFrame) => {
    setState((prev) =>
      applyGoalFrame(
        prev ?? {
          goal: { metric: frame.goal, platform: '', target: 0, starting_value: 0, title: '' },
          configured: true,
          current: 0,
          status: '',
        },
        frame
      )
    )
  }, [])

  return { state, onGoal, seed }
}

export function GoalBar({ state }: { state: GoalState | null }) {
  const t = useTranslations()
  if (!state) return null
  const { goal, current } = state
  const title = goal.title || t(`viewerOverlay.goalTitles.${goal.metric}`)
  const percent = goalPercent(current, goal.target)
  const progress = t('viewerOverlay.goal.progress', {
    current: formatNumber(current),
    target: formatNumber(goal.target),
  })

  return (
    <div className="flex h-screen w-full items-center justify-center overflow-hidden bg-transparent p-6">
      <div className="w-full max-w-3xl text-white drop-shadow-lg">
        <div className="mb-2 flex items-baseline justify-between gap-4">
          <span className="truncate text-2xl font-bold">{title}</span>
          {/* Keyed by the value: each change replays the flash. */}
          <span
            key={current}
            className="shrink-0 animate-in text-xl font-semibold tabular-nums fade-in zoom-in-110"
          >
            {progress}
          </span>
        </div>
        <div
          role="progressbar"
          aria-label={title}
          aria-valuemin={0}
          aria-valuemax={goal.target}
          aria-valuenow={current}
          aria-valuetext={progress}
          className="h-8 overflow-hidden rounded-full bg-black/40 ring-2 ring-white/70"
        >
          <div
            className="h-full rounded-full bg-primary transition-[width] duration-700 ease-out"
            style={{ width: `${percent}%` }}
          />
        </div>
      </div>
    </div>
  )
}
