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
 * Personal access token (PAT) management API.
 *
 * Wraps auth-service's /me/api-tokens surface, proxied by the API gateway at
 * /api/v1/auth/me/api-tokens. These are the credentials the Stream Deck and
 * StreamController plugins authenticate with (`allchat_pat_…`).
 *
 * THE ONE RULE: `createApiToken` is the only call in the whole codebase that ever
 * sees a plaintext token, and the server can never show it again — only a SHA-256
 * digest is stored (migration 086). Callers must therefore treat
 * `CreatedApiToken.token` as write-once, read-once, ephemeral React state. It must
 * never reach localStorage, sessionStorage, a Zustand store, a cookie or a URL.
 * `listApiTokens` returns metadata only; there is no server route that could
 * return a plaintext token a second time.
 */

import { apiClient } from './client'

/**
 * The closed set of grantable scopes, mirroring `allowedAPITokenScopes` in
 * services/auth-service/handlers/api_tokens.go. Anything outside it is rejected
 * by the server at create time.
 */
export const API_TOKEN_SCOPES = ['chat:write', 'engagement:write'] as const

export type ApiTokenScope = (typeof API_TOKEN_SCOPES)[number]

/**
 * Token METADATA — what the list endpoint returns. There is deliberately no
 * field for the plaintext or its digest: the server's projection never selects
 * `token_hash`, so this shape is the whole of what a client can know about an
 * existing token.
 */
export interface ApiToken {
  id: string
  name: string
  scopes: string[]
  created_at: string
  last_used_at: string | null
  expires_at: string | null
  revoked_at: string | null
}

/**
 * The create response: metadata plus the plaintext, which is returned EXACTLY
 * ONCE and is unrecoverable afterwards.
 */
export interface CreatedApiToken extends ApiToken {
  /** Plaintext `allchat_pat_…`. Shown once, never persisted, never fetched again. */
  token: string
}

const BASE = '/api/v1/auth/me/api-tokens'

/** Lists the signed-in user's tokens (metadata only, newest first per the server). */
export async function listApiTokens(): Promise<ApiToken[]> {
  const response = await apiClient.get<{ tokens: ApiToken[] | null }>(BASE)
  return response.tokens ?? []
}

/**
 * Mints a token. The resolved `token` field is the only copy of the secret that
 * will ever exist client-side — hold it in component state and drop it.
 */
export async function createApiToken(
  name: string,
  scopes: readonly string[]
): Promise<CreatedApiToken> {
  return apiClient.post<CreatedApiToken>(BASE, { name, scopes })
}

/**
 * Revokes a token. Revocation is read live by the server's resolver, so it takes
 * effect on the next request the plugin makes.
 */
export async function revokeApiToken(id: string): Promise<void> {
  await apiClient.delete(`${BASE}/${encodeURIComponent(id)}`)
}
