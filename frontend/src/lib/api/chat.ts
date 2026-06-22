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
 * Streamer chat-send API client.
 *
 * Mirrors the shape of `lib/api/moderation.ts`: a thin wrapper over the shared
 * `apiClient` (which attaches `Authorization: Bearer <jwt>`) that sends a fresh
 * `Idempotency-Key` so a double-click only sends once. Error responses propagate
 * as `ApiError`; the parsed body is on `error.data` (see `apiClient`).
 */

import { apiClient } from './client'

/** Platforms the streamer can target. `all` fans out to every sendable source. */
export type SendPlatform = 'twitch' | 'youtube' | 'kick' | 'all'

/** Backend message-length cap; mirror it client-side to avoid a doomed request. */
export const MAX_MESSAGE_LENGTH = 500

/** Request body for POST /api/v1/auth/chat/send. */
export interface SendStreamerMessageRequest {
  message: string
  platform: SendPlatform
}

/** Single-platform success envelope (200). */
export interface SingleSendResponse {
  success: true
  message: string
}

/** Per-platform outcome within a send-to-all response. */
export interface SendToAllResult {
  platform: string
  success: boolean
  error?: string
  error_kind?: string
}

/** Send-to-all success envelope (200); `success` is true if ≥1 platform sent. */
export interface SendToAllResponse {
  success: boolean
  results: SendToAllResult[]
}

/** Union of the two 200 shapes; discriminate on the `results` array. */
export type StreamerSendResponse = SingleSendResponse | SendToAllResponse

/**
 * Error body shape carried on a non-2xx response (`ApiError.data`). `error` is
 * the discriminant: `missing_scope` (403) and `reauth_required` (401) get
 * special UI; others (`not_live` 422, rate-limit 429 with `retry_after_seconds`,
 * 502, …) surface inline.
 */
export interface SendErrorBody {
  error: string
  platform?: string
  details?: string
  retry_after_seconds?: number
}

/** Type guard: is this 200 envelope the multi-platform (send-to-all) shape? */
export function isSendToAllResponse(res: StreamerSendResponse): res is SendToAllResponse {
  return Array.isArray((res as SendToAllResponse).results)
}

/** A fresh idempotency header for one send request. */
function idempotencyHeader(): Record<string, string> {
  return { 'Idempotency-Key': crypto.randomUUID() }
}

/**
 * Send a chat message as the overlay's streamer. POSTs to the auth-service
 * (which applies its own JWT + send scopes). On failure throws `ApiError`; read
 * `error.status` and `error.data` (typed as `SendErrorBody`) to branch.
 */
export function sendStreamerMessage(
  body: SendStreamerMessageRequest
): Promise<StreamerSendResponse> {
  return apiClient.post<StreamerSendResponse>('/api/v1/auth/chat/send', body, idempotencyHeader())
}
