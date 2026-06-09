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

/**
 * Custom API client for viewer requests
 * Uses a separate token key to avoid conflicts with streamer auth
 */
class ViewerApiClient {
  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    // Get viewer token from localStorage (different from streamer token)
    let token: string | null = null
    if (typeof window !== 'undefined') {
      token = localStorage.getItem('viewer_jwt_token')
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
}

const viewerApiClient = new ViewerApiClient()

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
}
