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

// M8 (code part): cookie auth (SameSite=Lax + credentials:'same-origin') only
// works when the frontend and the API gateway share an origin. A misconfigured
// NEXT_PUBLIC_API_URL (cross-origin) silently breaks login with no obvious
// error. Warn loudly in dev so the invariant is caught early; suppressed in
// production (prod ingress already serves both from allch.at).
if (
  typeof window !== 'undefined' &&
  window.location.origin !== API_URL &&
  process.env.NODE_ENV !== 'production'
) {
  console.warn(
    '[AllChat] API base origin differs from window origin — cookie auth requires same-origin. ' +
      'Set NEXT_PUBLIC_API_URL or serve both from the same origin.'
  )
}

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

/**
 * Outcome of a cookie-refresh attempt.
 * - 'refreshed': /refresh returned 200; the access cookie was rotated.
 * - 'concurrent': /refresh returned 409 — another in-flight refresh (e.g. a
 *   second tab) is rotating the same token. Its Set-Cookie updates the shared
 *   cookie jar, so the caller should retry the original request rather than
 *   treat this as a hard failure (PR #478 review M1).
 * - 'failed': refresh failed; the caller should bounce to login.
 */
type RefreshResult = 'refreshed' | 'concurrent' | 'failed'

class ApiClient {
  private refreshPromise: Promise<RefreshResult> | null = null

  private async fetch(
    endpoint: string,
    options: RequestInit = {},
    retried = false
  ): Promise<Response> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string>),
    }

    const url = endpoint.startsWith('http') ? endpoint : `${API_URL}${endpoint}`
    const response = await fetch(url, { ...options, headers, credentials: 'same-origin' })

    // L7: the guard prefixes must match the real endpoints (/api/v1/auth/...),
    // not the dead /auth/... prefixes that never matched — otherwise 401s on
    // /auth/refresh & /auth/login would wrongly trigger the refresh→retry path.
    if (
      response.status === 401 &&
      !endpoint.startsWith('/api/v1/auth/refresh') &&
      !endpoint.startsWith('/api/v1/auth/login')
    ) {
      // Try to determine if this is a `reauth_required` 401 (platform OAuth token
      // revoked) — that's an application-level signal the caller surfaces inline;
      // it must NOT trigger the generic refresh→retry path.
      let errorValue: string | undefined
      try {
        const errorData = await response.clone().json().catch(() => ({} as Record<string, unknown>))
        errorValue = typeof (errorData as Record<string, unknown>).error === 'string'
          ? (errorData as Record<string, unknown>).error as string
          : undefined
      } catch {
        errorValue = undefined
      }
      if (errorValue !== 'reauth_required') {
        // Try one cookie-based refresh, then retry the original request once.
        const result = await this.tryRefresh()
        if (result === 'refreshed' || result === 'concurrent') {
          // M2: route the retry back through this.fetch (not a bare fetch) so the
          // retried response goes through the same !response.ok / ApiError /
          // reauth_required handling. The `retried` guard prevents infinite
          // recursion if the retried request also returns 401.
          if (!retried) {
            if (result === 'concurrent') {
              // A concurrent refresh (another tab) is rotating the cookie; give it
              // a brief moment to land its Set-Cookie, then retry with the fresh
              // cookie instead of force-logging-out (PR #478 review M1).
              await new Promise((resolve) => setTimeout(resolve, 400))
            }
            return this.fetch(endpoint, options, true)
          }
          // Already retried — fall through to the generic error path below so a
          // second 401/403/500 surfaces as a structured ApiError instead of a raw
          // SyntaxError from the caller's .json().
        }
        // refresh failed (or retry exhausted). Do NOT navigate for the auth probe
        // (/api/v1/auth/me): init() runs it on EVERY page including the public
        // landing page, so a self-redirect to '/' creates an infinite full-page
        // reload loop for logged-out visitors (they can never sign in). Let the
        // caller drive navigation instead — init() catches and sets user:null, and
        // ProtectedRoute pushes '/' via the SPA router when user is null (audit #1).
        if (typeof window !== 'undefined' && !endpoint.startsWith('/api/v1/auth/me')) {
          window.location.href = '/'
        }
        throw new ApiError(401, errorValue || 'Unauthorized', { error: errorValue || 'Unauthorized' })
      }
      // reauth_required: fall through to the generic error path (no refresh, no redirect)
    }

    if (!response.ok) {
      const errorData: Record<string, unknown> = await response.json().catch(() => ({ error: 'Unknown error' }))
      const errorValue = typeof errorData.error === 'string' ? errorData.error : undefined
      throw new ApiError(response.status, errorValue || response.statusText, errorData)
    }

    return response
  }

  private async tryRefresh(): Promise<RefreshResult> {
    if (this.refreshPromise) return this.refreshPromise
    this.refreshPromise = (async () => {
      try {
        const r = await fetch(`${API_URL}/api/v1/auth/refresh`, {
          method: 'POST',
          credentials: 'same-origin',
          headers: { 'Content-Type': 'application/json' },
        })
        if (r.ok) return 'refreshed'
        // 409: a concurrent /refresh for the same token is in flight. Don't treat
        // it as a logout-worthy failure — the in-flight request rotates the shared
        // cookie, so signal the caller to retry the original request (review M1).
        if (r.status === 409) return 'concurrent'
        return 'failed'
      } catch {
        return 'failed'
      } finally {
        this.refreshPromise = null
      }
    })()
    return this.refreshPromise
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
