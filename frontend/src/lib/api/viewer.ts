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
 * Viewer API
 *
 * Functions for viewer (chat participant) operations.
 * Viewers authenticate with their own platform accounts to send messages.
 */

import type { ViewerInfo } from '../types/viewer'
import type { PaymentStatus } from './payment'
import type { Poll, Prediction, ViewerEngagement } from '../types/engagement'
import { inMemoryTokens } from '../auth/in-memory-store'
import { safeExternalRedirect } from '../auth/redirect-allowlist'

/**
 * Custom API client for viewer requests
 * Uses a separate token key to avoid conflicts with streamer auth
 */
class ViewerApiClient {
  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    // Get viewer token — prefer in-memory store (audit H3), fall back to
    // localStorage for legacy compatibility during the cookie migration.
    let token: string | null = null
    if (typeof window !== 'undefined') {
      token = inMemoryTokens.getViewerAccessToken() ?? localStorage.getItem('viewer_jwt_token')
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const API_URL = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080'
    const url = endpoint.startsWith('http') ? endpoint : `${API_URL}${endpoint}`

    const response = await fetch(url, {
      ...options,
      headers,
    })

    if (response.status === 401) {
      // Viewer token expired or invalid, clear it
      inMemoryTokens.setViewerAccessToken(null)
      if (typeof window !== 'undefined') {
        localStorage.removeItem('viewer_jwt_token')
      }
      throw new Error('Unauthorized')
    }

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      // Throw error with response and data attached for smart parsing
      const error = new Error(errorData.error || errorData.message || response.statusText)
      ;(error as any).response = response
      ;(error as any).data = errorData
      throw error
    }

    return response
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint)
    return response.json()
  }

  async post<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(data),
    })
    return response.json()
  }

  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' })
  }
}

const viewerApiClient = new ViewerApiClient()

/**
 * apiErrorReason extracts the machine-readable `reason` code the API attaches to a
 * rejected request body (e.g. a wager rejection: {error, reason:"insufficient"}). The
 * client puts the parsed body on `error.data`; this reads it without an `any` cast so
 * callers can map the reason to human copy (issue #523, L-U2).
 */
export function apiErrorReason(err: unknown): string | undefined {
  if (err && typeof err === 'object' && 'data' in err) {
    const data = (err as { data?: unknown }).data
    if (data && typeof data === 'object' && 'reason' in data) {
      const reason = (data as { reason?: unknown }).reason
      if (typeof reason === 'string') return reason
    }
  }
  return undefined
}

export const viewerApi = {
  /**
   * Get current viewer information using stored JWT token
   */
  async getMe(): Promise<ViewerInfo> {
    return viewerApiClient.get<ViewerInfo>('/api/v1/auth/viewer/me')
  },

  /**
   * Logout viewer (invalidate token on server)
   */
  async logout(): Promise<void> {
    await viewerApiClient.post('/api/v1/auth/viewer/logout', {})
  },

  /**
   * Viewer premium (ADR-0019): the viewer-scoped Patreon connection, authenticated
   * with the viewer JWT. Mirrors the streamer payment API but grants the cheaper
   * viewer cosmetic product.
   */
  async getPremiumStatus(): Promise<PaymentStatus> {
    return viewerApiClient.get<PaymentStatus>('/api/v1/payment/viewer/status')
  },

  /**
   * Fetch the Patreon consent URL and redirect the browser to it.
   */
  async startPatreonConnect(): Promise<void> {
    const data = await viewerApiClient.get<{ auth_url: string }>(
      '/api/v1/payment/viewer/patreon/connect'
    )
    if (data.auth_url) {
      // SECURITY (audit L32): validate the redirect host before navigating.
      safeExternalRedirect(data.auth_url)
    }
  },

  async disconnectPatreon(): Promise<void> {
    await viewerApiClient.delete('/api/v1/payment/viewer/patreon/connection')
  },

  // --- Engagement (issue #523): viewer-scoped polls/predictions/points ---

  /**
   * Private per-viewer snapshot for an overlay economy: balance (+ points name)
   * plus the viewer's current vote and wager. Pull-first delivery (the overlay
   * WebSocket is broadcast-only and can't carry per-viewer data).
   */
  async getEngagement(overlayId: string): Promise<ViewerEngagement> {
    return viewerApiClient.get<ViewerEngagement>(
      `/api/v1/engagement/viewers/me/engagement?overlay_id=${encodeURIComponent(overlayId)}`
    )
  },

  /** Cast/change a poll vote from the web page (option is 1-based). Returns the updated poll. */
  async votePoll(overlayId: string, pollId: string, optionIdx: number): Promise<Poll> {
    return viewerApiClient.post<Poll>(
      `/api/v1/engagement/overlays/${overlayId}/polls/${pollId}/vote`,
      { option_idx: optionIdx }
    )
  },

  /** Place a points wager on a prediction outcome (outcome is 1-based). */
  async wagerPrediction(
    overlayId: string,
    predictionId: string,
    outcomeIdx: number,
    amount: number
  ): Promise<{ balance: number; prediction?: Prediction }> {
    return viewerApiClient.post<{ balance: number; prediction?: Prediction }>(
      `/api/v1/engagement/overlays/${overlayId}/predictions/${predictionId}/wager`,
      { outcome_idx: outcomeIdx, amount }
    )
  },

  /** Watch-time heartbeat (awards points, deduped per minute server-side). */
  async engagementHeartbeat(overlayId: string): Promise<{ balance: number }> {
    return viewerApiClient.post<{ balance: number }>('/api/v1/engagement/viewers/me/heartbeat', {
      overlay_id: overlayId,
    })
  },
}
