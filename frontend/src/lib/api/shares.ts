/**
 * Shares API
 *
 * Functions for managing share requests and user search.
 * Communicates with the Share Service via API Gateway.
 */

import { apiClient } from './client';
import type { ShareRequest, UserSearchResult } from '../types/share';

export const sharesApi = {
  /**
   * Search for users by platform username
   */
  async searchUsers(platform: string, query: string): Promise<UserSearchResult[]> {
    const response = await apiClient.get<{ users: UserSearchResult[] }>(
      `/api/v1/users/search?platform=${encodeURIComponent(platform)}&query=${encodeURIComponent(query)}`
    );
    return response.users || [];
  },

  /**
   * Create a share request
   */
  async createRequest(recipientUsername: string, overlayId: string): Promise<ShareRequest> {
    return apiClient.post<ShareRequest>('/api/v1/shares', {
      recipient_username: recipientUsername,
      overlay_id: overlayId,
    });
  },

  /**
   * Fetch incoming share requests
   */
  async fetchIncoming(status?: string): Promise<ShareRequest[]> {
    let url = '/api/v1/shares/incoming';
    if (status) {
      url += `?status=${encodeURIComponent(status)}`;
    }

    const response = await apiClient.get<{ requests: ShareRequest[] }>(url);
    return response.requests || [];
  },
};
