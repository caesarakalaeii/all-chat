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

import { apiClient, ApiError } from './client'

/**
 * Response from the platform add-source / moderation re-consent endpoints
 * (auth-service `HandleAddSource`). Either an OAuth URL to redirect to, or —
 * when the streamer already holds valid credentials with the required scopes —
 * a short-circuit acknowledgement that the source was added without an OAuth
 * reflow.
 */
interface AddSourceResponse {
  auth_url?: string
  source_added?: string
  reused_existing_credentials?: boolean
}

/** Discriminated outcome of an add-source / moderation reflow. */
export type AddSourceReflowResult =
  | { kind: 'redirect'; authUrl: string }
  | { kind: 'added' }
  | { kind: 'error'; message: string }

/**
 * Starts an add-source / moderation re-consent reflow against the given auth
 * endpoint and returns a discriminated result the caller renders.
 *
 * Goes through `apiClient` (NOT a raw `fetch`) on purpose: under H3 cookie auth
 * the browser sends only the httpOnly `access_token` cookie, and `apiClient`
 * transparently refreshes-and-retries once on a 401 so an expired access cookie
 * is renewed instead of failing with "Authorization header required". A raw
 * fetch would skip that interceptor and dead-end on an expired session.
 */
export async function startAddSourceReflow(
  endpoint: string
): Promise<AddSourceReflowResult> {
  try {
    const data = await apiClient.get<AddSourceResponse>(endpoint)
    if (data.auth_url) {
      return { kind: 'redirect', authUrl: data.auth_url }
    }
    // Backend short-circuit: valid existing credentials, source added directly.
    if (data.source_added) {
      return { kind: 'added' }
    }
    return { kind: 'error', message: 'Unexpected response from the server.' }
  } catch (err) {
    if (err instanceof ApiError) {
      const serverError =
        typeof err.data?.error === 'string' ? err.data.error : undefined
      return { kind: 'error', message: serverError ?? err.message }
    }
    return { kind: 'error', message: 'Please try again.' }
  }
}
