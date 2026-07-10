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
 * Streamer-side engagement API (issue #523): the earn/points config plus the
 * poll & prediction controls for an overlay's owner. Mirrors lib/api/moderation.ts:
 * thin wrappers over the shared cookie-authed `apiClient`; every mutating call
 * sends a fresh `Idempotency-Key` so a double-click dedupes server-side.
 * Viewer-side participation (vote/wager/balance) lives in lib/api/viewer.ts.
 */

import { apiClient } from './client'
import type {
  CreatePollRequest,
  CreatePredictionRequest,
  EarnConfig,
  Poll,
  Prediction,
} from '@/lib/types/engagement'

/** A fresh idempotency header for one mutating request. */
function idempotencyHeader(): Record<string, string> {
  return { 'Idempotency-Key': crypto.randomUUID() }
}

/** Response of the Twitch mirror re-consent endpoint (auth-service). */
interface ConsentUrlResponse {
  auth_url: string
}

export const engagementApi = {
  getConfig(overlayId: string): Promise<EarnConfig> {
    return apiClient.get<EarnConfig>(`/api/v1/engagement/overlays/${overlayId}/points/config`)
  },

  /** Full upsert — the service stores the body as-is, so always send the complete object. */
  updateConfig(overlayId: string, config: EarnConfig): Promise<EarnConfig> {
    return apiClient.put<EarnConfig>(
      `/api/v1/engagement/overlays/${overlayId}/points/config`,
      config
    )
  },

  createPoll(overlayId: string, body: CreatePollRequest): Promise<Poll> {
    return apiClient.post<Poll>(
      `/api/v1/engagement/overlays/${overlayId}/polls`,
      body,
      idempotencyHeader()
    )
  },

  closePoll(overlayId: string, pollId: string): Promise<Poll> {
    return apiClient.post<Poll>(
      `/api/v1/engagement/overlays/${overlayId}/polls/${pollId}/close`,
      {},
      idempotencyHeader()
    )
  },

  createPrediction(overlayId: string, body: CreatePredictionRequest): Promise<Prediction> {
    return apiClient.post<Prediction>(
      `/api/v1/engagement/overlays/${overlayId}/predictions`,
      body,
      idempotencyHeader()
    )
  },

  lockPrediction(overlayId: string, predictionId: string): Promise<Prediction> {
    return apiClient.post<Prediction>(
      `/api/v1/engagement/overlays/${overlayId}/predictions/${predictionId}/lock`,
      {},
      idempotencyHeader()
    )
  },

  /**
   * Pay out to the winning outcome. Only valid while LOCKED — the service treats a
   * resolve on a still-ACTIVE prediction as an idempotent no-op (it returns the
   * unchanged prediction), so lock first and check the returned state.
   */
  resolvePrediction(
    overlayId: string,
    predictionId: string,
    winningOutcomeId: string
  ): Promise<Prediction> {
    return apiClient.post<Prediction>(
      `/api/v1/engagement/overlays/${overlayId}/predictions/${predictionId}/resolve`,
      { winning_outcome_id: winningOutcomeId },
      idempotencyHeader()
    )
  },

  /** Cancel an ACTIVE or LOCKED prediction; every stake is refunded. */
  cancelPrediction(overlayId: string, predictionId: string): Promise<Prediction> {
    return apiClient.post<Prediction>(
      `/api/v1/engagement/overlays/${overlayId}/predictions/${predictionId}/cancel`,
      {},
      idempotencyHeader()
    )
  },

  /**
   * Fetch the Twitch OAuth re-consent URL for the opt-in mirroring of native
   * Twitch polls/predictions onto the overlay. The grant adds channel:read:polls +
   * channel:read:predictions — least privilege, additive to any existing grant.
   * The endpoint is on the auth-service (which applies its own JWT); the caller
   * redirects the browser to the returned auth_url, and Twitch sends the user
   * back to /overlay/{id}/view afterwards.
   */
  async getTwitchMirrorConsentUrl(overlayId: string): Promise<string> {
    const res = await apiClient.get<ConsentUrlResponse>(
      `/api/v1/auth/twitch/moderation/${overlayId}?actions=engagement`
    )
    return res.auth_url
  },
}
