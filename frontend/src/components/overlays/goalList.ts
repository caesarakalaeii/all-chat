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

/** Pure state updates behind the goal bar and the leaderboard list (ADR-0072). */

import {
  GOAL_METRIC_PLATFORM,
  type GoalFrame,
  type GoalState,
  type ListTop,
  type ListUpdateFrame,
} from '@/lib/types/goal'

/** Progress toward target as 0..100; a non-positive target is 0. */
export function goalPercent(current: number, target: number): number {
  if (!(target > 0)) return 0
  return Math.min(100, Math.max(0, (current / target) * 100))
}

/**
 * A goal frame is the whole truth about the goal: the alert-processor sends
 * one per poll or counted event, overlay-manager one per save (which may
 * switch the metric).
 */
export function applyGoalFrame(state: GoalState, frame: GoalFrame): GoalState {
  return {
    ...state,
    current: frame.current,
    goal: {
      ...state.goal,
      metric: frame.goal,
      platform: GOAL_METRIC_PLATFORM[frame.goal] ?? state.goal.platform,
      target: frame.target,
      title: frame.title,
    },
  }
}

/**
 * A list_update carries the top of one board; the overlay shows only its own
 * board, ranked, cut to its configured limit. A frame from the owner's save
 * (it carries `limit`) switches the board, limit and title.
 */
export function applyListUpdate(top: ListTop, frame: ListUpdateFrame): ListTop {
  const saved = frame.limit !== undefined
  if (!saved && frame.category !== top.category) return top
  const next = saved
    ? { ...top, category: frame.category, limit: frame.limit!, title: frame.title ?? '' }
    : top
  const entries = [...frame.entries].sort((a, b) => a.rank - b.rank).slice(0, next.limit)
  return { ...next, entries }
}
