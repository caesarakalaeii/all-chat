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
 * In-memory (non-persisted) token storage — SECURITY (audit H3).
 *
 * Refresh tokens are stored ONLY in these module-level variables, never in
 * localStorage. This means an XSS payload cannot read the refresh token from
 * storage: it would have to be actively running while the token is in memory,
 * and the token is irrecoverably lost when the tab is closed or refreshed.
 *
 * Access (JWT) tokens are mirrored here so API clients can prefer the in-memory
 * copy. Access tokens are still also written to localStorage for legacy readers
 * (admin/settings pages that read `jwt_token` directly) — full removal is
 * deferred to the httpOnly-cookie migration.
 *
 * LONG-TERM FIX (deferred): move all tokens to httpOnly; Secure;
 * SameSite=Strict cookies set by the backend, eliminating XSS token theft
 * entirely. This in-memory store is an interim mitigation per the security
 * audit's "minimum fix" allowance.
 */

let refreshToken: string | null = null
let accessToken: string | null = null
let adminToken: string | null = null
let impersonating = false
let impersonatedUsername: string | null = null
let viewerAccessToken: string | null = null

export const inMemoryTokens = {
  // ---- Refresh token (streamer) — in-memory ONLY, never persisted. ----
  getRefreshToken(): string | null {
    return refreshToken
  },
  setRefreshToken(token: string | null): void {
    refreshToken = token
  },

  // ---- Access token (streamer JWT) — mirrored from localStorage. ----
  getAccessToken(): string | null {
    return accessToken
  },
  setAccessToken(token: string | null): void {
    accessToken = token
  },

  // ---- Admin token (impersonation restore) — mirrored. ----
  getAdminToken(): string | null {
    return adminToken
  },
  setAdminToken(token: string | null): void {
    adminToken = token
  },

  // ---- Impersonation state — mirrored. ----
  getImpersonating(): boolean {
    return impersonating
  },
  setImpersonating(val: boolean): void {
    impersonating = val
  },
  getImpersonatedUsername(): string | null {
    return impersonatedUsername
  },
  setImpersonatedUsername(name: string | null): void {
    impersonatedUsername = name
  },

  // ---- Viewer access token — mirrored. ----
  getViewerAccessToken(): string | null {
    return viewerAccessToken
  },
  setViewerAccessToken(token: string | null): void {
    viewerAccessToken = token
  },

  /** Clear every in-memory token (used on logout). */
  clearAll(): void {
    refreshToken = null
    accessToken = null
    adminToken = null
    impersonating = false
    impersonatedUsername = null
    viewerAccessToken = null
  },
}
