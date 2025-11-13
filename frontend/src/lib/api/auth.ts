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
   * Get the Twitch OAuth login URL
   */
  async getLoginUrl(): Promise<string> {
    const response = await apiClient.get<LoginResponse>('/api/v1/auth/login');
    return response.url;
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
  }
};
