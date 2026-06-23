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
 * API Client
 *
 * Base HTTP client for communicating with the All-Chat API Gateway.
 * Handles authentication, error handling, and request formatting.
 *
 * All API requests are proxied through Nginx at /api/* paths.
 * This provides same-origin requests (no CORS), SSL termination,
 * and better security by keeping backend services internal.
 *
 * Usage:
 *   const data = await apiClient.get<ResponseType>('/api/v1/endpoint');
 *   const result = await apiClient.post('/api/v1/endpoint', { data });
 */

// In production: /api/* is proxied to API Gateway by Nginx
// In development: use NEXT_PUBLIC_API_URL or localhost
function getApiUrl(): string {
  if (typeof window !== 'undefined') {
    // Browser: use same origin (Nginx will proxy /api/* to backend)
    return window.location.origin
  }
  // SSR: use env var or localhost for development
  return process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
}

const API_URL = getApiUrl()

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    /**
     * The parsed JSON error body, when the server returned one. Lets callers
     * inspect contract-specific fields (e.g. the chat-send endpoint's `error`
     * discriminant, `platform`, `details`, `retry_after_seconds`) instead of
     * only the flattened `message`. Undefined if the body wasn't JSON.
     */
    public data?: Record<string, unknown>
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

class ApiClient {
  private async fetch(endpoint: string, options: RequestInit = {}): Promise<Response> {
    // Get token from localStorage (client-side only)
    let token: string | null = null
    if (typeof window !== 'undefined') {
      token = localStorage.getItem('jwt_token')
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    const url = endpoint.startsWith('http') ? endpoint : `${API_URL}${endpoint}`

    const response = await fetch(url, {
      ...options,
      headers,
    })

    if (!response.ok) {
      const errorData: Record<string, unknown> = await response
        .json()
        .catch(() => ({ error: 'Unknown error' }))
      const errorValue = typeof errorData.error === 'string' ? errorData.error : undefined

      if (response.status === 401) {
        // A `reauth_required` 401 is an application-level signal (the platform
        // OAuth token was revoked) that the caller surfaces inline — it must NOT
        // trigger the generic "JWT expired → log out" path. Every other 401 means
        // our own session token is invalid: clear it and bounce to the landing page.
        if (errorValue !== 'reauth_required' && typeof window !== 'undefined') {
          localStorage.removeItem('jwt_token')
          window.location.href = '/'
        }
        throw new ApiError(401, errorValue || 'Unauthorized', errorData)
      }

      throw new ApiError(response.status, errorValue || response.statusText, errorData)
    }

    return response
  }

  async get<T>(endpoint: string): Promise<T> {
    const response = await this.fetch(endpoint)
    return response.json()
  }

  async post<T>(endpoint: string, data: unknown, headers?: Record<string, string>): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'POST',
      body: JSON.stringify(data),
      ...(headers ? { headers } : {}),
    })
    return response.json()
  }

  async put<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data),
    })
    return response.json()
  }

  async patch<T>(endpoint: string, data: unknown): Promise<T> {
    const response = await this.fetch(endpoint, {
      method: 'PATCH',
      body: JSON.stringify(data),
    })
    return response.json()
  }

  async delete(endpoint: string): Promise<void> {
    await this.fetch(endpoint, { method: 'DELETE' })
  }
}

export const apiClient = new ApiClient()
