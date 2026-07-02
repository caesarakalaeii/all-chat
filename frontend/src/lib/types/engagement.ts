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

// Types for the engagement feature (polls, predictions, points — issue #523).
// These mirror the JSON returned by engagement-service (services/engagement-service/models).

export type EngagementSource = 'allchat' | 'twitch_native'
export type PollState = 'ACTIVE' | 'CLOSED'
export type PredictionState = 'CREATED' | 'ACTIVE' | 'LOCKED' | 'RESOLVED' | 'CANCELED'

export interface PollOption {
  id: string
  idx: number
  label: string
  votes: number
}

export interface Poll {
  id: string
  overlay_id: string
  source: EngagementSource
  external_id?: string
  question: string
  state: PollState
  allow_change: boolean
  options: PollOption[]
  created_at: string
  ends_at?: string
  closed_at?: string
}

export interface PredictionOutcome {
  id: string
  idx: number
  label: string
  color?: string
  total_points: number
  entrants: number
}

export interface Prediction {
  id: string
  overlay_id: string
  source: EngagementSource
  external_id?: string
  title: string
  state: PredictionState
  winning_outcome_id?: string
  outcomes: PredictionOutcome[]
  auto_lock_at?: string
  created_at: string
  locked_at?: string
  resolved_at?: string
}

/** Private per-viewer snapshot from GET /viewers/me/engagement (web page / extension). */
export interface ViewerEngagement {
  points_name: string
  balance: number
  voted_option_id?: string
  wager_outcome_id?: string
  wager_amount?: number
}
