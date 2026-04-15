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
 * Viewer Authentication Types
 *
 * Types for viewer (chat participant) authentication and messaging.
 * Viewers are different from streamers - they authenticate to send messages.
 */

export interface ViewerInfo {
  session_id: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  platform_user_id: string
  username: string
  display_name?: string
  avatar_url?: string
}

export interface ViewerAuthResponse {
  token: string
  expires_in: number
  viewer_info: ViewerInfo
}

export interface ViewerLoginResponse {
  auth_url: string
}

export interface StreamerInfo {
  username: string
  display_name: string
  platforms: StreamerPlatform[]
  overlay_id?: string
}

export interface StreamerPlatform {
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  is_active: boolean
}

export interface SendMessageRequest {
  streamer_username: string
  message: string
  platform?: string
}

export interface StreamerSendMessageRequest {
  message: string
  platform: string
}

export interface SendMessageResponse {
  success: boolean
  message: string
}

export interface ViewerAuthState {
  viewerInfo: ViewerInfo | null
  viewerToken: string | null
  loading: boolean
  streamer: string | null
}
