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
 * Shares API
 *
 * Functions for managing share requests and user search.
 * Communicates with the Share Service via API Gateway.
 */

import { apiClient } from './client'
import type { ShareRequest, UserSearchResult, AcceptedShare } from '../types/share'

export const sharesApi = {
  /**
   * Search for users by platform username
   */
  async searchUsers(platform: string, query: string): Promise<UserSearchResult[]> {
    const response = await apiClient.get<{ users: UserSearchResult[] }>(
      `/api/v1/users/search?platform=${encodeURIComponent(platform)}&query=${encodeURIComponent(query)}`
    )
    return response.users || []
  },

  /**
   * Create a share request
   */
  async createRequest(recipientUsername: string, overlayId: string): Promise<ShareRequest> {
    return apiClient.post<ShareRequest>('/api/v1/shares', {
      recipient_username: recipientUsername,
      overlay_id: overlayId,
    })
  },

  /**
   * Fetch incoming share requests
   */
  async fetchIncoming(status?: string): Promise<ShareRequest[]> {
    let url = '/api/v1/shares/incoming'
    if (status) {
      url += `?status=${encodeURIComponent(status)}`
    }

    const response = await apiClient.get<{ requests: ShareRequest[] }>(url)
    return response.requests || []
  },

  /**
   * Accept a share request
   */
  async acceptRequest(
    shareId: string,
    recipientOverlayId: string,
    expiryOption: 'this_stream' | 'custom' | 'unlimited',
    expiryHours?: number
  ): Promise<{ share: ShareRequest; sender_overlay_id: string }> {
    return apiClient.post(`/api/v1/shares/${shareId}/accept`, {
      recipient_overlay_id: recipientOverlayId,
      expiry_option: expiryOption,
      expiry_hours: expiryHours,
    })
  },

  /**
   * Get unseen acceptances for the current user (sender who hasn't seen acceptance notification)
   */
  async getUnseenAcceptances(): Promise<ShareRequest[]> {
    const response = await apiClient.get<{ requests: ShareRequest[] }>(
      '/api/v1/shares/unseen-acceptances'
    )
    return response.requests || []
  },

  /**
   * Mark a share request acceptance as seen by the sender
   */
  async markAcceptanceSeen(shareId: string): Promise<void> {
    await apiClient.post(`/api/v1/shares/${shareId}/mark-seen`, {})
  },

  /**
   * Get accepted shares where the current user is the recipient.
   * These are the shared overlays the user can add as a source to their own overlays.
   */
  async getAcceptedShares(): Promise<AcceptedShare[]> {
    const response = await apiClient.get<{ shares: AcceptedShare[] }>('/api/v1/shares/accepted')
    return response.shares || []
  },

  /**
   * Revoke an active share (either participant can call)
   */
  async revokeShare(shareId: string): Promise<void> {
    await apiClient.post(`/api/v1/shares/${shareId}/revoke`, {})
  },
}
