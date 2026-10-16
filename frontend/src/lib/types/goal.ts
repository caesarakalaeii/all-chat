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

/** Goal and list overlays (ADR-0072): configs and the frames that keep them live. */

/** Each goal metric and the one platform that has it (mirrors shared/alerts). */
export const GOAL_METRIC_PLATFORM = {
  followers: 'twitch',
  subscribers: 'youtube',
  bits: 'twitch',
  super_chat: 'youtube',
} as const

export type GoalMetric = keyof typeof GOAL_METRIC_PLATFORM

export const GOAL_METRICS = Object.keys(GOAL_METRIC_PLATFORM) as GoalMetric[]

export interface GoalConfig {
  metric: GoalMetric
  platform: string
  target: number
  starting_value: number
  title: string
}

/**
 * GET /overlays/:id/goal and /overlays/public/:id/goal. `status` is why the
 * last poll produced no count: "reconnect" (no usable credential) or
 * "hidden" (the channel hides its subscriber count); empty when fine.
 */
export interface GoalState {
  goal: GoalConfig
  configured: boolean
  current: number
  status: '' | 'reconnect' | 'hidden'
}

/** The `type: "goal"` WebSocket frame. */
export interface GoalFrame {
  type: 'goal'
  overlay_id: string
  goal: GoalMetric
  current: number
  target: number
  title: string
}

export const LIST_CATEGORIES = ['bits', 'gifts', 'super_chat'] as const
export type ListCategory = (typeof LIST_CATEGORIES)[number]
export const MAX_LIST_LIMIT = 25

export interface ListConfig {
  category: ListCategory
  limit: number
  title: string
}

export interface LeaderboardEntry {
  rank: number
  name: string
  platform: string
  amount: number
}

/** GET /overlays/:id/list/top. */
export interface ListTop {
  title: string
  category: ListCategory
  limit: number
  entries: LeaderboardEntry[]
}

/**
 * The `type: "list_update"` WebSocket frame: one board's top entries. Every
 * board of a list overlay is scored, so frames arrive for boards it does not
 * show; `limit` and `title` are set only when the owner saved the list, which
 * is the signal to switch to `category`.
 */
export interface ListUpdateFrame {
  type: 'list_update'
  overlay_id: string
  category: ListCategory
  entries: LeaderboardEntry[]
  limit?: number
  title?: string
}
