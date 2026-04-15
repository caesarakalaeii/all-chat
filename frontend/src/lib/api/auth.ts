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
 * Authentication API
 *
 * Functions for interacting with the Auth Service.
 * Handles login, token management, and user info.
 */

import { apiClient } from './client'
import type { User, LoginResponse } from '../types/auth'

export const authApi = {
  /**
   * Get the OAuth login URL for a supported platform
   */
  async getLoginUrl(platform: 'twitch' | 'youtube' | 'kick' = 'twitch'): Promise<string> {
    const response = await apiClient.get<LoginResponse>(`/api/v1/auth/${platform}/login`)
    return response.auth_url
  },

  /**
   * Get current user information using stored JWT token
   */
  async getMe(): Promise<User> {
    return apiClient.get<User>('/api/v1/auth/me')
  },

  /**
   * Logout (invalidate token on server)
   */
  async logout(): Promise<void> {
    await apiClient.post('/api/v1/auth/logout', {})
  },

  /**
   * Delete the authenticated user's account
   */
  async deleteAccount(): Promise<void> {
    await apiClient.delete('/api/v1/auth/me')
  },
}
