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
import type { ChatBotPlatform, ChatBotStatus } from '../types/engagement'

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
   * Admin impersonates a user. Server sets an impersonated-user access cookie.
   * Returns the impersonated user (no token — cookie is httpOnly).
   */
  async impersonate(targetUserId: string): Promise<{
    user: { id: string; username: string; display_name?: string }
    impersonating: boolean
  }> {
    return apiClient.post(`/api/v1/admin/users/${targetUserId}/impersonate`, {})
  },

  /**
   * Stop impersonating; server restores the admin access cookie.
   * Returns the restored admin user.
   */
  async stopImpersonation(): Promise<{
    user: { id: string; username: string; is_admin: boolean }
  }> {
    return apiClient.post('/api/v1/auth/stop-impersonation', {})
  },

  /**
   * Delete the authenticated user's account
   */
  async deleteAccount(): Promise<void> {
    await apiClient.delete('/api/v1/auth/me')
  },

  /**
   * Set (true = finished/dismissed) or clear (false = restart from Settings)
   * the first-run setup guide state. Rejected with 403 while impersonating.
   */
  async updateOnboarding(completed: boolean): Promise<{ onboarding_completed_at: string | null }> {
    return apiClient.patch('/api/v1/auth/me/onboarding', { completed })
  },

  // --- Chat bots (ADR-0081) ---

  /** Who responds for a streamer's chat commands, and which bot slots are filled. */
  getChatBotsStatus(): Promise<ChatBotStatus> {
    return apiClient.get<ChatBotStatus>('/api/v1/auth/chat-bots')
  },

  /**
   * Connect a bot account. `scope=custom` is the premium per-owner bot, `allchat` is
   * admin-only. Kick supports custom only; the All-Chat bot there is the one on the
   * Kick app. The caller navigates the browser to the returned auth_url.
   */
  async getChatBotConnectUrl(
    platform: ChatBotPlatform,
    scope: 'custom' | 'allchat'
  ): Promise<string> {
    const res = await apiClient.get<{ auth_url: string }>(
      `/api/v1/auth/${platform}/bot-connect?scope=${scope}`
    )
    return res.auth_url
  },

  /** Disconnect a custom (owner) or allchat (admin) bot account on a platform. */
  async disconnectChatBot(platform: ChatBotPlatform, scope: 'custom' | 'allchat'): Promise<void> {
    await apiClient.delete(`/api/v1/auth/chat-bots/${scope}/${platform}`)
  },

  /**
   * The broadcaster's channel:bot grant for Twitch bot replies, through the same
   * overlay-scoped re-consent the clip grant on /commands uses; Twitch sends the user
   * back to /commands.
   */
  async getChatBotGrantUrl(overlayId: string): Promise<string> {
    const res = await apiClient.get<LoginResponse>(
      `/api/v1/auth/twitch/moderation/${overlayId}?actions=bot&return=commands`
    )
    return res.auth_url
  },
}
