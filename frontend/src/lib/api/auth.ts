/**
 * Authentication API
 *
 * Functions for interacting with the Auth Service.
 * Handles login, token management, and user info.
 */

import { apiClient } from './client';
import type { User, LoginResponse } from '../types/auth';

export const authApi = {
  /**
   * Get the OAuth login URL for a supported platform
   */
  async getLoginUrl(platform: 'twitch' | 'youtube' | 'kick' = 'twitch'): Promise<string> {
    const response = await apiClient.get<LoginResponse>(`/api/v1/auth/${platform}/login`);
    return response.auth_url;
  },

  /**
   * Get current user information using stored JWT token
   */
  async getMe(): Promise<User> {
    return apiClient.get<User>('/api/v1/auth/me');
  },

  /**
   * Logout (invalidate token on server)
   */
  async logout(): Promise<void> {
    await apiClient.post('/api/v1/auth/logout', {});
  },

  /**
   * Delete the authenticated user's account
   */
  async deleteAccount(): Promise<void> {
    await apiClient.delete('/api/v1/auth/me');
  }
};
